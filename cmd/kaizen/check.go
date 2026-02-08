package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/alexcollie/kaizen/pkg/check"
	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/spf13/cobra"
)

var (
	checkPath       string
	checkBaseBranch string
	checkFormat     string
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "CI quality gate — warn on high blast-radius function changes",
	Long: `Diffs the current branch against a base branch and warns if any
modified functions are called by many other functions (high fan-in).

Exit codes:
  0  No blast-radius concerns
  1  Execution error
  2  Blast-radius concerns detected`,
	Run: runCheck,
}

func runCheck(cmd *cobra.Command, args []string) {
	// Step 1: Run git diff
	rawDiff, err := check.RunGitDiff(checkPath, checkBaseBranch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Parse diff output
	hunks, err := check.ParseDiffOutput(rawDiff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing diff: %v\n", err)
		os.Exit(1)
	}

	if len(hunks) == 0 {
		fmt.Println("No changes detected.")
		os.Exit(0)
	}

	// Step 3: Map hunks to functions
	changedFunctions, err := check.MapHunksToFunctions(hunks, checkPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error mapping hunks to functions: %v\n", err)
		os.Exit(1)
	}

	if len(changedFunctions) == 0 {
		fmt.Println("No functions changed.")
		os.Exit(0)
	}

	// Step 4: Compute fan-in
	fanInResults, err := check.ComputeFanIn(changedFunctions, checkPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing fan-in: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Detect blast-radius concerns
	concerns := check.DetectBlastRadius(fanInResults)

	// Step 6: Output
	if checkFormat == "json" {
		outputBlastRadiusJSON(concerns)
	} else {
		outputBlastRadiusText(concerns, fanInResults)
	}

	// Step 7: Exit code
	if len(concerns) > 0 {
		os.Exit(2)
	}
}

// outputBlastRadiusText prints concerns in a human-readable table format
func outputBlastRadiusText(concerns []models.Concern, fanInResults []check.FanInResult) {
	if len(concerns) == 0 {
		fmt.Println("No blast-radius concerns detected.")
		return
	}

	// Create a table writer
	tabWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tabWriter, "FUNCTION\tFILE\tFAN-IN\tAPPROXIMATE\tSEVERITY")
	_, _ = fmt.Fprintln(tabWriter, "--------\t----\t------\t-----------\t--------")

	// Only print functions that are in the concerns (have blast-radius)
	for _, concern := range concerns {
		for _, item := range concern.AffectedItems {
			approxStr := "no"
			if item.Metrics["approximate"] > 0.5 {
				approxStr = "yes"
			}

			_, _ = fmt.Fprintf(tabWriter, "%s\t%s\t%.0f\t%s\t%s\n",
				item.FunctionName,
				item.FilePath,
				item.Metrics["fan_in"],
				approxStr,
				concern.Severity)
		}
	}

	_ = tabWriter.Flush()
}

// outputBlastRadiusJSON marshals concerns to JSON and prints to stdout
func outputBlastRadiusJSON(concerns []models.Concern) {
	data, err := json.MarshalIndent(concerns, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func init() {
	checkCmd.Flags().StringVarP(&checkPath, "path", "p", ".", "Path to analyze (default: current directory)")
	checkCmd.Flags().StringVarP(&checkBaseBranch, "base", "b", "main", "Base branch to diff against (default: main)")
	checkCmd.Flags().StringVarP(&checkFormat, "format", "f", "text", "Output format (text or json)")
}

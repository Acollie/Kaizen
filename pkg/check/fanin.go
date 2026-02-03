package check

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"

	"github.com/alexcollie/kaizen/pkg/languages/golang"
)

// FanInResult represents the computed fan-in for a function
type FanInResult struct {
	Function    ChangedFunction
	FanIn       int
	Approximate bool // true when computed via grep, not call graph
}

// ComputeFanIn computes fan-in for each changed function
func ComputeFanIn(changedFunctions []ChangedFunction, repoPath string) ([]FanInResult, error) {
	if len(changedFunctions) == 0 {
		return nil, nil
	}

	// Partition into Go vs non-Go
	var goFunctions []ChangedFunction
	var nonGoFunctions []ChangedFunction

	for _, fn := range changedFunctions {
		if fn.Language == "Go" {
			goFunctions = append(goFunctions, fn)
		} else {
			nonGoFunctions = append(nonGoFunctions, fn)
		}
	}

	var results []FanInResult

	// Compute Go fan-in via call graph
	if len(goFunctions) > 0 {
		goResults, err := computeGoFanIn(goFunctions, repoPath)
		if err != nil {
			return nil, err
		}
		results = append(results, goResults...)
	}

	// Compute non-Go fan-in via grep
	if len(nonGoFunctions) > 0 {
		grepResults, err := computeGrepFanIn(nonGoFunctions, repoPath)
		if err != nil {
			return nil, err
		}
		results = append(results, grepResults...)
	}

	return results, nil
}

// computeGoFanIn computes fan-in for Go functions using the call graph
func computeGoFanIn(goFunctions []ChangedFunction, repoPath string) ([]FanInResult, error) {
	analyzer := golang.NewCallGraphAnalyzer()
	graph, err := analyzer.AnalyzeDirectory(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build call graph: %w", err)
	}

	var results []FanInResult

	for _, fn := range goFunctions {
		fullName, err := buildGoFullName(fn)
		if err != nil {
			// Log but continue
			fmt.Fprintf(os.Stderr, "Warning: could not build full name for %s: %v\n", fn.Name, err)
			continue
		}

		fanIn := 0
		if node, exists := graph.Nodes[fullName]; exists {
			fanIn = node.CallCount
		}

		results = append(results, FanInResult{
			Function:    fn,
			FanIn:       fanIn,
			Approximate: false,
		})
	}

	return results, nil
}

// buildGoFullName constructs the fully qualified function name for Go
// Matches the logic in callgraph.go:getFunctionFullName
func buildGoFullName(fn ChangedFunction) (string, error) {
	// Read the file to get the package name
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fn.FilePath, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	packageName := "main"
	if file.Name != nil {
		packageName = file.Name.Name
	}

	// Build the full name
	if fn.HasReceiver {
		// Method: pkg.ReceiverType.Name
		return fmt.Sprintf("%s.%s.%s", packageName, fn.ReceiverType, fn.Name), nil
	}

	// Regular function: pkg.Name
	return fmt.Sprintf("%s.%s", packageName, fn.Name), nil
}

// computeGrepFanIn computes fan-in for non-Go functions using grep
func computeGrepFanIn(nonGoFunctions []ChangedFunction, repoPath string) ([]FanInResult, error) {
	var results []FanInResult

	for _, fn := range nonGoFunctions {
		ext := extensionForLanguage(fn.Language)
		if ext == "" {
			continue
		}

		// Use grep -rlw to find word matches
		command := exec.Command("grep", "-rlw", fn.Name, "--include="+ext, repoPath)
		output, err := command.Output()
		if err != nil && err.Error() != "exit status 1" {
			// Non-1 exit codes are real errors
			return nil, fmt.Errorf("grep failed: %w", err)
		}

		// Count matching files
		matches := strings.Split(string(output), "\n")
		count := 0
		definingFile := fn.FilePath

		for _, match := range matches {
			if strings.TrimSpace(match) == "" {
				continue
			}
			// Don't count the defining file itself
			if match != definingFile {
				count++
			}
		}

		results = append(results, FanInResult{
			Function:    fn,
			FanIn:       count,
			Approximate: true,
		})
	}

	return results, nil
}

// extensionForLanguage maps language name to file extension pattern for grep
func extensionForLanguage(language string) string {
	switch language {
	case "Python":
		return "*.py"
	case "Kotlin":
		return "*.kt"
	case "Swift":
		return "*.swift"
	default:
		return ""
	}
}

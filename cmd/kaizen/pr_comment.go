package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/spf13/cobra"
)

var (
	prBaseAnalysis string
	prHeadAnalysis string
	prCheckJSON    string
	prOutput       string
)

var prCommentCmd = &cobra.Command{
	Use:   "pr-comment",
	Short: "Generate a GitHub PR comment from analysis comparison",
	Long: `Compares two analysis JSON files (base vs head) and generates a
GitHub-flavored Markdown comment showing:
  - Overall grade and score delta
  - Metrics table (complexity, maintainability, hotspots)
  - Hotspot changes (new, fixed, persistent)
  - Blast-radius warnings (if check JSON provided)

Designed for use in CI pipelines and GitHub Actions.`,
	Run: runPRComment,
}

func init() {
	prCommentCmd.Flags().StringVar(&prBaseAnalysis, "base-analysis", "", "Path to baseline analysis JSON")
	prCommentCmd.Flags().StringVar(&prHeadAnalysis, "head-analysis", "", "Path to current (PR head) analysis JSON")
	prCommentCmd.Flags().StringVar(&prCheckJSON, "check-json", "", "Path to kaizen check --format=json output (optional)")
	prCommentCmd.Flags().StringVarP(&prOutput, "output", "o", "", "Write markdown to file (default: stdout)")
}

func runPRComment(cmd *cobra.Command, args []string) {
	if prBaseAnalysis == "" || prHeadAnalysis == "" {
		fmt.Fprintln(os.Stderr, "Error: --base-analysis and --head-analysis are required")
		os.Exit(1)
	}

	baseResult, err := loadAnalysisFromFile(prBaseAnalysis)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading base analysis: %v\n", err)
		os.Exit(1)
	}

	headResult, err := loadAnalysisFromFile(prHeadAnalysis)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading head analysis: %v\n", err)
		os.Exit(1)
	}

	var concerns []models.Concern
	if prCheckJSON != "" {
		concerns, err = loadConcernsFromFile(prCheckJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load check results: %v\n", err)
		}
	}

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, concerns)

	if prOutput != "" {
		err := os.WriteFile(prOutput, []byte(markdown), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Print(markdown)
	}
}

func loadAnalysisFromFile(path string) (*models.AnalysisResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}

	var result models.AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("could not parse JSON from %s: %w", path, err)
	}

	return &result, nil
}

func loadConcernsFromFile(path string) ([]models.Concern, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}

	var concerns []models.Concern
	if err := json.Unmarshal(data, &concerns); err != nil {
		return nil, fmt.Errorf("could not parse JSON from %s: %w", path, err)
	}

	return concerns, nil
}

// FormatDiffMarkdown generates a GitHub-flavored markdown comment from analysis diff
func FormatDiffMarkdown(diff *AnalysisDiff, headResult *models.AnalysisResult, concerns []models.Concern) string {
	var builder strings.Builder

	writeHeader(&builder, headResult, diff)
	writeMetricsTable(&builder, headResult, diff)
	writeHotspotChanges(&builder, diff)
	writeBlastRadiusWarnings(&builder, concerns)
	writeMetricsExplainer(&builder)
	writeFooter(&builder)

	return builder.String()
}

func writeHeader(builder *strings.Builder, headResult *models.AnalysisResult, diff *AnalysisDiff) {
	grade := "N/A"
	score := 0.0
	if headResult.ScoreReport != nil {
		grade = headResult.ScoreReport.OverallGrade
		score = headResult.ScoreReport.OverallScore
	}

	fmt.Fprintf(builder, "## Kaizen Code Analysis \u2014 Grade %s (%.0f/100)\n\n", grade, score)

	delta := diff.GlobalMetrics.ScoreDelta
	arrow := deltaArrow(delta)
	fmt.Fprintf(builder, "Score: **%+.1f** from previous %s\n\n", delta, arrow)
}

func writeMetricsTable(builder *strings.Builder, headResult *models.AnalysisResult, diff *AnalysisDiff) {
	builder.WriteString("### Metrics\n\n")
	builder.WriteString("| Metric | Previous | Current | Delta |\n")
	builder.WriteString("|--------|----------|---------|-------|\n")

	previousScore := headResult.ScoreReport.OverallScore - diff.GlobalMetrics.ScoreDelta
	currentScore := headResult.ScoreReport.OverallScore
	writeMetricRow(builder, "Overall Score",
		fmt.Sprintf("%.1f", previousScore),
		fmt.Sprintf("%.1f", currentScore),
		diff.GlobalMetrics.ScoreDelta, false)

	prevComplexity := headResult.Summary.AverageCyclomaticComplexity - diff.GlobalMetrics.ComplexityDelta
	writeMetricRow(builder, "Avg Complexity",
		fmt.Sprintf("%.1f", prevComplexity),
		fmt.Sprintf("%.1f", headResult.Summary.AverageCyclomaticComplexity),
		diff.GlobalMetrics.ComplexityDelta, true)

	prevMaint := headResult.Summary.AverageMaintainabilityIndex - diff.GlobalMetrics.MaintainabilityDelta
	writeMetricRow(builder, "Avg Maintainability",
		fmt.Sprintf("%.1f", prevMaint),
		fmt.Sprintf("%.1f", headResult.Summary.AverageMaintainabilityIndex),
		diff.GlobalMetrics.MaintainabilityDelta, false)

	prevHotspots := headResult.Summary.HotspotCount - diff.GlobalMetrics.HotspotCountDelta
	writeMetricRowInt(builder, "Hotspots", prevHotspots, headResult.Summary.HotspotCount, diff.GlobalMetrics.HotspotCountDelta, true)

	prevFuncs := headResult.Summary.TotalFunctions - diff.GlobalMetrics.FunctionCountDelta
	writeMetricRowInt(builder, "Functions", prevFuncs, headResult.Summary.TotalFunctions, diff.GlobalMetrics.FunctionCountDelta, false)

	builder.WriteString("\n")
}

func writeMetricRow(builder *strings.Builder, name, previous, current string, delta float64, invertArrow bool) {
	arrow := deltaArrowFloat(delta, invertArrow)
	fmt.Fprintf(builder, "| %s | %s | %s | %+.1f %s |\n", name, previous, current, delta, arrow)
}

func writeMetricRowInt(builder *strings.Builder, name string, previous, current, delta int, invertArrow bool) {
	arrow := deltaArrowInt(delta, invertArrow)
	fmt.Fprintf(builder, "| %s | %d | %d | %+d %s |\n", name, previous, current, delta, arrow)
}

func writeHotspotChanges(builder *strings.Builder, diff *AnalysisDiff) {
	hasChanges := len(diff.HotspotChanges.New) > 0 ||
		len(diff.HotspotChanges.Removed) > 0 ||
		len(diff.HotspotChanges.Persistent) > 0

	if !hasChanges {
		return
	}

	builder.WriteString("### Hotspot Changes\n\n")
	builder.WriteString("| Status | Function |\n")
	builder.WriteString("|--------|----------|\n")

	for _, spot := range diff.HotspotChanges.New {
		fmt.Fprintf(builder, "| :x: New | `%s` |\n", spot)
	}
	for _, spot := range diff.HotspotChanges.Removed {
		fmt.Fprintf(builder, "| :white_check_mark: Fixed | `%s` |\n", spot)
	}
	for idx, spot := range diff.HotspotChanges.Persistent {
		if idx >= 10 {
			remaining := len(diff.HotspotChanges.Persistent) - 10
			fmt.Fprintf(builder, "| :warning: Persistent | *...and %d more* |\n", remaining)
			break
		}
		fmt.Fprintf(builder, "| :warning: Persistent | `%s` |\n", spot)
	}

	builder.WriteString("\n")
}

func writeBlastRadiusWarnings(builder *strings.Builder, concerns []models.Concern) {
	if len(concerns) == 0 {
		return
	}

	builder.WriteString("### Blast-Radius Warnings\n\n")
	builder.WriteString("| Function | File | Fan-In | Severity |\n")
	builder.WriteString("|----------|------|--------|----------|\n")

	for _, concern := range concerns {
		for _, item := range concern.AffectedItems {
			fanIn := int(item.Metrics["fan_in"])
			fmt.Fprintf(builder, "| `%s` | `%s` | %d | %s |\n",
				item.FunctionName, item.FilePath, fanIn, concern.Severity)
		}
	}

	builder.WriteString("\n")
}

func writeMetricsExplainer(builder *strings.Builder) {
	builder.WriteString("<details><summary>What do these metrics mean?</summary>\n\n")
	builder.WriteString("- **Overall Score**: Composite code health score (0-100, higher is better)\n")
	builder.WriteString("- **Avg Complexity**: Average cyclomatic complexity across functions (lower is better)\n")
	builder.WriteString("- **Avg Maintainability**: Average maintainability index (higher is better)\n")
	builder.WriteString("- **Hotspots**: Functions with high complexity AND high churn\n")
	builder.WriteString("- **Blast-Radius**: Modified functions with high fan-in (many callers)\n\n")
	builder.WriteString("</details>\n\n")
}

func writeFooter(builder *strings.Builder) {
	builder.WriteString("---\n")
	builder.WriteString("*Generated by [Kaizen](https://github.com/acollie/kaizen)* <!-- kaizen-pr-analysis -->\n")
}

// deltaArrow returns an arrow emoji for score delta (positive = improvement)
func deltaArrow(delta float64) string {
	if delta > 0 {
		return "\u2b06\ufe0f"
	} else if delta < 0 {
		return "\u2b07\ufe0f"
	}
	return "\u27a1\ufe0f"
}

// deltaArrowFloat returns an arrow emoji, with optional inversion
// (e.g., complexity going up is bad, so invert the arrow color logic)
func deltaArrowFloat(delta float64, invertMeaning bool) string {
	if delta == 0 {
		return ""
	}
	if invertMeaning {
		if delta > 0 {
			return "\u2b06\ufe0f"
		}
		return "\u2b07\ufe0f"
	}
	if delta > 0 {
		return "\u2b06\ufe0f"
	}
	return "\u2b07\ufe0f"
}

// deltaArrowInt returns an arrow emoji for int deltas
func deltaArrowInt(delta int, invertMeaning bool) string {
	if delta == 0 {
		return ""
	}
	if invertMeaning {
		if delta > 0 {
			return "\u2b06\ufe0f"
		}
		return "\u2b07\ufe0f"
	}
	if delta > 0 {
		return "\u2b06\ufe0f"
	}
	return "\u2b07\ufe0f"
}

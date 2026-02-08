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

	gradeEmoji := gradeToEmoji(grade)
	fmt.Fprintf(builder, "## %s Kaizen Code Analysis \u2014 Grade %s (%.0f/100)\n\n", gradeEmoji, grade, score)

	delta := diff.GlobalMetrics.ScoreDelta
	deltaIndicator := scoreDeltaIndicator(delta)
	fmt.Fprintf(builder, "**Score Change:** %s **%+.1f** points\n\n", deltaIndicator, delta)
}

func writeMetricsTable(builder *strings.Builder, headResult *models.AnalysisResult, diff *AnalysisDiff) {
	builder.WriteString("### 📊 Metrics\n\n")
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
	indicator := metricDeltaIndicator(delta, invertArrow)
	fmt.Fprintf(builder, "| %s | %s | %s | %s %+.1f |\n", name, previous, current, indicator, delta)
}

func writeMetricRowInt(builder *strings.Builder, name string, previous, current, delta int, invertArrow bool) {
	indicator := metricDeltaIndicatorInt(delta, invertArrow)
	fmt.Fprintf(builder, "| %s | %d | %d | %s %+d |\n", name, previous, current, indicator, delta)
}

func writeHotspotChanges(builder *strings.Builder, diff *AnalysisDiff) {
	hasChanges := len(diff.HotspotChanges.New) > 0 ||
		len(diff.HotspotChanges.Removed) > 0 ||
		len(diff.HotspotChanges.Persistent) > 0

	if !hasChanges {
		return
	}

	builder.WriteString("### 🔥 Hotspot Changes\n\n")
	builder.WriteString("| Status | Function |\n")
	builder.WriteString("|--------|----------|\n")

	for _, spot := range diff.HotspotChanges.New {
		fmt.Fprintf(builder, "| 🔴 New | `%s` |\n", spot)
	}
	for _, spot := range diff.HotspotChanges.Removed {
		fmt.Fprintf(builder, "| ✅ Fixed | `%s` |\n", spot)
	}
	for idx, spot := range diff.HotspotChanges.Persistent {
		if idx >= 10 {
			remaining := len(diff.HotspotChanges.Persistent) - 10
			fmt.Fprintf(builder, "| ⚠️ Persistent | *...and %d more* |\n", remaining)
			break
		}
		fmt.Fprintf(builder, "| ⚠️ Persistent | `%s` |\n", spot)
	}

	builder.WriteString("\n")
}

func writeBlastRadiusWarnings(builder *strings.Builder, concerns []models.Concern) {
	if len(concerns) == 0 {
		return
	}

	builder.WriteString("### 💥 Blast-Radius Warnings\n\n")
	builder.WriteString("| Function | File | Fan-In | Severity |\n")
	builder.WriteString("|----------|------|--------|----------|\n")

	for _, concern := range concerns {
		for _, item := range concern.AffectedItems {
			fanIn := int(item.Metrics["fan_in"])
			severityIcon := severityToEmoji(concern.Severity)
			fmt.Fprintf(builder, "| `%s` | `%s` | %d | %s %s |\n",
				item.FunctionName, item.FilePath, fanIn, severityIcon, concern.Severity)
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

// gradeToEmoji converts letter grade to colored emoji
func gradeToEmoji(grade string) string {
	switch grade {
	case "A", "A+", "A-":
		return "🟢" // Green circle
	case "B", "B+", "B-":
		return "🟡" // Yellow circle
	case "C", "C+", "C-":
		return "🟠" // Orange circle
	case "D", "D+", "D-", "F":
		return "🔴" // Red circle
	default:
		return "⚪" // White circle
	}
}

// scoreDeltaIndicator returns emoji and text for score changes
func scoreDeltaIndicator(delta float64) string {
	if delta > 5 {
		return "🎉 +" // Significant improvement
	} else if delta > 0 {
		return "✅ +" // Improvement
	} else if delta == 0 {
		return "➡️" // No change
	} else if delta > -5 {
		return "⚠️" // Small regression
	}
	return "❌" // Significant regression
}

// metricDeltaIndicator returns emoji for metric changes with optional inversion
func metricDeltaIndicator(delta float64, invertMeaning bool) string {
	if delta == 0 {
		return "➖" // Neutral
	}

	isGood := delta > 0
	if invertMeaning {
		isGood = delta < 0 // For metrics where lower is better
	}

	if isGood {
		if absFloat(delta) > 1.0 {
			return "✅" // Good change
		}
		return "🟢" // Small good change
	}

	if absFloat(delta) > 1.0 {
		return "❌" // Bad change
	}
	return "🔴" // Small bad change
}

// metricDeltaIndicatorInt returns emoji for integer metric changes
func metricDeltaIndicatorInt(delta int, invertMeaning bool) string {
	if delta == 0 {
		return "➖" // Neutral
	}

	isGood := delta > 0
	if invertMeaning {
		isGood = delta < 0 // For metrics where lower is better
	}

	if isGood {
		if absInt(delta) > 2 {
			return "✅" // Good change
		}
		return "🟢" // Small good change
	}

	if absInt(delta) > 2 {
		return "❌" // Bad change
	}
	return "🔴" // Small bad change
}

// severityToEmoji converts severity level to colored emoji
func severityToEmoji(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "🔴"
	case "warning":
		return "🟠"
	case "info":
		return "🔵"
	default:
		return "⚪"
	}
}

// absFloat returns absolute value of float
func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// absInt returns absolute value of int
func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

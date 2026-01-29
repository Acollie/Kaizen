package visualization

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/fatih/color"
)

// TerminalVisualizer generates colored terminal output
type TerminalVisualizer struct {
	green  *color.Color
	yellow *color.Color
	red    *color.Color
}

// NewTerminalVisualizer creates a new terminal visualizer
func NewTerminalVisualizer() *TerminalVisualizer {
	return &TerminalVisualizer{
		green:  color.New(color.FgGreen),
		yellow: color.New(color.FgYellow),
		red:    color.New(color.FgRed),
	}
}

// RenderHeatMap renders a heat map to the terminal
func (visualizer *TerminalVisualizer) RenderHeatMap(result *models.AnalysisResult, metric string) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("\n🗺️  Heat Map - %s\n\n", metricTitle(metric)))

	// Sort folders by score (descending)
	folders := sortFoldersByMetric(result.FolderStats, metric)

	// Find max path length for alignment
	maxPathLen := 0
	for _, folder := range folders {
		if len(folder.Path) > maxPathLen {
			maxPathLen = len(folder.Path)
		}
	}
	if maxPathLen > 60 {
		maxPathLen = 60
	}

	// Render each folder
	for _, folder := range folders {
		score := getMetricScore(folder, metric)
		visualizer.renderFolderRow(&builder, folder, score, maxPathLen)
	}

	builder.WriteString("\n")
	builder.WriteString(visualizer.renderLegend())
	builder.WriteString("\n")

	return builder.String()
}

// renderFolderRow renders a single folder row with color coding
func (visualizer *TerminalVisualizer) renderFolderRow(builder *strings.Builder, folder models.FolderMetrics, score float64, maxPathLen int) {
	// Truncate path if needed
	displayPath := folder.Path
	if len(displayPath) > maxPathLen {
		displayPath = "..." + displayPath[len(displayPath)-maxPathLen+3:]
	}

	// Pad path for alignment
	paddedPath := fmt.Sprintf("%-*s", maxPathLen, displayPath)

	// Create visual bar
	bar := visualizer.createBar(score, 20)

	// Choose color
	colorFunc := visualizer.getColorForScore(score)

	// Format score
	scoreStr := fmt.Sprintf("%.1f", score)

	// Print colored line
	colorFunc.Fprintf(builder, "%s %s %s", paddedPath, bar, scoreStr)

	// Add hotspot indicator
	if folder.HotspotCount > 0 {
		builder.WriteString(fmt.Sprintf(" 🔥x%d", folder.HotspotCount))
	}

	builder.WriteString("\n")
}

// createBar creates a visual bar representing the score
func (visualizer *TerminalVisualizer) createBar(score float64, maxWidth int) string {
	filledWidth := int((score / 100.0) * float64(maxWidth))
	if filledWidth > maxWidth {
		filledWidth = maxWidth
	}

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", maxWidth-filledWidth)

	return "[" + filled + empty + "]"
}

// getColorForScore returns the appropriate color function for a score
func (visualizer *TerminalVisualizer) getColorForScore(score float64) *color.Color {
	switch {
	case score < 33:
		return visualizer.green
	case score < 67:
		return visualizer.yellow
	default:
		return visualizer.red
	}
}

// renderLegend renders the color legend
func (visualizer *TerminalVisualizer) renderLegend() string {
	var builder strings.Builder

	builder.WriteString("Legend:\n")
	visualizer.green.Fprint(&builder, "  █ Low (0-33)      - Good\n")
	visualizer.yellow.Fprint(&builder, "  █ Medium (33-67)  - Moderate\n")
	visualizer.red.Fprint(&builder, "  █ High (67-100)   - Needs attention\n")
	builder.WriteString("  🔥 = Hotspot (high churn + complexity)\n")

	return builder.String()
}

// RenderTopHotspots renders the top hotspot functions
func (visualizer *TerminalVisualizer) RenderTopHotspots(result *models.AnalysisResult, limit int) string {
	var builder strings.Builder

	builder.WriteString("\n🔥 Top Hotspots\n\n")

	// Collect all hotspot functions
	var hotspots []struct {
		file     string
		function models.FunctionAnalysis
	}

	for _, file := range result.Files {
		for _, function := range file.Functions {
			if function.IsHotspot {
				hotspots = append(hotspots, struct {
					file     string
					function models.FunctionAnalysis
				}{file.Path, function})
			}
		}
	}

	// Sort by complexity * churn
	sort.Slice(hotspots, func(firstIndex, secondIndex int) bool {
		firstScore := hotspots[firstIndex].function.CyclomaticComplexity
		secondScore := hotspots[secondIndex].function.CyclomaticComplexity

		if hotspots[firstIndex].function.Churn != nil {
			firstScore *= hotspots[firstIndex].function.Churn.TotalCommits
		}
		if hotspots[secondIndex].function.Churn != nil {
			secondScore *= hotspots[secondIndex].function.Churn.TotalCommits
		}

		return firstScore > secondScore
	})

	// Render top N
	count := limit
	if count > len(hotspots) {
		count = len(hotspots)
	}

	for index := 0; index < count; index++ {
		hotspot := hotspots[index]
		visualizer.renderHotspotRow(&builder, hotspot.file, hotspot.function, index+1)
	}

	if len(hotspots) == 0 {
		builder.WriteString("  No hotspots found! 🎉\n")
	}

	return builder.String()
}

// renderHotspotRow renders a single hotspot row
func (visualizer *TerminalVisualizer) renderHotspotRow(builder *strings.Builder, file string, function models.FunctionAnalysis, rank int) {
	visualizer.red.Fprintf(builder, "%d. %s:%d\n", rank, file, function.StartLine)
	builder.WriteString(fmt.Sprintf("   Function: %s\n", function.Name))
	builder.WriteString(fmt.Sprintf("   Complexity: %d | Length: %d lines\n",
		function.CyclomaticComplexity, function.Length))

	if function.Churn != nil {
		builder.WriteString(fmt.Sprintf("   Churn: %d commits, %d changes\n",
			function.Churn.TotalCommits, function.Churn.TotalChanges))
	}

	builder.WriteString("\n")
}

// Helper functions

func metricTitle(metric string) string {
	switch metric {
	case "complexity":
		return "Cyclomatic Complexity"
	case "cognitive":
		return "Cognitive Complexity"
	case "churn":
		return "Code Churn"
	case "hotspot":
		return "Hotspot Score (Churn + Complexity)"
	case "length":
		return "Function Length"
	case "maintainability":
		return "Maintainability Index"
	default:
		return strings.Title(metric)
	}
}

func getMetricScore(folder models.FolderMetrics, metric string) float64 {
	switch metric {
	case "complexity":
		return folder.ComplexityScore
	case "cognitive":
		return folder.ComplexityScore // Using same for now
	case "churn":
		return folder.ChurnScore
	case "hotspot":
		return folder.HotspotScore
	case "length":
		return folder.LengthScore
	case "maintainability":
		return folder.MaintainabilityScore
	default:
		return folder.HotspotScore
	}
}

func sortFoldersByMetric(folderStats map[string]models.FolderMetrics, metric string) []models.FolderMetrics {
	folders := make([]models.FolderMetrics, 0, len(folderStats))
	for _, folder := range folderStats {
		folders = append(folders, folder)
	}

	sort.Slice(folders, func(firstIndex, secondIndex int) bool {
		firstScore := getMetricScore(folders[firstIndex], metric)
		secondScore := getMetricScore(folders[secondIndex], metric)
		return firstScore > secondScore
	})

	return folders
}

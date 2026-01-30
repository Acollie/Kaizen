package trending

import (
	"fmt"
	"strings"

	"github.com/alexcollie/kaizen/pkg/storage"
)

// RenderASCIIChart renders time-series data as ASCII chart
func RenderASCIIChart(metricName string, points []storage.TimeSeriesPoint, scopePath string) string {
	if len(points) == 0 {
		return fmt.Sprintf("No data available for metric: %s\n", metricName)
	}

	// Find min/max values
	minVal := points[0].Value
	maxVal := points[0].Value
	for _, p := range points {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
		}
	}

	// Handle flat data (all same value)
	if minVal == maxVal {
		maxVal = minVal + 1
	}

	// Render chart
	return renderChart(metricName, scopePath, points, minVal, maxVal)
}

func renderChart(metricName, scopePath string, points []storage.TimeSeriesPoint, minVal, maxVal float64) string {
	const (
		width  = 80
		height = 15
	)

	// Create output buffer
	var output strings.Builder

	// Title
	title := fmt.Sprintf("📈 %s Trend", metricName)
	if scopePath != "" {
		title = fmt.Sprintf("📈 %s - %s", metricName, scopePath)
	}
	output.WriteString(title + "\n\n")

	// Create normalized data points (0-height scale)
	normalized := make([]float64, len(points))
	valueRange := maxVal - minVal
	if valueRange == 0 {
		valueRange = 1
	}

	for i, p := range points {
		normalized[i] = (p.Value - minVal) / valueRange * (height - 1)
	}

	// Calculate points per column for scaling
	if len(normalized) > width {
		normalized = scaleDownPoints(normalized, width)
	}

	// Build chart line by line
	for row := height - 1; row >= 0; row-- {
		// Y-axis label
		yValue := minVal + (float64(row)/float64(height-1))*valueRange
		output.WriteString(fmt.Sprintf("%7.1f │ ", yValue))

		// Chart line
		for col := 0; col < len(normalized); col++ {
			pointVal := normalized[col]
			if int(pointVal) == row {
				output.WriteString("●")
			} else if int(pointVal) > row {
				output.WriteString("█")
			} else if int(pointVal) == row-1 && pointVal > float64(row-1) {
				output.WriteString("▄")
			} else {
				output.WriteString(" ")
			}
		}
		output.WriteString("\n")
	}

	// X-axis
	output.WriteString("        └" + strings.Repeat("─", len(normalized)) + "\n")

	// X-axis labels (time range)
	if len(points) > 0 {
		startTime := points[0].Timestamp.Format("Jan 02")
		endTime := points[len(points)-1].Timestamp.Format("Jan 02")
		output.WriteString(fmt.Sprintf("         %s to %s (%d snapshots)\n", startTime, endTime, len(points)))
	}

	// Statistics
	output.WriteString("\n")
	output.WriteString(formatStats(metricName, points))

	return output.String()
}

func scaleDownPoints(data []float64, targetWidth int) []float64 {
	if len(data) <= targetWidth {
		return data
	}

	scaled := make([]float64, targetWidth)
	ratio := float64(len(data)) / float64(targetWidth)

	for i := 0; i < targetWidth; i++ {
		startIdx := int(float64(i) * ratio)
		endIdx := int(float64(i+1) * ratio)
		if endIdx > len(data) {
			endIdx = len(data)
		}

		// Average values in this bucket
		sum := 0.0
		for j := startIdx; j < endIdx; j++ {
			sum += data[j]
		}
		scaled[i] = sum / float64(endIdx-startIdx)
	}

	return scaled
}

func formatStats(metricName string, points []storage.TimeSeriesPoint) string {
	if len(points) == 0 {
		return ""
	}

	values := make([]float64, len(points))
	for i, p := range points {
		values[i] = p.Value
	}

	// Calculate statistics
	min := values[0]
	max := values[0]
	sum := 0.0

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	avg := sum / float64(len(values))
	current := points[len(points)-1].Value
	delta := current - points[0].Value

	// Format output
	stats := fmt.Sprintf("Stats: Min=%.1f Max=%.1f Avg=%.1f Current=%.1f", min, max, avg, current)
	if delta >= 0 {
		stats += fmt.Sprintf(" ↑ +%.1f", delta)
	} else {
		stats += fmt.Sprintf(" ↓ %.1f", delta)
	}

	return stats
}

// RenderComparisonTable renders side-by-side comparison of metrics
func RenderComparisonTable(snapshot1, snapshot2 *storage.SnapshotSummary) string {
	var output strings.Builder

	output.WriteString("Snapshot Comparison\n")
	output.WriteString("════════════════════════════════════════════════════════════════════\n\n")

	// Create rows
	rows := []struct {
		label  string
		val1   interface{}
		val2   interface{}
	}{
		{"Analyzed At", snapshot1.AnalyzedAt.Format("2006-01-02 15:04"), snapshot2.AnalyzedAt.Format("2006-01-02 15:04")},
		{"Overall Score", formatScore(snapshot1.OverallScore), formatScore(snapshot2.OverallScore)},
		{"Overall Grade", snapshot1.OverallGrade, snapshot2.OverallGrade},
		{"Total Files", snapshot1.TotalFiles, snapshot2.TotalFiles},
		{"Total Functions", snapshot1.TotalFunctions, snapshot2.TotalFunctions},
		{"Avg Cyclomatic Complexity", fmt.Sprintf("%.1f", snapshot1.AvgCyclomaticComplexity), fmt.Sprintf("%.1f", snapshot2.AvgCyclomaticComplexity)},
		{"Avg Maintainability Index", fmt.Sprintf("%.1f", snapshot1.AvgMaintainabilityIndex), fmt.Sprintf("%.1f", snapshot2.AvgMaintainabilityIndex)},
		{"Hotspot Count", snapshot1.HotspotCount, snapshot2.HotspotCount},
		{"Complexity Score", formatScore(snapshot1.ComplexityScore), formatScore(snapshot2.ComplexityScore)},
		{"Maintainability Score", formatScore(snapshot1.MaintainabilityScore), formatScore(snapshot2.MaintainabilityScore)},
		{"Churn Score", formatScore(snapshot1.ChurnScore), formatScore(snapshot2.ChurnScore)},
	}

	// Render table
	output.WriteString(fmt.Sprintf("%-35s │ %-25s │ %-25s\n", "Metric", "Snapshot 1", "Snapshot 2"))
	output.WriteString("─────────────────────────────────────┼──────────────────────────┼──────────────────────────\n")

	for _, row := range rows {
		val1Str := fmt.Sprintf("%v", row.val1)
		val2Str := fmt.Sprintf("%v", row.val2)
		output.WriteString(fmt.Sprintf("%-35s │ %-25s │ %-25s\n", row.label, val1Str, val2Str))
	}

	return output.String()
}

func formatScore(score float64) string {
	if score == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.1f/100", score)
}

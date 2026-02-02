package trending

import (
	"strings"
	"testing"
	"time"

	"github.com/alexcollie/kaizen/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderASCIIChartEmpty(t *testing.T) {
	output := RenderASCIIChart("complexity", []storage.TimeSeriesPoint{}, "pkg/api")

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "No data available")
}

func TestRenderASCIIChartSinglePoint(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
	}

	output := RenderASCIIChart("complexity", points, "")

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "complexity Trend")
	assert.Contains(t, output, "Stats:")
}

func TestRenderASCIIChartMultiplePoints(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     3.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     7.0,
		},
	}

	output := RenderASCIIChart("complexity", points, "")

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Jan 15 to Jan 17")
	assert.Contains(t, output, "3 snapshots")
	assert.Contains(t, output, "Stats:")
}

func TestRenderASCIIChartWithScopePath(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
	}

	output := RenderASCIIChart("maintainability", points, "pkg/storage")

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "maintainability - pkg/storage")
}

func TestRenderASCIIChartFlatData(t *testing.T) {
	// All values are the same - should handle gracefully
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
	}

	output := RenderASCIIChart("complexity", points, "")

	assert.NotEmpty(t, output)
	// Should not panic and should include stats
	assert.Contains(t, output, "Stats:")
}

func TestRenderComparisonTable(t *testing.T) {
	snapshot1 := &storage.SnapshotSummary{
		AnalyzedAt:               time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		OverallScore:             85.0,
		OverallGrade:             "B",
		TotalFiles:               50,
		TotalFunctions:           200,
		AvgCyclomaticComplexity:  5.5,
		AvgMaintainabilityIndex:  80.0,
		HotspotCount:             2,
		ComplexityScore:          75.0,
		MaintainabilityScore:     85.0,
		ChurnScore:               90.0,
	}

	snapshot2 := &storage.SnapshotSummary{
		AnalyzedAt:               time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
		OverallScore:             88.0,
		OverallGrade:             "B",
		TotalFiles:               52,
		TotalFunctions:           210,
		AvgCyclomaticComplexity:  5.2,
		AvgMaintainabilityIndex:  82.0,
		HotspotCount:             1,
		ComplexityScore:          78.0,
		MaintainabilityScore:     87.0,
		ChurnScore:               92.0,
	}

	output := RenderComparisonTable(snapshot1, snapshot2)

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Snapshot Comparison")
	assert.Contains(t, output, "Overall Score")
	assert.Contains(t, output, "Metric")
	assert.Contains(t, output, "Snapshot 1")
	assert.Contains(t, output, "Snapshot 2")
}

func TestScaleDownPoints(t *testing.T) {
	tests := []struct {
		name       string
		data       []float64
		targetWidth int
		expected    int
	}{
		{
			name:        "no scaling needed",
			data:        []float64{1, 2, 3, 4, 5},
			targetWidth: 10,
			expected:    5,
		},
		{
			name:        "scale down",
			data:        []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			targetWidth: 5,
			expected:    5,
		},
		{
			name:        "single point",
			data:        []float64{5.0},
			targetWidth: 10,
			expected:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scaleDownPoints(tt.data, tt.targetWidth)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestScaleDownPointsAveraging(t *testing.T) {
	// Test that values are averaged correctly
	data := []float64{10, 20, 30, 40}
	result := scaleDownPoints(data, 2)

	require.Len(t, result, 2)
	// First bucket: average of [10, 20] = 15
	assert.InDelta(t, 15.0, result[0], 1.0)
	// Second bucket: average of [30, 40] = 35
	assert.InDelta(t, 35.0, result[1], 1.0)
}

func TestFormatStats(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     8.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     10.0,
		},
	}

	stats := formatStats("complexity", points)

	assert.NotEmpty(t, stats)
	assert.Contains(t, stats, "Stats:")
	assert.Contains(t, stats, "Min=")
	assert.Contains(t, stats, "Max=")
	assert.Contains(t, stats, "Avg=")
	assert.Contains(t, stats, "Current=")
}

func TestFormatStatsUpwardTrend(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     8.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     12.0,
		},
	}

	stats := formatStats("complexity", points)

	assert.Contains(t, stats, "↑")
}

func TestFormatScore(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.0, "N/A"},
		{85.5, "85.5/100"},
		{100.0, "100.0/100"},
	}

	for _, tt := range tests {
		result := formatScore(tt.score)
		assert.Equal(t, tt.expected, result)
	}
}

func TestRenderChartLinesIncludeMetric(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     8.0,
		},
	}

	output := RenderASCIIChart("maintainability", points, "pkg/api")

	// Should include the metric name in title
	assert.Contains(t, output, "maintainability")
	// Should include scope path
	assert.Contains(t, output, "pkg/api")
	// Should include timestamp range
	assert.True(t, strings.Contains(output, "Jan 15") && strings.Contains(output, "Jan 16"))
}

func TestRenderChartWithHighValues(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     100.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     250.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     150.0,
		},
	}

	output := RenderASCIIChart("complexity", points, "")

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Stats:")
	// Check for arrow indicating trend
	assert.True(t, strings.Contains(output, "↑") || strings.Contains(output, "↓") || strings.Contains(output, "~"))
}

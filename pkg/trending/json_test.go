package trending

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexcollie/kaizen/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportToJSONEmpty(t *testing.T) {
	export, err := ExportToJSON("complexity", "pkg/api", []storage.TimeSeriesPoint{})

	assert.NoError(t, err)
	assert.Nil(t, export)
}

func TestExportToJSONSinglePoint(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
	}

	export, err := ExportToJSON("complexity", "pkg/api", points)

	require.NoError(t, err)
	require.NotNil(t, export)

	assert.Equal(t, "complexity", export.MetricName)
	assert.Equal(t, "pkg/api", export.ScopePath)
	assert.Equal(t, 1, export.DataPoints)
	assert.Len(t, export.Points, 1)

	// Check statistics
	assert.Equal(t, 5.0, export.Statistics.Min)
	assert.Equal(t, 5.0, export.Statistics.Max)
	assert.Equal(t, 5.0, export.Statistics.Current)
	assert.Equal(t, 0.0, export.Statistics.Change)
}

func TestExportToJSONMultiplePoints(t *testing.T) {
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

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)
	require.NotNil(t, export)

	assert.Equal(t, 3, export.DataPoints)
	assert.Len(t, export.Points, 3)

	// Check time range
	assert.Equal(t, "2024-01-15T10:00:00Z", export.StartTime)
	assert.Equal(t, "2024-01-17T10:00:00Z", export.EndTime)

	// Check statistics
	assert.Equal(t, 3.0, export.Statistics.Min)
	assert.Equal(t, 7.0, export.Statistics.Max)
	assert.InDelta(t, 5.0, export.Statistics.Average, 0.1)
	assert.Equal(t, 7.0, export.Statistics.Current)
	assert.Equal(t, 4.0, export.Statistics.Change)
}

func TestExportToJSONPointConversion(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			Value:     5.5,
		},
	}

	export, err := ExportToJSON("metric", "", points)

	require.NoError(t, err)
	require.Len(t, export.Points, 1)

	exportPoint := export.Points[0]
	assert.Equal(t, "2024-01-15T10:30:45Z", exportPoint.Timestamp)
	assert.Equal(t, 5.5, exportPoint.Value)
}

func TestExportToJSONTrendUpward(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     10.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     15.0,
		},
	}

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)
	assert.Equal(t, "up", export.Statistics.Trend)
	assert.Equal(t, 10.0, export.Statistics.Change)
}

func TestExportToJSONTrendDownward(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     15.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     10.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
	}

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)
	assert.Equal(t, "down", export.Statistics.Trend)
	assert.Equal(t, -10.0, export.Statistics.Change)
}

func TestExportToJSONTrendStable(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     10.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     10.1,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     9.9,
		},
	}

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)
	assert.Equal(t, "stable", export.Statistics.Trend)
}

func TestExportToJSONStatistics(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     10.0,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     20.0,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     30.0,
		},
	}

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)

	stats := export.Statistics
	assert.Equal(t, 10.0, stats.Min)
	assert.Equal(t, 30.0, stats.Max)
	assert.InDelta(t, 20.0, stats.Average, 0.1)
	assert.Equal(t, 30.0, stats.Current)
	assert.Equal(t, 20.0, stats.Change)
}

func TestWriteJSONToFile(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "export.json")

	export := &TimeSeriesExport{
		MetricName: "complexity",
		ScopePath:  "pkg/api",
		StartTime:  "2024-01-15T10:00:00Z",
		EndTime:    "2024-01-17T10:00:00Z",
		DataPoints: 3,
		Points: []TimeSeriesPointExport{
			{
				Timestamp: "2024-01-15T10:00:00Z",
				Value:     5.0,
			},
		},
		Statistics: TimeSeriesStatisticsExport{
			Min:     5.0,
			Max:     5.0,
			Average: 5.0,
			Current: 5.0,
			Change:  0.0,
			Trend:   "stable",
		},
	}

	err := WriteJSONToFile(export, outputPath)

	require.NoError(t, err)

	// Verify file exists and is readable
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify it contains expected fields
	content := string(data)
	assert.Contains(t, content, "complexity")
	assert.Contains(t, content, "pkg/api")
}

func TestWriteJSONToFileInvalidPath(t *testing.T) {
	export := &TimeSeriesExport{
		MetricName: "complexity",
	}

	err := WriteJSONToFile(export, "/invalid/path/that/does/not/exist/file.json")

	assert.Error(t, err)
}

func TestJSONToString(t *testing.T) {
	export := &TimeSeriesExport{
		MetricName: "complexity",
		ScopePath:  "pkg/api",
		StartTime:  "2024-01-15T10:00:00Z",
		EndTime:    "2024-01-17T10:00:00Z",
		DataPoints: 1,
		Points: []TimeSeriesPointExport{
			{
				Timestamp: "2024-01-15T10:00:00Z",
				Value:     5.0,
			},
		},
		Statistics: TimeSeriesStatisticsExport{
			Min:     5.0,
			Max:     5.0,
			Average: 5.0,
			Current: 5.0,
			Change:  0.0,
			Trend:   "stable",
		},
	}

	jsonStr, err := JSONToString(export)

	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "complexity")
	assert.Contains(t, jsonStr, "pkg/api")
	assert.Contains(t, jsonStr, "metric_name")
}

func TestExportToJSONWithoutScopePath(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     5.0,
		},
	}

	export, err := ExportToJSON("overall_score", "", points)

	require.NoError(t, err)
	assert.NotNil(t, export)
	assert.Equal(t, "overall_score", export.MetricName)
	assert.Empty(t, export.ScopePath)
}

func TestExportToJSONRounding(t *testing.T) {
	points := []storage.TimeSeriesPoint{
		{
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Value:     1.111111,
		},
		{
			Timestamp: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
			Value:     2.222222,
		},
		{
			Timestamp: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
			Value:     3.333333,
		},
	}

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)
	// Values should be preserved as-is in export
	assert.Equal(t, 1.111111, export.Points[0].Value)
	assert.Equal(t, 3.333333, export.Points[2].Value)
}

func TestExportToJSONLargeDataset(t *testing.T) {
	// Test with many data points
	var points []storage.TimeSeriesPoint
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		points = append(points, storage.TimeSeriesPoint{
			Timestamp: baseTime.AddDate(0, 0, i),
			Value:     float64(i) + 5.0,
		})
	}

	export, err := ExportToJSON("complexity", "", points)

	require.NoError(t, err)
	assert.Equal(t, 100, export.DataPoints)
	assert.Len(t, export.Points, 100)
	assert.Equal(t, 5.0, export.Statistics.Min)
	assert.Equal(t, 104.0, export.Statistics.Max)
}

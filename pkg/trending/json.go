package trending

import (
	"encoding/json"
	"os"

	"github.com/alexcollie/kaizen/pkg/storage"
)

// TimeSeriesExport represents exportable time-series data
type TimeSeriesExport struct {
	MetricName  string                      `json:"metric_name"`
	ScopePath   string                      `json:"scope_path,omitempty"`
	StartTime   string                      `json:"start_time"`
	EndTime     string                      `json:"end_time"`
	DataPoints  int                         `json:"data_points"`
	Points      []TimeSeriesPointExport     `json:"points"`
	Statistics  TimeSeriesStatisticsExport  `json:"statistics"`
}

// TimeSeriesPointExport represents a single data point
type TimeSeriesPointExport struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// TimeSeriesStatisticsExport contains summary statistics
type TimeSeriesStatisticsExport struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Average float64 `json:"average"`
	Current float64 `json:"current"`
	Change  float64 `json:"change"`
	Trend   string  `json:"trend"` // "up", "down", "stable"
}

// ExportToJSON converts time-series data to JSON export format
func ExportToJSON(metricName string, scopePath string, points []storage.TimeSeriesPoint) (*TimeSeriesExport, error) {
	if len(points) == 0 {
		return nil, nil
	}

	// Convert points
	exportPoints := make([]TimeSeriesPointExport, len(points))
	values := make([]float64, len(points))

	for i, p := range points {
		exportPoints[i] = TimeSeriesPointExport{
			Timestamp: p.Timestamp.Format("2006-01-02T15:04:05Z"),
			Value:     p.Value,
		}
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
	current := values[len(values)-1]
	change := current - values[0]

	// Determine trend
	trend := "stable"
	if change > avg*0.05 {
		trend = "up"
	} else if change < -avg*0.05 {
		trend = "down"
	}

	return &TimeSeriesExport{
		MetricName: metricName,
		ScopePath:  scopePath,
		StartTime:  points[0].Timestamp.Format("2006-01-02T15:04:05Z"),
		EndTime:    points[len(points)-1].Timestamp.Format("2006-01-02T15:04:05Z"),
		DataPoints: len(points),
		Points:     exportPoints,
		Statistics: TimeSeriesStatisticsExport{
			Min:     min,
			Max:     max,
			Average: avg,
			Current: current,
			Change:  change,
			Trend:   trend,
		},
	}, nil
}

// WriteJSONToFile writes JSON export to file
func WriteJSONToFile(export *TimeSeriesExport, outputPath string) error {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

// JSONToString converts export to formatted JSON string
func JSONToString(export *TimeSeriesExport) (string, error) {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

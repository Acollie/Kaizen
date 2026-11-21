package analyzer

import (
	"path/filepath"

	"github.com/alexcollie/kaizen/pkg/models"
)

// Aggregator rolls file metrics up into folder metrics
type Aggregator struct{}

// NewAggregator creates an aggregator
func NewAggregator() *Aggregator {
	return &Aggregator{}
}

// AggregateByFolder groups files by directory and averages their metrics
func (aggregator *Aggregator) AggregateByFolder(files []models.FileAnalysis) map[string]models.FolderMetrics {
	folders := make(map[string]models.FolderMetrics)
	for _, file := range files {
		folder := filepath.Dir(file.Path)
		metrics := folders[folder]
		metrics.Path = folder
		metrics.FileCount++
		metrics.TotalLines += file.TotalLines
		metrics.TotalComplexity += file.TotalComplexity
		if file.Churn != nil {
			metrics.TotalCommits += file.Churn.TotalCommits
		}
		folders[folder] = metrics
	}

	for folder, metrics := range folders {
		if metrics.FileCount > 0 {
			metrics.AverageComplexity = float64(metrics.TotalComplexity) / float64(metrics.FileCount)
		}
		folders[folder] = metrics
	}
	return folders
}

package analyzer

import (
	"math"
	"path/filepath"
	"sort"

	"github.com/alexcollie/kaizen/pkg/models"
)

// DefaultAggregator implements the Aggregator interface
type DefaultAggregator struct{}

// NewAggregator creates a new aggregator
func NewAggregator() Aggregator {
	return &DefaultAggregator{}
}

// AggregateByFolder groups file analyses by folder and calculates folder metrics
func (aggregator *DefaultAggregator) AggregateByFolder(files []models.FileAnalysis) map[string]models.FolderMetrics {
	folderMap := make(map[string]*models.FolderMetrics)

	// Group files by directory
	for _, file := range files {
		dir := filepath.Dir(file.Path)

		// Initialize folder if not exists
		if _, exists := folderMap[dir]; !exists {
			folderMap[dir] = &models.FolderMetrics{
				Path: dir,
			}
		}

		folder := folderMap[dir]
		folder.TotalFiles++
		folder.TotalLines += file.TotalLines
		folder.TotalCodeLines += file.CodeLines

		// Aggregate function metrics
		for _, function := range file.Functions {
			folder.TotalFunctions++

			// Sum for averaging
			folder.AverageComplexity += float64(function.CyclomaticComplexity)
			folder.AverageCognitive += float64(function.CognitiveComplexity)
			folder.AverageLength += float64(function.Length)
			folder.AverageMaintainability += function.MaintainabilityIndex

			// Count hotspots
			if function.IsHotspot {
				folder.HotspotCount++
			}

			// Sum churn
			if function.Churn != nil {
				folder.TotalChurn += function.Churn.TotalChanges
				folder.AverageChurn += float64(function.Churn.TotalChanges)
			}
		}
	}

	// Calculate averages
	result := make(map[string]models.FolderMetrics)
	for path, folder := range folderMap {
		if folder.TotalFunctions > 0 {
			folder.AverageComplexity /= float64(folder.TotalFunctions)
			folder.AverageCognitive /= float64(folder.TotalFunctions)
			folder.AverageLength /= float64(folder.TotalFunctions)
			folder.AverageMaintainability /= float64(folder.TotalFunctions)
			folder.AverageChurn /= float64(folder.TotalFunctions)
		}
		result[path] = *folder
	}

	return result
}

// CalculateScores normalizes raw metrics to 0-100 scores for visualization
func (aggregator *DefaultAggregator) CalculateScores(folders map[string]models.FolderMetrics) map[string]models.FolderMetrics {
	if len(folders) == 0 {
		return folders
	}

	// Collect all values for normalization
	complexities := make([]float64, 0, len(folders))
	churns := make([]float64, 0, len(folders))
	lengths := make([]float64, 0, len(folders))
	maintainabilities := make([]float64, 0, len(folders))

	for _, folder := range folders {
		complexities = append(complexities, folder.AverageComplexity)
		churns = append(churns, folder.AverageChurn)
		lengths = append(lengths, folder.AverageLength)
		maintainabilities = append(maintainabilities, folder.AverageMaintainability)
	}

	// Sort for percentile calculation
	sort.Float64s(complexities)
	sort.Float64s(churns)
	sort.Float64s(lengths)
	sort.Float64s(maintainabilities)

	// Calculate scores for each folder
	result := make(map[string]models.FolderMetrics)
	for path, folder := range folders {
		folder.ComplexityScore = percentileRank(folder.AverageComplexity, complexities)
		folder.ChurnScore = percentileRank(folder.AverageChurn, churns)
		folder.LengthScore = percentileRank(folder.AverageLength, lengths)

		// Maintainability is inverse (higher is better, so invert the score)
		folder.MaintainabilityScore = 100 - percentileRank(folder.AverageMaintainability, maintainabilities)

		// Hotspot score combines complexity and churn
		folder.HotspotScore = (folder.ComplexityScore + folder.ChurnScore) / 2

		result[path] = folder
	}

	return result
}

// percentileRank calculates the percentile rank (0-100) of a value in a sorted slice
func percentileRank(value float64, sortedValues []float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}

	// Handle edge cases
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	// Count values less than or equal to target
	count := 0
	for _, sortedValue := range sortedValues {
		if sortedValue <= value {
			count++
		}
	}

	// Calculate percentile: (count / total) * 100
	percentile := (float64(count) / float64(len(sortedValues))) * 100.0

	// Clamp to 0-100
	if percentile < 0 {
		percentile = 0
	}
	if percentile > 100 {
		percentile = 100
	}

	return percentile
}

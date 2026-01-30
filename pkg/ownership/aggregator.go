package ownership

import (
	"sort"

	"github.com/alexcollie/kaizen/pkg/models"
)

// Aggregator computes metrics aggregated by code owner
type Aggregator struct {
	codeowners *CodeOwners
}

// NewAggregator creates a new ownership aggregator
func NewAggregator(codeowners *CodeOwners) *Aggregator {
	return &Aggregator{
		codeowners: codeowners,
	}
}

// AggregateByOwner aggregates analysis metrics by file owner
func (agg *Aggregator) AggregateByOwner(result *models.AnalysisResult) (map[string]*OwnerMetrics, map[string][]string) {
	ownerMetrics := make(map[string]*OwnerMetrics)
	fileOwnershipMap := make(map[string][]string)

	// Initialize owner tracking
	ownerFunctions := make(map[string][]models.FunctionAnalysis)
	ownerFiles := make(map[string]map[string]bool)
	ownerLines := make(map[string]int)

	// Process each file
	for _, fileAnalysis := range result.Files {
		owners, pattern := agg.codeowners.GetOwnersWithPattern(fileAnalysis.Path)

		// Track file ownership
		fileOwnershipMap[fileAnalysis.Path] = owners

		// If no owner, skip
		if len(owners) == 0 {
			continue
		}

		// Assign to each owner (file may have multiple owners)
		for _, owner := range owners {
			// Initialize owner if needed
			if _, exists := ownerMetrics[owner]; !exists {
				ownerMetrics[owner] = &OwnerMetrics{
					Owner: owner,
				}
				ownerFiles[owner] = make(map[string]bool)
				ownerLines[owner] = 0
			}

			// Track file
			ownerFiles[owner][fileAnalysis.Path] = true

			// Track lines
			ownerLines[owner] += fileAnalysis.CodeLines

			// Track functions
			ownerFunctions[owner] = append(ownerFunctions[owner], fileAnalysis.Functions...)

			// Log ownership for reference
			_ = pattern // Pattern is logged for tracing
		}
	}

	// Calculate metrics for each owner
	for owner, metrics := range ownerMetrics {
		functions := ownerFunctions[owner]

		metrics.FileCount = len(ownerFiles[owner])
		metrics.FunctionCount = len(functions)
		metrics.TotalLines = ownerLines[owner]

		if len(functions) > 0 {
			// Calculate averages
			var sumComplexity, sumCognitive, sumMaintainability float64
			hotspotCount := 0
			highComplexityCount := 0

			for _, fn := range functions {
				sumComplexity += float64(fn.CyclomaticComplexity)
				sumCognitive += float64(fn.CognitiveComplexity)
				sumMaintainability += fn.MaintainabilityIndex

				if fn.IsHotspot {
					hotspotCount++
				}

				if fn.CyclomaticComplexity > 10 {
					highComplexityCount++
				}
			}

			metrics.AvgCyclomaticComplexity = sumComplexity / float64(len(functions))
			metrics.AvgCognitiveComplexity = sumCognitive / float64(len(functions))
			metrics.AvgMaintainabilityIndex = sumMaintainability / float64(len(functions))
			metrics.HotspotCount = hotspotCount
			metrics.HighComplexityFunctionCount = highComplexityCount
		}

		// Calculate health score (similar to overall score)
		metrics.OverallHealthScore = calculateOwnerHealthScore(metrics)
	}

	return ownerMetrics, fileOwnershipMap
}

// calculateOwnerHealthScore computes a health score (0-100) for an owner's code
func calculateOwnerHealthScore(metrics *OwnerMetrics) float64 {
	if metrics.FunctionCount == 0 {
		return 100.0 // No code = no issues
	}

	score := 100.0

	// Penalty for complexity
	complexityPenalty := (metrics.AvgCyclomaticComplexity / 10.0) * 20.0
	if complexityPenalty > 20 {
		complexityPenalty = 20
	}
	score -= complexityPenalty

	// Penalty for low maintainability
	maintPenalty := ((100.0 - metrics.AvgMaintainabilityIndex) / 100.0) * 20.0
	if maintPenalty > 20 {
		maintPenalty = 20
	}
	score -= maintPenalty

	// Penalty for hotspots
	hotspotPenalty := float64(metrics.HotspotCount) * 2.0
	if hotspotPenalty > 20 {
		hotspotPenalty = 20
	}
	score -= hotspotPenalty

	// Penalty for high complexity functions
	highComplexityPenalty := float64(metrics.HighComplexityFunctionCount) * 1.5
	if highComplexityPenalty > 15 {
		highComplexityPenalty = 15
	}
	score -= highComplexityPenalty

	if score < 0 {
		score = 0
	}

	return score
}

// GetOwnerReport generates a complete ownership report for a snapshot
func (agg *Aggregator) GetOwnerReport(result *models.AnalysisResult, snapshotID int64, analyzedAt string) *OwnerReport {
	ownerMetrics, fileOwnership := agg.AggregateByOwner(result)

	// Convert map to sorted slice
	var metrics []OwnerMetrics
	for _, m := range ownerMetrics {
		metrics = append(metrics, *m)
	}

	// Sort by health score (descending)
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].OverallHealthScore > metrics[j].OverallHealthScore
	})

	return &OwnerReport{
		SnapshotID:       snapshotID,
		AnalyzedAt:       analyzedAt,
		TotalOwners:      len(metrics),
		OwnerMetrics:     metrics,
		FileOwnershipMap: fileOwnership,
	}
}

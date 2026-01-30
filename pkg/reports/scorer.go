package reports

import (
	"github.com/alexcollie/kaizen/pkg/models"
)

// ScoreWeights defines contribution of each component to the overall score
type ScoreWeights struct {
	Complexity      float64
	Maintainability float64
	Churn           float64
	FunctionSize    float64
	CodeStructure   float64
}

// DefaultWeights returns the default score weights
func DefaultWeights() ScoreWeights {
	return ScoreWeights{
		Complexity:      0.25,
		Maintainability: 0.25,
		Churn:           0.20,
		FunctionSize:    0.15,
		CodeStructure:   0.15,
	}
}

// WeightsWithoutChurn redistributes churn weight when no churn data is available
func WeightsWithoutChurn() ScoreWeights {
	return ScoreWeights{
		Complexity:      0.30,
		Maintainability: 0.30,
		Churn:           0.0,
		FunctionSize:    0.20,
		CodeStructure:   0.20,
	}
}

// GenerateScoreReport calculates the overall score report for an analysis result
func GenerateScoreReport(result *models.AnalysisResult, hasChurnData bool) *models.ScoreReport {
	// Handle empty codebase
	if result.Summary.TotalFunctions == 0 {
		return createEmptyCodebaseReport()
	}

	weights := DefaultWeights()
	if !hasChurnData {
		weights = WeightsWithoutChurn()
	}

	componentScores := calculateComponentScores(result, hasChurnData, weights)
	overallScore := calculateOverallScore(componentScores, weights)
	overallGrade := CalculateGrade(overallScore)
	concerns := DetectConcerns(result, hasChurnData)

	return &models.ScoreReport{
		OverallGrade:    overallGrade,
		OverallScore:    overallScore,
		ComponentScores: componentScores,
		Concerns:        concerns,
		HasChurnData:    hasChurnData,
	}
}

func createEmptyCodebaseReport() *models.ScoreReport {
	return &models.ScoreReport{
		OverallGrade: "A",
		OverallScore: 100,
		ComponentScores: models.ComponentScores{
			Complexity:      models.CategoryScore{Score: 100, Weight: 0.25, Category: "excellent"},
			Maintainability: models.CategoryScore{Score: 100, Weight: 0.25, Category: "excellent"},
			Churn:           models.CategoryScore{Score: 100, Weight: 0.20, Category: "excellent"},
			FunctionSize:    models.CategoryScore{Score: 100, Weight: 0.15, Category: "excellent"},
			CodeStructure:   models.CategoryScore{Score: 100, Weight: 0.15, Category: "excellent"},
		},
		Concerns: []models.Concern{{
			Type:        "empty_codebase",
			Severity:    "info",
			Title:       "No Functions Found",
			Description: "No functions found to analyze",
		}},
		HasChurnData: false,
	}
}

func calculateComponentScores(
	result *models.AnalysisResult,
	hasChurnData bool,
	weights ScoreWeights,
) models.ComponentScores {
	complexityScore := calculateComplexityScore(result)
	maintainabilityScore := calculateMaintainabilityScore(result)
	churnScore := calculateChurnScore(result, hasChurnData)
	functionSizeScore := calculateFunctionSizeScore(result)
	codeStructureScore := calculateCodeStructureScore(result)

	return models.ComponentScores{
		Complexity: models.CategoryScore{
			Score:    complexityScore,
			Weight:   weights.Complexity,
			Category: GetCategoryLabel(complexityScore),
		},
		Maintainability: models.CategoryScore{
			Score:    maintainabilityScore,
			Weight:   weights.Maintainability,
			Category: GetCategoryLabel(maintainabilityScore),
		},
		Churn: models.CategoryScore{
			Score:    churnScore,
			Weight:   weights.Churn,
			Category: GetCategoryLabel(churnScore),
		},
		FunctionSize: models.CategoryScore{
			Score:    functionSizeScore,
			Weight:   weights.FunctionSize,
			Category: GetCategoryLabel(functionSizeScore),
		},
		CodeStructure: models.CategoryScore{
			Score:    codeStructureScore,
			Weight:   weights.CodeStructure,
			Category: GetCategoryLabel(codeStructureScore),
		},
	}
}

// calculateComplexityScore: 100 - clamp(avgCC * 5, 0, 100)
// CC of 20 = score of 0
func calculateComplexityScore(result *models.AnalysisResult) float64 {
	avgComplexity := result.Summary.AverageCyclomaticComplexity
	score := 100 - clamp(avgComplexity*5, 0, 100)
	return score
}

// calculateMaintainabilityScore: already 0-100, higher is better
func calculateMaintainabilityScore(result *models.AnalysisResult) float64 {
	return clamp(result.Summary.AverageMaintainabilityIndex, 0, 100)
}

// calculateChurnScore: 100 - clamp(avgCommits * 2, 0, 100)
// Returns neutral score of 70 if no churn data
func calculateChurnScore(result *models.AnalysisResult, hasChurnData bool) float64 {
	if !hasChurnData {
		return 70 // Neutral score when no churn data
	}

	// Calculate average commits per function
	totalCommits := 0
	functionCount := 0
	for _, file := range result.Files {
		for _, function := range file.Functions {
			if function.Churn != nil {
				totalCommits += function.Churn.TotalCommits
				functionCount++
			}
		}
	}

	if functionCount == 0 {
		return 70 // Neutral if no function-level churn
	}

	avgCommits := float64(totalCommits) / float64(functionCount)
	score := 100 - clamp(avgCommits*2, 0, 100)
	return score
}

// calculateFunctionSizeScore: 100 - (longFuncPct * 50 + veryLongFuncPct * 50)
func calculateFunctionSizeScore(result *models.AnalysisResult) float64 {
	summary := result.Summary
	if summary.TotalFunctions == 0 {
		return 100
	}

	longFuncPct := float64(summary.LongFunctionCount) / float64(summary.TotalFunctions)
	veryLongFuncPct := float64(summary.VeryLongFunctionCount) / float64(summary.TotalFunctions)

	score := 100 - (longFuncPct*50 + veryLongFuncPct*50)
	return clamp(score, 0, 100)
}

// calculateCodeStructureScore: 100 - (highNestingPct * 40 + highParamPct * 30 + veryHighCCPct * 30)
func calculateCodeStructureScore(result *models.AnalysisResult) float64 {
	summary := result.Summary
	if summary.TotalFunctions == 0 {
		return 100
	}

	// Count functions with deep nesting and many parameters
	highNestingCount := 0
	highParamCount := 0

	for _, file := range result.Files {
		for _, function := range file.Functions {
			if function.NestingDepth > 5 {
				highNestingCount++
			}
			if function.ParameterCount > 7 {
				highParamCount++
			}
		}
	}

	totalFunctions := float64(summary.TotalFunctions)
	highNestingPct := float64(highNestingCount) / totalFunctions
	highParamPct := float64(highParamCount) / totalFunctions
	veryHighCCPct := float64(summary.VeryHighComplexityCount) / totalFunctions

	score := 100 - (highNestingPct*40 + highParamPct*30 + veryHighCCPct*30)
	return clamp(score, 0, 100)
}

func calculateOverallScore(scores models.ComponentScores, weights ScoreWeights) float64 {
	overall := scores.Complexity.Score*weights.Complexity +
		scores.Maintainability.Score*weights.Maintainability +
		scores.Churn.Score*weights.Churn +
		scores.FunctionSize.Score*weights.FunctionSize +
		scores.CodeStructure.Score*weights.CodeStructure

	// Normalize if weights don't sum to 1 (when churn is excluded)
	totalWeight := weights.Complexity + weights.Maintainability + weights.Churn +
		weights.FunctionSize + weights.CodeStructure

	if totalWeight > 0 && totalWeight != 1.0 {
		overall = overall / totalWeight
	}

	return clamp(overall, 0, 100)
}

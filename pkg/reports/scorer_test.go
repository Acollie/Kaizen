package reports

import (
	"testing"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/models"
)

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()

	// Verify all weights are positive
	if weights.Complexity <= 0 {
		t.Error("Complexity weight should be positive")
	}
	if weights.Maintainability <= 0 {
		t.Error("Maintainability weight should be positive")
	}
	if weights.Churn <= 0 {
		t.Error("Churn weight should be positive")
	}
	if weights.FunctionSize <= 0 {
		t.Error("FunctionSize weight should be positive")
	}
	if weights.CodeStructure <= 0 {
		t.Error("CodeStructure weight should be positive")
	}

	// Verify weights sum to 1.0
	totalWeight := weights.Complexity + weights.Maintainability + weights.Churn +
		weights.FunctionSize + weights.CodeStructure
	if totalWeight != 1.0 {
		t.Errorf("Weights should sum to 1.0, got %v", totalWeight)
	}
}

func TestWeightsWithoutChurn(t *testing.T) {
	weights := WeightsWithoutChurn()

	// Verify churn weight is zero
	if weights.Churn != 0 {
		t.Errorf("Churn weight should be 0 when no churn data, got %v", weights.Churn)
	}

	// Verify remaining weights sum to 1.0
	totalWeight := weights.Complexity + weights.Maintainability + weights.Churn +
		weights.FunctionSize + weights.CodeStructure
	if totalWeight != 1.0 {
		t.Errorf("Weights should sum to 1.0, got %v", totalWeight)
	}
}

func TestGenerateScoreReportEmptyCodebase(t *testing.T) {
	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFunctions: 0,
		},
	}

	report := GenerateScoreReport(result, false, config.DefaultConfig().Thresholds)

	if report.OverallGrade != "A" {
		t.Errorf("Empty codebase should get grade A, got %v", report.OverallGrade)
	}
	if report.OverallScore != 100 {
		t.Errorf("Empty codebase should get score 100, got %v", report.OverallScore)
	}
	if len(report.Concerns) == 0 {
		t.Error("Empty codebase should have an info concern")
	}
	if report.Concerns[0].Severity != "info" {
		t.Errorf("Empty codebase concern should be info severity, got %v", report.Concerns[0].Severity)
	}
}

func TestGenerateScoreReportExcellentCode(t *testing.T) {
	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFunctions:              10,
			AverageCyclomaticComplexity: 2.0,   // Low complexity
			AverageMaintainabilityIndex: 95.0,  // High maintainability
			LongFunctionCount:           0,
			VeryLongFunctionCount:       0,
			VeryHighComplexityCount:     0,
		},
		Files: []models.FileAnalysis{
			{
				Functions: []models.FunctionAnalysis{
					{Name: "foo", NestingDepth: 2, ParameterCount: 2},
				},
			},
		},
	}

	report := GenerateScoreReport(result, false, config.DefaultConfig().Thresholds)

	if report.OverallGrade != "A" {
		t.Errorf("Excellent code should get grade A, got %v", report.OverallGrade)
	}
	if report.OverallScore < 90 {
		t.Errorf("Excellent code should get score >= 90, got %v", report.OverallScore)
	}
}

func TestGenerateScoreReportPoorCode(t *testing.T) {
	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFunctions:              10,
			AverageCyclomaticComplexity: 25.0, // Very high complexity
			AverageMaintainabilityIndex: 20.0, // Low maintainability
			LongFunctionCount:           8,
			VeryLongFunctionCount:       5,
			VeryHighComplexityCount:     8,
		},
		Files: []models.FileAnalysis{
			{
				Path: "test.go",
				Functions: []models.FunctionAnalysis{
					{Name: "bad", NestingDepth: 10, ParameterCount: 15},
				},
			},
		},
	}

	report := GenerateScoreReport(result, false, config.DefaultConfig().Thresholds)

	if report.OverallGrade == "A" {
		t.Error("Poor code should not get grade A")
	}
	if report.OverallScore > 60 {
		t.Errorf("Poor code should get score <= 60, got %v", report.OverallScore)
	}
}

func TestGenerateScoreReportWithChurnData(t *testing.T) {
	churnMetric := &models.ChurnMetric{
		TotalCommits: 5,
	}

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFunctions:              5,
			AverageCyclomaticComplexity: 5.0,
			AverageMaintainabilityIndex: 80.0,
		},
		Files: []models.FileAnalysis{
			{
				Functions: []models.FunctionAnalysis{
					{Name: "func1", Churn: churnMetric},
					{Name: "func2", Churn: churnMetric},
				},
			},
		},
	}

	report := GenerateScoreReport(result, true, config.DefaultConfig().Thresholds)

	if !report.HasChurnData {
		t.Error("Report should indicate churn data is present")
	}

	// Churn weight should be 0.20 with churn data
	if report.ComponentScores.Churn.Weight != 0.20 {
		t.Errorf("Churn weight should be 0.20 with churn data, got %v",
			report.ComponentScores.Churn.Weight)
	}
}

func TestGenerateScoreReportWithoutChurnData(t *testing.T) {
	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFunctions:              5,
			AverageCyclomaticComplexity: 5.0,
			AverageMaintainabilityIndex: 80.0,
		},
		Files: []models.FileAnalysis{
			{
				Functions: []models.FunctionAnalysis{
					{Name: "func1"},
				},
			},
		},
	}

	report := GenerateScoreReport(result, false, config.DefaultConfig().Thresholds)

	if report.HasChurnData {
		t.Error("Report should indicate churn data is not present")
	}

	// Churn weight should be 0 without churn data
	if report.ComponentScores.Churn.Weight != 0 {
		t.Errorf("Churn weight should be 0 without churn data, got %v",
			report.ComponentScores.Churn.Weight)
	}

	// Churn score should be neutral (70)
	if report.ComponentScores.Churn.Score != 70 {
		t.Errorf("Churn score should be 70 (neutral) without churn data, got %v",
			report.ComponentScores.Churn.Score)
	}
}

func TestCalculateComplexityScore(t *testing.T) {
	tests := []struct {
		name          string
		avgComplexity float64
		minExpected   float64
		maxExpected   float64
	}{
		{"low complexity", 2.0, 85.0, 100.0},
		{"moderate complexity", 10.0, 45.0, 55.0},
		{"high complexity", 20.0, 0.0, 5.0},
		{"very high complexity", 30.0, 0.0, 0.0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := &models.AnalysisResult{
				Summary: models.SummaryMetrics{
					AverageCyclomaticComplexity: testCase.avgComplexity,
				},
			}
			score := calculateComplexityScore(result)
			if score < testCase.minExpected || score > testCase.maxExpected {
				t.Errorf("calculateComplexityScore for %v = %v, expected between %v and %v",
					testCase.avgComplexity, score, testCase.minExpected, testCase.maxExpected)
			}
		})
	}
}

func TestCalculateMaintainabilityScore(t *testing.T) {
	tests := []struct {
		name     string
		avgMI    float64
		expected float64
	}{
		{"excellent maintainability", 95.0, 95.0},
		{"good maintainability", 75.0, 75.0},
		{"low maintainability", 30.0, 30.0},
		{"negative clamped", -10.0, 0.0},
		{"over 100 clamped", 110.0, 100.0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := &models.AnalysisResult{
				Summary: models.SummaryMetrics{
					AverageMaintainabilityIndex: testCase.avgMI,
				},
			}
			score := calculateMaintainabilityScore(result)
			if score != testCase.expected {
				t.Errorf("calculateMaintainabilityScore for %v = %v, expected %v",
					testCase.avgMI, score, testCase.expected)
			}
		})
	}
}

func TestCalculateFunctionSizeScore(t *testing.T) {
	tests := []struct {
		name           string
		totalFunctions int
		longCount      int
		veryLongCount  int
		minExpected    float64
	}{
		{"no long functions", 10, 0, 0, 100.0},
		{"some long functions", 10, 2, 0, 80.0},
		{"some very long functions", 10, 0, 2, 80.0},
		{"mixed long functions", 10, 2, 2, 60.0},
		{"all long functions", 10, 10, 0, 50.0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := &models.AnalysisResult{
				Summary: models.SummaryMetrics{
					TotalFunctions:        testCase.totalFunctions,
					LongFunctionCount:     testCase.longCount,
					VeryLongFunctionCount: testCase.veryLongCount,
				},
			}
			score := calculateFunctionSizeScore(result)
			if score < testCase.minExpected {
				t.Errorf("calculateFunctionSizeScore for long=%v, veryLong=%v = %v, expected >= %v",
					testCase.longCount, testCase.veryLongCount, score, testCase.minExpected)
			}
		})
	}
}

func TestCalculateCodeStructureScore(t *testing.T) {
	tests := []struct {
		name        string
		functions   []models.FunctionAnalysis
		highCCCount int
		minExpected float64
	}{
		{
			"clean code",
			[]models.FunctionAnalysis{
				{NestingDepth: 2, ParameterCount: 3},
			},
			0,
			95.0,
		},
		{
			"deep nesting",
			[]models.FunctionAnalysis{
				{NestingDepth: 8, ParameterCount: 3},
			},
			0,
			55.0,
		},
		{
			"many parameters",
			[]models.FunctionAnalysis{
				{NestingDepth: 2, ParameterCount: 12},
			},
			0,
			65.0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := &models.AnalysisResult{
				Summary: models.SummaryMetrics{
					TotalFunctions:          len(testCase.functions),
					VeryHighComplexityCount: testCase.highCCCount,
				},
				Files: []models.FileAnalysis{
					{Functions: testCase.functions},
				},
			}
			score := calculateCodeStructureScore(result, config.DefaultConfig().Thresholds)
			if score < testCase.minExpected {
				t.Errorf("calculateCodeStructureScore = %v, expected >= %v", score, testCase.minExpected)
			}
		})
	}
}

func TestCalculateOverallScore(t *testing.T) {
	weights := DefaultWeights()

	// Test with all perfect scores
	perfectScores := models.ComponentScores{
		Complexity:      models.CategoryScore{Score: 100},
		Maintainability: models.CategoryScore{Score: 100},
		Churn:           models.CategoryScore{Score: 100},
		FunctionSize:    models.CategoryScore{Score: 100},
		CodeStructure:   models.CategoryScore{Score: 100},
	}

	overall := calculateOverallScore(perfectScores, weights)
	if overall != 100 {
		t.Errorf("Perfect scores should give overall 100, got %v", overall)
	}

	// Test with all zero scores
	zeroScores := models.ComponentScores{
		Complexity:      models.CategoryScore{Score: 0},
		Maintainability: models.CategoryScore{Score: 0},
		Churn:           models.CategoryScore{Score: 0},
		FunctionSize:    models.CategoryScore{Score: 0},
		CodeStructure:   models.CategoryScore{Score: 0},
	}

	overall = calculateOverallScore(zeroScores, weights)
	if overall != 0 {
		t.Errorf("Zero scores should give overall 0, got %v", overall)
	}

	// Test with mixed scores
	mixedScores := models.ComponentScores{
		Complexity:      models.CategoryScore{Score: 80},
		Maintainability: models.CategoryScore{Score: 90},
		Churn:           models.CategoryScore{Score: 70},
		FunctionSize:    models.CategoryScore{Score: 85},
		CodeStructure:   models.CategoryScore{Score: 75},
	}

	overall = calculateOverallScore(mixedScores, weights)
	// Expected: 80*0.25 + 90*0.25 + 70*0.20 + 85*0.15 + 75*0.15 = 20 + 22.5 + 14 + 12.75 + 11.25 = 80.5
	if overall < 79 || overall > 82 {
		t.Errorf("Mixed scores should give overall around 80.5, got %v", overall)
	}
}

package analyzer

import (
	"math"
	"testing"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateByFolderEmptyList(t *testing.T) {
	aggregator := NewAggregator()
	result := aggregator.AggregateByFolder([]models.FileAnalysis{})
	assert.Empty(t, result)
}

func TestAggregateByFolderSingleFile(t *testing.T) {
	aggregator := NewAggregator()
	files := []models.FileAnalysis{
		{
			Path:       "pkg/analyzer/file.go",
			Language:   "Go",
			TotalLines: 100,
			CodeLines:  80,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "TestFunc",
					Length:                 10,
					CyclomaticComplexity:   3,
					CognitiveComplexity:    2,
					MaintainabilityIndex:   75.5,
					IsHotspot:              false,
				},
			},
		},
	}

	result := aggregator.AggregateByFolder(files)
	require.Len(t, result, 1)

	folder, exists := result["pkg/analyzer"]
	require.True(t, exists)
	assert.Equal(t, "pkg/analyzer", folder.Path)
	assert.Equal(t, 1, folder.TotalFiles)
	assert.Equal(t, 1, folder.TotalFunctions)
	assert.Equal(t, 100, folder.TotalLines)
	assert.Equal(t, 80, folder.TotalCodeLines)
	assert.Equal(t, 3.0, folder.AverageComplexity)
	assert.Equal(t, 2.0, folder.AverageCognitive)
	assert.Equal(t, 10.0, folder.AverageLength)
	assert.InDelta(t, 75.5, folder.AverageMaintainability, 0.01)
}

func TestAggregateByFolderMultipleFilesInFolder(t *testing.T) {
	aggregator := NewAggregator()
	files := []models.FileAnalysis{
		{
			Path:       "pkg/analyzer/file1.go",
			Language:   "Go",
			TotalLines: 100,
			CodeLines:  80,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "Func1",
					Length:                 10,
					CyclomaticComplexity:   2,
					CognitiveComplexity:    1,
					MaintainabilityIndex:   80.0,
				},
				{
					Name:                   "Func2",
					Length:                 20,
					CyclomaticComplexity:   4,
					CognitiveComplexity:    3,
					MaintainabilityIndex:   70.0,
				},
			},
		},
		{
			Path:       "pkg/analyzer/file2.go",
			Language:   "Go",
			TotalLines: 150,
			CodeLines:  120,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "Func3",
					Length:                 15,
					CyclomaticComplexity:   3,
					CognitiveComplexity:    2,
					MaintainabilityIndex:   75.0,
				},
			},
		},
	}

	result := aggregator.AggregateByFolder(files)
	require.Len(t, result, 1)

	folder := result["pkg/analyzer"]
	assert.Equal(t, 2, folder.TotalFiles)
	assert.Equal(t, 3, folder.TotalFunctions)
	assert.Equal(t, 250, folder.TotalLines)
	assert.Equal(t, 200, folder.TotalCodeLines)

	// Check averages
	expectedComplexity := (2.0 + 4.0 + 3.0) / 3.0
	expectedCognitive := (1.0 + 3.0 + 2.0) / 3.0
	expectedLength := (10.0 + 20.0 + 15.0) / 3.0
	expectedMaintainability := (80.0 + 70.0 + 75.0) / 3.0

	assert.InDelta(t, expectedComplexity, folder.AverageComplexity, 0.01)
	assert.InDelta(t, expectedCognitive, folder.AverageCognitive, 0.01)
	assert.InDelta(t, expectedLength, folder.AverageLength, 0.01)
	assert.InDelta(t, expectedMaintainability, folder.AverageMaintainability, 0.01)
}

func TestAggregateByFolderNestedFolders(t *testing.T) {
	aggregator := NewAggregator()
	files := []models.FileAnalysis{
		{
			Path:       "pkg/analyzer/file1.go",
			TotalLines: 100,
			CodeLines:  80,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "Func1",
					CyclomaticComplexity:   2,
					CognitiveComplexity:    1,
					MaintainabilityIndex:   80.0,
					Length:                 10,
				},
			},
		},
		{
			Path:       "pkg/churn/file2.go",
			TotalLines: 150,
			CodeLines:  120,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "Func2",
					CyclomaticComplexity:   3,
					CognitiveComplexity:    2,
					MaintainabilityIndex:   75.0,
					Length:                 15,
				},
			},
		},
	}

	result := aggregator.AggregateByFolder(files)
	require.Len(t, result, 2)

	_, hasAnalyzer := result["pkg/analyzer"]
	_, hasChurn := result["pkg/churn"]
	assert.True(t, hasAnalyzer)
	assert.True(t, hasChurn)
}

func TestAggregateByFolderWithHotspots(t *testing.T) {
	aggregator := NewAggregator()
	files := []models.FileAnalysis{
		{
			Path:       "pkg/analyzer/file.go",
			TotalLines: 100,
			CodeLines:  80,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "SimpleFunc",
					CyclomaticComplexity:   2,
					CognitiveComplexity:    1,
					MaintainabilityIndex:   80.0,
					IsHotspot:              false,
					Length:                 10,
				},
				{
					Name:                   "HotspotFunc",
					CyclomaticComplexity:   15,
					CognitiveComplexity:    12,
					MaintainabilityIndex:   30.0,
					IsHotspot:              true,
					Length:                 50,
				},
			},
		},
	}

	result := aggregator.AggregateByFolder(files)
	folder := result["pkg/analyzer"]
	assert.Equal(t, 1, folder.HotspotCount)
}

func TestAggregateByFolderWithChurn(t *testing.T) {
	aggregator := NewAggregator()
	files := []models.FileAnalysis{
		{
			Path:       "pkg/analyzer/file.go",
			TotalLines: 100,
			CodeLines:  80,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "Func1",
					CyclomaticComplexity:   2,
					CognitiveComplexity:    1,
					MaintainabilityIndex:   80.0,
					Length:                 10,
					Churn: &models.ChurnMetric{
						TotalChanges: 10,
					},
				},
				{
					Name:                   "Func2",
					CyclomaticComplexity:   3,
					CognitiveComplexity:    2,
					MaintainabilityIndex:   75.0,
					Length:                 15,
					Churn: &models.ChurnMetric{
						TotalChanges: 20,
					},
				},
			},
		},
	}

	result := aggregator.AggregateByFolder(files)
	folder := result["pkg/analyzer"]
	assert.Equal(t, 30, folder.TotalChurn)
	assert.InDelta(t, 15.0, folder.AverageChurn, 0.01)
}

func TestCalculateScoresEmptyFolders(t *testing.T) {
	aggregator := NewAggregator()
	result := aggregator.CalculateScores(map[string]models.FolderMetrics{})
	assert.Empty(t, result)
}

func TestCalculateScoresSingleFolder(t *testing.T) {
	aggregator := NewAggregator()
	folders := map[string]models.FolderMetrics{
		"pkg/analyzer": {
			Path:                   "pkg/analyzer",
			AverageComplexity:      5.0,
			AverageChurn:           10.0,
			AverageLength:          20.0,
			AverageMaintainability: 75.0,
		},
	}

	result := aggregator.CalculateScores(folders)
	require.Len(t, result, 1)

	folder := result["pkg/analyzer"]
	// With single value, it should be 100 (100% of values are <= to it)
	assert.InDelta(t, 100.0, folder.ComplexityScore, 0.01)
	assert.InDelta(t, 100.0, folder.ChurnScore, 0.01)
	assert.InDelta(t, 100.0, folder.LengthScore, 0.01)
	// Maintainability is inverted
	assert.InDelta(t, 0.0, folder.MaintainabilityScore, 0.01)
}

func TestCalculateScoresMultipleFolders(t *testing.T) {
	aggregator := NewAggregator()
	folders := map[string]models.FolderMetrics{
		"pkg/low": {
			Path:                   "pkg/low",
			AverageComplexity:      2.0,
			AverageChurn:           5.0,
			AverageLength:          10.0,
			AverageMaintainability: 90.0,
		},
		"pkg/high": {
			Path:                   "pkg/high",
			AverageComplexity:      10.0,
			AverageChurn:           20.0,
			AverageLength:          50.0,
			AverageMaintainability: 50.0,
		},
	}

	result := aggregator.CalculateScores(folders)
	lowFolder := result["pkg/low"]
	highFolder := result["pkg/high"]

	// Low folder should have lower scores (except maintainability which is inverted)
	assert.Less(t, lowFolder.ComplexityScore, highFolder.ComplexityScore)
	assert.Less(t, lowFolder.ChurnScore, highFolder.ChurnScore)
	assert.Less(t, lowFolder.LengthScore, highFolder.LengthScore)
	// Maintainability inverted: high maintainability -> LOW score (because 100 - highPercentile = low)
	assert.Less(t, lowFolder.MaintainabilityScore, highFolder.MaintainabilityScore)
}

func TestCalculateScoresHotspotScore(t *testing.T) {
	aggregator := NewAggregator()
	folders := map[string]models.FolderMetrics{
		"pkg/test": {
			Path:                   "pkg/test",
			AverageComplexity:      5.0,
			AverageChurn:           10.0,
			AverageLength:          20.0,
			AverageMaintainability: 75.0,
		},
	}

	result := aggregator.CalculateScores(folders)
	folder := result["pkg/test"]
	expectedHotspotScore := (folder.ComplexityScore + folder.ChurnScore) / 2
	assert.InDelta(t, expectedHotspotScore, folder.HotspotScore, 0.01)
}

func TestPercentileRankEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		values   []float64
		expected float64
	}{
		{
			name:     "empty slice",
			value:    5.0,
			values:   []float64{},
			expected: 0.0,
		},
		{
			name:     "single value equal",
			value:    5.0,
			values:   []float64{5.0},
			expected: 100.0,
		},
		{
			name:     "single value less than",
			value:    3.0,
			values:   []float64{5.0},
			expected: 0.0,
		},
		{
			name:     "single value greater than",
			value:    7.0,
			values:   []float64{5.0},
			expected: 100.0,
		},
		{
			name:     "minimum value",
			value:    1.0,
			values:   []float64{1.0, 5.0, 10.0},
			expected: 33.33,
		},
		{
			name:     "maximum value",
			value:    10.0,
			values:   []float64{1.0, 5.0, 10.0},
			expected: 100.0,
		},
		{
			name:     "middle value",
			value:    5.0,
			values:   []float64{1.0, 5.0, 10.0},
			expected: 66.67,
		},
		{
			name:     "NaN value",
			value:    math.NaN(),
			values:   []float64{1.0, 5.0, 10.0},
			expected: 0.0,
		},
		{
			name:     "Inf value",
			value:    math.Inf(1),
			values:   []float64{1.0, 5.0, 10.0},
			expected: 0.0,
		},
		{
			name:     "negative infinity",
			value:    math.Inf(-1),
			values:   []float64{1.0, 5.0, 10.0},
			expected: 0.0,
		},
		{
			name:     "duplicates",
			value:    5.0,
			values:   []float64{5.0, 5.0, 5.0},
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := percentileRank(tt.value, tt.values)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

func TestPercentileRankBoundaries(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	// All results should be between 0 and 100
	testCases := []float64{0.0, 1.0, 2.5, 5.0, 10.0}
	for _, val := range testCases {
		result := percentileRank(val, values)
		assert.GreaterOrEqual(t, result, 0.0, "percentile should be >= 0")
		assert.LessOrEqual(t, result, 100.0, "percentile should be <= 100")
	}
}

func TestAggregateByFolderRootFiles(t *testing.T) {
	aggregator := NewAggregator()
	files := []models.FileAnalysis{
		{
			Path:       "main.go",
			TotalLines: 100,
			CodeLines:  80,
			Functions: []models.FunctionAnalysis{
				{
					Name:                   "main",
					CyclomaticComplexity:   1,
					CognitiveComplexity:    0,
					MaintainabilityIndex:   85.0,
					Length:                 5,
				},
			},
		},
	}

	result := aggregator.AggregateByFolder(files)
	require.Len(t, result, 1)

	folder := result["."]
	assert.Equal(t, ".", folder.Path)
	assert.Equal(t, 1, folder.TotalFiles)
}

func TestCalculateScoresRanking(t *testing.T) {
	aggregator := NewAggregator()
	folders := map[string]models.FolderMetrics{
		"pkg/a": {
			Path:                   "pkg/a",
			AverageComplexity:      1.0,
			AverageChurn:           1.0,
			AverageLength:          1.0,
			AverageMaintainability: 90.0,
		},
		"pkg/b": {
			Path:                   "pkg/b",
			AverageComplexity:      5.0,
			AverageChurn:           5.0,
			AverageLength:          5.0,
			AverageMaintainability: 50.0,
		},
		"pkg/c": {
			Path:                   "pkg/c",
			AverageComplexity:      10.0,
			AverageChurn:           10.0,
			AverageLength:          10.0,
			AverageMaintainability: 30.0,
		},
	}

	result := aggregator.CalculateScores(folders)

	// Should rank in order: a < b < c (for complexity/churn/length)
	aComplexity := result["pkg/a"].ComplexityScore
	bComplexity := result["pkg/b"].ComplexityScore
	cComplexity := result["pkg/c"].ComplexityScore

	assert.Less(t, aComplexity, bComplexity)
	assert.Less(t, bComplexity, cComplexity)
}

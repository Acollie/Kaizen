package ownership

import (
	"testing"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestNewAggregator(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{},
	}

	agg := NewAggregator(codeowners)

	assert.NotNil(t, agg)
	assert.Equal(t, codeowners, agg.codeowners)
}

func TestAggregateByOwnerEmpty(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "*",
				Owners:  []string{"@default-team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{},
	}

	ownerMetrics, fileOwnership := agg.AggregateByOwner(result)

	assert.Empty(t, ownerMetrics)
	assert.Empty(t, fileOwnership)
}

func TestAggregateByOwnerSingleFile(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "*",
				Owners:  []string{"@default-team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path:      "main.go",
				CodeLines: 100,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "main",
						CyclomaticComplexity:  5,
						CognitiveComplexity:   5,
						MaintainabilityIndex:  80.0,
						IsHotspot:             false,
					},
				},
			},
		},
	}

	ownerMetrics, fileOwnership := agg.AggregateByOwner(result)

	assert.Len(t, ownerMetrics, 1)
	assert.Contains(t, ownerMetrics, "@default-team")

	metrics := ownerMetrics["@default-team"]
	assert.Equal(t, 1, metrics.FileCount)
	assert.Equal(t, 1, metrics.FunctionCount)
	assert.Equal(t, 100, metrics.TotalLines)
	assert.Equal(t, 5.0, metrics.AvgCyclomaticComplexity)
	assert.Equal(t, 80.0, metrics.AvgMaintainabilityIndex)
	assert.Equal(t, 0, metrics.HotspotCount)

	assert.Len(t, fileOwnership, 1)
	assert.Contains(t, fileOwnership, "main.go")
	assert.Equal(t, []string{"@default-team"}, fileOwnership["main.go"])
}

func TestAggregateByOwnerMultipleOwners(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "pkg/api/",
				Owners:  []string{"@api-team", "@shared-team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path:      "pkg/api/api.go",
				CodeLines: 200,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "HandleRequest",
						CyclomaticComplexity:  8,
						CognitiveComplexity:   8,
						MaintainabilityIndex:  75.0,
						IsHotspot:             false,
					},
				},
			},
		},
	}

	ownerMetrics, fileOwnership := agg.AggregateByOwner(result)

	// Both teams should have metrics
	assert.Len(t, ownerMetrics, 2)
	assert.Contains(t, ownerMetrics, "@api-team")
	assert.Contains(t, ownerMetrics, "@shared-team")

	// Each team should have the same file assigned
	for _, metrics := range ownerMetrics {
		assert.Equal(t, 1, metrics.FileCount)
		assert.Equal(t, 200, metrics.TotalLines)
	}

	// File should have both owners
	assert.Equal(t, []string{"@api-team", "@shared-team"}, fileOwnership["pkg/api/api.go"])
}

func TestAggregateByOwnerComplexityMetrics(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "*",
				Owners:  []string{"@team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path:      "functions.go",
				CodeLines: 300,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "SimpleFunc",
						CyclomaticComplexity:  2,
						CognitiveComplexity:   2,
						MaintainabilityIndex:  90.0,
						IsHotspot:             false,
					},
					{
						Name:                  "ComplexFunc",
						CyclomaticComplexity:  12,
						CognitiveComplexity:   15,
						MaintainabilityIndex:  50.0,
						IsHotspot:             true,
					},
					{
						Name:                  "NestedFunc",
						CyclomaticComplexity:  18,
						CognitiveComplexity:   25,
						MaintainabilityIndex:  35.0,
						IsHotspot:             true,
					},
				},
			},
		},
	}

	ownerMetrics, _ := agg.AggregateByOwner(result)

	metrics := ownerMetrics["@team"]
	assert.Equal(t, 3, metrics.FunctionCount)

	// Check averages
	expectedAvgCyclomaticComplexity := (2.0 + 12.0 + 18.0) / 3.0
	assert.Equal(t, expectedAvgCyclomaticComplexity, metrics.AvgCyclomaticComplexity)

	// Check hotspot count
	assert.Equal(t, 2, metrics.HotspotCount)

	// Check high complexity count (> 10)
	assert.Equal(t, 2, metrics.HighComplexityFunctionCount)
}

func TestCalculateOwnerHealthScore(t *testing.T) {
	tests := []struct {
		name          string
		metrics       *OwnerMetrics
		minScore      float64
		maxScore      float64
		shouldBeNone  bool
	}{
		{
			name: "no functions",
			metrics: &OwnerMetrics{
				FunctionCount: 0,
			},
			minScore:     100.0,
			maxScore:     100.0,
			shouldBeNone: false,
		},
		{
			name: "simple, maintainable code",
			metrics: &OwnerMetrics{
				FunctionCount:              1,
				AvgCyclomaticComplexity:    3.0,
				AvgCognitiveComplexity:     3.0,
				AvgMaintainabilityIndex:    90.0,
				HotspotCount:               0,
				HighComplexityFunctionCount: 0,
			},
			minScore:     60.0,
			maxScore:     100.0,
			shouldBeNone: false,
		},
		{
			name: "complex, problematic code",
			metrics: &OwnerMetrics{
				FunctionCount:              5,
				AvgCyclomaticComplexity:    15.0,
				AvgCognitiveComplexity:     20.0,
				AvgMaintainabilityIndex:    30.0,
				HotspotCount:               3,
				HighComplexityFunctionCount: 4,
			},
			minScore:     0.0,
			maxScore:     60.0,
			shouldBeNone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateOwnerHealthScore(tt.metrics)

			if tt.shouldBeNone {
				assert.Zero(t, score)
			} else {
				assert.GreaterOrEqual(t, score, tt.minScore)
				assert.LessOrEqual(t, score, tt.maxScore)
			}
		})
	}
}

func TestGetOwnerReport(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "pkg/storage/",
				Owners:  []string{"@storage-team"},
			},
			{
				Pattern: "pkg/api/",
				Owners:  []string{"@api-team"},
			},
			{
				Pattern: "*",
				Owners:  []string{"@default-team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path:      "pkg/storage/sqlite.go",
				CodeLines: 500,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "Query",
						CyclomaticComplexity:  4,
						CognitiveComplexity:   4,
						MaintainabilityIndex:  85.0,
						IsHotspot:             false,
					},
				},
			},
			{
				Path:      "main.go",
				CodeLines: 50,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "main",
						CyclomaticComplexity:  2,
						CognitiveComplexity:   2,
						MaintainabilityIndex:  95.0,
						IsHotspot:             false,
					},
				},
			},
		},
	}

	report := agg.GetOwnerReport(result, 1, "2024-01-15")

	assert.NotNil(t, report)
	assert.Equal(t, int64(1), report.SnapshotID)
	assert.Equal(t, "2024-01-15", report.AnalyzedAt)
	assert.GreaterOrEqual(t, report.TotalOwners, 1)
	assert.NotEmpty(t, report.OwnerMetrics)

	// Check that owners are sorted by health score descending
	for i := 0; i < len(report.OwnerMetrics)-1; i++ {
		assert.GreaterOrEqual(t,
			report.OwnerMetrics[i].OverallHealthScore,
			report.OwnerMetrics[i+1].OverallHealthScore)
	}
}

func TestAggregateByOwnerNoMatchingPattern(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "nonexistent/",
				Owners:  []string{"@team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path:      "main.go",
				CodeLines: 100,
				Functions: []models.FunctionAnalysis{},
			},
		},
	}

	ownerMetrics, _ := agg.AggregateByOwner(result)

	// File doesn't match any pattern, so no owner metrics
	assert.Empty(t, ownerMetrics)
}

func TestAggregateByOwnerFileLines(t *testing.T) {
	codeowners := &CodeOwners{
		Rules: []OwnershipRule{
			{
				Pattern: "*",
				Owners:  []string{"@team"},
			},
		},
	}
	agg := NewAggregator(codeowners)

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path:      "file1.go",
				CodeLines: 100,
				Functions: []models.FunctionAnalysis{},
			},
			{
				Path:      "file2.go",
				CodeLines: 200,
				Functions: []models.FunctionAnalysis{},
			},
		},
	}

	ownerMetrics, _ := agg.AggregateByOwner(result)

	metrics := ownerMetrics["@team"]
	assert.Equal(t, 2, metrics.FileCount)
	assert.Equal(t, 300, metrics.TotalLines)
}

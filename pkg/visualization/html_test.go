package visualization

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTMLVisualizer(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	assert.NotNil(t, visualizer)
}

func TestGenerateHTMLEmpty(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "d3")
}

func TestGenerateHTMLWithData(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles: 1,
		},
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
						Length:                50,
					},
				},
			},
		},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "<!DOCTYPE html>")
	// Should contain data embedded in the template
	assert.Contains(t, html, "main")
}

func TestGenerateHTMLWithScoreReport(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles: 1,
		},
		Files: []models.FileAnalysis{},
		ScoreReport: &models.ScoreReport{
			OverallScore: 85.0,
			OverallGrade: "B",
		},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	// Should include score report data
	assert.Contains(t, html, "85")
}

func TestGenerateHTMLContainsD3(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	// D3 library should be included
	assert.Contains(t, html, "d3")
	assert.Contains(t, html, "<script")
}

func TestGenerateHTMLContainsTreemap(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	// Should contain treemap-related content
	assert.Contains(t, html, "treemap")
}

func TestGenerateHTMLIsValidHTML(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles: 1,
		},
		Files: []models.FileAnalysis{
			{
				Path:      "main.go",
				CodeLines: 100,
				Functions: []models.FunctionAnalysis{},
			},
		},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)

	// Basic HTML validation
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "<html")
	assert.Contains(t, html, "<head")
	assert.Contains(t, html, "<body")
	assert.Contains(t, html, "</html>")
}

func TestGenerateHTMLMultipleFiles(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles: 2,
		},
		Files: []models.FileAnalysis{
			{
				Path:      "pkg/api/api.go",
				CodeLines: 150,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "Handler",
						CyclomaticComplexity:  8,
						CognitiveComplexity:   10,
						MaintainabilityIndex:  70.0,
						Length:                75,
					},
				},
			},
			{
				Path:      "pkg/storage/db.go",
				CodeLines: 200,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "Query",
						CyclomaticComplexity:  5,
						CognitiveComplexity:   5,
						MaintainabilityIndex:  85.0,
						Length:                100,
					},
				},
			},
		},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
}

func TestTreeNodeJSON(t *testing.T) {
	node := TreeNode{
		Name:  "pkg",
		Value: 1000,
		Metrics: TreeMetrics{
			ComplexityScore:      75.0,
			MaintainabilityScore: 80.0,
		},
	}

	data, err := json.Marshal(node)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify structure
	var unmarshaled TreeNode
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "pkg", unmarshaled.Name)
	assert.Equal(t, 1000, unmarshaled.Value)
}

func TestGenerateHTMLWithNilScoreReport(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles: 1,
		},
		Files: []models.FileAnalysis{
			{
				Path:      "main.go",
				CodeLines: 100,
				Functions: []models.FunctionAnalysis{},
			},
		},
		ScoreReport: nil,
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
}

func TestGenerateHTMLContainsNordicTheme(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	// Should reference Nordic theme elements (warm colors, etc.)
	assert.True(t,
		strings.Contains(html, "nordic") ||
			strings.Contains(html, "warm") ||
			strings.Contains(html, "color"),
		"HTML should contain theme-related content")
}

func TestGenerateHTMLMetricsPresent(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles:              1,
			TotalFunctions:          5,
			AverageCyclomaticComplexity: 6.5,
			AverageMaintainabilityIndex: 78.0,
		},
		Files: []models.FileAnalysis{},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	// Should include metrics data
	assert.Contains(t, html, "Complexity")
}

func TestGenerateHTMLRepositoryInfo(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Repository: "github.com/example/project",
		Files:      []models.FileAnalysis{},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
}

func TestHTMLVisualizerWithComplexStructure(t *testing.T) {
	visualizer := NewHTMLVisualizer()

	result := &models.AnalysisResult{
		Summary: models.SummaryMetrics{
			TotalFiles:              3,
			TotalFunctions:          15,
			AverageCyclomaticComplexity: 7.2,
			AverageMaintainabilityIndex: 75.5,
		},
		Files: []models.FileAnalysis{
			{
				Path:      "cmd/main.go",
				CodeLines: 50,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "main",
						CyclomaticComplexity:  2,
						CognitiveComplexity:   2,
						MaintainabilityIndex:  95.0,
						Length:                30,
					},
				},
			},
			{
				Path:      "pkg/api/handler.go",
				CodeLines: 200,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "HandleRequest",
						CyclomaticComplexity:  12,
						CognitiveComplexity:   15,
						MaintainabilityIndex:  60.0,
						Length:                120,
					},
					{
						Name:                  "ValidateInput",
						CyclomaticComplexity:  8,
						CognitiveComplexity:   10,
						MaintainabilityIndex:  70.0,
						Length:                80,
					},
				},
			},
			{
				Path:      "pkg/storage/store.go",
				CodeLines: 300,
				Functions: []models.FunctionAnalysis{
					{
						Name:                  "Query",
						CyclomaticComplexity:  6,
						CognitiveComplexity:   6,
						MaintainabilityIndex:  80.0,
						Length:                150,
					},
					{
						Name:                  "Save",
						CyclomaticComplexity:  5,
						CognitiveComplexity:   5,
						MaintainabilityIndex:  85.0,
						Length:                100,
					},
				},
			},
		},
		ScoreReport: &models.ScoreReport{
			OverallScore: 82.0,
			OverallGrade: "B",
		},
	}

	html, err := visualizer.GenerateHTML(result)

	require.NoError(t, err)
	assert.NotEmpty(t, html)
	assert.Greater(t, len(html), 5000)
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

func TestFormatDiffMarkdown_BasicOutput(t *testing.T) {
	baseResult := createTestAnalysisResult(84.3, "B", 4.2, 87.1, 2, 350, 50)
	headResult := createTestAnalysisResult(82.0, "B", 4.8, 85.4, 3, 358, 52)

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, nil, "")

	assertContains(t, markdown, "## Kaizen Code Analysis")
	assertContains(t, markdown, "Grade B")
	assertContains(t, markdown, "82/100")
	assertContains(t, markdown, "### Metrics")
	assertContains(t, markdown, "Overall Score")
	assertContains(t, markdown, "Avg Complexity")
	assertContains(t, markdown, "Avg Maintainability")
	assertContains(t, markdown, "<!-- kaizen-pr-analysis -->")
}

func TestFormatDiffMarkdown_ScoreDelta(t *testing.T) {
	baseResult := createTestAnalysisResult(84.3, "B", 4.2, 87.1, 2, 350, 50)
	headResult := createTestAnalysisResult(82.0, "B", 4.8, 85.4, 3, 358, 52)

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, nil, "")

	assertContains(t, markdown, "-2.3")
}

func TestFormatDiffMarkdown_WithHotspotChanges(t *testing.T) {
	baseResult := createTestAnalysisResultWithHotspots(80.0, "B",
		[]hotspotEntry{{file: "pkg/a.go", function: "oldHotspot"}})
	headResult := createTestAnalysisResultWithHotspots(78.0, "C",
		[]hotspotEntry{{file: "pkg/b.go", function: "newHotspot"}})

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, nil, "")

	assertContains(t, markdown, "### Hotspot Changes")
	assertContains(t, markdown, ":x: New")
	assertContains(t, markdown, "pkg/b.go:newHotspot")
	assertContains(t, markdown, ":white_check_mark: Fixed")
	assertContains(t, markdown, "pkg/a.go:oldHotspot")
}

func TestFormatDiffMarkdown_WithBlastRadiusConcerns(t *testing.T) {
	baseResult := createTestAnalysisResult(80.0, "B", 4.0, 85.0, 2, 100, 10)
	headResult := createTestAnalysisResult(78.0, "C", 5.0, 83.0, 3, 105, 11)

	concerns := []models.Concern{
		{
			Type:     "blast_radius",
			Severity: "warning",
			Title:    "High fan-in function modified",
			AffectedItems: []models.AffectedItem{
				{
					FilePath:     "cmd/kaizen/diff.go",
					FunctionName: "CompareAnalyses",
					Metrics:      map[string]float64{"fan_in": 12},
				},
			},
		},
	}

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, concerns, "")

	assertContains(t, markdown, "### Blast-Radius Warnings")
	assertContains(t, markdown, "CompareAnalyses")
	assertContains(t, markdown, "cmd/kaizen/diff.go")
	assertContains(t, markdown, "12")
	assertContains(t, markdown, "warning")
}

func TestFormatDiffMarkdown_NoConcernsOmitsSection(t *testing.T) {
	baseResult := createTestAnalysisResult(80.0, "B", 4.0, 85.0, 0, 100, 10)
	headResult := createTestAnalysisResult(80.0, "B", 4.0, 85.0, 0, 100, 10)

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, nil, "")

	if strings.Contains(markdown, "### Blast-Radius Warnings") {
		t.Error("should not contain blast-radius section when no concerns")
	}
	if strings.Contains(markdown, "### Hotspot Changes") {
		t.Error("should not contain hotspot changes when no changes")
	}
}

func TestFormatDiffMarkdown_ContainsExplainer(t *testing.T) {
	baseResult := createTestAnalysisResult(80.0, "B", 4.0, 85.0, 0, 100, 10)
	headResult := createTestAnalysisResult(80.0, "B", 4.0, 85.0, 0, 100, 10)

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, nil, "")

	assertContains(t, markdown, "<details>")
	assertContains(t, markdown, "What do these metrics mean?")
}

func TestLoadAnalysisFromFile(t *testing.T) {
	result := createTestAnalysisResult(85.0, "B", 3.5, 88.0, 1, 200, 30)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "analysis.json")

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	loaded, err := loadAnalysisFromFile(filePath)
	if err != nil {
		t.Fatalf("loadAnalysisFromFile failed: %v", err)
	}

	if loaded.ScoreReport.OverallScore != 85.0 {
		t.Errorf("expected score 85.0, got %.1f", loaded.ScoreReport.OverallScore)
	}
}

func TestLoadAnalysisFromFile_NotFound(t *testing.T) {
	_, err := loadAnalysisFromFile("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConcernsFromFile(t *testing.T) {
	concerns := []models.Concern{
		{
			Type:     "blast_radius",
			Severity: "warning",
			Title:    "Test concern",
			AffectedItems: []models.AffectedItem{
				{
					FilePath:     "test.go",
					FunctionName: "TestFunc",
					Metrics:      map[string]float64{"fan_in": 5},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "concerns.json")

	data, err := json.MarshalIndent(concerns, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	loaded, err := loadConcernsFromFile(filePath)
	if err != nil {
		t.Fatalf("loadConcernsFromFile failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("expected 1 concern, got %d", len(loaded))
	}
	if loaded[0].Title != "Test concern" {
		t.Errorf("expected title 'Test concern', got '%s'", loaded[0].Title)
	}
}

func TestFormatDiffMarkdown_WithCallGraph(t *testing.T) {
	baseResult := createTestAnalysisResult(80.0, "B", 4.0, 85.0, 0, 100, 10)
	headResult := createTestAnalysisResult(78.0, "C", 5.0, 83.0, 0, 105, 11)

	svgContent := `<svg width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`

	diff := CompareAnalyses(baseResult, headResult)
	markdown := FormatDiffMarkdown(diff, headResult, nil, svgContent)

	assertContains(t, markdown, "### Call Graph of Changed Functions")
	assertContains(t, markdown, "<details>")
	assertContains(t, markdown, svgContent)
}

func TestDeltaArrow(t *testing.T) {
	tests := []struct {
		delta    float64
		expected string
	}{
		{1.5, "\u2b06\ufe0f"},
		{-1.5, "\u2b07\ufe0f"},
		{0.0, "\u27a1\ufe0f"},
	}

	for _, test := range tests {
		result := deltaArrow(test.delta)
		if result != test.expected {
			t.Errorf("deltaArrow(%.1f) = %s, want %s", test.delta, result, test.expected)
		}
	}
}

// --- Test helpers ---

type hotspotEntry struct {
	file     string
	function string
}

func createTestAnalysisResult(score float64, grade string, avgComplexity, avgMaint float64, hotspots, functions, files int) *models.AnalysisResult {
	return &models.AnalysisResult{
		AnalyzedAt: time.Now(),
		Summary: models.SummaryMetrics{
			TotalFiles:                    files,
			TotalFunctions:                functions,
			AverageCyclomaticComplexity:   avgComplexity,
			AverageMaintainabilityIndex:   avgMaint,
			HotspotCount:                  hotspots,
		},
		ScoreReport: &models.ScoreReport{
			OverallScore: score,
			OverallGrade: grade,
		},
	}
}

func createTestAnalysisResultWithHotspots(score float64, grade string, hotspots []hotspotEntry) *models.AnalysisResult {
	result := createTestAnalysisResult(score, grade, 5.0, 80.0, len(hotspots), 100, 20)

	for _, entry := range hotspots {
		result.Files = append(result.Files, models.FileAnalysis{
			Path: entry.file,
			Functions: []models.FunctionAnalysis{
				{
					Name:      entry.function,
					IsHotspot: true,
				},
			},
		})
	}

	return result
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", needle, haystack)
	}
}

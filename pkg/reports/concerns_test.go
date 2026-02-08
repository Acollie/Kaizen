package reports

import (
	"strings"
	"testing"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/models"
)

func TestDetectConcernsEmpty(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)
	if len(concerns) != 0 {
		t.Errorf("Empty result should have no concerns, got %d", len(concerns))
	}
}

func TestDetectConcernsNoConcerns(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "clean.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "cleanFunction",
						CyclomaticComplexity: 3,
						MaintainabilityIndex: 85,
						NestingDepth:         2,
						ParameterCount:       3,
						Length:               20,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)
	if len(concerns) != 0 {
		t.Errorf("Clean code should have no concerns, got %d: %+v", len(concerns), concerns)
	}
}

func TestDetectChurnComplexityHotspots(t *testing.T) {
	churnHigh := &models.ChurnMetric{TotalCommits: 15}

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "hotspot.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "hotFunction",
						StartLine:            10,
						CyclomaticComplexity: 15,
						Churn:                churnHigh,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, true, config.DefaultConfig().Thresholds)

	foundHotspot := false
	for _, concern := range concerns {
		if concern.Type == "churn_complexity_hotspot" {
			foundHotspot = true
			if concern.Severity != "critical" {
				t.Errorf("Hotspot should be critical severity, got %v", concern.Severity)
			}
			if len(concern.AffectedItems) == 0 {
				t.Error("Hotspot concern should have affected items")
			}
		}
	}

	if !foundHotspot {
		t.Error("Should detect churn complexity hotspot")
	}
}

func TestDetectHighChurnLongFunctions(t *testing.T) {
	churnVeryHigh := &models.ChurnMetric{TotalCommits: 25}

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "long.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:      "veryLongFunction",
						StartLine: 10,
						Length:    150,
						Churn:     churnVeryHigh,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, true, config.DefaultConfig().Thresholds)

	foundLongChurn := false
	for _, concern := range concerns {
		if concern.Type == "high_churn_long_function" && concern.Severity == "critical" {
			foundLongChurn = true
		}
	}

	if !foundLongChurn {
		t.Error("Should detect critical high churn long function")
	}
}

func TestDetectLowMaintainabilityCritical(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "unmaintainable.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "terribleFunction",
						StartLine:            10,
						MaintainabilityIndex: 15,
						CyclomaticComplexity: 20,
						Length:               200,
						HalsteadVolume:       5000,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundMaintain := false
	for _, concern := range concerns {
		if concern.Type == "low_maintainability" && concern.Severity == "critical" {
			foundMaintain = true
			// Check that the description explains the issue
			if !strings.Contains(concern.Description, "long functions") &&
				!strings.Contains(concern.Description, "high complexity") {
				t.Error("Critical maintainability concern should explain contributing factors")
			}
		}
	}

	if !foundMaintain {
		t.Error("Should detect critical low maintainability")
	}
}

func TestDetectLowMaintainabilityWarning(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "poor.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "poorFunction",
						StartLine:            10,
						MaintainabilityIndex: 35,
						CyclomaticComplexity: 12,
						Length:               80,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundMaintain := false
	for _, concern := range concerns {
		if concern.Type == "low_maintainability" && concern.Severity == "warning" {
			foundMaintain = true
		}
	}

	if !foundMaintain {
		t.Error("Should detect warning low maintainability")
	}
}

func TestDetectDeepNestingWarning(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "nested.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:         "deeplyNested",
						StartLine:    10,
						NestingDepth: 9,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundNesting := false
	for _, concern := range concerns {
		if concern.Type == "deep_nesting" && concern.Severity == "warning" {
			foundNesting = true
		}
	}

	if !foundNesting {
		t.Error("Should detect warning deep nesting")
	}
}

func TestDetectDeepNestingInfo(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "nested.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:         "moderatelyNested",
						StartLine:    10,
						NestingDepth: 6,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundNesting := false
	for _, concern := range concerns {
		if concern.Type == "deep_nesting" && concern.Severity == "info" {
			foundNesting = true
		}
	}

	if !foundNesting {
		t.Error("Should detect info deep nesting")
	}
}

func TestDetectTooManyParametersWarning(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "params.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:           "tooManyParams",
						StartLine:      10,
						ParameterCount: 12,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundParams := false
	for _, concern := range concerns {
		if concern.Type == "too_many_parameters" && concern.Severity == "warning" {
			foundParams = true
		}
	}

	if !foundParams {
		t.Error("Should detect warning too many parameters")
	}
}

func TestDetectTooManyParametersInfo(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "params.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:           "manyParams",
						StartLine:      10,
						ParameterCount: 8,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundParams := false
	for _, concern := range concerns {
		if concern.Type == "too_many_parameters" && concern.Severity == "info" {
			foundParams = true
		}
	}

	if !foundParams {
		t.Error("Should detect info many parameters")
	}
}

func TestDetectGodFunctions(t *testing.T) {
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "god.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:           "godFunction",
						StartLine:      10,
						ParameterCount: 8,
						FanIn:          15,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	foundGod := false
	for _, concern := range concerns {
		if concern.Type == "god_function" {
			foundGod = true
			if concern.Severity != "warning" {
				t.Errorf("God function should be warning severity, got %v", concern.Severity)
			}
		}
	}

	if !foundGod {
		t.Error("Should detect god function")
	}
}

func TestConcernsSortedBySeverity(t *testing.T) {
	churnHigh := &models.ChurnMetric{TotalCommits: 15}

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "mixed.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "hotspot",
						CyclomaticComplexity: 15,
						Churn:                churnHigh,
					},
					{
						Name:         "nested",
						NestingDepth: 6,
					},
					{
						Name:                 "poor",
						MaintainabilityIndex: 35,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, true, config.DefaultConfig().Thresholds)

	if len(concerns) == 0 {
		t.Skip("No concerns detected")
	}

	// Verify sorting: critical first, then warning, then info
	severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
	for i := 1; i < len(concerns); i++ {
		prevOrder := severityOrder[concerns[i-1].Severity]
		currOrder := severityOrder[concerns[i].Severity]
		if prevOrder > currOrder {
			t.Errorf("Concerns not sorted by severity: %v before %v",
				concerns[i-1].Severity, concerns[i].Severity)
		}
	}
}

func TestLimitAffectedItems(t *testing.T) {
	items := []models.AffectedItem{
		{FilePath: "a.go"},
		{FilePath: "b.go"},
		{FilePath: "c.go"},
		{FilePath: "d.go"},
		{FilePath: "e.go"},
		{FilePath: "f.go"},
		{FilePath: "g.go"},
	}

	limited := limitAffectedItems(items, 5)
	if len(limited) != 5 {
		t.Errorf("Should limit to 5 items, got %d", len(limited))
	}

	// Test with fewer items than limit
	small := []models.AffectedItem{
		{FilePath: "a.go"},
		{FilePath: "b.go"},
	}

	limited = limitAffectedItems(small, 5)
	if len(limited) != 2 {
		t.Errorf("Should not limit when fewer items, got %d", len(limited))
	}
}

func TestBuildMaintainabilityDescription(t *testing.T) {
	tests := []struct {
		name     string
		items    []models.AffectedItem
		contains []string
	}{
		{
			name: "long functions",
			items: []models.AffectedItem{
				{Metrics: map[string]float64{"length": 100, "cyclomatic_complexity": 5, "halstead_volume": 500}},
			},
			contains: []string{"long functions"},
		},
		{
			name: "high complexity",
			items: []models.AffectedItem{
				{Metrics: map[string]float64{"length": 30, "cyclomatic_complexity": 15, "halstead_volume": 500}},
			},
			contains: []string{"high complexity"},
		},
		{
			name: "dense code",
			items: []models.AffectedItem{
				{Metrics: map[string]float64{"length": 30, "cyclomatic_complexity": 5, "halstead_volume": 2000}},
			},
			contains: []string{"dense code"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			desc := buildMaintainabilityDescription(testCase.items, config.DefaultConfig().Thresholds.MaintainabilityIndex.Warning)
			for _, expected := range testCase.contains {
				if !strings.Contains(desc, expected) {
					t.Errorf("Description should contain '%s', got: %s", expected, desc)
				}
			}
		})
	}
}

func TestBuildHotspotDescription(t *testing.T) {
	items := []models.AffectedItem{
		{Metrics: map[string]float64{"complexity": 15, "churn": 20}},
		{Metrics: map[string]float64{"complexity": 12, "churn": 18}},
	}

	desc := buildHotspotDescription(items)

	if !strings.Contains(desc, "CC:") {
		t.Error("Hotspot description should mention complexity")
	}
	if !strings.Contains(desc, "commits") {
		t.Error("Hotspot description should mention commits")
	}
}

func TestBuildNestingDescription(t *testing.T) {
	items := []models.AffectedItem{
		{Metrics: map[string]float64{"nesting_depth": 8}},
	}

	warningDesc := buildNestingDescription(items, "warning")
	if !strings.Contains(warningDesc, "nesting") {
		t.Error("Warning nesting description should mention nesting")
	}

	infoDesc := buildNestingDescription(items, "info")
	if !strings.Contains(infoDesc, "early returns") {
		t.Error("Info nesting description should suggest early returns")
	}
}

func TestBuildParameterDescription(t *testing.T) {
	items := []models.AffectedItem{
		{Metrics: map[string]float64{"parameter_count": 12}},
	}

	warningDesc := buildParameterDescription(items, "warning")
	if !strings.Contains(warningDesc, "parameters") {
		t.Error("Warning parameter description should mention parameters")
	}

	infoDesc := buildParameterDescription(items, "info")
	if !strings.Contains(infoDesc, "config struct") || !strings.Contains(infoDesc, "options pattern") {
		t.Error("Info parameter description should suggest alternatives")
	}
}

func TestNoChurnConcernsWithoutChurnData(t *testing.T) {
	// Even with high complexity and length, churn-related concerns shouldn't appear
	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "nochurn.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "complexLong",
						CyclomaticComplexity: 20,
						Length:               150,
					},
				},
			},
		},
	}

	concerns := DetectConcerns(result, false, config.DefaultConfig().Thresholds)

	for _, concern := range concerns {
		if concern.Type == "churn_complexity_hotspot" || concern.Type == "high_churn_long_function" {
			t.Errorf("Should not detect churn-related concerns without churn data: %v", concern.Type)
		}
	}
}

func TestDetectConcernsWithCustomThresholds(t *testing.T) {
	customThresholds := config.DefaultConfig().Thresholds
	customThresholds.Hotspot.MinComplexity = 5
	customThresholds.Hotspot.MinChurn = 5

	churn := &models.ChurnMetric{TotalCommits: 7}

	result := &models.AnalysisResult{
		Files: []models.FileAnalysis{
			{
				Path: "test.go",
				Functions: []models.FunctionAnalysis{
					{
						Name:                 "testFunc",
						CyclomaticComplexity: 7, // Between 5 and default 10
						Churn:                churn,
					},
				},
			},
		},
	}

	// With default thresholds (min_complexity=10, min_churn=10), no hotspot
	defaultConcerns := DetectConcerns(result, true, config.DefaultConfig().Thresholds)
	foundDefaultHotspot := false
	for _, concern := range defaultConcerns {
		if concern.Type == "churn_complexity_hotspot" {
			foundDefaultHotspot = true
		}
	}
	if foundDefaultHotspot {
		t.Error("Should NOT detect hotspot with default thresholds")
	}

	// With custom lower thresholds, should detect hotspot
	customConcerns := DetectConcerns(result, true, customThresholds)
	foundCustomHotspot := false
	for _, concern := range customConcerns {
		if concern.Type == "churn_complexity_hotspot" {
			foundCustomHotspot = true
		}
	}
	if !foundCustomHotspot {
		t.Error("Should detect hotspot with custom lower thresholds")
	}
}

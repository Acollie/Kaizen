package check

import (
	"testing"

	"github.com/alexcollie/kaizen/pkg/models"
)

func TestDetectBlastRadiusNoConcerns(t *testing.T) {
	results := []FanInResult{
		{
			Function: ChangedFunction{Name: "func1"},
			FanIn:    2,
		},
		{
			Function: ChangedFunction{Name: "func2"},
			FanIn:    3,
		},
	}

	concerns := DetectBlastRadius(results)
	if len(concerns) != 0 {
		t.Fatalf("expected 0 concerns, got %d", len(concerns))
	}
}

func TestDetectBlastRadiusWarningTier(t *testing.T) {
	results := []FanInResult{
		{
			Function: ChangedFunction{Name: "func1", FilePath: "file1.go", StartLine: 10},
			FanIn:    6,
		},
		{
			Function: ChangedFunction{Name: "func2", FilePath: "file2.go", StartLine: 20},
			FanIn:    8,
		},
	}

	concerns := DetectBlastRadius(results)
	if len(concerns) != 1 {
		t.Fatalf("expected 1 concern, got %d", len(concerns))
	}
	if concerns[0].Severity != "warning" {
		t.Fatalf("expected warning severity, got %s", concerns[0].Severity)
	}
	if concerns[0].Type != "blast_radius" {
		t.Fatalf("expected blast_radius type, got %s", concerns[0].Type)
	}
	if len(concerns[0].AffectedItems) != 2 {
		t.Fatalf("expected 2 affected items, got %d", len(concerns[0].AffectedItems))
	}
}

func TestDetectBlastRadiusCriticalTier(t *testing.T) {
	results := []FanInResult{
		{
			Function: ChangedFunction{Name: "func1", FilePath: "file1.go", StartLine: 10},
			FanIn:    20,
		},
	}

	concerns := DetectBlastRadius(results)
	if len(concerns) != 1 {
		t.Fatalf("expected 1 concern, got %d", len(concerns))
	}
	if concerns[0].Severity != "critical" {
		t.Fatalf("expected critical severity, got %s", concerns[0].Severity)
	}
}

func TestDetectBlastRadiusBothTiers(t *testing.T) {
	results := []FanInResult{
		{
			Function: ChangedFunction{Name: "critical", FilePath: "file1.go", StartLine: 10},
			FanIn:    20,
		},
		{
			Function: ChangedFunction{Name: "warning", FilePath: "file2.go", StartLine: 20},
			FanIn:    7,
		},
	}

	concerns := DetectBlastRadius(results)
	if len(concerns) != 2 {
		t.Fatalf("expected 2 concerns, got %d", len(concerns))
	}
	// Critical should come first
	if concerns[0].Severity != "critical" {
		t.Fatalf("expected critical first, got %s", concerns[0].Severity)
	}
	if concerns[1].Severity != "warning" {
		t.Fatalf("expected warning second, got %s", concerns[1].Severity)
	}
}

func TestDetectBlastRadiusSorting(t *testing.T) {
	results := []FanInResult{
		{
			Function: ChangedFunction{Name: "func3", FilePath: "file3.go", StartLine: 30},
			FanIn:    8,
		},
		{
			Function: ChangedFunction{Name: "func1", FilePath: "file1.go", StartLine: 10},
			FanIn:    20,
		},
		{
			Function: ChangedFunction{Name: "func2", FilePath: "file2.go", StartLine: 20},
			FanIn:    25,
		},
	}

	concerns := DetectBlastRadius(results)
	// Should have 1 critical and 1 warning concern
	if len(concerns) != 2 {
		t.Fatalf("expected 2 concerns, got %d", len(concerns))
	}

	// Critical concern should have items sorted by fan-in descending
	if concerns[0].Severity == "critical" {
		items := concerns[0].AffectedItems
		if len(items) > 0 {
			if items[0].Metrics["fan_in"] != 25 {
				t.Fatalf("expected highest fan-in first in critical, got %v", items[0].Metrics["fan_in"])
			}
		}
	}
}

func TestDetectBlastRadiusApproximateFlag(t *testing.T) {
	results := []FanInResult{
		{
			Function:    ChangedFunction{Name: "func1", FilePath: "file1.py", StartLine: 10},
			FanIn:       10,
			Approximate: true,
		},
	}

	concerns := DetectBlastRadius(results)
	if len(concerns) != 1 {
		t.Fatalf("expected 1 concern, got %d", len(concerns))
	}

	// Description should mention approximate
	if len(concerns[0].AffectedItems) > 0 {
		item := concerns[0].AffectedItems[0]
		if item.Metrics["approximate"] != 1.0 {
			t.Fatalf("expected approximate flag 1.0, got %v", item.Metrics["approximate"])
		}
	}
}

func TestDetectBlastRadiusMaxItems(t *testing.T) {
	var results []FanInResult
	for i := 1; i <= 10; i++ {
		results = append(results, FanInResult{
			Function: ChangedFunction{
				Name:      "func" + string(rune('0'+i)),
				FilePath:  "file.go",
				StartLine: i * 10,
			},
			FanIn: 20, // All critical tier
		})
	}

	concerns := DetectBlastRadius(results)
	if len(concerns) != 1 {
		t.Fatalf("expected 1 concern, got %d", len(concerns))
	}

	// Should be capped at maxBlastRadiusItems (5)
	if len(concerns[0].AffectedItems) != maxBlastRadiusItems {
		t.Fatalf("expected %d items (capped), got %d", maxBlastRadiusItems, len(concerns[0].AffectedItems))
	}
}

func TestBuildBlastRadiusDescriptionCritical(t *testing.T) {
	items := []models.AffectedItem{
		{
			FunctionName: "criticalFunc",
			Line:         42,
			Metrics: map[string]float64{
				"fan_in": 20,
			},
		},
	}

	description := buildBlastRadiusDescription(items, false, "critical")
	if !contains(description, "critical") {
		t.Fatalf("expected 'critical' in description, got: %s", description)
	}
	if !contains(description, "criticalFunc") {
		t.Fatalf("expected function name in description, got: %s", description)
	}
}

func TestBuildBlastRadiusDescriptionWithApproximate(t *testing.T) {
	items := []models.AffectedItem{
		{
			FunctionName: "func",
			Line:         10,
			Metrics: map[string]float64{
				"fan_in": 10,
			},
		},
	}

	description := buildBlastRadiusDescription(items, true, "warning")
	if !contains(description, "approximate") {
		t.Fatalf("expected 'approximate' in description, got: %s", description)
	}
	if !contains(description, "grep-based") {
		t.Fatalf("expected 'grep-based' in description, got: %s", description)
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

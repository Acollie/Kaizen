package reports

import (
	"testing"
)

func TestCalculateGrade(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"perfect score", 100.0, "A"},
		{"A grade lower bound", 90.0, "A"},
		{"B grade upper bound", 89.9, "B"},
		{"B grade lower bound", 75.0, "B"},
		{"C grade upper bound", 74.9, "C"},
		{"C grade lower bound", 60.0, "C"},
		{"D grade upper bound", 59.9, "D"},
		{"D grade lower bound", 40.0, "D"},
		{"F grade upper bound", 39.9, "F"},
		{"zero score", 0.0, "F"},
		{"negative score", -10.0, "F"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := CalculateGrade(testCase.score)
			if result != testCase.expected {
				t.Errorf("CalculateGrade(%v) = %v, expected %v", testCase.score, result, testCase.expected)
			}
		})
	}
}

func TestGetCategoryLabel(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"excellent score", 95.0, "excellent"},
		{"excellent lower bound", 90.0, "excellent"},
		{"good upper bound", 89.9, "good"},
		{"good lower bound", 75.0, "good"},
		{"moderate upper bound", 74.9, "moderate"},
		{"moderate lower bound", 60.0, "moderate"},
		{"poor upper bound", 59.9, "poor"},
		{"poor lower bound", 40.0, "poor"},
		{"critical upper bound", 39.9, "critical"},
		{"zero score", 0.0, "critical"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := GetCategoryLabel(testCase.score)
			if result != testCase.expected {
				t.Errorf("GetCategoryLabel(%v) = %v, expected %v", testCase.score, result, testCase.expected)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		minVal   float64
		maxVal   float64
		expected float64
	}{
		{"value within range", 50.0, 0.0, 100.0, 50.0},
		{"value below min", -10.0, 0.0, 100.0, 0.0},
		{"value above max", 150.0, 0.0, 100.0, 100.0},
		{"value at min", 0.0, 0.0, 100.0, 0.0},
		{"value at max", 100.0, 0.0, 100.0, 100.0},
		{"negative range", -50.0, -100.0, -10.0, -50.0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := clamp(testCase.value, testCase.minVal, testCase.maxVal)
			if result != testCase.expected {
				t.Errorf("clamp(%v, %v, %v) = %v, expected %v",
					testCase.value, testCase.minVal, testCase.maxVal, result, testCase.expected)
			}
		})
	}
}

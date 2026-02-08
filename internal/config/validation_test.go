package config

import (
	"testing"
)

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedCount int
		shouldContain string
	}{
		{
			name: "valid configuration",
			config: &Config{
				Thresholds: ThresholdConfig{
					CyclomaticComplexity: 10,
					CognitiveComplexity:  15,
					FunctionLength:       100,
					NestingDepth:         4,
					ParameterCount:       5,
					MaintainabilityIndex: 20,
				},
				Analysis: AnalysisConfig{
					Languages:  []string{"go", "python"},
					MaxWorkers: 4,
				},
				Storage: StorageConfig{
					Backend: "sqlite",
				},
			},
			expectedCount: 0,
		},
		{
			name: "invalid cyclomatic complexity",
			config: &Config{
				Thresholds: ThresholdConfig{
					CyclomaticComplexity: 0,
					CognitiveComplexity:  15,
					FunctionLength:       100,
					NestingDepth:         4,
					ParameterCount:       5,
					MaintainabilityIndex: 20,
				},
			},
			expectedCount: 1,
			shouldContain: "cyclomatic complexity",
		},
		{
			name: "invalid function length",
			config: &Config{
				Thresholds: ThresholdConfig{
					CyclomaticComplexity: 10,
					CognitiveComplexity:  15,
					FunctionLength:       5000,
					NestingDepth:         4,
					ParameterCount:       5,
					MaintainabilityIndex: 20,
				},
			},
			expectedCount: 1,
			shouldContain: "function length",
		},
		{
			name: "invalid language",
			config: &Config{
				Thresholds: DefaultConfig().Thresholds,
				Analysis: AnalysisConfig{
					Languages: []string{"rust", "javascript"},
				},
			},
			expectedCount: 2,
			shouldContain: "unsupported language",
		},
		{
			name: "invalid storage backend",
			config: &Config{
				Thresholds: DefaultConfig().Thresholds,
				Storage: StorageConfig{
					Backend: "postgresql",
				},
			},
			expectedCount: 1,
			shouldContain: "unsupported storage backend",
		},
		{
			name: "multiple validation errors",
			config: &Config{
				Thresholds: ThresholdConfig{
					CyclomaticComplexity: 200,
					CognitiveComplexity:  0,
					FunctionLength:       5,
					NestingDepth:         4,
					ParameterCount:       5,
					MaintainabilityIndex: 20,
				},
			},
			expectedCount: 3,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			errors := testCase.config.ValidateConfiguration()

			if len(errors) != testCase.expectedCount {
				t.Errorf("expected %d errors, got %d: %v", testCase.expectedCount, len(errors), errors)
			}

			if testCase.shouldContain != "" {
				found := false
				for _, err := range errors {
					if containsSubstring(err, testCase.shouldContain) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing '%s', got: %v", testCase.shouldContain, errors)
				}
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	validConfig := &Config{
		Thresholds: DefaultConfig().Thresholds,
		Analysis: AnalysisConfig{
			Languages:  []string{"go"},
			MaxWorkers: 4,
		},
	}

	if !validConfig.IsValid() {
		t.Error("expected valid configuration to return true")
	}

	invalidConfig := &Config{
		Thresholds: ThresholdConfig{
			CyclomaticComplexity: 200,
		},
	}

	if invalidConfig.IsValid() {
		t.Error("expected invalid configuration to return false")
	}
}

func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && findSubstring(str, substr)
}

func findSubstring(str, substr string) bool {
	for index := 0; index <= len(str)-len(substr); index++ {
		if matchesAt(str, substr, index) {
			return true
		}
	}
	return false
}

func matchesAt(str, substr string, pos int) bool {
	for offset := 0; offset < len(substr); offset++ {
		if str[pos+offset] != substr[offset] {
			return false
		}
	}
	return true
}

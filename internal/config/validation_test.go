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
				Thresholds: DefaultConfig().Thresholds,
				Analysis: AnalysisConfig{
					Languages:  []string{"go", "python"},
					MaxWorkers: 4,
				},
				Storage: StorageConfig{
					Type: "sqlite",
				},
			},
			expectedCount: 0,
		},
		{
			name: "invalid complexity thresholds - out of order",
			config: &Config{
				Thresholds: ThresholdConfig{
					Complexity: SeverityThresholds{
						Info:     10,
						Warning:  5, // Warning less than info
						Critical: 20,
					},
					CognitiveComplexity:  DefaultConfig().Thresholds.CognitiveComplexity,
					FunctionLength:       DefaultConfig().Thresholds.FunctionLength,
					NestingDepth:         DefaultConfig().Thresholds.NestingDepth,
					ParameterCount:       DefaultConfig().Thresholds.ParameterCount,
					MaintainabilityIndex: DefaultConfig().Thresholds.MaintainabilityIndex,
					Churn:                DefaultConfig().Thresholds.Churn,
					GodFunction:          DefaultConfig().Thresholds.GodFunction,
					Hotspot:              DefaultConfig().Thresholds.Hotspot,
				},
			},
			expectedCount: 1,
			shouldContain: "info threshold must be less than warning",
		},
		{
			name: "invalid function length - out of range",
			config: &Config{
				Thresholds: ThresholdConfig{
					Complexity:          DefaultConfig().Thresholds.Complexity,
					CognitiveComplexity: DefaultConfig().Thresholds.CognitiveComplexity,
					FunctionLength: SeverityThresholds{
						Info:     5000, // Too high
						Warning:  6000,
						Critical: 7000,
					},
					NestingDepth:         DefaultConfig().Thresholds.NestingDepth,
					ParameterCount:       DefaultConfig().Thresholds.ParameterCount,
					MaintainabilityIndex: DefaultConfig().Thresholds.MaintainabilityIndex,
					Churn:                DefaultConfig().Thresholds.Churn,
					GodFunction:          DefaultConfig().Thresholds.GodFunction,
					Hotspot:              DefaultConfig().Thresholds.Hotspot,
				},
			},
			expectedCount: 3,
			shouldContain: "function_length",
		},
		{
			name: "invalid maintainability index - wrong order",
			config: &Config{
				Thresholds: ThresholdConfig{
					Complexity:          DefaultConfig().Thresholds.Complexity,
					CognitiveComplexity: DefaultConfig().Thresholds.CognitiveComplexity,
					FunctionLength:      DefaultConfig().Thresholds.FunctionLength,
					NestingDepth:        DefaultConfig().Thresholds.NestingDepth,
					ParameterCount:      DefaultConfig().Thresholds.ParameterCount,
					MaintainabilityIndex: MaintainabilityThresholds{
						Info:     20,
						Warning:  40,
						Critical: 60, // Should be lowest
					},
					Churn:       DefaultConfig().Thresholds.Churn,
					GodFunction: DefaultConfig().Thresholds.GodFunction,
					Hotspot:     DefaultConfig().Thresholds.Hotspot,
				},
			},
			expectedCount: 2,
			shouldContain: "maintainability_index",
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
			name: "invalid storage type",
			config: &Config{
				Thresholds: DefaultConfig().Thresholds,
				Storage: StorageConfig{
					Type: "postgresql",
				},
			},
			expectedCount: 1,
			shouldContain: "unsupported storage type",
		},
		{
			name: "invalid god function thresholds",
			config: &Config{
				Thresholds: ThresholdConfig{
					Complexity:           DefaultConfig().Thresholds.Complexity,
					CognitiveComplexity:  DefaultConfig().Thresholds.CognitiveComplexity,
					FunctionLength:       DefaultConfig().Thresholds.FunctionLength,
					NestingDepth:         DefaultConfig().Thresholds.NestingDepth,
					ParameterCount:       DefaultConfig().Thresholds.ParameterCount,
					MaintainabilityIndex: DefaultConfig().Thresholds.MaintainabilityIndex,
					Churn:                DefaultConfig().Thresholds.Churn,
					GodFunction: GodFunctionThresholds{
						MinParameters: 0,   // Too low
						MinFanIn:      200, // Too high
					},
					Hotspot: DefaultConfig().Thresholds.Hotspot,
				},
			},
			expectedCount: 2,
			shouldContain: "god_function",
		},
		{
			name: "negative max workers",
			config: &Config{
				Thresholds: DefaultConfig().Thresholds,
				Analysis: AnalysisConfig{
					MaxWorkers: -1,
				},
			},
			expectedCount: 1,
			shouldContain: "max_workers",
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
		Storage: StorageConfig{
			Type: "sqlite",
		},
	}

	if !validConfig.IsValid() {
		errors := validConfig.ValidateConfiguration()
		t.Errorf("expected valid configuration to return true, but got errors: %v", errors)
	}

	invalidConfig := &Config{
		Thresholds: ThresholdConfig{
			Complexity: SeverityThresholds{
				Info:     200, // Out of range
				Warning:  300,
				Critical: 400,
			},
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

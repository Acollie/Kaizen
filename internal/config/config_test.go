package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()

	// Verify complexity defaults match previous hardcoded values
	if cfg.Thresholds.Complexity.Warning != 10 {
		t.Errorf("Default complexity warning should be 10, got %d", cfg.Thresholds.Complexity.Warning)
	}
	if cfg.Thresholds.Complexity.Critical != 20 {
		t.Errorf("Default complexity critical should be 20, got %d", cfg.Thresholds.Complexity.Critical)
	}
	if cfg.Thresholds.FunctionLength.Warning != 50 {
		t.Errorf("Default function_length warning should be 50, got %d", cfg.Thresholds.FunctionLength.Warning)
	}
	if cfg.Thresholds.FunctionLength.Critical != 100 {
		t.Errorf("Default function_length critical should be 100, got %d", cfg.Thresholds.FunctionLength.Critical)
	}
	if cfg.Thresholds.NestingDepth.Warning != 5 {
		t.Errorf("Default nesting_depth warning should be 5, got %d", cfg.Thresholds.NestingDepth.Warning)
	}
	if cfg.Thresholds.ParameterCount.Warning != 7 {
		t.Errorf("Default parameter_count warning should be 7, got %d", cfg.Thresholds.ParameterCount.Warning)
	}
	if cfg.Thresholds.MaintainabilityIndex.Warning != 40 {
		t.Errorf("Default MI warning should be 40, got %d", cfg.Thresholds.MaintainabilityIndex.Warning)
	}
	if cfg.Thresholds.MaintainabilityIndex.Critical != 20 {
		t.Errorf("Default MI critical should be 20, got %d", cfg.Thresholds.MaintainabilityIndex.Critical)
	}
	if cfg.Thresholds.Churn.Warning != 10 {
		t.Errorf("Default churn warning should be 10, got %d", cfg.Thresholds.Churn.Warning)
	}
	if cfg.Thresholds.Hotspot.MinComplexity != 10 {
		t.Errorf("Default hotspot min_complexity should be 10, got %d", cfg.Thresholds.Hotspot.MinComplexity)
	}
	if cfg.Thresholds.Hotspot.MinChurn != 10 {
		t.Errorf("Default hotspot min_churn should be 10, got %d", cfg.Thresholds.Hotspot.MinChurn)
	}
	if cfg.Thresholds.GodFunction.MinParameters != 6 {
		t.Errorf("Default god_function min_parameters should be 6, got %d", cfg.Thresholds.GodFunction.MinParameters)
	}
}

func TestLoadConfigWithFullThresholds(t *testing.T) {
	tmpDir := t.TempDir()
	configYAML := `
thresholds:
  complexity:
    info: 3
    warning: 8
    critical: 15
  function_length:
    info: 25
    warning: 40
    critical: 80
  nesting_depth:
    info: 3
    warning: 4
    critical: 6
`
	configPath := filepath.Join(tmpDir, ".kaizen.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Thresholds.Complexity.Info != 3 {
		t.Errorf("Expected complexity info=3, got %d", cfg.Thresholds.Complexity.Info)
	}
	if cfg.Thresholds.Complexity.Warning != 8 {
		t.Errorf("Expected complexity warning=8, got %d", cfg.Thresholds.Complexity.Warning)
	}
	if cfg.Thresholds.Complexity.Critical != 15 {
		t.Errorf("Expected complexity critical=15, got %d", cfg.Thresholds.Complexity.Critical)
	}
	if cfg.Thresholds.FunctionLength.Info != 25 {
		t.Errorf("Expected function_length info=25, got %d", cfg.Thresholds.FunctionLength.Info)
	}
	if cfg.Thresholds.NestingDepth.Critical != 6 {
		t.Errorf("Expected nesting_depth critical=6, got %d", cfg.Thresholds.NestingDepth.Critical)
	}
}

func TestLoadConfigPartialThresholds(t *testing.T) {
	tmpDir := t.TempDir()
	configYAML := `
thresholds:
  complexity:
    warning: 8
`
	configPath := filepath.Join(tmpDir, ".kaizen.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Custom value should be set
	if cfg.Thresholds.Complexity.Warning != 8 {
		t.Errorf("Expected complexity warning=8, got %d", cfg.Thresholds.Complexity.Warning)
	}

	// Missing values should use defaults
	defaults := DefaultConfig().Thresholds
	if cfg.Thresholds.Complexity.Info != defaults.Complexity.Info {
		t.Errorf("Expected complexity info=%d (default), got %d", defaults.Complexity.Info, cfg.Thresholds.Complexity.Info)
	}
	if cfg.Thresholds.Complexity.Critical != defaults.Complexity.Critical {
		t.Errorf("Expected complexity critical=%d (default), got %d", defaults.Complexity.Critical, cfg.Thresholds.Complexity.Critical)
	}

	// Entirely unspecified threshold groups should use defaults
	if cfg.Thresholds.FunctionLength.Warning != defaults.FunctionLength.Warning {
		t.Errorf("Expected function_length warning=%d (default), got %d",
			defaults.FunctionLength.Warning, cfg.Thresholds.FunctionLength.Warning)
	}
	if cfg.Thresholds.Hotspot.MinComplexity != defaults.Hotspot.MinComplexity {
		t.Errorf("Expected hotspot min_complexity=%d (default), got %d",
			defaults.Hotspot.MinComplexity, cfg.Thresholds.Hotspot.MinComplexity)
	}
}

func TestLoadConfigNoThresholds(t *testing.T) {
	tmpDir := t.TempDir()
	configYAML := `
analysis:
  since: "30d"
`
	configPath := filepath.Join(tmpDir, ".kaizen.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	defaults := DefaultConfig().Thresholds
	if cfg.Thresholds.Complexity.Warning != defaults.Complexity.Warning {
		t.Errorf("Expected default complexity warning, got %d", cfg.Thresholds.Complexity.Warning)
	}
	if cfg.Thresholds.MaintainabilityIndex.Critical != defaults.MaintainabilityIndex.Critical {
		t.Errorf("Expected default MI critical, got %d", cfg.Thresholds.MaintainabilityIndex.Critical)
	}
}

func TestLoadConfigNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	defaults := DefaultConfig().Thresholds
	if cfg.Thresholds.Complexity.Warning != defaults.Complexity.Warning {
		t.Errorf("Expected default complexity warning without config file, got %d", cfg.Thresholds.Complexity.Warning)
	}
}

func TestThresholdValidationValid(t *testing.T) {
	thresholds := DefaultConfig().Thresholds
	if err := thresholds.Validate(); err != nil {
		t.Errorf("Default thresholds should be valid, got: %v", err)
	}
}

func TestThresholdValidationInvalidSeverityOrder(t *testing.T) {
	tests := []struct {
		name       string
		thresholds ThresholdConfig
	}{
		{
			name: "complexity info > warning",
			thresholds: func() ThresholdConfig {
				tc := DefaultConfig().Thresholds
				tc.Complexity = SeverityThresholds{Info: 15, Warning: 10, Critical: 20}
				return tc
			}(),
		},
		{
			name: "complexity warning > critical",
			thresholds: func() ThresholdConfig {
				tc := DefaultConfig().Thresholds
				tc.Complexity = SeverityThresholds{Info: 5, Warning: 25, Critical: 20}
				return tc
			}(),
		},
		{
			name: "maintainability critical > warning",
			thresholds: func() ThresholdConfig {
				tc := DefaultConfig().Thresholds
				tc.MaintainabilityIndex = MaintainabilityThresholds{Info: 60, Warning: 30, Critical: 40}
				return tc
			}(),
		},
		{
			name: "maintainability warning > info",
			thresholds: func() ThresholdConfig {
				tc := DefaultConfig().Thresholds
				tc.MaintainabilityIndex = MaintainabilityThresholds{Info: 30, Warning: 40, Critical: 20}
				return tc
			}(),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.thresholds.Validate()
			if err == nil {
				t.Error("Expected validation error for invalid threshold ordering")
			}
		})
	}
}

func TestThresholdValidationAllMetrics(t *testing.T) {
	// Verify validation catches issues in all severity threshold groups
	metricsToTest := []string{"function_length", "nesting_depth", "parameter_count", "churn", "cognitive_complexity"}

	for _, metric := range metricsToTest {
		t.Run(metric, func(t *testing.T) {
			tc := DefaultConfig().Thresholds
			invalid := SeverityThresholds{Info: 20, Warning: 10, Critical: 30}
			switch metric {
			case "function_length":
				tc.FunctionLength = invalid
			case "nesting_depth":
				tc.NestingDepth = invalid
			case "parameter_count":
				tc.ParameterCount = invalid
			case "churn":
				tc.Churn = invalid
			case "cognitive_complexity":
				tc.CognitiveComplexity = invalid
			}
			err := tc.Validate()
			if err == nil {
				t.Errorf("Expected validation error for %s with info > warning", metric)
			}
		})
	}
}

func TestApplyDefaultThresholdsZeroValues(t *testing.T) {
	tc := ThresholdConfig{} // all zeros
	tc.applyDefaultThresholds()

	defaults := DefaultConfig().Thresholds

	if tc.Complexity.Warning != defaults.Complexity.Warning {
		t.Errorf("Expected complexity warning=%d after applying defaults, got %d",
			defaults.Complexity.Warning, tc.Complexity.Warning)
	}
	if tc.MaintainabilityIndex.Critical != defaults.MaintainabilityIndex.Critical {
		t.Errorf("Expected MI critical=%d after applying defaults, got %d",
			defaults.MaintainabilityIndex.Critical, tc.MaintainabilityIndex.Critical)
	}
	if tc.Hotspot.MinComplexity != defaults.Hotspot.MinComplexity {
		t.Errorf("Expected hotspot min_complexity=%d after applying defaults, got %d",
			defaults.Hotspot.MinComplexity, tc.Hotspot.MinComplexity)
	}
	if tc.GodFunction.MinParameters != defaults.GodFunction.MinParameters {
		t.Errorf("Expected god_function min_parameters=%d after applying defaults, got %d",
			defaults.GodFunction.MinParameters, tc.GodFunction.MinParameters)
	}
}

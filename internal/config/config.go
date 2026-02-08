package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the Kaizen configuration
type Config struct {
	// Analysis settings
	Analysis AnalysisConfig `yaml:"analysis"`

	// Thresholds for warnings
	Thresholds ThresholdConfig `yaml:"thresholds"`

	// Visualization settings
	Visualization VisualizationConfig `yaml:"visualization"`

	// Storage settings
	Storage StorageConfig `yaml:"storage"`

	// Ignore patterns from .kaizenignore
	IgnorePatterns []string `yaml:"-"`
}

// AnalysisConfig contains analysis-specific settings
type AnalysisConfig struct {
	Since          string   `yaml:"since"`           // Default time range for churn (e.g., "90d")
	Languages      []string `yaml:"languages"`       // Languages to analyze
	ExcludePattern []string `yaml:"exclude"`         // Additional exclude patterns
	SkipChurn      bool     `yaml:"skip_churn"`      // Skip git churn analysis
	MaxWorkers     int      `yaml:"max_workers"`     // Number of parallel workers
}

// ThresholdConfig contains all configurable thresholds for concern detection
type ThresholdConfig struct {
	Complexity           SeverityThresholds        `yaml:"complexity"`
	CognitiveComplexity  SeverityThresholds        `yaml:"cognitive_complexity"`
	FunctionLength       SeverityThresholds        `yaml:"function_length"`
	NestingDepth         SeverityThresholds        `yaml:"nesting_depth"`
	ParameterCount       SeverityThresholds        `yaml:"parameter_count"`
	MaintainabilityIndex MaintainabilityThresholds `yaml:"maintainability_index"`
	Churn                SeverityThresholds        `yaml:"churn"`
	GodFunction          GodFunctionThresholds     `yaml:"god_function"`
	Hotspot              HotspotThresholds         `yaml:"hotspot"`
}

// SeverityThresholds defines info/warning/critical levels for upward metrics
// (higher values = worse, e.g. complexity, churn)
type SeverityThresholds struct {
	Info     int `yaml:"info"`
	Warning  int `yaml:"warning"`
	Critical int `yaml:"critical"`
}

// MaintainabilityThresholds are inverted (lower values = worse)
type MaintainabilityThresholds struct {
	Info     int `yaml:"info"`     // Below this = info concern
	Warning  int `yaml:"warning"`  // Below this = warning concern
	Critical int `yaml:"critical"` // Below this = critical concern
}

// GodFunctionThresholds require both conditions to be met
type GodFunctionThresholds struct {
	MinParameters int `yaml:"min_parameters"`
	MinFanIn      int `yaml:"min_fan_in"`
}

// HotspotThresholds require both conditions to be met
type HotspotThresholds struct {
	MinComplexity int `yaml:"min_complexity"`
	MinChurn      int `yaml:"min_churn"`
}

// VisualizationConfig contains visualization settings
type VisualizationConfig struct {
	DefaultMetric    string `yaml:"default_metric"`     // Default metric to show
	ColorScheme      string `yaml:"color_scheme"`       // Color scheme name
	ShowPercentages  bool   `yaml:"show_percentages"`   // Show percentages in output
	AutoOpenBrowser  bool   `yaml:"auto_open_browser"`  // Auto-open HTML in browser
}

// StorageConfig contains storage settings
type StorageConfig struct {
	Type           string `yaml:"type"`              // Storage backend: sqlite
	Path           string `yaml:"path"`              // Path to database file
	KeepJSONBackup bool   `yaml:"keep_json_backup"` // Also save JSON files
	RetentionDays  int    `yaml:"retention_days"`   // Auto-prune after N days (0=disabled)
	AutoPrune      bool   `yaml:"auto_prune"`       // Auto-prune on each analyze
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Analysis: AnalysisConfig{
			Since:      "90d",
			Languages:  []string{},
			ExcludePattern: []string{"vendor", "node_modules", "*_test.go"},
			SkipChurn:  false,
			MaxWorkers: 8,
		},
		Thresholds: ThresholdConfig{
			Complexity: SeverityThresholds{
				Info: 5, Warning: 10, Critical: 20,
			},
			CognitiveComplexity: SeverityThresholds{
				Info: 10, Warning: 15, Critical: 25,
			},
			FunctionLength: SeverityThresholds{
				Info: 30, Warning: 50, Critical: 100,
			},
			NestingDepth: SeverityThresholds{
				Info: 4, Warning: 5, Critical: 7,
			},
			ParameterCount: SeverityThresholds{
				Info: 5, Warning: 7, Critical: 10,
			},
			MaintainabilityIndex: MaintainabilityThresholds{
				Info: 60, Warning: 40, Critical: 20,
			},
			Churn: SeverityThresholds{
				Info: 5, Warning: 10, Critical: 20,
			},
			GodFunction: GodFunctionThresholds{
				MinParameters: 6, MinFanIn: 10,
			},
			Hotspot: HotspotThresholds{
				MinComplexity: 10, MinChurn: 10,
			},
		},
		Visualization: VisualizationConfig{
			DefaultMetric:   "hotspot",
			ColorScheme:     "red-yellow-green",
			ShowPercentages: true,
			AutoOpenBrowser: true,
		},
		Storage: StorageConfig{
			Type:           "sqlite",
			Path:           "", // Will be set dynamically
			KeepJSONBackup: true,
			RetentionDays:  90,
			AutoPrune:      false,
		},
		IgnorePatterns: []string{},
	}
}

// LoadConfig loads configuration from .kaizen.yaml and .kaizenignore
func LoadConfig(rootPath string) (*Config, error) {
	config := DefaultConfig()

	// Try to load .kaizen.yaml
	yamlPath := filepath.Join(rootPath, ".kaizen.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		err = config.loadYAML(yamlPath)
		if err != nil {
			return nil, err
		}
	}

	// Load .kaizenignore
	ignorePath := filepath.Join(rootPath, ".kaizenignore")
	if _, err := os.Stat(ignorePath); err == nil {
		err = config.loadIgnoreFile(ignorePath)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

// loadYAML loads configuration from a YAML file
func (config *Config) loadYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return err
	}

	// Fill in zero values with defaults (partial YAML config support)
	config.Thresholds.applyDefaultThresholds()

	return nil
}

// Validate ensures threshold values follow correct ordering
func (tc *ThresholdConfig) Validate() error {
	if err := validateSeverityOrder("complexity", tc.Complexity); err != nil {
		return err
	}
	if err := validateSeverityOrder("cognitive_complexity", tc.CognitiveComplexity); err != nil {
		return err
	}
	if err := validateSeverityOrder("function_length", tc.FunctionLength); err != nil {
		return err
	}
	if err := validateSeverityOrder("nesting_depth", tc.NestingDepth); err != nil {
		return err
	}
	if err := validateSeverityOrder("parameter_count", tc.ParameterCount); err != nil {
		return err
	}
	if err := validateSeverityOrder("churn", tc.Churn); err != nil {
		return err
	}
	// Maintainability is inverted: critical <= warning <= info
	mi := tc.MaintainabilityIndex
	if mi.Critical > mi.Warning {
		return fmt.Errorf("maintainability_index: critical (%d) must be <= warning (%d)", mi.Critical, mi.Warning)
	}
	if mi.Warning > mi.Info {
		return fmt.Errorf("maintainability_index: warning (%d) must be <= info (%d)", mi.Warning, mi.Info)
	}
	return nil
}

func validateSeverityOrder(name string, thresholds SeverityThresholds) error {
	if thresholds.Info > thresholds.Warning {
		return fmt.Errorf("%s: info (%d) must be <= warning (%d)", name, thresholds.Info, thresholds.Warning)
	}
	if thresholds.Warning > thresholds.Critical {
		return fmt.Errorf("%s: warning (%d) must be <= critical (%d)", name, thresholds.Warning, thresholds.Critical)
	}
	return nil
}

// applyDefaultThresholds fills in zero values with defaults from DefaultConfig
func (tc *ThresholdConfig) applyDefaultThresholds() {
	defaults := DefaultConfig().Thresholds
	applySeverityDefaults(&tc.Complexity, defaults.Complexity)
	applySeverityDefaults(&tc.CognitiveComplexity, defaults.CognitiveComplexity)
	applySeverityDefaults(&tc.FunctionLength, defaults.FunctionLength)
	applySeverityDefaults(&tc.NestingDepth, defaults.NestingDepth)
	applySeverityDefaults(&tc.ParameterCount, defaults.ParameterCount)
	applySeverityDefaults(&tc.Churn, defaults.Churn)
	applyMaintainabilityDefaults(&tc.MaintainabilityIndex, defaults.MaintainabilityIndex)
	applyGodFunctionDefaults(&tc.GodFunction, defaults.GodFunction)
	applyHotspotDefaults(&tc.Hotspot, defaults.Hotspot)
}

func applySeverityDefaults(target *SeverityThresholds, defaults SeverityThresholds) {
	if target.Info == 0 {
		target.Info = defaults.Info
	}
	if target.Warning == 0 {
		target.Warning = defaults.Warning
	}
	if target.Critical == 0 {
		target.Critical = defaults.Critical
	}
}

func applyMaintainabilityDefaults(target *MaintainabilityThresholds, defaults MaintainabilityThresholds) {
	if target.Info == 0 {
		target.Info = defaults.Info
	}
	if target.Warning == 0 {
		target.Warning = defaults.Warning
	}
	if target.Critical == 0 {
		target.Critical = defaults.Critical
	}
}

func applyGodFunctionDefaults(target *GodFunctionThresholds, defaults GodFunctionThresholds) {
	if target.MinParameters == 0 {
		target.MinParameters = defaults.MinParameters
	}
	if target.MinFanIn == 0 {
		target.MinFanIn = defaults.MinFanIn
	}
}

func applyHotspotDefaults(target *HotspotThresholds, defaults HotspotThresholds) {
	if target.MinComplexity == 0 {
		target.MinComplexity = defaults.MinComplexity
	}
	if target.MinChurn == 0 {
		target.MinChurn = defaults.MinChurn
	}
}

// loadIgnoreFile loads ignore patterns from .kaizenignore file
func (config *Config) loadIgnoreFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		config.IgnorePatterns = append(config.IgnorePatterns, line)
	}

	return scanner.Err()
}

// ShouldIgnore checks if a path should be ignored based on patterns
func (config *Config) ShouldIgnore(path string) bool {
	// Check ignore patterns from .kaizenignore
	for _, pattern := range config.IgnorePatterns {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	// Check exclude patterns from .kaizen.yaml
	for _, pattern := range config.Analysis.ExcludePattern {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a path matches a gitignore-style pattern
func matchesPattern(path string, pattern string) bool {
	// Handle negation patterns (starting with !)
	if strings.HasPrefix(pattern, "!") {
		pattern = pattern[1:]
		return !matchesPattern(path, pattern)
	}

	// Handle directory-only patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		pattern = pattern[:len(pattern)-1]
		// Check if path starts with this directory
		return strings.HasPrefix(path, pattern+"/") || path == pattern
	}

	// Handle patterns starting with / (absolute from project root)
	if strings.HasPrefix(pattern, "/") {
		pattern = pattern[1:]
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	// Handle ** wildcard (matches any number of directories)
	if strings.Contains(pattern, "**") {
		// Convert ** to a regex-like pattern
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
				return true
			}
		}
	}

	// Check if pattern matches the basename
	basename := filepath.Base(path)
	matched, _ := filepath.Match(pattern, basename)
	if matched {
		return true
	}

	// Check if pattern matches any part of the path
	if strings.Contains(path, pattern) {
		return true
	}

	// Standard glob pattern matching
	matched, _ = filepath.Match(pattern, path)
	return matched
}

// GetExcludePatterns returns all exclude patterns (from both sources)
func (config *Config) GetExcludePatterns() []string {
	patterns := make([]string, 0, len(config.IgnorePatterns)+len(config.Analysis.ExcludePattern))
	patterns = append(patterns, config.IgnorePatterns...)
	patterns = append(patterns, config.Analysis.ExcludePattern...)
	return patterns
}

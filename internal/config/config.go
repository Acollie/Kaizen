package config

import (
	"bufio"
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

// ThresholdConfig contains metric thresholds
type ThresholdConfig struct {
	CyclomaticComplexity int `yaml:"cyclomatic_complexity"` // Warn if > this value
	CognitiveComplexity  int `yaml:"cognitive_complexity"`  // Warn if > this value
	FunctionLength       int `yaml:"function_length"`       // Warn if > this value
	NestingDepth         int `yaml:"nesting_depth"`         // Warn if > this value
	ParameterCount       int `yaml:"parameter_count"`       // Warn if > this value
	MaintainabilityIndex int `yaml:"maintainability_index"` // Warn if < this value
}

// VisualizationConfig contains visualization settings
type VisualizationConfig struct {
	DefaultMetric    string `yaml:"default_metric"`     // Default metric to show
	ColorScheme      string `yaml:"color_scheme"`       // Color scheme name
	ShowPercentages  bool   `yaml:"show_percentages"`   // Show percentages in output
	AutoOpenBrowser  bool   `yaml:"auto_open_browser"`  // Auto-open HTML in browser
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
			CyclomaticComplexity: 10,
			CognitiveComplexity:  15,
			FunctionLength:       50,
			NestingDepth:         4,
			ParameterCount:       5,
			MaintainabilityIndex: 60,
		},
		Visualization: VisualizationConfig{
			DefaultMetric:   "hotspot",
			ColorScheme:     "red-yellow-green",
			ShowPercentages: true,
			AutoOpenBrowser: true,
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

	return nil
}

// loadIgnoreFile loads ignore patterns from .kaizenignore file
func (config *Config) loadIgnoreFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

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

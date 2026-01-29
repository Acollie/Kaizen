package analyzer

import (
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

// LanguageAnalyzer defines the contract for analyzing code in any language
type LanguageAnalyzer interface {
	// Name returns the language name (e.g., "Go", "Kotlin")
	Name() string

	// FileExtensions returns the file extensions this analyzer handles
	FileExtensions() []string

	// CanAnalyze checks if this analyzer can handle the given file
	CanAnalyze(filePath string) bool

	// AnalyzeFile performs full analysis on a single file
	AnalyzeFile(filePath string) (*models.FileAnalysis, error)

	// IsStub indicates if this is a stub implementation (not fully functional)
	IsStub() bool
}

// FunctionNode represents a function in any language
type FunctionNode interface {
	// Name returns the function name
	Name() string

	// StartLine returns the starting line number
	StartLine() int

	// EndLine returns the ending line number
	EndLine() int

	// LineCount returns the total lines (including blank/comments)
	LineCount() int

	// LogicalLineCount returns the number of actual code statements
	LogicalLineCount() int

	// ParameterCount returns the number of parameters
	ParameterCount() int

	// ReturnCount returns the number of return statements
	ReturnCount() int

	// MaxNestingDepth returns the maximum nesting level
	MaxNestingDepth() int

	// CalculateCyclomaticComplexity calculates McCabe's cyclomatic complexity
	CalculateCyclomaticComplexity() int

	// CalculateCognitiveComplexity calculates cognitive complexity (penalizes nesting)
	CalculateCognitiveComplexity() int
}

// TypeNode represents a class, struct, or interface in any language
type TypeNode interface {
	// Name returns the type name
	Name() string

	// Kind returns the type kind (struct, class, interface)
	Kind() string

	// Methods returns all methods/functions in this type
	Methods() []FunctionNode

	// FieldCount returns the number of fields
	FieldCount() int
}

// ChurnAnalyzer analyzes git history for churn metrics
type ChurnAnalyzer interface {
	// GetFileChurn analyzes churn for a specific file
	GetFileChurn(filePath string, since time.Time) (*models.ChurnMetric, error)

	// GetFunctionChurn analyzes churn for a specific function
	GetFunctionChurn(filePath string, functionName string, since time.Time) (*models.ChurnMetric, error)

	// IsGitRepository checks if the path is in a git repository
	IsGitRepository(repoPath string) bool
}

// MetricCalculator provides utility functions for calculating metrics
type MetricCalculator interface {
	// CalculateMaintainabilityIndex computes the maintainability index
	CalculateMaintainabilityIndex(
		halsteadVolume float64,
		cyclomaticComplexity int,
		linesOfCode int,
	) float64

	// CalculateHalsteadMetrics computes Halstead complexity metrics
	CalculateHalsteadMetrics(
		distinctOperators int,
		distinctOperands int,
		totalOperators int,
		totalOperands int,
	) models.HalsteadMetrics

	// IsHotspot determines if a function is a hotspot (high churn + high complexity)
	IsHotspot(churnScore float64, complexityScore float64) bool
}

// Aggregator aggregates file-level metrics to folder-level metrics
type Aggregator interface {
	// AggregateByFolder groups file analyses by folder and calculates folder metrics
	AggregateByFolder(files []models.FileAnalysis) map[string]models.FolderMetrics

	// CalculateScores normalizes raw metrics to 0-100 scores for visualization
	CalculateScores(folders map[string]models.FolderMetrics) map[string]models.FolderMetrics
}

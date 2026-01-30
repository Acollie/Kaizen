package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/alexcollie/kaizen/pkg/reports"
)

// AnalysisOptions contains configuration for the analysis
type AnalysisOptions struct {
	RootPath          string
	Since             time.Time
	IncludeLanguages  []string
	ExcludePatterns   []string
	IncludeChurn      bool
	MaxWorkers        int
	ProgressCallback  func(file string, current int, total int)
}

// Pipeline orchestrates the analysis process
type Pipeline struct {
	registry      interface{ GetAnalyzerForFile(string) (LanguageAnalyzer, error) }
	churnAnalyzer ChurnAnalyzer
	aggregator    Aggregator
}

// NewPipeline creates a new analysis pipeline
func NewPipeline(
	registry interface{ GetAnalyzerForFile(string) (LanguageAnalyzer, error) },
	churnAnalyzer ChurnAnalyzer,
	aggregator Aggregator,
) *Pipeline {
	return &Pipeline{
		registry:      registry,
		churnAnalyzer: churnAnalyzer,
		aggregator:    aggregator,
	}
}

// Analyze performs the complete analysis on a codebase
func (pipeline *Pipeline) Analyze(options AnalysisOptions) (*models.AnalysisResult, error) {
	// Discover all analyzable files
	files, err := pipeline.discoverFiles(options)
	if err != nil {
		return nil, fmt.Errorf("failed to discover files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no analyzable files found in %s", options.RootPath)
	}

	// Analyze each file
	fileAnalyses := make([]models.FileAnalysis, 0, len(files))
	for index, file := range files {
		if options.ProgressCallback != nil {
			options.ProgressCallback(file, index+1, len(files))
		}

		analysis, err := pipeline.analyzeFile(file, options)
		if err != nil {
			// Log error but continue with other files
			fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", file, err)
			continue
		}

		fileAnalyses = append(fileAnalyses, *analysis)
	}

	// Aggregate by folder
	folderStats := pipeline.aggregator.AggregateByFolder(fileAnalyses)

	// Calculate normalized scores
	folderStats = pipeline.aggregator.CalculateScores(folderStats)

	// Generate summary
	summary := pipeline.generateSummary(fileAnalyses)

	// Build result for score report generation
	result := &models.AnalysisResult{
		Repository: options.RootPath,
		AnalyzedAt: time.Now(),
		TimeRange: models.TimeRange{
			Since: options.Since,
			Until: time.Now(),
		},
		Files:       fileAnalyses,
		FolderStats: folderStats,
		Summary:     summary,
	}

	// Generate score report
	hasChurnData := options.IncludeChurn && pipeline.churnAnalyzer != nil
	result.ScoreReport = reports.GenerateScoreReport(result, hasChurnData)

	return result, nil
}

// discoverFiles finds all files that can be analyzed
func (pipeline *Pipeline) discoverFiles(options AnalysisOptions) ([]string, error) {
	var files []string

	err := filepath.Walk(options.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Check if directory should be excluded
			if pipeline.shouldExclude(path, options.ExcludePatterns) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be excluded
		if pipeline.shouldExclude(path, options.ExcludePatterns) {
			return nil
		}

		// Check if we can analyze this file
		analyzer, err := pipeline.registry.GetAnalyzerForFile(path)
		if err != nil {
			// No analyzer for this file type, skip
			return nil
		}

		// Check if language is in the include list (if specified)
		if len(options.IncludeLanguages) > 0 {
			langName := analyzer.Name()
			found := false
			for _, includedLang := range options.IncludeLanguages {
				if strings.EqualFold(langName, includedLang) {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// shouldExclude checks if a path matches any exclude pattern
func (pipeline *Pipeline) shouldExclude(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}

		// Also check if pattern is in the path
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// analyzeFile analyzes a single file
func (pipeline *Pipeline) analyzeFile(filePath string, options AnalysisOptions) (*models.FileAnalysis, error) {
	// Get the appropriate analyzer
	analyzer, err := pipeline.registry.GetAnalyzerForFile(filePath)
	if err != nil {
		return nil, err
	}

	// Check if it's a stub
	if analyzer.IsStub() {
		return nil, fmt.Errorf("analyzer for %s is a stub (not implemented)", analyzer.Name())
	}

	// Analyze the file
	analysis, err := analyzer.AnalyzeFile(filePath)
	if err != nil {
		return nil, err
	}

	// Add churn metrics if enabled
	if options.IncludeChurn && pipeline.churnAnalyzer != nil {
		churn, err := pipeline.churnAnalyzer.GetFileChurn(filePath, options.Since)
		if err != nil {
			// Log warning but don't fail
			fmt.Fprintf(os.Stderr, "Warning: failed to get churn for %s: %v\n", filePath, err)
		} else {
			analysis.Churn = churn

			// Add function-level churn
			for index := range analysis.Functions {
				funcChurn, err := pipeline.churnAnalyzer.GetFunctionChurn(
					filePath,
					analysis.Functions[index].Name,
					options.Since,
				)
				if err == nil {
					analysis.Functions[index].Churn = funcChurn
				}
			}
		}
	}

	// Mark hotspots
	for index := range analysis.Functions {
		function := &analysis.Functions[index]
		if function.Churn != nil {
			// Simple hotspot detection: high churn (>10 commits) + high complexity (>10)
			if function.Churn.TotalCommits > 10 && function.CyclomaticComplexity > 10 {
				function.IsHotspot = true
			}
		}
	}

	return analysis, nil
}

// generateSummary creates summary metrics from all file analyses
func (pipeline *Pipeline) generateSummary(files []models.FileAnalysis) models.SummaryMetrics {
	summary := models.SummaryMetrics{}

	totalComplexity := 0
	totalCognitive := 0
	totalLength := 0
	totalMaintainability := 0.0
	functionCount := 0

	for _, file := range files {
		summary.TotalFiles++
		summary.TotalLines += file.TotalLines
		summary.TotalCodeLines += file.CodeLines
		summary.TotalTypes += len(file.Types)

		for _, function := range file.Functions {
			functionCount++
			summary.TotalFunctions++

			totalComplexity += function.CyclomaticComplexity
			totalCognitive += function.CognitiveComplexity
			totalLength += function.Length
			totalMaintainability += function.MaintainabilityIndex

			// Count categories
			if function.CyclomaticComplexity > 10 {
				summary.HighComplexityCount++
			}
			if function.CyclomaticComplexity > 20 {
				summary.VeryHighComplexityCount++
			}
			if function.Length > 50 {
				summary.LongFunctionCount++
			}
			if function.Length > 100 {
				summary.VeryLongFunctionCount++
			}
			if function.IsHotspot {
				summary.HotspotCount++
			}
		}
	}

	// Calculate averages
	if functionCount > 0 {
		summary.AverageCyclomaticComplexity = float64(totalComplexity) / float64(functionCount)
		summary.AverageCognitiveComplexity = float64(totalCognitive) / float64(functionCount)
		summary.AverageFunctionLength = float64(totalLength) / float64(functionCount)
		summary.AverageMaintainabilityIndex = totalMaintainability / float64(functionCount)
	}

	return summary
}

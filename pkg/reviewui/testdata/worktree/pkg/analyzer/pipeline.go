// Package analyzer orchestrates file discovery, per-language analysis and aggregation.
package analyzer

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/churn"
	"github.com/alexcollie/kaizen/pkg/languages"
	"github.com/alexcollie/kaizen/pkg/models"
)

// Pipeline runs the full analysis for one repository
type Pipeline struct {
	rootPath      string
	registry      *languages.Registry
	ignore        *config.IgnoreMatcher
	churnAnalyzer *churn.Analyzer
	aggregator    *Aggregator
	workers       int
}

// NewPipeline wires a pipeline with the default registry; churnAnalyzer may be nil to skip churn
func NewPipeline(rootPath string, ignore *config.IgnoreMatcher, churnAnalyzer *churn.Analyzer) *Pipeline {
	return &Pipeline{
		rootPath:      rootPath,
		registry:      languages.NewRegistry(),
		ignore:        ignore,
		churnAnalyzer: churnAnalyzer,
		aggregator:    NewAggregator(),
		workers:       runtime.NumCPU(),
	}
}

// RootPath returns the directory the pipeline analyzes
func (pipeline *Pipeline) RootPath() string {
	return pipeline.rootPath
}

// Analyze discovers source files, analyzes them in parallel and aggregates folder metrics
func (pipeline *Pipeline) Analyze(ctx context.Context) (*models.AnalysisResult, error) {
	files, err := pipeline.discoverFiles()
	if err != nil {
		return nil, err
	}

	results := make([]*models.FileAnalysis, len(files))
	jobs := make(chan int)
	var waitGroup sync.WaitGroup

	for worker := 0; worker < pipeline.workers; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := range jobs {
				fileAnalyzer := pipeline.registry.GetAnalyzerForFile(files[index])
				if fileAnalyzer == nil || fileAnalyzer.IsStub() {
					continue
				}
				analysis, err := fileAnalyzer.AnalyzeFile(files[index])
				if err != nil {
					continue
				}
				if pipeline.churnAnalyzer != nil {
					pipeline.analyzeChurn(ctx, analysis)
				}
				results[index] = analysis
			}
		}()
	}

	for index := range files {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			waitGroup.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	waitGroup.Wait()

	analyzed := make([]models.FileAnalysis, 0, len(results))
	for _, analysis := range results {
		if analysis != nil {
			analyzed = append(analyzed, *analysis)
		}
	}

	return &models.AnalysisResult{
		Files:   analyzed,
		Folders: pipeline.aggregator.AggregateByFolder(analyzed),
	}, nil
}

// analyzeChurn attaches file-level churn to an analysis; failures leave churn empty
func (pipeline *Pipeline) analyzeChurn(ctx context.Context, analysis *models.FileAnalysis) {
	metric, err := pipeline.churnAnalyzer.GetFileChurn(ctx, analysis.Path)
	if err != nil {
		return
	}
	analysis.Churn = metric
}

// discoverFiles walks the root and returns analyzable files that are not ignored
func (pipeline *Pipeline) discoverFiles() ([]string, error) {
	var files []string
	err := filepath.WalkDir(pipeline.rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, _ := filepath.Rel(pipeline.rootPath, path)
		if entry.IsDir() {
			if entry.Name() == ".git" || pipeline.ignore.Match(relativePath, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if pipeline.ignore.Match(relativePath, false) {
			return nil
		}
		if pipeline.registry.GetAnalyzerForFile(path) != nil {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// SupportedLanguages lists the languages the pipeline can analyze
func (pipeline *Pipeline) SupportedLanguages() []string {
	var names []string
	for _, languageAnalyzer := range pipeline.registry.All() {
		if !languageAnalyzer.IsStub() {
			names = append(names, languageAnalyzer.Name())
		}
	}
	return names
}

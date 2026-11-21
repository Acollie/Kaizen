// Package languages dispatches files to the analyzer for their language.
package languages

import (
	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/languages/golang"
	"github.com/alexcollie/kaizen/pkg/languages/kotlin"
	"github.com/alexcollie/kaizen/pkg/languages/python"
	"github.com/alexcollie/kaizen/pkg/languages/swift"
)

// Registry holds every known language analyzer
type Registry struct {
	analyzers []analyzer.LanguageAnalyzer
}

// NewRegistry returns a registry with all built-in analyzers
func NewRegistry() *Registry {
	return &Registry{
		analyzers: []analyzer.LanguageAnalyzer{
			golang.NewGoAnalyzer(),
			kotlin.NewKotlinAnalyzer(),
			swift.NewSwiftAnalyzer(),
			python.NewPythonAnalyzer(),
		},
	}
}

// GetAnalyzerForFile returns the first analyzer that accepts filePath, or nil
func (registry *Registry) GetAnalyzerForFile(filePath string) analyzer.LanguageAnalyzer {
	for _, languageAnalyzer := range registry.analyzers {
		if languageAnalyzer.CanAnalyze(filePath) {
			return languageAnalyzer
		}
	}
	return nil
}

// All returns every registered analyzer
func (registry *Registry) All() []analyzer.LanguageAnalyzer {
	return registry.analyzers
}

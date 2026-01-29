package languages

import (
	"fmt"
	"path/filepath"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/languages/golang"
	"github.com/alexcollie/kaizen/pkg/languages/kotlin"
)

// Registry manages all available language analyzers
type Registry struct {
	analyzers []analyzer.LanguageAnalyzer
}

// NewRegistry creates a new language registry with all available analyzers
func NewRegistry() *Registry {
	return &Registry{
		analyzers: []analyzer.LanguageAnalyzer{
			golang.NewGoAnalyzer(),
			kotlin.NewKotlinAnalyzer(),
		},
	}
}

// GetAnalyzerForFile returns the appropriate analyzer for a given file
func (registry *Registry) GetAnalyzerForFile(filePath string) (analyzer.LanguageAnalyzer, error) {
	for _, languageAnalyzer := range registry.analyzers {
		if languageAnalyzer.CanAnalyze(filePath) {
			return languageAnalyzer, nil
		}
	}

	ext := filepath.Ext(filePath)
	return nil, fmt.Errorf("no analyzer found for file extension: %s", ext)
}

// GetAnalyzerByName returns an analyzer by language name
func (registry *Registry) GetAnalyzerByName(name string) (analyzer.LanguageAnalyzer, error) {
	for _, languageAnalyzer := range registry.analyzers {
		if languageAnalyzer.Name() == name {
			return languageAnalyzer, nil
		}
	}

	return nil, fmt.Errorf("no analyzer found for language: %s", name)
}

// GetAllAnalyzers returns all registered analyzers
func (registry *Registry) GetAllAnalyzers() []analyzer.LanguageAnalyzer {
	return registry.analyzers
}

// GetSupportedExtensions returns all supported file extensions
func (registry *Registry) GetSupportedExtensions() []string {
	extensions := []string{}
	for _, languageAnalyzer := range registry.analyzers {
		extensions = append(extensions, languageAnalyzer.FileExtensions()...)
	}
	return extensions
}

// GetSupportedLanguages returns all supported language names
func (registry *Registry) GetSupportedLanguages() []string {
	languages := []string{}
	for _, languageAnalyzer := range registry.analyzers {
		languages = append(languages, languageAnalyzer.Name())
	}
	return languages
}

// IsStubAnalyzer checks if the analyzer for a file is a stub
func (registry *Registry) IsStubAnalyzer(filePath string) (bool, error) {
	languageAnalyzer, err := registry.GetAnalyzerForFile(filePath)
	if err != nil {
		return false, err
	}
	return languageAnalyzer.IsStub(), nil
}

// FilterStubLanguages returns only fully implemented language names
func (registry *Registry) FilterStubLanguages() []string {
	languages := []string{}
	for _, languageAnalyzer := range registry.analyzers {
		if !languageAnalyzer.IsStub() {
			languages = append(languages, languageAnalyzer.Name())
		}
	}
	return languages
}

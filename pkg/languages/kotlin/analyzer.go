package kotlin

import (
	"fmt"
	"path/filepath"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
)

// KotlinAnalyzer is a stub implementation of LanguageAnalyzer for Kotlin
// This serves as a placeholder to demonstrate extensibility
type KotlinAnalyzer struct{}

// NewKotlinAnalyzer creates a new Kotlin analyzer stub
func NewKotlinAnalyzer() analyzer.LanguageAnalyzer {
	return &KotlinAnalyzer{}
}

// Name returns the language name
func (kotlinAnalyzer *KotlinAnalyzer) Name() string {
	return "Kotlin"
}

// FileExtensions returns the file extensions this analyzer would handle
func (kotlinAnalyzer *KotlinAnalyzer) FileExtensions() []string {
	return []string{".kt", ".kts"}
}

// CanAnalyze checks if this analyzer can handle the given file
func (kotlinAnalyzer *KotlinAnalyzer) CanAnalyze(filePath string) bool {
	ext := filepath.Ext(filePath)
	for _, supportedExt := range kotlinAnalyzer.FileExtensions() {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// IsStub indicates this is a stub implementation
func (kotlinAnalyzer *KotlinAnalyzer) IsStub() bool {
	return true
}

// AnalyzeFile returns an error indicating Kotlin support is not implemented
func (kotlinAnalyzer *KotlinAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
	return nil, fmt.Errorf("Kotlin analysis not yet implemented for file: %s\n\n"+
		"To implement Kotlin support:\n"+
		"1. Choose a parsing strategy:\n"+
		"   - Use tree-sitter-kotlin via go-tree-sitter bindings\n"+
		"   - Parse with external Kotlin compiler tools\n"+
		"   - Use a Kotlin AST library\n"+
		"2. Implement AST traversal to extract functions and types\n"+
		"3. Calculate complexity metrics (similar to Go implementation)\n"+
		"4. Update IsStub() to return false\n\n"+
		"See PLAN.md for detailed architecture guidance.",
		filePath)
}

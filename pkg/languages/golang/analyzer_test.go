package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoAnalyzerName(t *testing.T) {
	analyzer := NewGoAnalyzer()
	assert.Equal(t, "Go", analyzer.Name())
}

func TestGoAnalyzerFileExtensions(t *testing.T) {
	analyzer := NewGoAnalyzer()
	extensions := analyzer.FileExtensions()
	assert.Len(t, extensions, 1)
	assert.Equal(t, ".go", extensions[0])
}

func TestGoAnalyzerCanAnalyze(t *testing.T) {
	analyzer := NewGoAnalyzer()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{"go file", "main.go", true},
		{"nested go file", "pkg/analyzer/main.go", true},
		{"python file", "main.py", false},
		{"java file", "Main.java", false},
		{"no extension", "Makefile", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.CanAnalyze(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGoAnalyzerIsStub(t *testing.T) {
	analyzer := NewGoAnalyzer()
	assert.False(t, analyzer.IsStub())
}

func TestAnalyzeFileSimple(t *testing.T) {
	code := `package main

import "fmt"

// HelloFunc prints a message
func HelloFunc() {
	fmt.Println("Hello, World!")
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, filePath, result.Path)
	assert.Equal(t, "Go", result.Language)
	assert.Greater(t, result.TotalLines, 0)
	assert.Greater(t, result.CodeLines, 0)
	assert.Equal(t, 1, result.ImportCount)
	assert.Len(t, result.Functions, 1)
}

func TestAnalyzeFileWithComments(t *testing.T) {
	code := `package main

// SingleLineComment
/* MultiLineComment
   spanning lines
*/

func HelloFunc() {
	// Inside comment
	fmt.Println("Hello")
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Greater(t, result.CommentLines, 0)
	assert.Greater(t, result.CommentDensity, 0.0)
}

func TestAnalyzeFileParseError(t *testing.T) {
	code := `package main

func unclosed() {
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	_, err = analyzer.AnalyzeFile(filePath)

	assert.Error(t, err)
}

func TestAnalyzeFileNotFound(t *testing.T) {
	analyzer := NewGoAnalyzer()
	_, err := analyzer.AnalyzeFile("/nonexistent/file.go")

	assert.Error(t, err)
}

func TestAnalyzeFileMultipleFunctions(t *testing.T) {
	code := `package main

func FirstFunc() {
}

func SecondFunc() {
}

func ThirdFunc() {
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Functions))
	assert.Equal(t, "FirstFunc", result.Functions[0].Name)
	assert.Equal(t, "SecondFunc", result.Functions[1].Name)
	assert.Equal(t, "ThirdFunc", result.Functions[2].Name)
}

func TestAnalyzeFileMethods(t *testing.T) {
	code := `package main

type MyType struct {}

func (mt MyType) Method1() {
}

func (mt *MyType) Method2() {
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Functions))
}

func TestAnalyzeFileImports(t *testing.T) {
	code := `package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Equal(t, 3, result.ImportCount)
}

func TestAnalyzeFileBlankLines(t *testing.T) {
	code := `package main

func FirstFunc() {

}

func SecondFunc() {
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Greater(t, result.BlankLines, 0)
	assert.Equal(t, result.TotalLines, result.CodeLines+result.CommentLines+result.BlankLines)
}

func TestCountLinesMinimal(t *testing.T) {
	code := `package main`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalLines)
	assert.Greater(t, result.CodeLines, 0)
}

func TestExtractFunctionBasic(t *testing.T) {
	code := `package main

// MyFunction does something
func MyFunction(arg1 string, arg2 int) error {
	return nil
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	require.Len(t, result.Functions, 1)

	fn := result.Functions[0]
	assert.Equal(t, "MyFunction", fn.Name)
	assert.Greater(t, fn.StartLine, 0)
	assert.Greater(t, fn.EndLine, fn.StartLine)
	assert.Greater(t, fn.Length, 0)
	assert.Greater(t, fn.ParameterCount, 0)
}

func TestExtractTypesStruct(t *testing.T) {
	code := `package main

type MyStruct struct {
	Field1 string
	Field2 int
}

type MyInterface interface {
	Method() error
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Types)
}

func TestAnalyzeFileCommentDensity(t *testing.T) {
	code := `package main

// This is a comment
func MyFunc() {
	// Another comment
	x := 1 // inline comment
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Greater(t, result.CommentDensity, 0.0)
	assert.LessOrEqual(t, result.CommentDensity, 100.0)
}

func TestAnalyzeFileInitFunction(t *testing.T) {
	code := `package main

func init() {
	// Initialization
}
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte(code), 0644)
	require.NoError(t, err)

	analyzer := NewGoAnalyzer()
	result, err := analyzer.AnalyzeFile(filePath)

	require.NoError(t, err)
	assert.Len(t, result.Functions, 1)
	assert.Equal(t, "init", result.Functions[0].Name)
}

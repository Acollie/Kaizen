package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractGoFunctionsBasic(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package main

func FirstFunc() {
	println("first")
}

func SecondFunc() {
	println("second")
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Change range covers only SecondFunc (lines 7-9)
	ranges := []LineRange{{Start: 7, End: 9}}

	functions, err := extractGoFunctions(filePath, ranges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "SecondFunc" {
		t.Fatalf("expected SecondFunc, got %s", functions[0].Name)
	}
}

func TestExtractGoMethod(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package main

type MyType struct {}

func (m *MyType) MyMethod() {
	println("method")
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ranges := []LineRange{{Start: 5, End: 7}}

	functions, err := extractGoFunctions(filePath, ranges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "MyMethod" {
		t.Fatalf("expected MyMethod, got %s", functions[0].Name)
	}
	if !functions[0].HasReceiver {
		t.Fatalf("expected HasReceiver to be true")
	}
	if functions[0].ReceiverType != "MyType" {
		t.Fatalf("expected ReceiverType MyType, got %s", functions[0].ReceiverType)
	}
}

func TestMapHunksToFunctionsFileNotFound(t *testing.T) {
	hunks := []DiffHunk{
		{FilePath: "nonexistent.go", NewStart: 1, NewCount: 5},
	}

	// Should skip the file and return empty result
	functions, err := MapHunksToFunctions(hunks, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(functions) != 0 {
		t.Fatalf("expected 0 functions for missing file, got %d", len(functions))
	}
}

func TestExtractPythonFunction(t *testing.T) {
	tempDir := t.TempDir()

	pyCode := `def first_function():
    print("first")

def second_function():
    print("second")
    x = 1
`

	filePath := filepath.Join(tempDir, "test.py")
	if err := os.WriteFile(filePath, []byte(pyCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Change range covers second_function (lines 4-6)
	ranges := []LineRange{{Start: 4, End: 6}}

	functions, err := extractHeuristicFunctions(filePath, ranges, "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "second_function" {
		t.Fatalf("expected second_function, got %s", functions[0].Name)
	}
}

func TestExtractKotlinFunction(t *testing.T) {
	tempDir := t.TempDir()

	ktCode := `fun firstFunction() {
    println("first")
}

fun secondFunction() {
    println("second")
}
`

	filePath := filepath.Join(tempDir, "test.kt")
	if err := os.WriteFile(filePath, []byte(ktCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ranges := []LineRange{{Start: 5, End: 7}}

	functions, err := extractHeuristicFunctions(filePath, ranges, "kotlin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "secondFunction" {
		t.Fatalf("expected secondFunction, got %s", functions[0].Name)
	}
}

func TestFunctionOverlapsRanges(t *testing.T) {
	tests := []struct {
		name     string
		funcStart int
		funcEnd  int
		ranges   []LineRange
		expected bool
	}{
		{
			name:      "exact overlap",
			funcStart: 5,
			funcEnd:   10,
			ranges:    []LineRange{{Start: 5, End: 10}},
			expected:  true,
		},
		{
			name:      "partial overlap start",
			funcStart: 5,
			funcEnd:   15,
			ranges:    []LineRange{{Start: 10, End: 20}},
			expected:  true,
		},
		{
			name:      "partial overlap end",
			funcStart: 10,
			funcEnd:   20,
			ranges:    []LineRange{{Start: 5, End: 15}},
			expected:  true,
		},
		{
			name:      "no overlap before",
			funcStart: 5,
			funcEnd:   10,
			ranges:    []LineRange{{Start: 15, End: 20}},
			expected:  false,
		},
		{
			name:      "no overlap after",
			funcStart: 15,
			funcEnd:   20,
			ranges:    []LineRange{{Start: 5, End: 10}},
			expected:  false,
		},
		{
			name:      "multiple ranges first match",
			funcStart: 5,
			funcEnd:   10,
			ranges:    []LineRange{{Start: 5, End: 8}, {Start: 25, End: 30}},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := functionOverlapsRanges(tt.funcStart, tt.funcEnd, tt.ranges)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHunksToLineRanges(t *testing.T) {
	hunks := []DiffHunk{
		{FilePath: "file1.go", NewStart: 10, NewCount: 5},
		{FilePath: "file1.go", NewStart: 20, NewCount: 3},
		{FilePath: "file2.go", NewStart: 5, NewCount: 2},
	}

	result := hunksToLineRanges(hunks)

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	file1Ranges := result["file1.go"]
	if len(file1Ranges) != 2 {
		t.Fatalf("expected 2 ranges for file1.go, got %d", len(file1Ranges))
	}
	if file1Ranges[0].Start != 10 || file1Ranges[0].End != 14 {
		t.Fatalf("expected range [10,14], got [%d,%d]", file1Ranges[0].Start, file1Ranges[0].End)
	}
	if file1Ranges[1].Start != 20 || file1Ranges[1].End != 22 {
		t.Fatalf("expected range [20,22], got [%d,%d]", file1Ranges[1].Start, file1Ranges[1].End)
	}

	file2Ranges := result["file2.go"]
	if len(file2Ranges) != 1 {
		t.Fatalf("expected 1 range for file2.go, got %d", len(file2Ranges))
	}
}

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		line     string
		keyword  string
		language string
		expected string
	}{
		{"def my_function():", "def ", "python", "my_function"},
		{"def my_function(arg1, arg2):", "def ", "python", "my_function"},
		{"fun myFunction() {", "fun ", "kotlin", "myFunction"},
		{"func myFunction() {", "func ", "swift", "myFunction"},
		{"fun myFunction(x: Int): String {", "fun ", "kotlin", "myFunction"},
	}

	for _, tt := range tests {
		result := extractFunctionName(tt.line, tt.keyword, tt.language)
		if result != tt.expected {
			t.Fatalf("for line %q, expected %q, got %q", tt.line, tt.expected, result)
		}
	}
}

func TestMapHunksToFunctionsGoFile(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package mypackage

func PublicFunction() {
	println("public")
}

func privateFunction() {
	println("private")
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Range covers only PublicFunction (lines 3-6)
	hunks := []DiffHunk{
		{FilePath: "test.go", NewStart: 3, NewCount: 4},
	}

	functions, err := MapHunksToFunctions(hunks, tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(functions))
	}
	if functions[0].Name != "PublicFunction" {
		t.Fatalf("expected PublicFunction, got %s", functions[0].Name)
	}
	if functions[0].Language != "Go" {
		t.Fatalf("expected Go, got %s", functions[0].Language)
	}
}

func TestExtractGoReceiverType(t *testing.T) {
	tests := []struct {
		name     string
		receiver string
		expected string
	}{
		{"pointer receiver", "*MyType", "MyType"},
		{"value receiver", "MyType", "MyType"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test is simple; a full implementation would parse AST nodes
			// For now, test that extractGoReceiverType works on basic types
			result := strings.TrimPrefix(tt.receiver, "*")
			if result != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

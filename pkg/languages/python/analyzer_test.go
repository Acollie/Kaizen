package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonAnalyzerName(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	if analyzer.Name() != "Python" {
		t.Errorf("Expected name 'Python', got '%s'", analyzer.Name())
	}
}

func TestPythonAnalyzerFileExtensions(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	extensions := analyzer.FileExtensions()

	if len(extensions) != 1 {
		t.Errorf("Expected 1 extension, got %d", len(extensions))
	}
	if extensions[0] != ".py" {
		t.Errorf("Expected '.py', got '%s'", extensions[0])
	}
}

func TestPythonAnalyzerCanAnalyze(t *testing.T) {
	analyzer := NewPythonAnalyzer()

	tests := []struct {
		filePath string
		expected bool
	}{
		{"test.py", true},
		{"module/script.py", true},
		{"test.go", false},
		{"test.js", false},
		{"test.pyw", false},
		{"python", false},
	}

	for _, testCase := range tests {
		result := analyzer.CanAnalyze(testCase.filePath)
		if result != testCase.expected {
			t.Errorf("CanAnalyze(%s) = %v, expected %v",
				testCase.filePath, result, testCase.expected)
		}
	}
}

func TestPythonAnalyzerIsStub(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	if analyzer.IsStub() {
		t.Error("Python analyzer should not be a stub")
	}
}

func TestCountLines(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name            string
		code            string
		expectedTotal   int
		expectedCode    int
		expectedComment int
		expectedBlank   int
	}{
		{
			name:            "simple function",
			code:            "def foo():\n    return 1",
			expectedTotal:   2,
			expectedCode:    2,
			expectedComment: 0,
			expectedBlank:   0,
		},
		{
			name:            "with comments",
			code:            "# Comment\ndef foo():\n    return 1",
			expectedTotal:   3,
			expectedCode:    2,
			expectedComment: 1,
			expectedBlank:   0,
		},
		{
			name:            "with blank lines",
			code:            "def foo():\n\n    return 1\n",
			expectedTotal:   4,
			expectedCode:    2,
			expectedComment: 0,
			expectedBlank:   2,
		},
		{
			name:            "with docstring",
			code:            "def foo():\n    \"\"\"Docstring\"\"\"\n    return 1",
			expectedTotal:   3,
			expectedCode:    2,
			expectedComment: 1,
			expectedBlank:   0,
		},
		{
			name:            "multiline docstring",
			code:            "def foo():\n    \"\"\"\n    Multi\n    line\n    \"\"\"\n    return 1",
			expectedTotal:   6,
			expectedCode:    2,
			expectedComment: 4,
			expectedBlank:   0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			total, code, comment, blank := analyzer.countLines(testCase.code)
			if total != testCase.expectedTotal {
				t.Errorf("Total lines: got %d, expected %d", total, testCase.expectedTotal)
			}
			if code != testCase.expectedCode {
				t.Errorf("Code lines: got %d, expected %d", code, testCase.expectedCode)
			}
			if comment != testCase.expectedComment {
				t.Errorf("Comment lines: got %d, expected %d", comment, testCase.expectedComment)
			}
			if blank != testCase.expectedBlank {
				t.Errorf("Blank lines: got %d, expected %d", blank, testCase.expectedBlank)
			}
		})
	}
}

func TestCountImports(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "simple import",
			code:     "import os",
			expected: 1,
		},
		{
			name:     "multiple imports",
			code:     "import os\nimport sys",
			expected: 2,
		},
		{
			name:     "from import",
			code:     "from os import path",
			expected: 1,
		},
		{
			name:     "mixed imports",
			code:     "import os\nfrom sys import argv\nimport json",
			expected: 3,
		},
		{
			name:     "no imports",
			code:     "def foo():\n    pass",
			expected: 0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.countImports(testCase.code)
			if result != testCase.expected {
				t.Errorf("countImports: got %d, expected %d", result, testCase.expected)
			}
		})
	}
}

func TestCountParameters(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		params   string
		expected int
	}{
		{"no params", "", 0},
		{"one param", "x", 1},
		{"multiple params", "x, y, z", 3},
		{"self excluded", "self, x, y", 2},
		{"cls excluded", "cls, x", 1},
		{"with defaults", "x, y=10, z=None", 3},
		{"with type hints", "x: int, y: str", 2},
		{"complex types", "x: List[int], y: Dict[str, int]", 2},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.countParameters(testCase.params)
			if result != testCase.expected {
				t.Errorf("countParameters(%s): got %d, expected %d",
					testCase.params, result, testCase.expected)
			}
		})
	}
}

func TestCalculateCyclomaticComplexity(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "simple function",
			code:     "def foo():\n    return 1",
			expected: 1,
		},
		{
			name:     "single if",
			code:     "def foo(x):\n    if x > 0:\n        return x\n    return 0",
			expected: 2,
		},
		{
			name:     "if elif else",
			code:     "def foo(x):\n    if x > 0:\n        return 1\n    elif x < 0:\n        return -1\n    else:\n        return 0",
			expected: 3,
		},
		{
			name:     "for loop",
			code:     "def foo(items):\n    for item in items:\n        print(item)",
			expected: 2,
		},
		{
			name:     "while loop",
			code:     "def foo(x):\n    while x > 0:\n        x -= 1",
			expected: 2,
		},
		{
			name:     "try except",
			code:     "def foo():\n    try:\n        x = 1\n    except:\n        pass",
			expected: 2,
		},
		{
			name:     "and operator",
			code:     "def foo(x, y):\n    if x and y:\n        return True",
			expected: 3, // if + and
		},
		{
			name:     "or operator",
			code:     "def foo(x, y):\n    if x or y:\n        return True",
			expected: 3, // if + or
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.calculateCyclomaticComplexity(testCase.code)
			if result != testCase.expected {
				t.Errorf("calculateCyclomaticComplexity: got %d, expected %d\nCode:\n%s",
					result, testCase.expected, testCase.code)
			}
		})
	}
}

func TestCalculateNestingDepth(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		code     string
		indent   int
		expected int
	}{
		{
			name:     "flat function",
			code:     "def foo():\n    return 1",
			indent:   0,
			expected: 1,
		},
		{
			name:     "one level nesting",
			code:     "def foo():\n    if True:\n        return 1",
			indent:   0,
			expected: 2,
		},
		{
			name:     "two level nesting",
			code:     "def foo():\n    if True:\n        for i in range(10):\n            print(i)",
			indent:   0,
			expected: 3,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			lines := strings.Split(testCase.code, "\n")
			result := analyzer.calculateNestingDepth(lines, testCase.indent)
			if result != testCase.expected {
				t.Errorf("calculateNestingDepth: got %d, expected %d", result, testCase.expected)
			}
		})
	}
}

func TestExtractFunctions(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	code := `def simple_function():
    return 1

def function_with_params(x, y, z):
    result = x + y + z
    return result

class MyClass:
    def method(self, value):
        if value > 0:
            return value
        return 0
`

	functions := analyzer.extractFunctions(code)

	if len(functions) != 3 {
		t.Errorf("Expected 3 functions, got %d", len(functions))
	}

	// Check first function
	if functions[0].Name != "simple_function" {
		t.Errorf("First function should be 'simple_function', got '%s'", functions[0].Name)
	}
	if functions[0].ParameterCount != 0 {
		t.Errorf("simple_function should have 0 params, got %d", functions[0].ParameterCount)
	}

	// Check second function
	if functions[1].Name != "function_with_params" {
		t.Errorf("Second function should be 'function_with_params', got '%s'", functions[1].Name)
	}
	if functions[1].ParameterCount != 3 {
		t.Errorf("function_with_params should have 3 params, got %d", functions[1].ParameterCount)
	}

	// Check method (self excluded)
	if functions[2].Name != "method" {
		t.Errorf("Third function should be 'method', got '%s'", functions[2].Name)
	}
	if functions[2].ParameterCount != 1 {
		t.Errorf("method should have 1 param (excluding self), got %d", functions[2].ParameterCount)
	}
}

func TestExtractClasses(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	code := `class MyClass:
    def method(self):
        pass

class AnotherClass(BaseClass):
    pass
`

	types := analyzer.extractClasses(code)

	if len(types) != 2 {
		t.Errorf("Expected 2 classes, got %d", len(types))
	}

	if types[0].Name != "MyClass" {
		t.Errorf("First class should be 'MyClass', got '%s'", types[0].Name)
	}
	if types[0].Kind != "class" {
		t.Errorf("Kind should be 'class', got '%s'", types[0].Kind)
	}

	if types[1].Name != "AnotherClass" {
		t.Errorf("Second class should be 'AnotherClass', got '%s'", types[1].Name)
	}
}

func TestCalculateMaintainabilityIndex(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name       string
		volume     float64
		complexity int
		loc        int
		minScore   float64
		maxScore   float64
	}{
		{
			name:       "simple function",
			volume:     100,
			complexity: 1,
			loc:        5,
			minScore:   80,
			maxScore:   100,
		},
		{
			name:       "complex function",
			volume:     1000,
			complexity: 15,
			loc:        100,
			minScore:   30,
			maxScore:   60,
		},
		{
			name:       "zero loc",
			volume:     100,
			complexity: 5,
			loc:        0,
			minScore:   100,
			maxScore:   100,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.calculateMaintainabilityIndex(
				testCase.volume, testCase.complexity, testCase.loc)
			if result < testCase.minScore || result > testCase.maxScore {
				t.Errorf("calculateMaintainabilityIndex: got %.2f, expected between %.2f and %.2f",
					result, testCase.minScore, testCase.maxScore)
			}
		})
	}
}

func TestCountFunctionCalls(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "no calls",
			code:     "x = 1\ny = 2",
			expected: 0,
		},
		{
			name:     "simple call",
			code:     "result = foo()",
			expected: 1,
		},
		{
			name:     "multiple calls",
			code:     "foo()\nbar()\nbaz()",
			expected: 3,
		},
		{
			name:     "method calls",
			code:     "obj.method()\nobj.another()",
			expected: 2,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.countFunctionCalls(testCase.code)
			if result != testCase.expected {
				t.Errorf("countFunctionCalls: got %d, expected %d", result, testCase.expected)
			}
		})
	}
}

func TestAnalyzeFile(t *testing.T) {
	// Create a temporary Python file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	code := `#!/usr/bin/env python3
"""Module docstring"""

import os
import sys
from typing import List

def simple_function():
    """Simple function docstring"""
    return 42

def complex_function(items: List[int], multiplier: int = 1) -> int:
    """
    A more complex function with multiple branches.
    """
    total = 0
    for item in items:
        if item > 0:
            total += item * multiplier
        elif item < 0:
            total -= item
        else:
            continue
    return total

class Calculator:
    """Calculator class"""

    def __init__(self, value: int = 0):
        self.value = value

    def add(self, amount: int) -> int:
        self.value += amount
        return self.value
`

	err := os.WriteFile(testFile, []byte(code), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	analyzer := NewPythonAnalyzer()
	analysis, err := analyzer.AnalyzeFile(testFile)

	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Basic checks
	if analysis.Language != "Python" {
		t.Errorf("Expected language 'Python', got '%s'", analysis.Language)
	}
	if analysis.Path != testFile {
		t.Errorf("Expected path '%s', got '%s'", testFile, analysis.Path)
	}

	// Line counts
	if analysis.TotalLines < 30 {
		t.Errorf("Expected at least 30 total lines, got %d", analysis.TotalLines)
	}

	// Import count
	if analysis.ImportCount != 3 {
		t.Errorf("Expected 3 imports, got %d", analysis.ImportCount)
	}

	// Function count
	if len(analysis.Functions) < 4 {
		t.Errorf("Expected at least 4 functions, got %d", len(analysis.Functions))
	}

	// Class count
	if len(analysis.Types) != 1 {
		t.Errorf("Expected 1 class, got %d", len(analysis.Types))
	}

	// Check complex_function complexity
	for _, fn := range analysis.Functions {
		if fn.Name == "complex_function" {
			if fn.CyclomaticComplexity < 4 {
				t.Errorf("complex_function should have CC >= 4, got %d", fn.CyclomaticComplexity)
			}
			if fn.ParameterCount != 2 {
				t.Errorf("complex_function should have 2 params, got %d", fn.ParameterCount)
			}
		}
	}
}

func TestAnalyzeFileNotFound(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	_, err := analyzer.AnalyzeFile("/nonexistent/file.py")

	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFindFunctionEnd(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name        string
		lines       []string
		startIndex  int
		baseIndent  int
		expectedEnd int
	}{
		{
			name: "simple function",
			lines: []string{
				"def foo():",
				"    return 1",
				"",
				"def bar():",
			},
			startIndex:  0,
			baseIndent:  0,
			expectedEnd: 3,
		},
		{
			name: "function with nested blocks",
			lines: []string{
				"def foo():",
				"    if True:",
				"        return 1",
				"    return 0",
				"",
				"def bar():",
			},
			startIndex:  0,
			baseIndent:  0,
			expectedEnd: 5,
		},
		{
			name: "indented method",
			lines: []string{
				"class Foo:",
				"    def method(self):",
				"        return 1",
				"",
				"    def other(self):",
			},
			startIndex:  1,
			baseIndent:  4,
			expectedEnd: 4,
		},
		{
			name: "function at end of file",
			lines: []string{
				"def foo():",
				"    return 1",
			},
			startIndex:  0,
			baseIndent:  0,
			expectedEnd: 2,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.findFunctionEnd(testCase.lines, testCase.startIndex, testCase.baseIndent)
			if result != testCase.expectedEnd {
				t.Errorf("findFunctionEnd: got %d, expected %d", result, testCase.expectedEnd)
			}
		})
	}
}

func TestCountLocalVariables(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "no variables",
			code:     "def foo():\n    return 1",
			expected: 0,
		},
		{
			name:     "single variable",
			code:     "def foo():\n    x = 1\n    return x",
			expected: 1,
		},
		{
			name:     "multiple variables",
			code:     "def foo():\n    x = 1\n    y = 2\n    z = x + y\n    return z",
			expected: 3,
		},
		{
			name:     "self excluded",
			code:     "def foo(self):\n    self.x = 1\n    y = 2\n    return y",
			expected: 1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.countLocalVariables(testCase.code)
			if result != testCase.expected {
				t.Errorf("countLocalVariables: got %d, expected %d", result, testCase.expected)
			}
		})
	}
}

func TestCountReturns(t *testing.T) {
	analyzer := &PythonAnalyzer{}

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "no return",
			code:     "def foo():\n    pass",
			expected: 0,
		},
		{
			name:     "single return",
			code:     "def foo():\n    return 1",
			expected: 1,
		},
		{
			name:     "multiple returns",
			code:     "def foo(x):\n    if x > 0:\n        return x\n    return 0",
			expected: 2,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := analyzer.countReturns(testCase.code)
			if result != testCase.expected {
				t.Errorf("countReturns: got %d, expected %d", result, testCase.expected)
			}
		})
	}
}

func TestIsAllCaps(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"CONSTANT", true},
		{"MAX_VALUE", true},
		{"variable", false},
		{"camelCase", false},
		{"PascalCase", false},
		{"X", false}, // Single char excluded
		{"AB", true},
	}

	for _, testCase := range tests {
		result := isAllCaps(testCase.input)
		if result != testCase.expected {
			t.Errorf("isAllCaps(%s): got %v, expected %v",
				testCase.input, result, testCase.expected)
		}
	}
}

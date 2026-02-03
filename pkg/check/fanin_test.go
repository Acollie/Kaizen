package check

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGoFullNamePlainFunction(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package mypackage

func MyFunction() {
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fn := ChangedFunction{
		FilePath:    filePath,
		Name:        "MyFunction",
		HasReceiver: false,
		Language:    "Go",
	}

	fullName, err := buildGoFullName(fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fullName != "mypackage.MyFunction" {
		t.Fatalf("expected mypackage.MyFunction, got %s", fullName)
	}
}

func TestBuildGoFullNameMethod(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package mypackage

type MyType struct {}

func (m *MyType) MyMethod() {
}
`

	filePath := filepath.Join(tempDir, "test.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fn := ChangedFunction{
		FilePath:     filePath,
		Name:         "MyMethod",
		HasReceiver:  true,
		ReceiverType: "MyType",
		Language:     "Go",
	}

	fullName, err := buildGoFullName(fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fullName != "mypackage.MyType.MyMethod" {
		t.Fatalf("expected mypackage.MyType.MyMethod, got %s", fullName)
	}
}

func TestBuildGoFullNameMainPackage(t *testing.T) {
	tempDir := t.TempDir()

	goCode := `package main

func main() {
}
`

	filePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filePath, []byte(goCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fn := ChangedFunction{
		FilePath:    filePath,
		Name:        "main",
		HasReceiver: false,
		Language:    "Go",
	}

	fullName, err := buildGoFullName(fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fullName != "main.main" {
		t.Fatalf("expected main.main, got %s", fullName)
	}
}

func TestComputeGrepFanInBasic(t *testing.T) {
	tempDir := t.TempDir()

	// Create Python files
	pyFile1 := filepath.Join(tempDir, "file1.py")
	pyFile2 := filepath.Join(tempDir, "file2.py")
	pyFile3 := filepath.Join(tempDir, "file3.py")

	if err := os.WriteFile(pyFile1, []byte("def my_func():\n    pass\n"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(pyFile2, []byte("my_func()  # call 1\n"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	if err := os.WriteFile(pyFile3, []byte("my_func()  # call 2\n"), 0644); err != nil {
		t.Fatalf("failed to write file3: %v", err)
	}

	changedFunctions := []ChangedFunction{
		{
			FilePath: pyFile1,
			Name:     "my_func",
			Language: "Python",
		},
	}

	results, err := computeGrepFanIn(changedFunctions, tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Should find 2 call sites (file2 and file3, excluding the definition in file1)
	if results[0].FanIn != 2 {
		t.Fatalf("expected fan-in 2, got %d", results[0].FanIn)
	}
	if !results[0].Approximate {
		t.Fatalf("expected Approximate=true for grep-based result")
	}
}

func TestExtensionForLanguage(t *testing.T) {
	tests := []struct {
		language string
		expected string
	}{
		{"Python", "*.py"},
		{"Kotlin", "*.kt"},
		{"Swift", "*.swift"},
		{"Go", ""},
		{"Unknown", ""},
	}

	for _, tt := range tests {
		result := extensionForLanguage(tt.language)
		if result != tt.expected {
			t.Fatalf("for %s, expected %s, got %s", tt.language, tt.expected, result)
		}
	}
}

func TestComputeFanInEmpty(t *testing.T) {
	results, err := ComputeFanIn(nil, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil for empty input, got %v", results)
	}
}

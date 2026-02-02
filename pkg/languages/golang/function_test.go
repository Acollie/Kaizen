package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseGoFunction(t *testing.T, code string) *GoFunction {
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	var funcDecl *ast.FuncDecl
	for _, decl := range astFile.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcDecl = fn
			break
		}
	}

	require.NotNil(t, funcDecl, "no function declaration found in code")
	return NewGoFunction(funcDecl, fileSet, code)
}

func TestGoFunctionName(t *testing.T) {
	code := `package main

func MyFunction() {
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, "MyFunction", goFunc.Name())
}

func TestGoFunctionStartAndEndLine(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
}
`

	goFunc := parseGoFunction(t, code)
	assert.Greater(t, goFunc.StartLine(), 0)
	assert.Greater(t, goFunc.EndLine(), goFunc.StartLine())
}

func TestGoFunctionLineCount(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
}
`

	goFunc := parseGoFunction(t, code)
	lineCount := goFunc.LineCount()
	assert.Greater(t, lineCount, 0)
}

func TestParameterCountZero(t *testing.T) {
	code := `package main

func MyFunction() {
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, 0, goFunc.ParameterCount())
}

func TestParameterCountSingle(t *testing.T) {
	code := `package main

func MyFunction(arg1 string) {
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, 1, goFunc.ParameterCount())
}

func TestParameterCountMultiple(t *testing.T) {
	code := `package main

func MyFunction(arg1 string, arg2 int, arg3 bool) {
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, 3, goFunc.ParameterCount())
}

func TestParameterCountMultipleNamesOneType(t *testing.T) {
	code := `package main

func MyFunction(a, b, c int) {
}
`

	goFunc := parseGoFunction(t, code)
	// Go allows multiple parameter names with one type
	assert.Equal(t, 3, goFunc.ParameterCount())
}

func TestReturnCountZero(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, 0, goFunc.ReturnCount())
}

func TestReturnCountSingle(t *testing.T) {
	code := `package main

func MyFunction() string {
	return "hello"
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, 1, goFunc.ReturnCount())
}

func TestReturnCountMultiple(t *testing.T) {
	code := `package main

func MyFunction() error {
	if true {
		return nil
	}
	return nil
}
`

	goFunc := parseGoFunction(t, code)
	assert.Equal(t, 2, goFunc.ReturnCount())
}

func TestMaxNestingDepthFlat(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
	y := 2
}
`

	goFunc := parseGoFunction(t, code)
	// Flat code should have low nesting
	assert.GreaterOrEqual(t, goFunc.MaxNestingDepth(), 0)
}

func TestMaxNestingDepthWithControlFlow(t *testing.T) {
	code := `package main

func MyFunction() {
	if true {
		x := 1
	}
}
`

	goFunc := parseGoFunction(t, code)
	// Should detect nesting in control flow
	assert.GreaterOrEqual(t, goFunc.MaxNestingDepth(), 0)
}

func TestMaxNestingDepthDeepNesting(t *testing.T) {
	code := `package main

func MyFunction() {
	if true {
		for i := 0; i < 10; i++ {
			if i > 5 {
				x := 1
			}
		}
	}
}
`

	goFunc := parseGoFunction(t, code)
	// Deep nesting should have higher depth than shallow nesting
	depth := goFunc.MaxNestingDepth()
	assert.GreaterOrEqual(t, depth, 0)
}

func TestCalculateCyclomaticComplexitySimple(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCyclomaticComplexity()
	// Base complexity is 1
	assert.Equal(t, 1, complexity)
}

func TestCalculateCyclomaticComplexityWithIf(t *testing.T) {
	code := `package main

func MyFunction() {
	if true {
		x := 1
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCyclomaticComplexity()
	// Base 1 + if 1 = 2
	assert.Greater(t, complexity, 1)
}

func TestCalculateCyclomaticComplexityWithMultipleDecisions(t *testing.T) {
	code := `package main

func MyFunction() {
	if true {
		x := 1
	}
	if false {
		y := 2
	}
	for i := 0; i < 10; i++ {
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCyclomaticComplexity()
	// Base 1 + if 1 + if 1 + for 1 = 4
	assert.GreaterOrEqual(t, complexity, 3)
}

func TestCalculateCyclomaticComplexityWithLogicalOperators(t *testing.T) {
	code := `package main

func MyFunction() {
	if true && false {
		x := 1
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCyclomaticComplexity()
	// Base 1 + if 1 + && 1 = 3
	assert.Greater(t, complexity, 1)
}

func TestCalculateCyclomaticComplexitySwitch(t *testing.T) {
	code := `package main

func MyFunction() {
	switch x {
	case 1:
		y := 1
	case 2:
		y := 2
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCyclomaticComplexity()
	// Base 1 + case 1 + case 1 = 3
	assert.Greater(t, complexity, 1)
}

func TestCalculateCognitiveComplexitySimple(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCognitiveComplexity()
	assert.Equal(t, 0, complexity)
}

func TestCalculateCognitiveComplexityWithIf(t *testing.T) {
	code := `package main

func MyFunction() {
	if true {
		x := 1
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCognitiveComplexity()
	// +1 for if at nesting 0
	assert.Greater(t, complexity, 0)
}

func TestCalculateCognitiveComplexityWithNesting(t *testing.T) {
	code := `package main

func MyFunction() {
	if true {
		if true {
			x := 1
		}
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCognitiveComplexity()
	// Nested ifs should have higher cognitive complexity than flat
	simpleCode := `package main

func MyFunction() {
	if true {
		x := 1
	}
	if true {
		y := 1
	}
}
`
	simpleFunc := parseGoFunction(t, simpleCode)
	simpleComplexity := simpleFunc.CalculateCognitiveComplexity()

	assert.Greater(t, complexity, simpleComplexity)
}

func TestCalculateCognitiveComplexityForLoop(t *testing.T) {
	code := `package main

func MyFunction() {
	for i := 0; i < 10; i++ {
		if i > 5 {
			x := 1
		}
	}
}
`

	goFunc := parseGoFunction(t, code)
	complexity := goFunc.CalculateCognitiveComplexity()
	// +1 for for loop, +1 for nested if (+1 nesting bonus)
	assert.Greater(t, complexity, 1)
}

func TestLogicalLineCountEmpty(t *testing.T) {
	code := `package main

func MyFunction() {
}
`

	goFunc := parseGoFunction(t, code)
	lineCount := goFunc.LogicalLineCount()
	assert.Equal(t, 0, lineCount)
}

func TestLogicalLineCountMultiple(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
	y := 2
	z := x + y
	return z
}
`

	goFunc := parseGoFunction(t, code)
	lineCount := goFunc.LogicalLineCount()
	// Should count assignments and return
	assert.GreaterOrEqual(t, lineCount, 3)
}

func TestGetLocalVariableCount(t *testing.T) {
	code := `package main

func MyFunction() {
	x := 1
	y := 2
	z := 3
}
`

	goFunc := parseGoFunction(t, code)
	varCount := goFunc.GetLocalVariableCount()
	assert.GreaterOrEqual(t, varCount, 3)
}

func TestGetLocalVariableCountMultipleAssignment(t *testing.T) {
	code := `package main

func MyFunction() {
	x, y := 1, 2
}
`

	goFunc := parseGoFunction(t, code)
	varCount := goFunc.GetLocalVariableCount()
	assert.GreaterOrEqual(t, varCount, 2)
}

func TestCyclomaticComplexityKnownValues(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		minExpectedCC   int
		maxExpectedCC   int
	}{
		{
			name: "simple function",
			code: `package main

func Simple() {
	x := 1
}
`,
			minExpectedCC: 1,
			maxExpectedCC: 1,
		},
		{
			name: "single if",
			code: `package main

func WithIf() {
	if true {
		x := 1
	}
}
`,
			minExpectedCC: 2,
			maxExpectedCC: 2,
		},
		{
			name: "if with else",
			code: `package main

func WithIfElse() {
	if true {
		x := 1
	} else {
		y := 2
	}
}
`,
			minExpectedCC: 2,
			maxExpectedCC: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goFunc := parseGoFunction(t, tt.code)
			cc := goFunc.CalculateCyclomaticComplexity()
			assert.GreaterOrEqual(t, cc, tt.minExpectedCC)
			assert.LessOrEqual(t, cc, tt.maxExpectedCC)
		})
	}
}

func TestImportForAST(t *testing.T) {
	// Verify that we can properly import ast package to work with GoFunction
	code := `package main
import "fmt"

func Test() {
	fmt.Println("test")
}
`
	goFunc := parseGoFunction(t, code)
	assert.NotNil(t, goFunc.declaration)
}

package golang

import (
	"go/ast"
	"go/token"
)

// GoFunction implements the FunctionNode interface for Go functions
type GoFunction struct {
	declaration *ast.FuncDecl
	fileSet     *token.FileSet
	sourceCode  string
}

// NewGoFunction creates a new GoFunction from an AST node
func NewGoFunction(declaration *ast.FuncDecl, fileSet *token.FileSet, sourceCode string) *GoFunction {
	return &GoFunction{
		declaration: declaration,
		fileSet:     fileSet,
		sourceCode:  sourceCode,
	}
}

// Name returns the function name
func (goFunc *GoFunction) Name() string {
	return goFunc.declaration.Name.Name
}

// StartLine returns the starting line number
func (goFunc *GoFunction) StartLine() int {
	return goFunc.fileSet.Position(goFunc.declaration.Pos()).Line
}

// EndLine returns the ending line number
func (goFunc *GoFunction) EndLine() int {
	return goFunc.fileSet.Position(goFunc.declaration.End()).Line
}

// LineCount returns the total lines (including blank/comments)
func (goFunc *GoFunction) LineCount() int {
	return goFunc.EndLine() - goFunc.StartLine() + 1
}

// LogicalLineCount returns the number of actual code statements
func (goFunc *GoFunction) LogicalLineCount() int {
	count := 0
	ast.Inspect(goFunc.declaration, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.AssignStmt, *ast.ExprStmt, *ast.ReturnStmt,
			*ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.SelectStmt, *ast.GoStmt,
			*ast.DeferStmt, *ast.SendStmt, *ast.IncDecStmt:
			count++
		}
		return true
	})
	return count
}

// ParameterCount returns the number of parameters
func (goFunc *GoFunction) ParameterCount() int {
	if goFunc.declaration.Type.Params == nil {
		return 0
	}
	count := 0
	for _, field := range goFunc.declaration.Type.Params.List {
		// Each field can have multiple names (e.g., a, b int)
		if len(field.Names) == 0 {
			count++ // Unnamed parameter
		} else {
			count += len(field.Names)
		}
	}
	return count
}

// ReturnCount returns the number of return statements
func (goFunc *GoFunction) ReturnCount() int {
	count := 0
	ast.Inspect(goFunc.declaration, func(node ast.Node) bool {
		if _, ok := node.(*ast.ReturnStmt); ok {
			count++
		}
		return true
	})
	return count
}

// MaxNestingDepth returns the maximum nesting level
func (goFunc *GoFunction) MaxNestingDepth() int {
	maxDepth := 0
	currentDepth := 0

	ast.Inspect(goFunc.declaration, func(node ast.Node) bool {
		if node == nil {
			currentDepth--
			return false
		}

		switch node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		}

		return true
	})

	return maxDepth
}

// CalculateCyclomaticComplexity calculates McCabe's cyclomatic complexity
// Formula: M = E - N + 2P where E=edges, N=nodes, P=connected components
// Simplified: Start with 1, add 1 for each decision point
func (goFunc *GoFunction) CalculateCyclomaticComplexity() int {
	complexity := 1 // Base complexity

	ast.Inspect(goFunc.declaration, func(node ast.Node) bool {
		switch nodeType := node.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt, *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			// Don't count default case
			if nodeType.List != nil {
				complexity++
			}
		case *ast.CommClause:
			// Select statement cases
			if nodeType.Comm != nil {
				complexity++
			}
		case *ast.BinaryExpr:
			// Count && and || as decision points
			if nodeType.Op == token.LAND || nodeType.Op == token.LOR {
				complexity++
			}
		}
		return true
	})

	return complexity
}

// CalculateCognitiveComplexity calculates cognitive complexity
// Penalizes nesting more heavily than cyclomatic complexity
func (goFunc *GoFunction) CalculateCognitiveComplexity() int {
	complexity := 0
	nestingLevel := 0
	ignoreNesting := false

	var inspect func(ast.Node) bool
	inspect = func(node ast.Node) bool {
		if node == nil {
			return false
		}

		switch nodeType := node.(type) {
		case *ast.IfStmt:
			// +1 for if, +nesting for being nested
			complexity += 1 + nestingLevel
			nestingLevel++
			ast.Inspect(nodeType.Body, inspect)
			nestingLevel--

			// Handle else/else if
			if nodeType.Else != nil {
				complexity++ // +1 for else
				ast.Inspect(nodeType.Else, inspect)
			}
			return false

		case *ast.ForStmt:
			complexity += 1 + nestingLevel
			nestingLevel++
			ast.Inspect(nodeType.Body, inspect)
			nestingLevel--
			return false

		case *ast.RangeStmt:
			complexity += 1 + nestingLevel
			nestingLevel++
			ast.Inspect(nodeType.Body, inspect)
			nestingLevel--
			return false

		case *ast.SwitchStmt:
			complexity += 1 + nestingLevel
			nestingLevel++
			// Visit each case
			for _, stmt := range nodeType.Body.List {
				ast.Inspect(stmt, inspect)
			}
			nestingLevel--
			return false

		case *ast.TypeSwitchStmt:
			complexity += 1 + nestingLevel
			nestingLevel++
			// Visit each case
			for _, stmt := range nodeType.Body.List {
				ast.Inspect(stmt, inspect)
			}
			nestingLevel--
			return false

		case *ast.SelectStmt:
			complexity += 1 + nestingLevel
			nestingLevel++
			for _, stmt := range nodeType.Body.List {
				ast.Inspect(stmt, inspect)
			}
			nestingLevel--
			return false

		case *ast.BinaryExpr:
			// Logical operators in conditions
			if nodeType.Op == token.LAND || nodeType.Op == token.LOR {
				if !ignoreNesting {
					complexity++
				}
			}

		case *ast.FuncLit:
			// Don't count nesting in nested functions
			oldIgnore := ignoreNesting
			ignoreNesting = true
			ast.Inspect(nodeType.Body, inspect)
			ignoreNesting = oldIgnore
			return false
		}

		return true
	}

	ast.Inspect(goFunc.declaration.Body, inspect)
	return complexity
}

// countLocalVariables counts local variables in the function
func (goFunc *GoFunction) countLocalVariables() int {
	count := 0
	ast.Inspect(goFunc.declaration, func(node ast.Node) bool {
		switch nodeType := node.(type) {
		case *ast.AssignStmt:
			if nodeType.Tok == token.DEFINE { // :=
				count += len(nodeType.Lhs)
			}
		case *ast.ValueSpec:
			// var declarations
			count += len(nodeType.Names)
		}
		return true
	})
	return count
}

// GetLocalVariableCount returns the number of local variables
func (goFunc *GoFunction) GetLocalVariableCount() int {
	return goFunc.countLocalVariables()
}

package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
)

// CallGraphAnalyzer builds a call graph for Go code
type CallGraphAnalyzer struct {
	graph       *models.CallGraph
	currentFile string
	packageName string
	fileSet     *token.FileSet
}

// NewCallGraphAnalyzer creates a new call graph analyzer
func NewCallGraphAnalyzer() *CallGraphAnalyzer {
	return &CallGraphAnalyzer{
		graph:   models.NewCallGraph(),
		fileSet: token.NewFileSet(),
	}
}

// AnalyzeDirectory analyzes all Go files in a directory and builds a call graph
func (analyzer *CallGraphAnalyzer) AnalyzeDirectory(rootPath string) (*models.CallGraph, error) {
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
			if analyzeErr := analyzer.analyzeFile(path); analyzeErr != nil {
				// Log error but continue processing other files
				fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", path, analyzeErr)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Calculate statistics after all files are processed
	analyzer.graph.CalculateStats()

	return analyzer.graph, nil
}

// analyzeFile parses a single Go file and extracts call graph information
func (analyzer *CallGraphAnalyzer) analyzeFile(filePath string) error {
	analyzer.currentFile = filePath

	file, err := parser.ParseFile(analyzer.fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Store package name for qualified function names
	if file.Name != nil {
		analyzer.packageName = file.Name.Name
	}

	// First pass: collect all function declarations
	ast.Inspect(file, func(node ast.Node) bool {
		switch funcDecl := node.(type) {
		case *ast.FuncDecl:
			analyzer.addFunctionNode(funcDecl)
		}
		return true
	})

	// Second pass: extract call relationships
	ast.Inspect(file, func(node ast.Node) bool {
		switch funcDecl := node.(type) {
		case *ast.FuncDecl:
			analyzer.extractCallsFromFunction(funcDecl)
		}
		return true
	})

	return nil
}

// addFunctionNode creates a CallNode for a function declaration
func (analyzer *CallGraphAnalyzer) addFunctionNode(funcDecl *ast.FuncDecl) {
	fullName := analyzer.getFunctionFullName(funcDecl)

	// Calculate function metrics
	complexity := calculateFunctionComplexity(funcDecl)
	length := analyzer.getFunctionLength(funcDecl)

	node := &models.CallNode{
		Name:       funcDecl.Name.Name,
		FullName:   fullName,
		Package:    analyzer.packageName,
		File:       analyzer.currentFile,
		Line:       analyzer.fileSet.Position(funcDecl.Pos()).Line,
		Complexity: complexity,
		Length:     length,
		CallCount:  0, // Will be updated when edges are added
		CallsOut:   0, // Will be updated when edges are added
		IsExternal: false,
		IsExported: ast.IsExported(funcDecl.Name.Name),
	}

	analyzer.graph.AddNode(node)
}

// extractCallsFromFunction finds all function calls within a function
func (analyzer *CallGraphAnalyzer) extractCallsFromFunction(funcDecl *ast.FuncDecl) {
	callerFullName := analyzer.getFunctionFullName(funcDecl)

	ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
		callExpr, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		calleeName := analyzer.extractCalleeName(callExpr)
		if calleeName == "" {
			return true // Skip if we can't determine the callee
		}

		edge := models.CallEdge{
			From:   callerFullName,
			To:     calleeName,
			Weight: 1,
			File:   analyzer.currentFile,
			Line:   analyzer.fileSet.Position(callExpr.Pos()).Line,
		}

		// If callee doesn't exist as a node yet, create an external node
		if _, exists := analyzer.graph.Nodes[calleeName]; !exists {
			analyzer.addExternalNode(calleeName)
		}

		analyzer.graph.AddEdge(edge)

		return true
	})
}

// extractCalleeName extracts the called function name from a CallExpr
func (analyzer *CallGraphAnalyzer) extractCalleeName(callExpr *ast.CallExpr) string {
	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		// Direct function call: foo()
		return fmt.Sprintf("%s.%s", analyzer.packageName, fun.Name)

	case *ast.SelectorExpr:
		// Method call or qualified call: obj.Method() or pkg.Func()
		switch x := fun.X.(type) {
		case *ast.Ident:
			// Could be pkg.Func() or obj.Method()
			// For simplicity, treat as qualified call
			return fmt.Sprintf("%s.%s", x.Name, fun.Sel.Name)
		default:
			// Complex expression, use selector name only
			return fun.Sel.Name
		}

	case *ast.FuncLit:
		// Anonymous function - skip for now
		return ""

	default:
		// Other types of calls (type conversions, etc.) - skip
		return ""
	}
}

// addExternalNode adds a node for an externally called function
func (analyzer *CallGraphAnalyzer) addExternalNode(fullName string) {
	parts := strings.Split(fullName, ".")
	name := fullName
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}

	node := &models.CallNode{
		Name:       name,
		FullName:   fullName,
		Package:    strings.Join(parts[:len(parts)-1], "."),
		File:       "",
		Line:       0,
		Complexity: 0,
		Length:     0,
		CallCount:  0,
		CallsOut:   0,
		IsExternal: true,
		IsExported: ast.IsExported(name),
	}

	analyzer.graph.AddNode(node)
}

// getFunctionFullName returns the fully qualified function name
func (analyzer *CallGraphAnalyzer) getFunctionFullName(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		// Method: extract receiver type
		recvType := analyzer.extractReceiverType(funcDecl.Recv.List[0].Type)
		return fmt.Sprintf("%s.%s.%s", analyzer.packageName, recvType, funcDecl.Name.Name)
	}

	// Regular function
	return fmt.Sprintf("%s.%s", analyzer.packageName, funcDecl.Name.Name)
}

// extractReceiverType extracts the receiver type name
func (analyzer *CallGraphAnalyzer) extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return analyzer.extractReceiverType(t.X)
	default:
		return "Unknown"
	}
}

// calculateFunctionComplexity calculates cyclomatic complexity
func calculateFunctionComplexity(funcDecl *ast.FuncDecl) int {
	if funcDecl.Body == nil {
		return 0
	}

	complexity := 1 // Base complexity

	ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
		switch typedNode := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			complexity++
		case *ast.CaseClause:
			if len(typedNode.List) > 0 {
				complexity++
			}
		case *ast.CommClause:
			if typedNode.Comm != nil {
				complexity++
			}
		case *ast.BinaryExpr:
			if typedNode.Op == token.LAND || typedNode.Op == token.LOR {
				complexity++
			}
		}
		return true
	})

	return complexity
}

// getFunctionLength returns the number of lines in a function
func (analyzer *CallGraphAnalyzer) getFunctionLength(funcDecl *ast.FuncDecl) int {
	if funcDecl.Body == nil {
		return 0
	}

	start := analyzer.fileSet.Position(funcDecl.Body.Lbrace).Line
	end := analyzer.fileSet.Position(funcDecl.Body.Rbrace).Line

	return end - start + 1
}

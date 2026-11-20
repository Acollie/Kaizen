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

// CallGraphAnalyzer builds a call graph for Go code.
//
// It works in two passes over the whole directory: the first parses every file and
// registers all function declarations, the second resolves call expressions against
// that complete set. Resolution is syntactic (no type checking), using, in order:
// the file's imports, the enclosing method's receiver name, and method names that
// are declared exactly once in the analyzed code. That last guess is skipped for
// common interface method names (String, Close, ...) and calls rooted in an import,
// and the edges it produces are marked Inferred.
type CallGraphAnalyzer struct {
	graph       *models.CallGraph
	currentFile string
	packageName string
	fileSet     *token.FileSet

	// methodsByName maps a method name to the full names of every method declaring it
	methodsByName map[string][]string
}

// parsedGoFile is a file parsed in the first pass and resolved in the second
type parsedGoFile struct {
	path    string
	syntax  *ast.File
	imports map[string]string // local import name -> package name used in node keys
}

// NewCallGraphAnalyzer creates a new call graph analyzer
func NewCallGraphAnalyzer() *CallGraphAnalyzer {
	return &CallGraphAnalyzer{
		graph:         models.NewCallGraph(),
		fileSet:       token.NewFileSet(),
		methodsByName: make(map[string][]string),
	}
}

// AnalyzeDirectory analyzes all Go files in a directory and builds a call graph
func (analyzer *CallGraphAnalyzer) AnalyzeDirectory(rootPath string) (*models.CallGraph, error) {
	files, err := analyzer.parseDirectory(rootPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		analyzer.registerDeclarations(file)
	}
	for _, file := range files {
		analyzer.extractCalls(file)
	}

	analyzer.graph.RecomputeCallCounts()
	analyzer.graph.CalculateStats()

	return analyzer.graph, nil
}

// parseDirectory parses every non-test Go file under rootPath, skipping hidden,
// vendor and testdata directories
func (analyzer *CallGraphAnalyzer) parseDirectory(rootPath string) ([]*parsedGoFile, error) {
	var files []*parsedGoFile

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != rootPath && shouldSkipDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Never follow symlinks or read devices/FIFOs or huge files: analyzed code may
		// be untrusted (a PR checkout), and a link to /dev/zero would exhaust memory.
		if !info.Mode().IsRegular() || info.Size() > maxParsedFileBytes {
			return nil
		}

		parsed, parseErr := analyzer.parseFile(path)
		if parseErr != nil {
			// Log error but continue processing other files
			fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", path, parseErr)
			return nil
		}
		files = append(files, parsed)
		return nil
	})

	return files, err
}

// maxParsedFileBytes caps the size of a Go file the call graph will parse
const maxParsedFileBytes = 2 << 20

// shouldSkipDirectory reports whether a directory never contains analyzable first-party code
func shouldSkipDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules"
}

// parseFile parses one Go file and records its imports
func (analyzer *CallGraphAnalyzer) parseFile(filePath string) (*parsedGoFile, error) {
	syntax, err := parser.ParseFile(analyzer.fileSet, filePath, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	return &parsedGoFile{path: filePath, syntax: syntax, imports: importNames(syntax)}, nil
}

// importNames maps each import's local name to the package name used in node keys.
// Without type information the package name is assumed to be the last path element.
func importNames(syntax *ast.File) map[string]string {
	names := make(map[string]string)
	for _, spec := range syntax.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		packageName := importPath[strings.LastIndex(importPath, "/")+1:]
		localName := packageName
		if spec.Name != nil {
			localName = spec.Name.Name
		}
		if localName == "_" || localName == "." {
			continue
		}
		names[localName] = packageName
	}
	return names
}

// enterFile sets the per-file state used while building names
func (analyzer *CallGraphAnalyzer) enterFile(file *parsedGoFile) {
	analyzer.currentFile = file.path
	analyzer.packageName = file.syntax.Name.Name
}

// registerDeclarations adds a node for every function and method in the file (first pass)
func (analyzer *CallGraphAnalyzer) registerDeclarations(file *parsedGoFile) {
	analyzer.enterFile(file)
	for _, declaration := range file.syntax.Decls {
		funcDecl, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		analyzer.addFunctionNode(funcDecl)
		if funcDecl.Recv != nil {
			methodName := funcDecl.Name.Name
			fullName := analyzer.getFunctionFullName(funcDecl)
			analyzer.methodsByName[methodName] = append(analyzer.methodsByName[methodName], fullName)
		}
	}
}

// extractCalls adds edges for every call made in the file (second pass)
func (analyzer *CallGraphAnalyzer) extractCalls(file *parsedGoFile) {
	analyzer.enterFile(file)
	for _, declaration := range file.syntax.Decls {
		if funcDecl, ok := declaration.(*ast.FuncDecl); ok && funcDecl.Body != nil {
			analyzer.extractCallsFromFunction(funcDecl, file.imports)
		}
	}
}

// addFunctionNode creates a CallNode for a function declaration
func (analyzer *CallGraphAnalyzer) addFunctionNode(funcDecl *ast.FuncDecl) {
	fullName := analyzer.getFunctionFullName(funcDecl)

	node := &models.CallNode{
		Name:       funcDecl.Name.Name,
		FullName:   fullName,
		Package:    analyzer.packageName,
		File:       analyzer.currentFile,
		Line:       analyzer.fileSet.Position(funcDecl.Pos()).Line,
		Complexity: calculateFunctionComplexity(funcDecl),
		Length:     analyzer.getFunctionLength(funcDecl),
		IsExternal: false,
		IsExported: ast.IsExported(funcDecl.Name.Name),
	}

	analyzer.graph.AddNode(node)
}

// extractCallsFromFunction finds all function calls within a function
func (analyzer *CallGraphAnalyzer) extractCallsFromFunction(funcDecl *ast.FuncDecl, imports map[string]string) {
	callerFullName := analyzer.getFunctionFullName(funcDecl)
	scope := callScope{imports: imports, receiverName: receiverName(funcDecl), receiverType: ""}
	if scope.receiverName != "" {
		scope.receiverType = analyzer.extractReceiverType(funcDecl.Recv.List[0].Type)
	}

	ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
		callExpr, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		calleeName, inferred := analyzer.resolveCallee(callExpr, scope)
		if calleeName == "" {
			return true // builtins, conversions and anonymous functions
		}

		if _, exists := analyzer.graph.Nodes[calleeName]; !exists {
			analyzer.addExternalNode(calleeName)
		}

		analyzer.graph.AddEdge(models.CallEdge{
			From:     callerFullName,
			To:       calleeName,
			Weight:   1,
			File:     analyzer.currentFile,
			Line:     analyzer.fileSet.Position(callExpr.Pos()).Line,
			Inferred: inferred,
		})

		return true
	})
}

// callScope is what call resolution needs to know about the enclosing function
type callScope struct {
	imports      map[string]string
	receiverName string // e.g. "analyzer" in func (analyzer *CallGraphAnalyzer) ...
	receiverType string // e.g. "CallGraphAnalyzer"
}

// receiverName returns the name bound to a method's receiver, or "" for functions
func receiverName(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 || len(funcDecl.Recv.List[0].Names) == 0 {
		return ""
	}
	return funcDecl.Recv.List[0].Names[0].Name
}

// resolveCallee returns the node key for the function a call expression targets,
// or "" when the call should not be recorded. inferred marks a name-only guess.
func (analyzer *CallGraphAnalyzer) resolveCallee(callExpr *ast.CallExpr, scope callScope) (callee string, inferred bool) {
	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		return analyzer.resolveIdentCall(fun.Name), false
	case *ast.SelectorExpr:
		return analyzer.resolveSelectorCall(fun, scope)
	case *ast.IndexExpr:
		// Explicitly instantiated generic: Map[int](...)
		if ident, ok := fun.X.(*ast.Ident); ok {
			return analyzer.resolveIdentCall(ident.Name), false
		}
		return "", false
	default:
		// Anonymous functions, calls on call results, etc.
		return "", false
	}
}

// resolveIdentCall resolves foo() to a same-package function, skipping builtins
func (analyzer *CallGraphAnalyzer) resolveIdentCall(name string) string {
	fullName := analyzer.packageName + "." + name
	if _, exists := analyzer.graph.Nodes[fullName]; exists {
		return fullName
	}
	if goBuiltins[name] {
		return ""
	}
	// Likely a type conversion (MyType(x)) or a function from a dot-import
	return ""
}

// resolveSelectorCall resolves pkg.Func(), receiver.Method() and value.Method().
// inferred is true when the target was guessed from a method name declared only
// once in the analyzed code, rather than from an import or the enclosing receiver.
func (analyzer *CallGraphAnalyzer) resolveSelectorCall(selector *ast.SelectorExpr, scope callScope) (callee string, inferred bool) {
	methodName := selector.Sel.Name

	if ident, ok := selector.X.(*ast.Ident); ok {
		if packageName, isImport := scope.imports[ident.Name]; isImport {
			return packageName + "." + methodName, false
		}
		if scope.receiverName != "" && ident.Name == scope.receiverName {
			candidate := analyzer.packageName + "." + scope.receiverType + "." + methodName
			if _, exists := analyzer.graph.Nodes[candidate]; exists {
				return candidate, false
			}
		}
	}

	if analyzer.canInferUniqueMethod(selector, scope) {
		if candidates := analyzer.methodsByName[methodName]; len(candidates) == 1 {
			return candidates[0], true
		}
	}

	// Ambiguous or external method: keep a readable placeholder
	if ident, ok := selector.X.(*ast.Ident); ok {
		return ident.Name + "." + methodName, false
	}
	return methodName, false
}

// canInferUniqueMethod reports whether value.Method() may be resolved by method name
// alone. Without type information that guess is only safe for distinctive names:
// String, Close or Write are implemented all over the standard library, so a single
// repo method with that name would otherwise collect every buf.String() call. Calls
// rooted in an imported package (os.Stdout.Write) are never repo methods either.
func (analyzer *CallGraphAnalyzer) canInferUniqueMethod(selector *ast.SelectorExpr, scope callScope) bool {
	if commonMethodNames[selector.Sel.Name] {
		return false
	}
	root := rootIdentifier(selector.X)
	if root == nil {
		return true
	}
	_, isImport := scope.imports[root.Name]
	return !isImport
}

// rootIdentifier returns the left-most identifier of a selector chain such as
// os.Stdout.Write or items[0].field.Method, or nil when the chain starts elsewhere
func rootIdentifier(expr ast.Expr) *ast.Ident {
	for {
		switch typed := expr.(type) {
		case *ast.Ident:
			return typed
		case *ast.SelectorExpr:
			expr = typed.X
		case *ast.CallExpr:
			expr = typed.Fun
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.StarExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		default:
			return nil
		}
	}
}

// commonMethodNames are method names implemented by many standard-library and
// interface types; a call to one of them is never resolved by name alone
var commonMethodNames = map[string]bool{
	"String": true, "Error": true, "Close": true, "Read": true, "Write": true, "Len": true,
	"Less": true, "Swap": true, "Get": true, "Set": true, "Run": true, "Do": true, "Next": true,
	"Reset": true, "Lock": true, "Unlock": true, "RLock": true, "RUnlock": true, "Wait": true,
	"Add": true, "Done": true, "Stop": true, "Start": true, "Bytes": true, "Value": true,
	"Key": true, "Name": true, "Load": true, "Store": true, "Scan": true, "Format": true,
	"Copy": true, "Clone": true, "Equal": true, "ServeHTTP": true, "Header": true,
	"WriteHeader": true, "Unwrap": true, "Is": true, "As": true, "Flush": true, "Sync": true,
	"Seek": true, "ReadFrom": true, "WriteTo": true, "WriteString": true, "WriteByte": true,
	"WriteRune": true, "ReadByte": true, "ReadRune": true, "ReadString": true, "ReadLine": true,
	"MarshalJSON": true, "UnmarshalJSON": true, "MarshalText": true, "UnmarshalText": true,
	"Err": true, "Deadline": true, "Delete": true, "Range": true, "Len64": true, "Cap": true,
	"Type": true, "Kind": true, "Interface": true, "Elem": true, "Index": true, "Path": true,
	"Open": true, "Execute": true, "Parse": true,
	"Size": true, "Mode": true, "ModTime": true, "IsDir": true, "Sys": true,
	"Info": true, "Debug": true, "Warn": true, "Printf": true, "Println": true, "Print": true,
	"Fatal": true, "Fatalf": true, "Errorf": true, "Logf": true, "Log": true, "Helper": true,
	"Cleanup": true, "Context": true, "WithContext": true, "Query": true, "Exec": true,
	"Commit": true, "Rollback": true, "Ping": true, "Count": true, "Push": true, "Pop": true,
	"Peek": true, "Contains": true, "Keys": true, "Values": true, "Items": true, "Send": true,
	"Recv": true, "Accept": true, "Dial": true, "Listen": true, "Serve": true, "Shutdown": true,
	"Handle": true, "Handler": true, "Hash": true, "Sum": true, "Encode": true, "Decode": true,
	"Lookup": true, "Match": true, "Update": true, "Insert": true, "Remove": true,
}

// goBuiltins are predeclared functions that never become call graph nodes
var goBuiltins = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "complex": true, "copy": true,
	"delete": true, "imag": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "panic": true, "print": true, "println": true, "real": true, "recover": true,
}

// addExternalNode adds a node for a function that is not declared in the analyzed code
func (analyzer *CallGraphAnalyzer) addExternalNode(fullName string) {
	parts := strings.Split(fullName, ".")
	name := parts[len(parts)-1]

	node := &models.CallNode{
		Name:       name,
		FullName:   fullName,
		Package:    strings.Join(parts[:len(parts)-1], "."),
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

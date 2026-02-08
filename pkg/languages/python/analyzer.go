package python

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// PythonAnalyzer implements the LanguageAnalyzer interface for Python
type PythonAnalyzer struct {
	language *sitter.Language
}

// NewPythonAnalyzer creates a new Python analyzer
func NewPythonAnalyzer() analyzer.LanguageAnalyzer {
	return &PythonAnalyzer{
		language: python.GetLanguage(),
	}
}

// Name returns the language name
func (pyAnalyzer *PythonAnalyzer) Name() string {
	return "Python"
}

// FileExtensions returns the file extensions this analyzer handles
func (pyAnalyzer *PythonAnalyzer) FileExtensions() []string {
	return []string{".py"}
}

// CanAnalyze checks if this analyzer can handle the given file
func (pyAnalyzer *PythonAnalyzer) CanAnalyze(filePath string) bool {
	ext := filepath.Ext(filePath)
	for _, supportedExt := range pyAnalyzer.FileExtensions() {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// IsStub indicates if this is a stub implementation
func (pyAnalyzer *PythonAnalyzer) IsStub() bool {
	return false // Python analyzer is fully implemented
}

// AnalyzeFile performs full analysis on a single Python file
func (pyAnalyzer *PythonAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
	sourceBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	sourceCode := string(sourceBytes)

	// Keep existing line counting (works well for docstrings/comments)
	totalLines, codeLines, commentLines, blankLines := pyAnalyzer.countLines(sourceCode)
	commentDensity := 0.0
	if totalLines > 0 {
		commentDensity = float64(commentLines) / float64(totalLines) * 100
	}

	// Keep existing import counting (simple and effective)
	importCount := pyAnalyzer.countImports(sourceCode)

	// Parse with tree-sitter
	parser := sitter.NewParser()
	parser.SetLanguage(pyAnalyzer.language)
	tree, err := parser.ParseCtx(context.Background(), nil, sourceBytes)
	if err != nil || tree == nil {
		return nil, fmt.Errorf("failed to parse Python file: %w", err)
	}
	defer tree.Close()

	// Extract functions using AST
	functions := pyAnalyzer.extractFunctions(tree.RootNode(), sourceBytes)

	// Extract types using AST
	types := pyAnalyzer.extractTypes(tree.RootNode(), sourceBytes)

	return &models.FileAnalysis{
		Path:                  filePath,
		Language:              pyAnalyzer.Name(),
		TotalLines:            totalLines,
		CodeLines:             codeLines,
		CommentLines:          commentLines,
		BlankLines:            blankLines,
		CommentDensity:        commentDensity,
		DuplicatedLines:       0,
		DuplicationPercentage: 0,
		ImportCount:           importCount,
		Functions:             functions,
		Types:                 types,
	}, nil
}

// countLines counts different types of lines in Python source
func (pyAnalyzer *PythonAnalyzer) countLines(sourceCode string) (total, code, comment, blank int) {
	lines := strings.Split(sourceCode, "\n")
	total = len(lines)

	inMultilineString := false
	multilineDelimiter := ""

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" {
			blank++
			continue
		}

		// Handle multiline strings (docstrings)
		if inMultilineString {
			comment++
			if strings.Contains(line, multilineDelimiter) {
				inMultilineString = false
				multilineDelimiter = ""
			}
			continue
		}

		// Check for start of multiline string
		if strings.HasPrefix(trimmedLine, `"""`) || strings.HasPrefix(trimmedLine, `'''`) {
			delimiter := trimmedLine[:3]
			// Check if it ends on the same line
			restOfLine := trimmedLine[3:]
			if strings.Contains(restOfLine, delimiter) {
				comment++
			} else {
				inMultilineString = true
				multilineDelimiter = delimiter
				comment++
			}
			continue
		}

		// Single-line comment
		if strings.HasPrefix(trimmedLine, "#") {
			comment++
			continue
		}

		code++
	}

	return
}

// countImports counts import statements
func (pyAnalyzer *PythonAnalyzer) countImports(sourceCode string) int {
	importPattern := regexp.MustCompile(`(?m)^(?:from\s+\S+\s+)?import\s+`)
	matches := importPattern.FindAllString(sourceCode, -1)
	return len(matches)
}

// extractFunctions extracts and analyzes all functions using AST walking
func (pyAnalyzer *PythonAnalyzer) extractFunctions(rootNode *sitter.Node, sourceBytes []byte) []models.FunctionAnalysis {
	var functions []models.FunctionAnalysis

	cursor := sitter.NewTreeCursor(rootNode)
	defer cursor.Close()

	pyAnalyzer.walkFunctions(cursor, sourceBytes, &functions)
	return functions
}

// walkFunctions recursively walks the AST to find all function definitions
func (pyAnalyzer *PythonAnalyzer) walkFunctions(cursor *sitter.TreeCursor, sourceBytes []byte, functions *[]models.FunctionAnalysis) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Handle both regular and async functions
	if nodeType == "function_definition" || nodeType == "async_function_definition" {
		funcAnalysis := pyAnalyzer.analyzeFunctionNode(node, sourceBytes)
		*functions = append(*functions, funcAnalysis)
	}

	// Handle decorated functions (decorators wrap the function_definition node)
	if nodeType == "decorated_definition" {
		// Find the actual function definition within the decorator
		decoratedCursor := sitter.NewTreeCursor(node)
		if decoratedCursor.GoToFirstChild() {
			for {
				childNode := decoratedCursor.CurrentNode()
				childType := childNode.Type()
				if childType == "function_definition" || childType == "async_function_definition" {
					funcAnalysis := pyAnalyzer.analyzeFunctionNode(childNode, sourceBytes)
					*functions = append(*functions, funcAnalysis)
					break
				}
				if !decoratedCursor.GoToNextSibling() {
					break
				}
			}
		}
		decoratedCursor.Close()
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pyAnalyzer.walkFunctions(cursor, sourceBytes, functions)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// analyzeFunctionNode analyzes a single function node
func (pyAnalyzer *PythonAnalyzer) analyzeFunctionNode(node *sitter.Node, sourceBytes []byte) models.FunctionAnalysis {
	pythonFunc := NewPythonFunction(node, sourceBytes)

	// Calculate Halstead metrics
	funcCode := node.Content(sourceBytes)
	halsteadVol, halsteadDiff := pyAnalyzer.calculateHalsteadMetrics(funcCode)

	// Calculate maintainability index
	maintainabilityIndex := pyAnalyzer.calculateMaintainabilityIndex(
		halsteadVol,
		pythonFunc.CalculateCyclomaticComplexity(),
		pythonFunc.LineCount(),
	)

	return models.FunctionAnalysis{
		Name:                 pythonFunc.Name(),
		StartLine:            pythonFunc.StartLine(),
		EndLine:              pythonFunc.EndLine(),
		Length:               pythonFunc.LineCount(),
		LogicalLines:         pythonFunc.LogicalLineCount(),
		ParameterCount:       pythonFunc.ParameterCount(),
		LocalVariableCount:   pythonFunc.CountLocalVariables(),
		ReturnCount:          pythonFunc.ReturnCount(),
		CyclomaticComplexity: pythonFunc.CalculateCyclomaticComplexity(),
		CognitiveComplexity:  pythonFunc.CalculateCognitiveComplexity(),
		NestingDepth:         pythonFunc.MaxNestingDepth(),
		HalsteadVolume:       halsteadVol,
		HalsteadDifficulty:   halsteadDiff,
		MaintainabilityIndex: maintainabilityIndex,
		FanIn:                0,
		FanOut:               pythonFunc.CountFunctionCalls(),
	}
}


// calculateHalsteadMetrics calculates Halstead complexity metrics for Python
func (pyAnalyzer *PythonAnalyzer) calculateHalsteadMetrics(funcCode string) (volume, difficulty float64) {
	operators := make(map[string]bool)
	operands := make(map[string]bool)
	totalOperators := 0
	totalOperands := 0

	// Python operators
	operatorPatterns := []string{
		`\+`, `-`, `\*`, `/`, `//`, `%`, `\*\*`,
		`==`, `!=`, `<`, `>`, `<=`, `>=`,
		`=`, `\+=`, `-=`, `\*=`, `/=`,
		`and`, `or`, `not`, `in`, `is`,
		`\[`, `\]`, `\(`, `\)`, `\{`, `\}`,
		`:`, `,`, `\.`, `->`,
		`if`, `else`, `elif`, `for`, `while`, `try`, `except`, `return`, `def`, `class`,
	}

	for _, op := range operatorPatterns {
		re := regexp.MustCompile(op)
		matches := re.FindAllString(funcCode, -1)
		if len(matches) > 0 {
			operators[op] = true
			totalOperators += len(matches)
		}
	}

	// Operands: identifiers and literals
	identPattern := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\b`)
	identMatches := identPattern.FindAllStringSubmatch(funcCode, -1)
	keywords := map[string]bool{
		"if": true, "else": true, "elif": true, "for": true, "while": true,
		"try": true, "except": true, "finally": true, "return": true,
		"def": true, "class": true, "import": true, "from": true,
		"and": true, "or": true, "not": true, "in": true, "is": true,
		"True": true, "False": true, "None": true,
		"pass": true, "break": true, "continue": true, "raise": true,
		"with": true, "as": true, "global": true, "nonlocal": true,
		"lambda": true, "yield": true, "assert": true, "del": true,
	}

	for _, match := range identMatches {
		if len(match) > 1 && !keywords[match[1]] {
			operands[match[1]] = true
			totalOperands++
		}
	}

	// Literals
	literalPatterns := []string{
		`\d+\.?\d*`,       // Numbers
		`"[^"]*"`,         // Double-quoted strings
		`'[^']*'`,         // Single-quoted strings
		`"""[\s\S]*?"""`,  // Triple double-quoted
		`'''[\s\S]*?'''`,  // Triple single-quoted
	}

	for _, pattern := range literalPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(funcCode, -1)
		for _, match := range matches {
			operands[match] = true
			totalOperands++
		}
	}

	distinctOperators := len(operators)
	distinctOperands := len(operands)

	if distinctOperators == 0 || distinctOperands == 0 {
		return 0, 0
	}

	vocab := float64(distinctOperators + distinctOperands)
	length := float64(totalOperators + totalOperands)

	if vocab > 0 && length > 0 {
		volume = length * math.Log2(vocab)
		difficulty = (float64(distinctOperators) / 2.0) * (float64(totalOperands) / float64(distinctOperands))
	}

	return volume, difficulty
}


// calculateMaintainabilityIndex calculates the maintainability index
func (pyAnalyzer *PythonAnalyzer) calculateMaintainabilityIndex(halsteadVolume float64, cyclomaticComplexity int, linesOfCode int) float64 {
	if linesOfCode == 0 {
		return 100
	}

	// MI = 171 - 5.2 * ln(HV) - 0.23 * CC - 16.2 * ln(LOC)
	hvTerm := 0.0
	if halsteadVolume > 0 {
		hvTerm = 5.2 * math.Log(halsteadVolume)
	}

	ccTerm := 0.23 * float64(cyclomaticComplexity)
	locTerm := 16.2 * math.Log(float64(linesOfCode))

	maintainabilityIndex := 171 - hvTerm - ccTerm - locTerm

	// Normalize to 0-100
	if maintainabilityIndex < 0 {
		maintainabilityIndex = 0
	}
	if maintainabilityIndex > 100 {
		maintainabilityIndex = 100
	}

	return maintainabilityIndex
}

// extractTypes extracts class definitions using AST
func (pyAnalyzer *PythonAnalyzer) extractTypes(rootNode *sitter.Node, sourceBytes []byte) []models.TypeAnalysis {
	var types []models.TypeAnalysis

	cursor := sitter.NewTreeCursor(rootNode)
	defer cursor.Close()

	pyAnalyzer.walkTypes(cursor, sourceBytes, &types)
	return types
}

// walkTypes recursively walks the AST to find all class definitions
func (pyAnalyzer *PythonAnalyzer) walkTypes(cursor *sitter.TreeCursor, sourceBytes []byte, types *[]models.TypeAnalysis) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	if nodeType == "class_definition" {
		typeAnalysis := pyAnalyzer.analyzeClassNode(node, sourceBytes)
		*types = append(*types, typeAnalysis)
	}

	// Handle decorated classes
	if nodeType == "decorated_definition" {
		decoratedCursor := sitter.NewTreeCursor(node)
		if decoratedCursor.GoToFirstChild() {
			for {
				childNode := decoratedCursor.CurrentNode()
				if childNode.Type() == "class_definition" {
					typeAnalysis := pyAnalyzer.analyzeClassNode(childNode, sourceBytes)
					*types = append(*types, typeAnalysis)
					break
				}
				if !decoratedCursor.GoToNextSibling() {
					break
				}
			}
		}
		decoratedCursor.Close()
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pyAnalyzer.walkTypes(cursor, sourceBytes, types)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// analyzeClassNode analyzes a single class node
func (pyAnalyzer *PythonAnalyzer) analyzeClassNode(node *sitter.Node, sourceBytes []byte) models.TypeAnalysis {
	className := pyAnalyzer.extractClassName(node, sourceBytes)
	methodCount := pyAnalyzer.countMethods(node)

	return models.TypeAnalysis{
		Name:                    className,
		Kind:                    "class",
		AfferentCoupling:        0,
		EfferentCoupling:        0,
		Instability:             0,
		LCOM:                    0,
		DepthOfInheritance:      0,
		NumberOfChildren:        0,
		MethodCount:             methodCount,
		WeightedMethodsPerClass: 0,
		PublicMethodCount:       0,
	}
}

// extractClassName extracts the class name from a class_definition node
func (pyAnalyzer *PythonAnalyzer) extractClassName(node *sitter.Node, sourceBytes []byte) string {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			childNode := cursor.CurrentNode()
			if childNode.Type() == "identifier" {
				return childNode.Content(sourceBytes)
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return "unknown"
}

// countMethods counts methods in a class
func (pyAnalyzer *PythonAnalyzer) countMethods(classNode *sitter.Node) int {
	count := 0
	cursor := sitter.NewTreeCursor(classNode)
	defer cursor.Close()

	pyAnalyzer.countMethodsRecursive(cursor, &count)
	return count
}

// countMethodsRecursive recursively counts function definitions within a class
func (pyAnalyzer *PythonAnalyzer) countMethodsRecursive(cursor *sitter.TreeCursor, count *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Count function definitions (methods)
	if nodeType == "function_definition" || nodeType == "async_function_definition" {
		*count++
		// Don't recurse into nested functions within methods
		return
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pyAnalyzer.countMethodsRecursive(cursor, count)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// ReadFileByLine reads a file line by line for efficient processing
func ReadFileByLine(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

package kotlin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// KotlinAnalyzer implements the LanguageAnalyzer interface for Kotlin
type KotlinAnalyzer struct {
	language *sitter.Language
}

// NewKotlinAnalyzer creates a new Kotlin analyzer
func NewKotlinAnalyzer() analyzer.LanguageAnalyzer {
	return &KotlinAnalyzer{
		language: kotlin.GetLanguage(),
	}
}

// Name returns the language name
func (kotlinAnalyzer *KotlinAnalyzer) Name() string {
	return "Kotlin"
}

// FileExtensions returns the file extensions this analyzer handles
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

// IsStub indicates if this is a stub implementation
func (kotlinAnalyzer *KotlinAnalyzer) IsStub() bool {
	return false
}

// AnalyzeFile performs full analysis on a single Kotlin file
func (kotlinAnalyzer *KotlinAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
	// Read source code
	sourceBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	sourceCode := string(sourceBytes)

	// Count lines
	totalLines, codeLines, commentLines, blankLines := kotlinAnalyzer.countLines(sourceCode)

	// Calculate comment density
	commentDensity := 0.0
	if totalLines > 0 {
		commentDensity = float64(commentLines) / float64(totalLines) * 100
	}

	// Count imports
	importCount := kotlinAnalyzer.countImports(sourceCode)

	// Parse with tree-sitter
	parser := sitter.NewParser()
	parser.SetLanguage(kotlinAnalyzer.language)
	tree, err := parser.ParseCtx(context.Background(), nil, sourceBytes)
	if err != nil || tree == nil {
		return nil, fmt.Errorf("failed to parse Kotlin file")
	}
	defer tree.Close()

	// Extract and analyze functions
	functions := kotlinAnalyzer.extractFunctions(tree.RootNode(), sourceBytes)

	// Analyze types (classes, interfaces)
	types := kotlinAnalyzer.extractTypes(tree.RootNode())

	return &models.FileAnalysis{
		Path:                  filePath,
		Language:              kotlinAnalyzer.Name(),
		TotalLines:            totalLines,
		CodeLines:             codeLines,
		CommentLines:          commentLines,
		BlankLines:            blankLines,
		CommentDensity:        commentDensity,
		DuplicatedLines:       0, // TODO: Implement duplication detection
		DuplicationPercentage: 0,
		ImportCount:           importCount,
		Functions:             functions,
		Types:                 types,
	}, nil
}

// countLines counts different types of lines in the source
func (kotlinAnalyzer *KotlinAnalyzer) countLines(sourceCode string) (total, code, comment, blank int) {
	lines := strings.Split(sourceCode, "\n")
	total = len(lines)

	inBlockComment := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for block comments
		if strings.Contains(trimmedLine, "/*") {
			inBlockComment = true
		}

		if inBlockComment {
			comment++
			if strings.Contains(trimmedLine, "*/") {
				inBlockComment = false
			}
		} else if trimmedLine == "" {
			blank++
		} else if strings.HasPrefix(trimmedLine, "//") {
			comment++
		} else {
			code++
		}
	}

	return
}

// countImports counts the number of import statements
func (kotlinAnalyzer *KotlinAnalyzer) countImports(sourceCode string) int {
	lines := strings.Split(sourceCode, "\n")

	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			count++
		}
	}
	return count
}

// extractFunctions extracts and analyzes all functions in the file using AST
func (kotlinAnalyzer *KotlinAnalyzer) extractFunctions(node *sitter.Node, sourceBytes []byte) []models.FunctionAnalysis {
	var functions []models.FunctionAnalysis

	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	kotlinAnalyzer.walkFunctions(cursor, &functions, sourceBytes)

	return functions
}

// walkFunctions recursively walks the AST to find function declarations
func (kotlinAnalyzer *KotlinAnalyzer) walkFunctions(cursor *sitter.TreeCursor, functions *[]models.FunctionAnalysis, sourceBytes []byte) {
	node := cursor.CurrentNode()

	// Check if this is a function declaration
	if node.Type() == "function_declaration" {
		funcAnalysis := kotlinAnalyzer.analyzeFunctionNode(node, sourceBytes)
		if funcAnalysis != nil {
			*functions = append(*functions, *funcAnalysis)
		}
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			kotlinAnalyzer.walkFunctions(cursor, functions, sourceBytes)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// analyzeFunctionNode analyzes a single function declaration node
func (kotlinAnalyzer *KotlinAnalyzer) analyzeFunctionNode(node *sitter.Node, sourceBytes []byte) *models.FunctionAnalysis {
	// Extract function name
	functionName := kotlinAnalyzer.extractFunctionName(node, sourceBytes)
	if functionName == "" {
		return nil
	}

	// Get function boundaries
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	// Get the function body for metric calculations
	functionText := node.Content(sourceBytes)

	// Create function object
	kotlinFunc := NewKotlinFunction(functionName, startLine, endLine, functionText)

	// Calculate metrics
	cyclomaticComplexity := kotlinFunc.CalculateCyclomaticComplexity()
	cognitiveComplexity := kotlinFunc.CalculateCognitiveComplexity()
	halsteadVol, halsteadDiff := kotlinAnalyzer.calculateHalsteadForFunction(functionText)

	// Calculate maintainability index
	maintainabilityIndex := calculateMaintainabilityIndex(
		halsteadVol,
		cyclomaticComplexity,
		kotlinFunc.LineCount(),
	)

	return &models.FunctionAnalysis{
		Name:                 functionName,
		StartLine:            startLine,
		EndLine:              endLine,
		Length:               kotlinFunc.LineCount(),
		LogicalLines:         kotlinFunc.LogicalLineCount(),
		ParameterCount:       kotlinFunc.ParameterCount(),
		LocalVariableCount:   kotlinFunc.GetLocalVariableCount(),
		ReturnCount:          kotlinFunc.ReturnCount(),
		CyclomaticComplexity: cyclomaticComplexity,
		CognitiveComplexity:  cognitiveComplexity,
		NestingDepth:         kotlinFunc.MaxNestingDepth(),
		HalsteadVolume:       halsteadVol,
		HalsteadDifficulty:   halsteadDiff,
		MaintainabilityIndex: maintainabilityIndex,
		FanIn:                0, // TODO: Implement call graph analysis
		FanOut:               kotlinAnalyzer.countFunctionCalls(functionText),
	}
}

// extractFunctionName extracts the name of a function from its AST node
func (kotlinAnalyzer *KotlinAnalyzer) extractFunctionName(node *sitter.Node, sourceBytes []byte) string {
	// Function name is typically the first identifier after the "fun" keyword
	for childIdx := 0; childIdx < int(node.ChildCount()); childIdx++ {
		child := node.Child(childIdx)
		if child == nil {
			continue
		}

		// Look for an identifier node which represents the function name
		if child.Type() == "simple_identifier" {
			return child.Content(sourceBytes)
		}
	}

	return ""
}

// extractTypes extracts and analyzes types (classes, interfaces) from AST
func (kotlinAnalyzer *KotlinAnalyzer) extractTypes(node *sitter.Node) []models.TypeAnalysis {
	var types []models.TypeAnalysis

	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	kotlinAnalyzer.walkTypes(cursor, &types)

	return types
}

// walkTypes recursively walks the AST to find type declarations
func (kotlinAnalyzer *KotlinAnalyzer) walkTypes(cursor *sitter.TreeCursor, types *[]models.TypeAnalysis) {
	node := cursor.CurrentNode()

	// Check if this is a class or interface declaration
	if node.Type() == "class_declaration" || node.Type() == "interface_declaration" || node.Type() == "object_declaration" {
		typeAnalysis := kotlinAnalyzer.analyzeTypeNode(node)
		if typeAnalysis != nil {
			*types = append(*types, *typeAnalysis)
		}
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			kotlinAnalyzer.walkTypes(cursor, types)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// analyzeTypeNode analyzes a single type declaration node
func (kotlinAnalyzer *KotlinAnalyzer) analyzeTypeNode(node *sitter.Node) *models.TypeAnalysis {
	kind := ""
	switch node.Type() {
	case "class_declaration":
		kind = "class"
	case "interface_declaration":
		kind = "interface"
	case "object_declaration":
		kind = "object"
	}

	if kind == "" {
		return nil
	}

	// Extract type name (first identifier child)
	var typeName string
	for childIdx := 0; childIdx < int(node.ChildCount()); childIdx++ {
		child := node.Child(childIdx)
		if child != nil && child.Type() == "simple_identifier" {
			typeName = child.Content(nil)
			break
		}
	}

	if typeName == "" {
		return nil
	}

	return &models.TypeAnalysis{
		Name:                    typeName,
		Kind:                    kind,
		AfferentCoupling:        0, // TODO: Implement coupling analysis
		EfferentCoupling:        0,
		Instability:             0,
		LCOM:                    0, // TODO: Implement cohesion analysis
		DepthOfInheritance:      0,
		NumberOfChildren:        0,
		MethodCount:             0, // Will be filled by method analysis
		WeightedMethodsPerClass: 0,
		PublicMethodCount:       0,
	}
}

// countFunctionCalls counts the number of function calls (fan-out)
func (kotlinAnalyzer *KotlinAnalyzer) countFunctionCalls(functionBody string) int {
	// Simple heuristic: count opening parentheses that follow identifiers
	count := 0
	for idx := 0; idx < len(functionBody)-1; idx++ {
		char := functionBody[idx]
		nextChar := functionBody[idx+1]

		// Check if character is part of identifier and next is (
		if (isIdentifierChar(char) || char == ')' || char == ']') && nextChar == '(' {
			count++
		}
	}
	return count
}

// isIdentifierChar checks if a character can be part of an identifier
func isIdentifierChar(char byte) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '_' || char == '$'
}

// calculateHalsteadForFunction calculates Halstead metrics for a function
func (kotlinAnalyzer *KotlinAnalyzer) calculateHalsteadForFunction(functionBody string) (volume, difficulty float64) {
	operators := make(map[string]bool)
	operands := make(map[string]bool)
	totalOperators := 0
	totalOperands := 0

	// Count operators and operands by analyzing characters
	inString := false
	stringDelim := byte(0)

	for idx := 0; idx < len(functionBody); idx++ {
		char := functionBody[idx]

		// Handle string literals
		if !inString && (char == '"' || char == '\'') {
			inString = true
			stringDelim = char
			operands["string"] = true
			totalOperands++
			continue
		}

		if inString {
			if char == stringDelim && (idx == 0 || functionBody[idx-1] != '\\') {
				inString = false
			}
			continue
		}

		// Detect operators
		if isOperatorChar(char) {
			opStr := string(char)
			if idx+1 < len(functionBody) {
				nextChar := functionBody[idx+1]
				if isOperatorChar(nextChar) {
					opStr = string([]byte{char, nextChar})
					idx++ // Skip next character
				}
			}
			operators[opStr] = true
			totalOperators++
		} else if isIdentifierChar(char) {
			// Collect identifier tokens
			ident := ""
			for idx < len(functionBody) && isIdentifierChar(functionBody[idx]) {
				ident += string(functionBody[idx])
				idx++
			}
			idx-- // Back up one since the loop will increment

			if ident != "" {
				operands[ident] = true
				totalOperands++
			}
		} else if char >= '0' && char <= '9' {
			// Collect numeric literals
			num := ""
			for idx < len(functionBody) && ((functionBody[idx] >= '0' && functionBody[idx] <= '9') || functionBody[idx] == '.') {
				num += string(functionBody[idx])
				idx++
			}
			idx-- // Back up one since the loop will increment

			if num != "" {
				operands[num] = true
				totalOperands++
			}
		}
	}

	distinctOperators := len(operators)
	distinctOperands := len(operands)

	if distinctOperators == 0 || distinctOperands == 0 {
		return 0, 0
	}

	// Halstead Volume = (N1 + N2) * log2(n1 + n2)
	vocab := float64(distinctOperators + distinctOperands)
	length := float64(totalOperators + totalOperands)

	if vocab > 0 && length > 0 {
		volume = length * log2(vocab)
		// Halstead Difficulty = (n1/2) * (N2/n2)
		difficulty = (float64(distinctOperators) / 2.0) * (float64(totalOperands) / float64(distinctOperands))
	}

	return volume, difficulty
}

// isOperatorChar checks if a character is an operator
func isOperatorChar(char byte) bool {
	operatorChars := "+-*/%=<>!&|"
	for _, op := range operatorChars {
		if char == byte(op) {
			return true
		}
	}
	return false
}

// calculateMaintainabilityIndex calculates the maintainability index
func calculateMaintainabilityIndex(halsteadVolume float64, cyclomaticComplexity int, linesOfCode int) float64 {
	if linesOfCode == 0 {
		return 100
	}

	// MI = 171 - 5.2 * ln(HV) - 0.23 * CC - 16.2 * ln(LOC)
	hvTerm := 0.0
	if halsteadVolume > 0 {
		hvTerm = 5.2 * log(halsteadVolume)
	}

	ccTerm := 0.23 * float64(cyclomaticComplexity)
	locTerm := 16.2 * log(float64(linesOfCode))

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

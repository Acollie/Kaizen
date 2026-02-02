package swift

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/swift"
)

// SwiftAnalyzer implements the LanguageAnalyzer interface for Swift
type SwiftAnalyzer struct {
	language *sitter.Language
}

// NewSwiftAnalyzer creates a new Swift analyzer
func NewSwiftAnalyzer() analyzer.LanguageAnalyzer {
	return &SwiftAnalyzer{
		language: swift.GetLanguage(),
	}
}

// Name returns the language name
func (swiftAnalyzer *SwiftAnalyzer) Name() string {
	return "Swift"
}

// FileExtensions returns the file extensions this analyzer handles
func (swiftAnalyzer *SwiftAnalyzer) FileExtensions() []string {
	return []string{".swift"}
}

// CanAnalyze checks if this analyzer can handle the given file
func (swiftAnalyzer *SwiftAnalyzer) CanAnalyze(filePath string) bool {
	ext := filepath.Ext(filePath)
	for _, supportedExt := range swiftAnalyzer.FileExtensions() {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// IsStub indicates if this is a stub implementation
func (swiftAnalyzer *SwiftAnalyzer) IsStub() bool {
	return false
}

// AnalyzeFile performs full analysis on a single Swift file
func (swiftAnalyzer *SwiftAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
	// Read source code
	sourceBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	sourceCode := string(sourceBytes)

	// Count lines
	totalLines, codeLines, commentLines, blankLines := swiftAnalyzer.countLines(sourceCode)

	// Calculate comment density
	commentDensity := 0.0
	if totalLines > 0 {
		commentDensity = float64(commentLines) / float64(totalLines) * 100
	}

	// Count imports
	importCount := swiftAnalyzer.countImports(sourceCode)

	// Parse with tree-sitter
	parser := sitter.NewParser()
	parser.SetLanguage(swiftAnalyzer.language)
	tree := parser.Parse(nil, sourceBytes)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse Swift file")
	}
	defer tree.Close()

	// Extract and analyze functions
	functions := swiftAnalyzer.extractFunctions(tree.RootNode(), sourceBytes)

	// Analyze types (structs, classes, protocols)
	types := swiftAnalyzer.extractTypes(tree.RootNode())

	return &models.FileAnalysis{
		Path:                  filePath,
		Language:              swiftAnalyzer.Name(),
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
func (swiftAnalyzer *SwiftAnalyzer) countLines(sourceCode string) (total, code, comment, blank int) {
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
func (swiftAnalyzer *SwiftAnalyzer) countImports(sourceCode string) int {
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
func (swiftAnalyzer *SwiftAnalyzer) extractFunctions(node *sitter.Node, sourceBytes []byte) []models.FunctionAnalysis {
	var functions []models.FunctionAnalysis

	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	swiftAnalyzer.walkFunctions(cursor, &functions, sourceBytes)

	return functions
}

// walkFunctions recursively walks the AST to find function declarations
func (swiftAnalyzer *SwiftAnalyzer) walkFunctions(cursor *sitter.TreeCursor, functions *[]models.FunctionAnalysis, sourceBytes []byte) {
	node := cursor.CurrentNode()

	// Check if this is a function declaration
	if node.Type() == "function_declaration" {
		funcAnalysis := swiftAnalyzer.analyzeFunctionNode(node, sourceBytes)
		if funcAnalysis != nil {
			*functions = append(*functions, *funcAnalysis)
		}
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			swiftAnalyzer.walkFunctions(cursor, functions, sourceBytes)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// analyzeFunctionNode extracts details from a function declaration node
func (swiftAnalyzer *SwiftAnalyzer) analyzeFunctionNode(node *sitter.Node, sourceBytes []byte) *models.FunctionAnalysis {
	funcName := swiftAnalyzer.extractFunctionName(node, sourceBytes)
	if funcName == "" {
		return nil
	}

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	length := endLine - startLine + 1

	// Create a function node for complexity calculation
	funcNode := NewSwiftFunction(node, sourceBytes)

	// Calculate complexity metrics
	cyclomaticComplexity := funcNode.CalculateCyclomaticComplexity()
	cognitiveComplexity := funcNode.CalculateCognitiveComplexity()
	nestingDepth := funcNode.CalculateNestingDepth()

	return &models.FunctionAnalysis{
		Name:                    funcName,
		StartLine:               startLine,
		EndLine:                 endLine,
		Length:                  length,
		CyclomaticComplexity:    cyclomaticComplexity,
		CognitiveComplexity:     cognitiveComplexity,
		NestingDepth:            nestingDepth,
		ParameterCount:          swiftAnalyzer.countParameters(node, sourceBytes),
		IsHotspot:               false,
		HalsteadVolume:          0,
		HalsteadDifficulty:      0,
		MaintainabilityIndex:    0,
	}
}

// extractFunctionName extracts the function name from a function declaration node
func (swiftAnalyzer *SwiftAnalyzer) extractFunctionName(node *sitter.Node, sourceBytes []byte) string {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			current := cursor.CurrentNode()
			// In Swift, the function name is typically identified after the func keyword
			if current.Type() == "simple_identifier" {
				return string(sourceBytes[current.StartByte():current.EndByte()])
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return ""
}

// countParameters counts the number of parameters in a function
func (swiftAnalyzer *SwiftAnalyzer) countParameters(node *sitter.Node, sourceBytes []byte) int {
	count := 0
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			current := cursor.CurrentNode()
			if current.Type() == "parameter" {
				count++
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return count
}

// extractTypes extracts and analyzes all types (structs, classes, protocols) in the file
func (swiftAnalyzer *SwiftAnalyzer) extractTypes(node *sitter.Node) []models.TypeAnalysis {
	var types []models.TypeAnalysis

	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	swiftAnalyzer.walkTypes(cursor, &types)

	return types
}

// walkTypes recursively walks the AST to find type declarations
func (swiftAnalyzer *SwiftAnalyzer) walkTypes(cursor *sitter.TreeCursor, types *[]models.TypeAnalysis) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Check for class, struct, protocol, enum declarations
	if nodeType == "class_declaration" || nodeType == "struct_declaration" ||
		nodeType == "protocol_declaration" || nodeType == "enum_declaration" {
		typeAnalysis := swiftAnalyzer.analyzeTypeNode(node)
		if typeAnalysis != nil {
			*types = append(*types, *typeAnalysis)
		}
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			swiftAnalyzer.walkTypes(cursor, types)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// analyzeTypeNode extracts details from a type declaration node
func (swiftAnalyzer *SwiftAnalyzer) analyzeTypeNode(node *sitter.Node) *models.TypeAnalysis {
	typeKind := swiftAnalyzer.getTypeKind(node.Type())

	return &models.TypeAnalysis{
		Name: "Type", // TODO: Extract actual type name
		Kind: typeKind,
	}
}

// getTypeKind maps AST node type to TypeKind string
func (swiftAnalyzer *SwiftAnalyzer) getTypeKind(nodeType string) string {
	switch nodeType {
	case "class_declaration":
		return "class"
	case "struct_declaration":
		return "struct"
	case "protocol_declaration":
		return "protocol"
	case "enum_declaration":
		return "enum"
	default:
		return "type"
	}
}

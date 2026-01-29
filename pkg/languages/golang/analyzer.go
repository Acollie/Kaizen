package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
)

// GoAnalyzer implements the LanguageAnalyzer interface for Go
type GoAnalyzer struct{}

// NewGoAnalyzer creates a new Go analyzer
func NewGoAnalyzer() analyzer.LanguageAnalyzer {
	return &GoAnalyzer{}
}

// Name returns the language name
func (goAnalyzer *GoAnalyzer) Name() string {
	return "Go"
}

// FileExtensions returns the file extensions this analyzer handles
func (goAnalyzer *GoAnalyzer) FileExtensions() []string {
	return []string{".go"}
}

// CanAnalyze checks if this analyzer can handle the given file
func (goAnalyzer *GoAnalyzer) CanAnalyze(filePath string) bool {
	ext := filepath.Ext(filePath)
	for _, supportedExt := range goAnalyzer.FileExtensions() {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// IsStub indicates if this is a stub implementation
func (goAnalyzer *GoAnalyzer) IsStub() bool {
	return false // Go analyzer is fully implemented
}

// AnalyzeFile performs full analysis on a single Go file
func (goAnalyzer *GoAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
	// Read source code
	sourceBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	sourceCode := string(sourceBytes)

	// Parse the file
	fileSet := token.NewFileSet()
	astFile, err := parser.ParseFile(fileSet, filePath, sourceCode, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	// Count lines
	totalLines, codeLines, commentLines, blankLines := goAnalyzer.countLines(sourceCode, astFile)

	// Calculate comment density
	commentDensity := 0.0
	if totalLines > 0 {
		commentDensity = float64(commentLines) / float64(totalLines) * 100
	}

	// Count imports
	importCount := len(astFile.Imports)

	// Extract and analyze functions
	functions := goAnalyzer.extractFunctions(astFile, fileSet, sourceCode)

	// Analyze types (structs, interfaces)
	types := goAnalyzer.extractTypes(astFile, fileSet, sourceCode)

	return &models.FileAnalysis{
		Path:                  filePath,
		Language:              goAnalyzer.Name(),
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
func (goAnalyzer *GoAnalyzer) countLines(sourceCode string, astFile *ast.File) (total, code, comment, blank int) {
	lines := strings.Split(sourceCode, "\n")
	total = len(lines)

	// Build a set of comment line numbers
	commentLineSet := make(map[int]bool)
	for _, commentGroup := range astFile.Comments {
		for _, commentLine := range commentGroup.List {
			// Get line number from position
			lineNum := strings.Count(sourceCode[:commentLine.Pos()], "\n") + 1
			// Multi-line comments span multiple lines
			commentText := commentLine.Text
			if strings.HasPrefix(commentText, "/*") {
				lineCount := strings.Count(commentText, "\n") + 1
				for index := 0; index < lineCount; index++ {
					commentLineSet[lineNum+index] = true
				}
			} else {
				commentLineSet[lineNum] = true
			}
		}
	}

	// Count each line type
	for index, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		lineNum := index + 1

		if trimmedLine == "" {
			blank++
		} else if commentLineSet[lineNum] {
			comment++
		} else {
			code++
		}
	}

	return
}

// extractFunctions extracts and analyzes all functions in the file
func (goAnalyzer *GoAnalyzer) extractFunctions(astFile *ast.File, fileSet *token.FileSet, sourceCode string) []models.FunctionAnalysis {
	var functions []models.FunctionAnalysis

	ast.Inspect(astFile, func(node ast.Node) bool {
		funcDecl, ok := node.(*ast.FuncDecl)
		if !ok {
			return true
		}

		goFunc := NewGoFunction(funcDecl, fileSet, sourceCode)

		// Calculate all metrics
		cyclomaticComplexity := goFunc.CalculateCyclomaticComplexity()
		cognitiveComplexity := goFunc.CalculateCognitiveComplexity()

		// Calculate Halstead metrics
		halsteadVol, halsteadDiff := goAnalyzer.calculateHalsteadForFunction(funcDecl)

		// Calculate maintainability index
		maintainabilityIndex := calculateMaintainabilityIndex(
			halsteadVol,
			cyclomaticComplexity,
			goFunc.LineCount(),
		)

		functionAnalysis := models.FunctionAnalysis{
			Name:                 goFunc.Name(),
			StartLine:            goFunc.StartLine(),
			EndLine:              goFunc.EndLine(),
			Length:               goFunc.LineCount(),
			LogicalLines:         goFunc.LogicalLineCount(),
			ParameterCount:       goFunc.ParameterCount(),
			LocalVariableCount:   goFunc.GetLocalVariableCount(),
			ReturnCount:          goFunc.ReturnCount(),
			CyclomaticComplexity: cyclomaticComplexity,
			CognitiveComplexity:  cognitiveComplexity,
			NestingDepth:         goFunc.MaxNestingDepth(),
			HalsteadVolume:       halsteadVol,
			HalsteadDifficulty:   halsteadDiff,
			MaintainabilityIndex: maintainabilityIndex,
			FanIn:                0, // TODO: Implement call graph analysis
			FanOut:               goAnalyzer.countFunctionCalls(funcDecl),
		}

		functions = append(functions, functionAnalysis)
		return true
	})

	return functions
}

// extractTypes extracts and analyzes types (structs, interfaces)
func (goAnalyzer *GoAnalyzer) extractTypes(astFile *ast.File, fileSet *token.FileSet, sourceCode string) []models.TypeAnalysis {
	var types []models.TypeAnalysis

	ast.Inspect(astFile, func(node ast.Node) bool {
		genDecl, ok := node.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			var kind string

			switch typeSpec.Type.(type) {
			case *ast.StructType:
				kind = "struct"
			case *ast.InterfaceType:
				kind = "interface"
			default:
				continue
			}

			typeAnalysis := models.TypeAnalysis{
				Name:                    typeSpec.Name.Name,
				Kind:                    kind,
				AfferentCoupling:        0, // TODO: Implement coupling analysis
				EfferentCoupling:        0,
				Instability:             0,
				LCOM:                    0, // TODO: Implement cohesion analysis
				DepthOfInheritance:      0, // Go doesn't have inheritance
				NumberOfChildren:        0,
				MethodCount:             0, // Will be filled by method analysis
				WeightedMethodsPerClass: 0,
				PublicMethodCount:       0,
			}

			types = append(types, typeAnalysis)
		}

		return true
	})

	return types
}

// countFunctionCalls counts the number of function calls (fan-out)
func (goAnalyzer *GoAnalyzer) countFunctionCalls(funcDecl *ast.FuncDecl) int {
	count := 0
	ast.Inspect(funcDecl, func(node ast.Node) bool {
		if _, ok := node.(*ast.CallExpr); ok {
			count++
		}
		return true
	})
	return count
}

// calculateHalsteadForFunction calculates Halstead metrics for a function
func (goAnalyzer *GoAnalyzer) calculateHalsteadForFunction(funcDecl *ast.FuncDecl) (volume, difficulty float64) {
	operators := make(map[string]bool)
	operands := make(map[string]bool)
	totalOperators := 0
	totalOperands := 0

	ast.Inspect(funcDecl, func(node ast.Node) bool {
		switch nodeType := node.(type) {
		case *ast.BinaryExpr:
			operators[nodeType.Op.String()] = true
			totalOperators++
		case *ast.UnaryExpr:
			operators[nodeType.Op.String()] = true
			totalOperators++
		case *ast.AssignStmt:
			operators[nodeType.Tok.String()] = true
			totalOperators++
		case *ast.Ident:
			operands[nodeType.Name] = true
			totalOperands++
		case *ast.BasicLit:
			operands[nodeType.Value] = true
			totalOperands++
		}
		return true
	})

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

// calculateMaintainabilityIndex calculates the maintainability index
func calculateMaintainabilityIndex(halsteadVolume float64, cyclomaticComplexity int, linesOfCode int) float64 {
	if linesOfCode == 0 {
		return 100
	}

	// MI = 171 - 5.2 * ln(HV) - 0.23 * CC - 16.2 * ln(LOC)
	// Normalized to 0-100 scale
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

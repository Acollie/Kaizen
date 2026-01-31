package kotlin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
)

// KotlinAnalyzer implements the LanguageAnalyzer interface for Kotlin
type KotlinAnalyzer struct{}

// NewKotlinAnalyzer creates a new Kotlin analyzer
func NewKotlinAnalyzer() analyzer.LanguageAnalyzer {
	return &KotlinAnalyzer{}
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

	// Extract and analyze functions
	functions := kotlinAnalyzer.extractFunctions(sourceCode)

	// Analyze types (classes, interfaces)
	types := kotlinAnalyzer.extractTypes(sourceCode)

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
	importRegex := regexp.MustCompile(`^\s*import\s+`)
	lines := strings.Split(sourceCode, "\n")

	count := 0
	for _, line := range lines {
		if importRegex.MatchString(line) {
			count++
		}
	}
	return count
}

// extractFunctions extracts and analyzes all functions in the file
func (kotlinAnalyzer *KotlinAnalyzer) extractFunctions(sourceCode string) []models.FunctionAnalysis {
	var functions []models.FunctionAnalysis

	// Remove comments to avoid false matches
	cleanCode := kotlinAnalyzer.removeComments(sourceCode)

	// Simpler regex that captures function name in a single group
	functionRegex := regexp.MustCompile(`(?m)\bfun\s+([a-zA-Z_$]\w*)\s*\(`)

	matches := functionRegex.FindAllStringSubmatchIndex(cleanCode, -1)

	for _, match := range matches {
		startPos := match[0]
		// Calculate line number from position
		startLine := strings.Count(cleanCode[:startPos], "\n") + 1

		// Extract function name from match group 1
		funcNameStart := match[2]
		funcNameEnd := match[3]
		if funcNameStart < 0 || funcNameEnd < 0 || funcNameEnd > len(cleanCode) || funcNameStart >= funcNameEnd {
			continue
		}
		functionName := cleanCode[funcNameStart:funcNameEnd]

		// Find matching opening brace
		openBracePos := -1
		for searchPos := startPos; searchPos < len(cleanCode); searchPos++ {
			if cleanCode[searchPos] == '{' {
				openBracePos = searchPos
				break
			}
		}

		if openBracePos == -1 {
			continue
		}

		endLine := startLine + kotlinAnalyzer.findMatchingBraceLineCount(cleanCode, openBracePos)

		// Extract the function body for analysis
		endBracePos := kotlinAnalyzer.findMatchingBrace(cleanCode, openBracePos)
		if endBracePos == -1 || endBracePos > len(cleanCode) {
			endBracePos = len(cleanCode)
		}
		if startPos >= endBracePos {
			continue
		}

		functionBody := cleanCode[startPos:endBracePos]

		// Create function object
		kotlinFunc := NewKotlinFunction(functionName, startLine, endLine, functionBody)

		// Calculate metrics
		cyclomaticComplexity := kotlinFunc.CalculateCyclomaticComplexity()
		cognitiveComplexity := kotlinFunc.CalculateCognitiveComplexity()
		halsteadVol, halsteadDiff := kotlinAnalyzer.calculateHalsteadForFunction(functionBody)

		// Calculate maintainability index
		maintainabilityIndex := calculateMaintainabilityIndex(
			halsteadVol,
			cyclomaticComplexity,
			kotlinFunc.LineCount(),
		)

		functionAnalysis := models.FunctionAnalysis{
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
			FanOut:               kotlinAnalyzer.countFunctionCalls(functionBody),
		}

		functions = append(functions, functionAnalysis)
	}

	return functions
}

// extractTypes extracts and analyzes types (classes, interfaces)
func (kotlinAnalyzer *KotlinAnalyzer) extractTypes(sourceCode string) []models.TypeAnalysis {
	var types []models.TypeAnalysis

	cleanCode := kotlinAnalyzer.removeComments(sourceCode)

	// Match classes
	classRegex := regexp.MustCompile(`(?m)^\s*(data\s+|sealed\s+)?(class|object|interface)\s+([a-zA-Z_$]\w*)`)
	matches := classRegex.FindAllStringSubmatchIndex(cleanCode, -1)

	for _, match := range matches {
		// Type kind is in group 2 (class, object, interface)
		kindStart := match[4]
		kindEnd := match[5]
		kind := cleanCode[kindStart:kindEnd]

		// Type name is in group 3
		nameStart := match[6]
		nameEnd := match[7]
		name := cleanCode[nameStart:nameEnd]

		typeAnalysis := models.TypeAnalysis{
			Name:                    name,
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

		types = append(types, typeAnalysis)
	}

	return types
}

// removeComments removes single-line and block comments from source code
func (kotlinAnalyzer *KotlinAnalyzer) removeComments(sourceCode string) string {
	// Remove block comments
	blockCommentRegex := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	result := blockCommentRegex.ReplaceAllString(sourceCode, "")

	// Remove line comments
	lineCommentRegex := regexp.MustCompile(`//.*`)
	result = lineCommentRegex.ReplaceAllString(result, "")

	return result
}

// findMatchingBrace finds the line number offset of the matching closing brace
func (kotlinAnalyzer *KotlinAnalyzer) findMatchingBraceLineCount(sourceCode string, openPos int) int {
	if openPos >= len(sourceCode) || sourceCode[openPos] != '{' {
		return 0
	}

	braceCount := 0
	lineCount := 0

	for position := openPos; position < len(sourceCode); position++ {
		char := sourceCode[position]

		if char == '\n' {
			lineCount++
		}

		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				return lineCount
			}
		}
	}

	return lineCount
}

// findMatchingBrace finds the position of the matching closing brace
func (kotlinAnalyzer *KotlinAnalyzer) findMatchingBrace(sourceCode string, openPos int) int {
	if openPos >= len(sourceCode) || sourceCode[openPos] != '{' {
		return -1
	}

	braceCount := 0

	for position := openPos; position < len(sourceCode); position++ {
		if sourceCode[position] == '{' {
			braceCount++
		} else if sourceCode[position] == '}' {
			braceCount--
			if braceCount == 0 {
				return position + 1
			}
		}
	}

	return -1
}

// countFunctionCalls counts the number of function calls (fan-out)
func (kotlinAnalyzer *KotlinAnalyzer) countFunctionCalls(functionBody string) int {
	// Match function calls: name(
	callRegex := regexp.MustCompile(`([a-zA-Z_$]\w*)\s*\(`)
	matches := callRegex.FindAllString(functionBody, -1)
	return len(matches)
}

// calculateHalsteadForFunction calculates Halstead metrics for a function
func (kotlinAnalyzer *KotlinAnalyzer) calculateHalsteadForFunction(functionBody string) (volume, difficulty float64) {
	operators := make(map[string]bool)
	operands := make(map[string]bool)
	totalOperators := 0
	totalOperands := 0

	// Extract operators
	operatorRegex := regexp.MustCompile(`(\+|-|\*|/|%|==|!=|<=|>=|<|>|&&|\|\||!|=|\+=|-=|\*=|/=)`)
	operatorMatches := operatorRegex.FindAllString(functionBody, -1)
	for _, op := range operatorMatches {
		operators[op] = true
		totalOperators++
	}

	// Extract operands (identifiers and literals)
	operandRegex := regexp.MustCompile(`([a-zA-Z_$]\w*|"\w+"|'.'|\d+)`)
	operandMatches := operandRegex.FindAllString(functionBody, -1)
	for _, operand := range operandMatches {
		operands[operand] = true
		totalOperands++
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

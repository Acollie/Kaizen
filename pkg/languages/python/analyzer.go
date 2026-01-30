package python

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/models"
)

// PythonAnalyzer implements the LanguageAnalyzer interface for Python
type PythonAnalyzer struct{}

// NewPythonAnalyzer creates a new Python analyzer
func NewPythonAnalyzer() analyzer.LanguageAnalyzer {
	return &PythonAnalyzer{}
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

	totalLines, codeLines, commentLines, blankLines := pyAnalyzer.countLines(sourceCode)
	commentDensity := 0.0
	if totalLines > 0 {
		commentDensity = float64(commentLines) / float64(totalLines) * 100
	}

	importCount := pyAnalyzer.countImports(sourceCode)
	functions := pyAnalyzer.extractFunctions(sourceCode)
	types := pyAnalyzer.extractClasses(sourceCode)

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

// extractFunctions extracts and analyzes all functions in the file
func (pyAnalyzer *PythonAnalyzer) extractFunctions(sourceCode string) []models.FunctionAnalysis {
	var functions []models.FunctionAnalysis
	lines := strings.Split(sourceCode, "\n")

	// Pattern to match function definitions
	funcPattern := regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(([^)]*)\)`)

	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		match := funcPattern.FindStringSubmatch(lines[lineIndex])
		if match == nil {
			continue
		}

		indent := len(match[1])
		funcName := match[2]
		params := match[3]
		startLine := lineIndex + 1

		// Find the end of the function by looking for the next line with same or less indent
		endLine := pyAnalyzer.findFunctionEnd(lines, lineIndex, indent)
		funcLines := lines[lineIndex:endLine]
		funcCode := strings.Join(funcLines, "\n")

		paramCount := pyAnalyzer.countParameters(params)
		localVars := pyAnalyzer.countLocalVariables(funcCode)
		returnCount := pyAnalyzer.countReturns(funcCode)
		cyclomaticComplexity := pyAnalyzer.calculateCyclomaticComplexity(funcCode)
		cognitiveComplexity := pyAnalyzer.calculateCognitiveComplexity(funcCode, indent)
		nestingDepth := pyAnalyzer.calculateNestingDepth(funcLines, indent)
		halsteadVol, halsteadDiff := pyAnalyzer.calculateHalsteadMetrics(funcCode)
		logicalLines := pyAnalyzer.countLogicalLines(funcCode)

		maintainabilityIndex := pyAnalyzer.calculateMaintainabilityIndex(
			halsteadVol,
			cyclomaticComplexity,
			endLine-lineIndex,
		)

		functionAnalysis := models.FunctionAnalysis{
			Name:                 funcName,
			StartLine:            startLine,
			EndLine:              endLine,
			Length:               endLine - lineIndex,
			LogicalLines:         logicalLines,
			ParameterCount:       paramCount,
			LocalVariableCount:   localVars,
			ReturnCount:          returnCount,
			CyclomaticComplexity: cyclomaticComplexity,
			CognitiveComplexity:  cognitiveComplexity,
			NestingDepth:         nestingDepth,
			HalsteadVolume:       halsteadVol,
			HalsteadDifficulty:   halsteadDiff,
			MaintainabilityIndex: maintainabilityIndex,
			FanIn:                0,
			FanOut:               pyAnalyzer.countFunctionCalls(funcCode),
		}

		functions = append(functions, functionAnalysis)
	}

	return functions
}

// findFunctionEnd finds the line where a function ends based on indentation
func (pyAnalyzer *PythonAnalyzer) findFunctionEnd(lines []string, startIndex, baseIndent int) int {
	// Start from the line after the def statement
	for lineIndex := startIndex + 1; lineIndex < len(lines); lineIndex++ {
		line := lines[lineIndex]

		// Skip empty lines and comments
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Calculate the indent of this line
		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		// If we find a line with less or equal indent than the function definition,
		// the function has ended (unless it's a decorator or another def at same level)
		if currentIndent <= baseIndent {
			return lineIndex
		}
	}

	return len(lines)
}

// countParameters counts function parameters
func (pyAnalyzer *PythonAnalyzer) countParameters(params string) int {
	params = strings.TrimSpace(params)
	if params == "" {
		return 0
	}

	// Split by comma, but handle default values and type hints
	count := 0
	depth := 0
	current := ""

	for _, char := range params {
		switch char {
		case '(':
			depth++
			current += string(char)
		case ')':
			depth--
			current += string(char)
		case '[':
			depth++
			current += string(char)
		case ']':
			depth--
			current += string(char)
		case ',':
			if depth == 0 {
				param := strings.TrimSpace(current)
				if param != "" && param != "self" && param != "cls" {
					count++
				}
				current = ""
			} else {
				current += string(char)
			}
		default:
			current += string(char)
		}
	}

	// Don't forget the last parameter
	param := strings.TrimSpace(current)
	if param != "" && param != "self" && param != "cls" {
		count++
	}

	return count
}

// countLocalVariables counts local variable assignments
func (pyAnalyzer *PythonAnalyzer) countLocalVariables(funcCode string) int {
	// Match assignment patterns like: var = or var: type =
	assignPattern := regexp.MustCompile(`(?m)^\s+(\w+)\s*(?::\s*\S+)?\s*=\s*[^=]`)
	matches := assignPattern.FindAllStringSubmatch(funcCode, -1)

	// Use a set to count unique variable names
	varSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			// Exclude self, cls, and common constants
			if varName != "self" && varName != "cls" && !isAllCaps(varName) {
				varSet[varName] = true
			}
		}
	}

	return len(varSet)
}

// isAllCaps checks if a string is all uppercase (constant)
func isAllCaps(str string) bool {
	return str == strings.ToUpper(str) && len(str) > 1
}

// countReturns counts return statements
func (pyAnalyzer *PythonAnalyzer) countReturns(funcCode string) int {
	returnPattern := regexp.MustCompile(`(?m)^\s*return\b`)
	matches := returnPattern.FindAllString(funcCode, -1)
	return len(matches)
}

// calculateCyclomaticComplexity calculates cyclomatic complexity for Python
func (pyAnalyzer *PythonAnalyzer) calculateCyclomaticComplexity(funcCode string) int {
	complexity := 1 // Base complexity

	// Decision points in Python
	patterns := []string{
		`\bif\b`,
		`\belif\b`,
		`\bfor\b`,
		`\bwhile\b`,
		`\bexcept\b`,
		`\band\b`,
		`\bor\b`,
		`\bif\s+\S+\s+else\b`, // Ternary operator
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(funcCode, -1)
		complexity += len(matches)
	}

	// Handle list/dict/set comprehension conditions
	comprehensionPattern := regexp.MustCompile(`\bfor\s+\w+\s+in\b.*\bif\b`)
	compMatches := comprehensionPattern.FindAllString(funcCode, -1)
	complexity += len(compMatches)

	return complexity
}

// calculateCognitiveComplexity calculates cognitive complexity with nesting penalties
func (pyAnalyzer *PythonAnalyzer) calculateCognitiveComplexity(funcCode string, baseIndent int) int {
	complexity := 0
	lines := strings.Split(funcCode, "\n")

	// Keywords that add complexity and increase nesting
	nestingKeywords := map[string]bool{
		"if": true, "elif": true, "else": true,
		"for": true, "while": true,
		"try": true, "except": true, "finally": true,
		"with": true,
	}

	// Keywords that only add complexity
	flatKeywords := map[string]bool{
		"and": true, "or": true,
	}

	currentNesting := 0
	indentStack := []int{baseIndent}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Update nesting level based on indent
		for len(indentStack) > 0 && currentIndent <= indentStack[len(indentStack)-1] {
			if len(indentStack) > 1 {
				indentStack = indentStack[:len(indentStack)-1]
				currentNesting--
			} else {
				break
			}
		}

		// Check for nesting keywords
		for keyword := range nestingKeywords {
			pattern := regexp.MustCompile(`\b` + keyword + `\b`)
			if pattern.MatchString(trimmedLine) {
				// Add 1 for the keyword + nesting penalty
				complexity += 1 + currentNesting
				indentStack = append(indentStack, currentIndent)
				currentNesting++
				break
			}
		}

		// Check for flat keywords (and/or)
		for keyword := range flatKeywords {
			pattern := regexp.MustCompile(`\b` + keyword + `\b`)
			matches := pattern.FindAllString(trimmedLine, -1)
			complexity += len(matches)
		}
	}

	return complexity
}

// calculateNestingDepth calculates maximum nesting depth
func (pyAnalyzer *PythonAnalyzer) calculateNestingDepth(funcLines []string, baseIndent int) int {
	maxDepth := 0

	for _, line := range funcLines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		// Calculate depth relative to function indent
		// Assuming 4 spaces per indent level (Python standard)
		depth := (currentIndent - baseIndent) / 4
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
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

// countLogicalLines counts actual code statements
func (pyAnalyzer *PythonAnalyzer) countLogicalLines(funcCode string) int {
	count := 0
	lines := strings.Split(funcCode, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// Skip empty lines, comments, and docstrings
		if trimmedLine == "" ||
			strings.HasPrefix(trimmedLine, "#") ||
			strings.HasPrefix(trimmedLine, `"""`) ||
			strings.HasPrefix(trimmedLine, `'''`) {
			continue
		}
		count++
	}

	return count
}

// countFunctionCalls counts function/method calls (fan-out)
func (pyAnalyzer *PythonAnalyzer) countFunctionCalls(funcCode string) int {
	// Match function calls: name( or name.method(
	callPattern := regexp.MustCompile(`\b\w+\s*\(`)
	matches := callPattern.FindAllString(funcCode, -1)

	// Exclude the function definition itself and keywords
	keywords := map[string]bool{
		"if": true, "elif": true, "while": true, "for": true,
		"def": true, "class": true, "except": true, "with": true,
		"print": true, // Built-in, but still a call
	}

	count := 0
	for _, match := range matches {
		name := strings.TrimSpace(strings.TrimSuffix(match, "("))
		if !keywords[name] {
			count++
		}
	}

	return count
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

// extractClasses extracts class definitions
func (pyAnalyzer *PythonAnalyzer) extractClasses(sourceCode string) []models.TypeAnalysis {
	var types []models.TypeAnalysis

	classPattern := regexp.MustCompile(`(?m)^class\s+(\w+)`)
	matches := classPattern.FindAllStringSubmatch(sourceCode, -1)

	for _, match := range matches {
		if len(match) > 1 {
			typeAnalysis := models.TypeAnalysis{
				Name:                    match[1],
				Kind:                    "class",
				AfferentCoupling:        0,
				EfferentCoupling:        0,
				Instability:             0,
				LCOM:                    0,
				DepthOfInheritance:      0,
				NumberOfChildren:        0,
				MethodCount:             0,
				WeightedMethodsPerClass: 0,
				PublicMethodCount:       0,
			}
			types = append(types, typeAnalysis)
		}
	}

	return types
}

// ReadFileByLine reads a file line by line for efficient processing
func ReadFileByLine(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

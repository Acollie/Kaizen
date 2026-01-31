package kotlin

import (
	"regexp"
	"strings"
)

// KotlinFunction implements the FunctionNode interface for Kotlin functions
type KotlinFunction struct {
	name         string
	startLine    int
	endLine      int
	functionBody string
}

// NewKotlinFunction creates a new KotlinFunction
func NewKotlinFunction(name string, startLine, endLine int, functionBody string) *KotlinFunction {
	return &KotlinFunction{
		name:         name,
		startLine:    startLine,
		endLine:      endLine,
		functionBody: functionBody,
	}
}

// Name returns the function name
func (kotlinFunc *KotlinFunction) Name() string {
	return kotlinFunc.name
}

// StartLine returns the starting line number
func (kotlinFunc *KotlinFunction) StartLine() int {
	return kotlinFunc.startLine
}

// EndLine returns the ending line number
func (kotlinFunc *KotlinFunction) EndLine() int {
	return kotlinFunc.endLine
}

// LineCount returns the total lines (including blank/comments)
func (kotlinFunc *KotlinFunction) LineCount() int {
	return kotlinFunc.endLine - kotlinFunc.startLine + 1
}

// LogicalLineCount returns the number of actual code statements
func (kotlinFunc *KotlinFunction) LogicalLineCount() int {
	lines := strings.Split(kotlinFunc.functionBody, "\n")
	count := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "*") {
			count++
		}
	}

	return count
}

// ParameterCount returns the number of parameters
func (kotlinFunc *KotlinFunction) ParameterCount() int {
	// Extract parameter list from function signature
	parenStart := strings.Index(kotlinFunc.functionBody, "(")
	parenEnd := strings.Index(kotlinFunc.functionBody, ")")

	if parenStart == -1 || parenEnd == -1 || parenEnd <= parenStart {
		return 0
	}

	paramString := kotlinFunc.functionBody[parenStart+1 : parenEnd]
	paramString = strings.TrimSpace(paramString)

	if paramString == "" {
		return 0
	}

	// Count parameters separated by commas, but respect nested generics
	count := 1
	bracketDepth := 0

	for _, char := range paramString {
		if char == '<' {
			bracketDepth++
		} else if char == '>' {
			bracketDepth--
		} else if char == ',' && bracketDepth == 0 {
			count++
		}
	}

	return count
}

// ReturnCount returns the number of return statements
func (kotlinFunc *KotlinFunction) ReturnCount() int {
	returnRegex := regexp.MustCompile(`\breturn\b`)
	matches := returnRegex.FindAllString(kotlinFunc.functionBody, -1)
	return len(matches)
}

// MaxNestingDepth returns the maximum nesting level
func (kotlinFunc *KotlinFunction) MaxNestingDepth() int {
	maxDepth := 0
	currentDepth := 0

	for _, char := range kotlinFunc.functionBody {
		if char == '{' {
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		} else if char == '}' {
			currentDepth--
		}
	}

	return maxDepth
}

// CalculateCyclomaticComplexity calculates McCabe's cyclomatic complexity
func (kotlinFunc *KotlinFunction) CalculateCyclomaticComplexity() int {
	complexity := 1 // Base complexity

	// Count control flow keywords
	controlKeywords := []string{
		"if", "else if",
		"when",
		"for", "while", "do",
		"try", "catch",
	}

	for _, keyword := range controlKeywords {
		keywordRegex := regexp.MustCompile(`\b` + keyword + `\b`)
		matches := keywordRegex.FindAllString(kotlinFunc.functionBody, -1)
		complexity += len(matches)
	}

	// Count logical operators
	logicalRegex := regexp.MustCompile(`(&&|\|\||!?)`)
	logicalMatches := logicalRegex.FindAllString(kotlinFunc.functionBody, -1)
	for _, match := range logicalMatches {
		if match == "&&" || match == "||" {
			complexity++
		}
	}

	return complexity
}

// CalculateCognitiveComplexity calculates cognitive complexity
func (kotlinFunc *KotlinFunction) CalculateCognitiveComplexity() int {
	complexity := 0
	nestingLevel := 0

	lines := strings.Split(kotlinFunc.functionBody, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Count control structures with nesting penalty
		controlStructures := []string{
			"if ", "if(", "else if ", "else if(",
			"when ", "when(",
			"for ", "for(",
			"while ", "while(",
			"try ", "try{",
			"catch ", "catch(",
		}

		for _, keyword := range controlStructures {
			if strings.Contains(trimmed, keyword) {
				complexity += 1 + nestingLevel
				break
			}
		}

		// Update nesting level
		nestingLevel += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
		if nestingLevel < 0 {
			nestingLevel = 0
		}
	}

	return complexity
}

// countLocalVariables counts local variables in the function
func (kotlinFunc *KotlinFunction) countLocalVariables() int {
	count := 0

	// Count val declarations
	valRegex := regexp.MustCompile(`\bval\b`)
	valMatches := valRegex.FindAllString(kotlinFunc.functionBody, -1)
	count += len(valMatches)

	// Count var declarations
	varRegex := regexp.MustCompile(`\bvar\b`)
	varMatches := varRegex.FindAllString(kotlinFunc.functionBody, -1)
	count += len(varMatches)

	return count
}

// GetLocalVariableCount returns the number of local variables
func (kotlinFunc *KotlinFunction) GetLocalVariableCount() int {
	return kotlinFunc.countLocalVariables()
}

package python

import (
	"strings"

	"github.com/smacker/go-tree-sitter"
)

// PythonFunction represents a Python function for complexity analysis
type PythonFunction struct {
	node        *sitter.Node
	sourceBytes []byte
}

// NewPythonFunction creates a new Python function node
func NewPythonFunction(node *sitter.Node, sourceBytes []byte) *PythonFunction {
	return &PythonFunction{
		node:        node,
		sourceBytes: sourceBytes,
	}
}

// Name extracts the function name from the AST
func (pythonFunc *PythonFunction) Name() string {
	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			node := cursor.CurrentNode()
			if node.Type() == "identifier" {
				return node.Content(pythonFunc.sourceBytes)
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return "unknown"
}

// StartLine returns the starting line number
func (pythonFunc *PythonFunction) StartLine() int {
	return int(pythonFunc.node.StartPoint().Row) + 1
}

// EndLine returns the ending line number
func (pythonFunc *PythonFunction) EndLine() int {
	return int(pythonFunc.node.EndPoint().Row) + 1
}

// LineCount returns the total number of lines
func (pythonFunc *PythonFunction) LineCount() int {
	return pythonFunc.EndLine() - pythonFunc.StartLine() + 1
}

// LogicalLineCount counts non-comment statement nodes in the function body
func (pythonFunc *PythonFunction) LogicalLineCount() int {
	count := 0
	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.countLogicalStatements(cursor, &count)
	return count
}

// countLogicalStatements recursively counts statement nodes
func (pythonFunc *PythonFunction) countLogicalStatements(cursor *sitter.TreeCursor, count *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Count statement nodes
	switch nodeType {
	case "expression_statement", "return_statement", "assert_statement",
		"assignment", "augmented_assignment", "delete_statement",
		"raise_statement", "pass_statement", "break_statement",
		"continue_statement", "import_statement", "import_from_statement",
		"future_import_statement", "global_statement", "nonlocal_statement",
		"exec_statement":
		*count++
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.countLogicalStatements(cursor, count)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// ParameterCount counts function parameters excluding self/cls
func (pythonFunc *PythonFunction) ParameterCount() int {
	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			node := cursor.CurrentNode()
			if node.Type() == "parameters" {
				return pythonFunc.countParametersInNode(node)
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return 0
}

// countParametersInNode counts parameters in a parameters node
func (pythonFunc *PythonFunction) countParametersInNode(paramsNode *sitter.Node) int {
	count := 0
	cursor := sitter.NewTreeCursor(paramsNode)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			node := cursor.CurrentNode()
			nodeType := node.Type()

			// Count different parameter types
			if nodeType == "identifier" || nodeType == "typed_parameter" ||
				nodeType == "default_parameter" || nodeType == "typed_default_parameter" ||
				nodeType == "list_splat_pattern" || nodeType == "dictionary_splat_pattern" {

				// Get parameter name
				paramName := ""
				if nodeType == "identifier" {
					paramName = node.Content(pythonFunc.sourceBytes)
				} else {
					// For complex parameters, extract the identifier child
					paramCursor := sitter.NewTreeCursor(node)
					if paramCursor.GoToFirstChild() {
						for {
							paramNode := paramCursor.CurrentNode()
							if paramNode.Type() == "identifier" {
								paramName = paramNode.Content(pythonFunc.sourceBytes)
								break
							}
							if !paramCursor.GoToNextSibling() {
								break
							}
						}
					}
					paramCursor.Close()
				}

				// Exclude self and cls
				if paramName != "self" && paramName != "cls" && paramName != "" {
					count++
				}
			}

			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return count
}

// ReturnCount counts return statements
func (pythonFunc *PythonFunction) ReturnCount() int {
	count := 0
	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.countReturnStatements(cursor, &count)
	return count
}

// countReturnStatements recursively counts return statements
func (pythonFunc *PythonFunction) countReturnStatements(cursor *sitter.TreeCursor, count *int) {
	node := cursor.CurrentNode()
	if node.Type() == "return_statement" {
		*count++
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.countReturnStatements(cursor, count)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// MaxNestingDepth calculates the maximum nesting depth
func (pythonFunc *PythonFunction) MaxNestingDepth() int {
	maxDepth := 0
	currentDepth := 0

	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.findMaxNestingDepth(cursor, &maxDepth, &currentDepth)
	return maxDepth
}

// findMaxNestingDepth recursively finds the maximum nesting depth
func (pythonFunc *PythonFunction) findMaxNestingDepth(cursor *sitter.TreeCursor, maxDepth *int, currentDepth *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Increase depth for control structures
	shouldIncreaseDepth := false
	switch nodeType {
	case "if_statement", "for_statement", "while_statement",
		"try_statement", "with_statement", "match_statement",
		"function_definition", "async_function_definition",
		"class_definition":
		*currentDepth++
		shouldIncreaseDepth = true
		if *currentDepth > *maxDepth {
			*maxDepth = *currentDepth
		}
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.findMaxNestingDepth(cursor, maxDepth, currentDepth)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}

	// Decrease depth when leaving structure
	if shouldIncreaseDepth {
		*currentDepth--
	}
}

// CalculateCyclomaticComplexity calculates the cyclomatic complexity
// CC = 1 + count of decision points
func (pythonFunc *PythonFunction) CalculateCyclomaticComplexity() int {
	complexity := 1

	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.countComplexityNodes(cursor, &complexity)
	return complexity
}

// countComplexityNodes recursively counts complexity-increasing nodes
func (pythonFunc *PythonFunction) countComplexityNodes(cursor *sitter.TreeCursor, complexity *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Count decision points for cyclomatic complexity
	switch nodeType {
	case "if_statement":
		*complexity++
	case "elif_clause":
		*complexity++
	case "for_statement":
		*complexity++
	case "while_statement":
		*complexity++
	case "try_statement":
		*complexity++
	case "except_clause":
		*complexity++
	case "with_statement":
		*complexity++
	case "match_statement":
		*complexity++
	case "case_clause":
		*complexity++
	case "boolean_operator":
		// and/or operators
		*complexity++
	case "conditional_expression":
		// Ternary: x if condition else y
		*complexity++
	case "list_comprehension", "dictionary_comprehension",
		"set_comprehension", "generator_expression":
		// Check if comprehension has an if clause
		if pythonFunc.hasIfClause(node) {
			*complexity++
		}
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.countComplexityNodes(cursor, complexity)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// hasIfClause checks if a comprehension has an if clause
func (pythonFunc *PythonFunction) hasIfClause(node *sitter.Node) bool {
	cursor := sitter.NewTreeCursor(node)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			if cursor.CurrentNode().Type() == "if_clause" {
				return true
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}
	return false
}

// CalculateCognitiveComplexity calculates cognitive complexity
// Adds nesting penalty on top of cyclomatic complexity
func (pythonFunc *PythonFunction) CalculateCognitiveComplexity() int {
	complexity := 0
	nestingLevel := 0

	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.countCognitiveComplexity(cursor, &complexity, &nestingLevel)
	return complexity
}

// countCognitiveComplexity recursively counts cognitive complexity with nesting penalties
func (pythonFunc *PythonFunction) countCognitiveComplexity(cursor *sitter.TreeCursor, complexity *int, nestingLevel *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Increment nesting level for control structures
	isStructure := false
	switch nodeType {
	case "if_statement":
		*complexity += 1 + *nestingLevel
		isStructure = true
		*nestingLevel++
	case "elif_clause":
		// elif doesn't increase nesting, just complexity
		*complexity++
	case "for_statement", "while_statement":
		*complexity += 1 + *nestingLevel
		isStructure = true
		*nestingLevel++
	case "try_statement", "with_statement":
		*complexity += 1 + *nestingLevel
		isStructure = true
		*nestingLevel++
	case "except_clause":
		*complexity += 1 + *nestingLevel
	case "match_statement":
		*complexity += 1 + *nestingLevel
		isStructure = true
		*nestingLevel++
	case "case_clause":
		*complexity++
	case "boolean_operator":
		*complexity++
	case "conditional_expression":
		*complexity += 1 + *nestingLevel
	case "list_comprehension", "dictionary_comprehension",
		"set_comprehension", "generator_expression":
		if pythonFunc.hasIfClause(node) {
			*complexity += 1 + *nestingLevel
		}
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.countCognitiveComplexity(cursor, complexity, nestingLevel)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}

	// Decrement nesting level when leaving structure
	if isStructure {
		*nestingLevel--
	}
}

// CountLocalVariables counts local variable assignments
func (pythonFunc *PythonFunction) CountLocalVariables() int {
	varSet := make(map[string]bool)
	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.findLocalVariables(cursor, varSet)
	return len(varSet)
}

// findLocalVariables recursively finds local variable assignments
func (pythonFunc *PythonFunction) findLocalVariables(cursor *sitter.TreeCursor, varSet map[string]bool) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Look for assignment patterns
	if nodeType == "assignment" || nodeType == "augmented_assignment" {
		// Get the left side (target)
		if cursor.GoToFirstChild() {
			targetNode := cursor.CurrentNode()
			if targetNode.Type() == "identifier" {
				varName := targetNode.Content(pythonFunc.sourceBytes)
				// Exclude self, cls, and all-caps constants
				if varName != "self" && varName != "cls" && !isAllUpperCase(varName) {
					varSet[varName] = true
				}
			}
			cursor.GoToParent()
		}
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.findLocalVariables(cursor, varSet)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// isAllUpperCase checks if a string is all uppercase (constant naming convention)
func isAllUpperCase(str string) bool {
	if len(str) <= 1 {
		return false
	}
	return str == strings.ToUpper(str)
}

// CountFunctionCalls counts function/method calls (fan-out)
func (pythonFunc *PythonFunction) CountFunctionCalls() int {
	count := 0
	cursor := sitter.NewTreeCursor(pythonFunc.node)
	defer cursor.Close()

	pythonFunc.countCalls(cursor, &count)
	return count
}

// countCalls recursively counts function call nodes
func (pythonFunc *PythonFunction) countCalls(cursor *sitter.TreeCursor, count *int) {
	node := cursor.CurrentNode()
	if node.Type() == "call" {
		*count++
	}

	// Recurse to children
	if cursor.GoToFirstChild() {
		for {
			pythonFunc.countCalls(cursor, count)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

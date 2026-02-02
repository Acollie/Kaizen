package swift

import (
	"github.com/smacker/go-tree-sitter"
)

// SwiftFunction represents a Swift function for complexity analysis
type SwiftFunction struct {
	node        *sitter.Node
	sourceBytes []byte
}

// NewSwiftFunction creates a new Swift function node
func NewSwiftFunction(node *sitter.Node, sourceBytes []byte) *SwiftFunction {
	return &SwiftFunction{
		node:        node,
		sourceBytes: sourceBytes,
	}
}

// CalculateCyclomaticComplexity calculates the cyclomatic complexity
// CC = 1 + count of decision points (if, guard, switch case, for, while, catch)
func (swiftFunc *SwiftFunction) CalculateCyclomaticComplexity() int {
	complexity := 1

	cursor := sitter.NewTreeCursor(swiftFunc.node)
	defer cursor.Close()

	swiftFunc.countComplexityNodes(cursor, &complexity, false)

	return complexity
}

// countComplexityNodes recursively counts complexity-increasing nodes
func (swiftFunc *SwiftFunction) countComplexityNodes(cursor *sitter.TreeCursor, complexity *int, inNesting bool) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Count decision points for cyclomatic complexity
	switch nodeType {
	case "if_statement", "guard_statement", "switch_statement":
		*complexity++
	case "for_statement", "while_statement", "repeat_while_statement":
		*complexity++
	case "catch_clause":
		*complexity++
	case "boolean_literal":
		// Check for && and || operators
		// This is handled by checking the source code
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			swiftFunc.countComplexityNodes(cursor, complexity, inNesting)
			if !cursor.GoToNextSibling() {
				break
			}
		}
		cursor.GoToParent()
	}
}

// CalculateCognitiveComplexity calculates cognitive complexity
// Adds nesting penalty on top of cyclomatic complexity
func (swiftFunc *SwiftFunction) CalculateCognitiveComplexity() int {
	complexity := 0
	nestingLevel := 0

	cursor := sitter.NewTreeCursor(swiftFunc.node)
	defer cursor.Close()

	swiftFunc.countCognitiveComplexity(cursor, &complexity, &nestingLevel)

	return complexity
}

// countCognitiveComplexity recursively counts cognitive complexity with nesting penalties
func (swiftFunc *SwiftFunction) countCognitiveComplexity(cursor *sitter.TreeCursor, complexity *int, nestingLevel *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Increment nesting level for control structures
	isStructure := false
	switch nodeType {
	case "if_statement", "guard_statement", "switch_statement",
		"for_statement", "while_statement", "repeat_while_statement":
		*complexity += 1 + *nestingLevel
		isStructure = true
		*nestingLevel++
	case "catch_clause":
		*complexity += 1 + *nestingLevel
		isStructure = true
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			swiftFunc.countCognitiveComplexity(cursor, complexity, nestingLevel)
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

// CalculateNestingDepth calculates the maximum nesting depth
func (swiftFunc *SwiftFunction) CalculateNestingDepth() int {
	maxDepth := 0
	currentDepth := 0

	cursor := sitter.NewTreeCursor(swiftFunc.node)
	defer cursor.Close()

	swiftFunc.findMaxNestingDepth(cursor, &maxDepth, &currentDepth)

	return maxDepth
}

// findMaxNestingDepth recursively finds the maximum nesting depth
func (swiftFunc *SwiftFunction) findMaxNestingDepth(cursor *sitter.TreeCursor, maxDepth *int, currentDepth *int) {
	node := cursor.CurrentNode()
	nodeType := node.Type()

	// Increase depth for control structures
	shouldIncreaseDepth := false
	switch nodeType {
	case "if_statement", "guard_statement", "switch_statement",
		"for_statement", "while_statement", "repeat_while_statement",
		"do_catch_statement", "block":
		*currentDepth++
		shouldIncreaseDepth = true
		if *currentDepth > *maxDepth {
			*maxDepth = *currentDepth
		}
	}

	// Recursively visit children
	if cursor.GoToFirstChild() {
		for {
			swiftFunc.findMaxNestingDepth(cursor, maxDepth, currentDepth)
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

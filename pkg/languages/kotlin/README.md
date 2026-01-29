# Kotlin Analyzer - Implementation Guide

This directory contains a **stub implementation** of the Kotlin language analyzer. It demonstrates the extensibility of Kaizen but does not yet perform actual Kotlin code analysis.

## Current Status

✅ Implements the `LanguageAnalyzer` interface
✅ Detects `.kt` and `.kts` files
❌ Does not parse Kotlin AST
❌ Does not calculate metrics

## How to Implement Kotlin Support

### Option 1: Tree-Sitter (Recommended)

Tree-sitter provides robust, incremental parsing for many languages.

**Steps:**
1. Install tree-sitter-kotlin:
   ```bash
   go get github.com/smacker/go-tree-sitter
   go get github.com/smacker/go-tree-sitter/kotlin
   ```

2. Parse Kotlin files:
   ```go
   import (
       sitter "github.com/smacker/go-tree-sitter"
       "github.com/smacker/go-tree-sitter/kotlin"
   )

   parser := sitter.NewParser()
   parser.SetLanguage(kotlin.GetLanguage())
   tree := parser.Parse(nil, sourceCode)
   ```

3. Traverse the syntax tree to find functions, classes, and control flow

### Option 2: External Kotlin Compiler

Use `kotlinc` to generate AST or bytecode, then analyze.

**Steps:**
1. Execute kotlinc with special flags:
   ```bash
   kotlinc -Xdump-declarations file.kt
   ```

2. Parse the output to extract structure

3. Calculate metrics from the parsed data

### Option 3: Pure Go Kotlin Parser

Write a Kotlin parser from scratch (most effort, most control).

## Metrics to Implement

Once you can parse Kotlin AST, implement these calculations:

### Essential
- **Cyclomatic Complexity**: Count `if`, `when`, `while`, `for`, `&&`, `||`
- **Function Length**: Line count per function
- **Nesting Depth**: Maximum depth of nested blocks
- **Parameter Count**: Function parameter count

### Kotlin-Specific
- **Nullable Type Usage**: Count `?` operators
- **Extension Function Count**: Track extension functions
- **Sealed Class Usage**: Analyze sealed class hierarchies
- **Coroutine Usage**: Count `suspend` functions and coroutine builders
- **Lambda Complexity**: Measure complexity of lambda expressions

## Implementation Checklist

- [ ] Choose parsing approach (tree-sitter recommended)
- [ ] Set up parser dependencies
- [ ] Implement `parseFile()` to create AST
- [ ] Implement `extractFunctions()` to find all functions
- [ ] Implement `extractTypes()` to find classes/interfaces
- [ ] Calculate cyclomatic complexity
- [ ] Calculate cognitive complexity
- [ ] Calculate Halstead metrics
- [ ] Count lines (code, comments, blank)
- [ ] Update `IsStub()` to return `false`
- [ ] Write unit tests with sample Kotlin files
- [ ] Update documentation

## Example Function Structure

```go
type KotlinFunction struct {
    node       *sitter.Node
    name       string
    startLine  int
    endLine    int
    parameters []string
}

func (kf *KotlinFunction) CalculateCyclomaticComplexity() int {
    complexity := 1

    // Walk the syntax tree
    cursor := sitter.NewTreeCursor(kf.node)
    defer cursor.Close()

    // Count decision points
    for {
        node := cursor.CurrentNode()
        switch node.Type() {
        case "if_expression":
            complexity++
        case "when_expression":
            complexity++
        case "for_statement", "while_statement":
            complexity++
        case "binary_expression":
            // Check for && or ||
            if isLogicalOperator(node) {
                complexity++
            }
        }

        if !cursor.GoToNextSibling() {
            break
        }
    }

    return complexity
}
```

## Testing

Create test files in `testdata/`:

```
testdata/
├── simple.kt       # Basic function
├── complex.kt      # High complexity
├── nested.kt       # Deep nesting
└── extensions.kt   # Extension functions
```

Run tests:
```bash
go test ./pkg/languages/kotlin/...
```

## Resources

- [Tree-Sitter Kotlin Grammar](https://github.com/fwcd/tree-sitter-kotlin)
- [Kotlin Language Spec](https://kotlinlang.org/spec/)
- [go-tree-sitter Documentation](https://github.com/smacker/go-tree-sitter)
- [Cognitive Complexity Paper](https://www.sonarsource.com/resources/cognitive-complexity/)

## Questions?

See the main PLAN.md for overall architecture and design patterns.

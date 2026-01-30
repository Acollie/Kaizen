# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the binary
go build -o kaizen ./cmd/kaizen

# Install globally (adds to $GOPATH/bin)
go install ./cmd/kaizen

# Run analysis on the project itself (dogfooding)
./kaizen analyze --path=. --skip-churn --output=analysis.json

# Generate HTML visualization
./kaizen visualize --input=analysis.json --format=html --open=false

# Test a change end-to-end
go build -o kaizen ./cmd/kaizen && ./kaizen analyze --path=. --skip-churn
```

## Architecture Overview

### Core Design Pattern: Interface-Based Language Analyzers

Kaizen uses an **interface-driven architecture** to support multiple programming languages. The key abstraction is the `LanguageAnalyzer` interface in `pkg/analyzer/interfaces.go`:

```go
type LanguageAnalyzer interface {
    Name() string
    FileExtensions() []string
    CanAnalyze(filePath string) bool
    AnalyzeFile(filePath string) (*models.FileAnalysis, error)
    IsStub() bool
}
```

Each language implementation lives in `pkg/languages/<lang>/` and must implement this interface. The `pkg/languages/registry.go` provides auto-detection.

### Analysis Pipeline Flow

1. **Discovery**: `Pipeline.Analyze()` in `pkg/analyzer/pipeline.go` walks the filesystem
2. **Filtering**: Applies `.kaizenignore` patterns via `internal/config/config.go`
3. **Language Detection**: Registry matches file extensions to analyzers
4. **AST Parsing**: Language-specific analyzer parses code (e.g., `pkg/languages/golang/analyzer.go` uses Go's `go/ast`)
5. **Metric Calculation**: Extracts functions and calculates complexity, length, etc.
6. **Churn Analysis**: `pkg/churn/analyzer.go` runs git commands to get commit history
7. **Aggregation**: `pkg/analyzer/aggregator.go` rolls up file metrics to folder metrics
8. **Output**: JSON saved with all analysis results

### Key Components

**`pkg/languages/golang/function.go`**: Implements `FunctionNode` interface
- `CalculateCyclomaticComplexity()`: Counts decision points (if, for, case, &&, ||)
- `CalculateCognitiveComplexity()`: Penalizes nesting more heavily than cyclomatic
- Uses Go's `ast.Inspect()` to walk the AST tree

**`pkg/churn/analyzer.go`**: Git integration
- Executes `git log --numstat` for file-level churn
- Executes `git log -L :<funcname>:` for function-level churn (experimental)
- Parses commit history to count contributors, changes, dates

**`pkg/visualization/html.go`**: D3.js treemap generator
- Builds hierarchical tree from folder metrics
- Embeds JSON data directly in HTML template
- Client-side D3.js renders interactive treemap with metric switching

**`internal/config/config.go`**: Configuration system
- Loads `.kaizenignore` (gitignore-style patterns)
- Loads `.kaizen.yaml` (YAML configuration)
- Supports pattern matching: `*`, `**`, `/absolute`, `!negation`

### Data Flow

```
Source Files
    ↓ (Pipeline.discoverFiles)
Filtered File List
    ↓ (Registry.GetAnalyzerForFile)
Language Analyzer
    ↓ (analyzer.AnalyzeFile)
FileAnalysis (with FunctionAnalysis[])
    ↓ (ChurnAnalyzer.GetFileChurn)
FileAnalysis + ChurnMetrics
    ↓ (Aggregator.AggregateByFolder)
FolderMetrics map
    ↓ (JSON marshal)
AnalysisResult file
    ↓ (HTMLVisualizer.GenerateHTML)
Interactive HTML heat map
```

## Adding a New Language

To add support for a new language (e.g., Python):

1. Create `pkg/languages/python/analyzer.go`:
   ```go
   type PythonAnalyzer struct{}

   func (p *PythonAnalyzer) AnalyzeFile(path string) (*models.FileAnalysis, error) {
       // Parse Python AST (use tree-sitter or python parser)
       // Extract functions
       // Calculate metrics per function
       // Return FileAnalysis
   }
   ```

2. Register in `pkg/languages/registry.go`:
   ```go
   analyzers: []analyzer.LanguageAnalyzer{
       golang.NewGoAnalyzer(),
       kotlin.NewKotlinAnalyzer(),
       python.NewPythonAnalyzer(), // ADD THIS
   }
   ```

3. The rest works automatically - pipeline discovers files, churn analyzer works, visualization updates.

## Important Patterns

### Stub Implementations
The Kotlin analyzer is a **stub** (`IsStub() = true`) that demonstrates the interface but doesn't parse code. This pattern allows the system to recognize Kotlin files but return helpful errors rather than silently failing.

### Configuration Priority
CLI flags > `.kaizen.yaml` > `.kaizenignore` > defaults

This is handled in `cmd/kaizen/main.go` in `runAnalyze()` - config is loaded first, then CLI flags override specific values.

### Metric Normalization
Raw metrics (like complexity=8, churn=15 commits) are normalized to 0-100 scores using **percentile ranking** in `pkg/analyzer/aggregator.go`. This allows different metrics to be compared visually in the heat map.

### AST Walking Pattern (Go-specific)
The Go analyzer uses `ast.Inspect()` with closures to walk the AST:
```go
ast.Inspect(funcDecl, func(node ast.Node) bool {
    switch nodeType := node.(type) {
    case *ast.IfStmt:
        complexity++
    case *ast.ForStmt:
        complexity++
    }
    return true // continue walking
})
```

Always use type switches when handling `ast.Node` - don't use combined cases like `case *ast.ForStmt, *ast.RangeStmt:` because type assertions will fail.

## Configuration Files

- `.kaizenignore`: Gitignore-style patterns (one per line, supports `#` comments, `!` negation)
- `.kaizen.yaml`: Full config with analysis settings, thresholds, visualization options
- Both are optional and loaded automatically from the analyzed directory

## Code Style Guidelines

From user's global CLAUDE.md:
- Avoid single-character variable names
- Prefer clean code style (Uncle Bob principles)
- Smaller methods preferred
- Function length of 100 lines is acceptable but not preferred

## Output Files

- `kaizen-results.json`: Default analysis output (can customize with `--output`)
- `kaizen-heatmap.html`: Default HTML visualization (can customize with `--output`)
- Both are self-contained and can be shared/archived

## Metrics Reference

**Cyclomatic Complexity**: Count of `if`, `for`, `switch`, `case`, `&&`, `||` - base complexity is 1
**Cognitive Complexity**: Like cyclomatic but adds `+nestingLevel` penalty for nested structures
**Halstead Volume**: Based on operator/operand counts - uses `math.Log2(vocabulary) * length`
**Maintainability Index**: Formula: `171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)`, clamped to 0-100
**Hotspot**: Function with `complexity > 10` AND `churn > 10 commits`

## Common Issues

**Type assertion panics in Go analyzer**: Always separate multiple case types in switch statements. Use individual cases or use a type switch variable.

**Git churn not working**: Requires being in a git repository. Use `--skip-churn` if not in git repo or if git commands are slow.

**HTML visualization JSON escaping**: Use `template.JS()` when embedding JSON in HTML templates to prevent over-escaping.

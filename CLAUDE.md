# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Building and Running

```bash
# Build the binary to ./kaizen
go build -o kaizen ./cmd/kaizen

# Install globally to $GOPATH/bin
go install ./cmd/kaizen

# Build with version info embedded
go build -ldflags="-X main.Version=$(git describe --tags --always)" -o kaizen ./cmd/kaizen
```

### Testing

```bash
# Run all tests
go test -v -race ./...

# Run single package tests
go test -v -race ./pkg/analyzer
go test -v -race ./pkg/languages/golang

# Run specific test
go test -v -race -run TestFunctionComplexity ./pkg/languages/golang

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...

# View coverage report (generates HTML)
go tool cover -html=coverage.out

# Run tests for entire repository with codecov format
go test -v -race -coverprofile=coverage.out ./... && \
  go tool cover -func=coverage.out | tail -1 | awk '{print $NF}'
```

### Development Workflow

```bash
# Build and test end-to-end
go build -o kaizen ./cmd/kaizen && ./kaizen analyze --path=. --skip-churn --output=analysis.json

# Analyze the project itself (dogfooding)
./kaizen analyze --path=. --skip-churn --output=analysis.json

# Generate interactive HTML visualization
./kaizen visualize --input=analysis.json --format=html --open=false

# View in terminal (faster feedback)
./kaizen visualize --input=analysis.json --format=terminal

# Compare with last snapshot (requires prior analysis)
./kaizen diff --path=.

# Generate analysis report
./kaizen report owners --format=html

# List analysis history
./kaizen history list

# Linting with golangci-lint
golangci-lint run ./...

# Vet the code
go vet ./...
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

### Key Components and Their Responsibilities

**`cmd/kaizen/main.go`** (1600+ lines)
- Cobra CLI framework with all subcommands
- Config loading and flag parsing
- Main entry point orchestrating the full workflow

**`pkg/analyzer/`** - Analysis orchestration
- `pipeline.go` - Main analysis flow (discovery → analysis → aggregation)
- `interfaces.go` - Core abstractions (`LanguageAnalyzer`, `FunctionNode`, `TypeNode`)
- `aggregator.go` - Folder-level metric aggregation and percentile normalization
- `metrics.go` - Utility metric calculations

**`pkg/languages/`** - Multi-language support
- `registry.go` - Central dispatcher and auto-detection
- `golang/` - Native Go AST parsing (fully implemented, highest accuracy)
  - `analyzer.go` - Entry point using `go/parser` and `go/ast`
  - `function.go` - Complexity calculations (cyclomatic, cognitive), AST walking
  - `utils.go` - Helper functions for Go-specific logic
- `kotlin/`, `swift/`, `python/` - Tree-sitter based implementations
  - Each has `analyzer.go`, `function.go`, `utils.go`
  - Kotlin is a stub demonstrating the interface pattern

**`pkg/churn/analyzer.go`** - Git integration
- `GetFileChurn()` - File-level commits via `git log --numstat`
- `GetFunctionChurn()` - Function-level commits via `git log -L :<funcname>:` (experimental)
- Parses commit history for contributor count, change dates, additions/deletions

**`pkg/storage/`** - Persistence layer
- `interface.go` - Backend abstraction with Save/Get/List/Prune operations
- `sqlite.go` - SQLite implementation with GORM ORM
- `migrations.go` - Schema initialization
- Time-series storage for historical snapshots

**`pkg/visualization/`** - Output rendering
- `html.go` - Interactive D3.js treemaps with metric switching
- `svg.go` - Static SVG heatmaps
- `terminal.go` - ANSI-based color heat maps for CLI
- `callgraph.go` - Function call graph visualization (Go only)
- `sankey.go` - Code ownership flow diagrams

**`pkg/reports/`** - Score generation
- `scorer.go` - Health grades (A-F) and component scoring
- `concerns.go` - Automatic issue detection (high complexity, deep nesting, etc.)
- `grading.go` - Letter grade calculation based on normalized metrics

**`pkg/ownership/`** - Code ownership
- Parses CODEOWNERS files
- Aggregates metrics by team
- Generates ownership reports

**`internal/config/`** - Configuration
- Loads `.kaizen.yaml` (full YAML config)
- Loads `.kaizenignore` (gitignore-style patterns)
- Priority: CLI flags > `.kaizen.yaml` > `.kaizenignore` > defaults

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
    ↓ (Scorer.GenerateScoreReport)
ScoreReport with grades and concerns
    ↓ (Storage.Save)
SQLite database + JSON file
    ↓ (Visualizers)
HTML/SVG/Terminal output
```

## Key Implementation Patterns

### Language Analyzer Implementation

When implementing a new language analyzer:

1. **Implement the `LanguageAnalyzer` interface** in `pkg/languages/<lang>/analyzer.go`
   - `Name()` - Language name
   - `FileExtensions()` - Supported file extensions (`.go`, `.kt`, etc.)
   - `CanAnalyze(filePath)` - Check if file can be analyzed
   - `AnalyzeFile(filePath)` - Parse and extract metrics, return `*models.FileAnalysis`
   - `IsStub()` - Return true for stub implementations

2. **Extract functions with a `FunctionNode` implementation** in `function.go`
   - Must satisfy the `FunctionNode` interface
   - Implement `CalculateCyclomaticComplexity()` - Count decision points
   - Implement `CalculateCognitiveComplexity()` - Penalize nesting
   - Return `*models.FunctionAnalysis` with all calculated metrics

3. **Register in `pkg/languages/registry.go`**
   ```go
   analyzers: []analyzer.LanguageAnalyzer{
       golang.NewGoAnalyzer(),
       kotlin.NewKotlinAnalyzer(),
       myLanguage.NewMyLanguageAnalyzer(), // ADD THIS
   }
   ```

### Metric Calculation Patterns

**Cyclomatic Complexity**: Count of decision points + 1
- Increment for: `if`, `for`, `while`, `switch case`, `&&`, `||`
- Base complexity: 1

**Cognitive Complexity**: Cyclomatic + nesting penalty
- Add nesting level bonus for each nested structure
- Penalizes deeply nested conditionals more heavily

**Halstead Metrics**:
- Operators: `=`, `+`, `-`, `*`, function calls, etc.
- Operands: variable names, constants
- Volume = `length * log2(vocabulary)`

**Maintainability Index**: `171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)`
- HV = Halstead Volume
- CC = Cyclomatic Complexity
- LOC = Lines of Code

### AST Walking Pattern (Go-specific)

The Go analyzer uses `ast.Inspect()` with closures for tree traversal:

```go
ast.Inspect(funcDecl, func(node ast.Node) bool {
    switch nodeType := node.(type) {
    case *ast.IfStmt:
        complexity++
    case *ast.ForStmt, *ast.RangeStmt:
        complexity++
    case *ast.SwitchStmt:
        complexity++ // +1 per case handled separately
    }
    return true // continue walking
})
```

**Important**: Always separate `case` statements for different types in switch - combined cases like `case *ast.ForStmt, *ast.RangeStmt:` will fail type assertions.

### Configuration Priority

CLI flags override `.kaizen.yaml` which overrides `.kaizenignore` which overrides defaults. This is enforced in `cmd/kaizen/main.go` in `runAnalyze()` - load config first, then explicitly override with CLI flags.

### Metric Normalization

Raw metrics (e.g., complexity=8, churn=15 commits) are normalized to 0-100 percentile scores in `pkg/analyzer/aggregator.go`. This allows comparing different metrics visually in heatmaps without unit confusion.

### Stub Implementations

The Kotlin analyzer demonstrates the stub pattern (`IsStub() = true`). Stubs:
- Implement the full interface
- Recognize files but return structured errors
- Enable graceful degradation instead of silent failures
- Provide templates for future full implementations

### Storage and Snapshots

**SQLite Backend**:
- Auto-initialized in `.kaizen/` subdirectory
- Stores snapshots with metadata (timestamp, commit hash, branch)
- Efficient time-series queries for trending
- GORM ORM handles schema and migrations

**Snapshot Operations**:
- `Save()` - Store new analysis
- `GetLatest()`, `GetByID()` - Retrieve snapshots
- `GetTimeSeries()` - Query metrics over time range
- `ListSnapshots()` - List all stored snapshots
- `Prune()` - Remove old snapshots for storage management

## Adding a New Language

To add support for a new language (e.g., complete Python implementation):

1. Create `pkg/languages/python/analyzer.go`:
   ```go
   type PythonAnalyzer struct{}

   func (p *PythonAnalyzer) Name() string {
       return "Python"
   }

   func (p *PythonAnalyzer) FileExtensions() []string {
       return []string{".py"}
   }

   func (p *PythonAnalyzer) CanAnalyze(filePath string) bool {
       return strings.HasSuffix(filePath, ".py")
   }

   func (p *PythonAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
       // 1. Parse Python file using tree-sitter
       // 2. Walk AST and extract functions
       // 3. Calculate metrics for each function
       // 4. Aggregate to file level
       // 5. Return FileAnalysis
   }

   func (p *PythonAnalyzer) IsStub() bool {
       return false
   }
   ```

2. Implement function extraction in `pkg/languages/python/function.go`:
   - Parse Python function definitions
   - Calculate cyclomatic and cognitive complexity
   - Extract parameters, length, nesting depth

3. Register in `pkg/languages/registry.go`:
   ```go
   analyzers: []analyzer.LanguageAnalyzer{
       golang.NewGoAnalyzer(),
       kotlin.NewKotlinAnalyzer(),
       swift.NewSwiftAnalyzer(),
       python.NewPythonAnalyzer(), // ADD THIS
   }
   ```

4. Add tests in `pkg/languages/python/analyzer_test.go` and `function_test.go`

5. The rest of the system (churn, storage, visualization, reporting) works automatically.

## Configuration Files

- `.kaizenignore` - Gitignore-style exclusion patterns (one per line, supports `#` comments, `!` negation, `**` wildcards)
- `.kaizen.yaml` - Full YAML configuration with analysis settings, thresholds, visualization options, language-specific config
- Both are optional and auto-loaded from the analyzed directory

## Code Style Guidelines

From user's global CLAUDE.md:
- Avoid single-character variable names (except loop counters in tight loops)
- Prefer clean code style (Uncle Bob principles): smaller functions, clear names, single responsibility
- Function length of 100 lines is acceptable but not preferred; aim for 50-75 lines
- Extract helper functions and use meaningful names to improve readability

## Output Files

- `kaizen-results.json` - Default analysis output (customizable with `--output`)
- `kaizen-heatmap.html` - Interactive HTML visualization with D3.js treemap (customizable with `--output`)
- Both are self-contained and can be shared/archived

## Metrics Reference

**Cyclomatic Complexity (CC)**: Count of decision points + 1 base complexity
- Measures linear independent paths through code
- Each `if`, `for`, `switch case`, `&&`, `||` adds +1
- Reference: CC > 10 is considered error-prone

**Cognitive Complexity**: CC + nesting penalty for deeply nested structures
- Penalizes nesting more heavily than cyclomatic
- Better matches human perception of code complexity

**Halstead Volume**: `length * log2(vocabulary)` where vocabulary = unique operators + operands
- Measures program size
- Higher volume = more complex

**Maintainability Index (MI)**: Formula: `171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)`
- Combines complexity, volume, and length
- Scaled to 0-100 (higher = more maintainable)
- MI < 20 is considered low maintainability

**Hotspot**: Function with CC > 10 AND churn > 10 commits
- Identifies high-risk code: complex AND frequently changing
- Highest priority for refactoring

**Churn**: Number of commits touching a file/function from git history
- High churn = unstable, frequently changing code
- Indicator of technical debt or active development

## Common Development Tasks

### Adding a New Metric

1. Define in `pkg/models/models.go` - Add field to `FunctionAnalysis` or `FileAnalysis`
2. Calculate in language analyzer - Update `pkg/languages/<lang>/function.go`
3. Normalize in aggregator - Add percentile ranking logic to `pkg/analyzer/aggregator.go`
4. Report in scoring - Update `pkg/reports/scorer.go` if it affects overall score
5. Visualize - Update `pkg/visualization/*.go` to render the new metric

### Debugging Language Parsing

1. Create a test file with known complexity patterns
2. Run analyzer directly: `go test -v ./pkg/languages/golang -run TestAnalyzeFile`
3. Add debug output to AST walking loop
4. Verify metrics match expected values
5. Add to test suite in `*_test.go`

### Performance Optimization

- **Churn analysis is slowest** - Use `--skip-churn` for iteration, add later
- **Language parsing** - Go's native `ast` is fastest; tree-sitter slower
- **Filesystem I/O** - Disk speed limits overall performance
- **Memory** - Large codebases load entire AST into memory; consider incremental parsing for future

### Testing Changes

1. Build: `go build -o kaizen ./cmd/kaizen`
2. Test: `go test -v -race ./...`
3. Run on project: `./kaizen analyze --path=. --skip-churn --output=/tmp/test.json`
4. Visualize: `./kaizen visualize --input=/tmp/test.json --format=terminal`

## Important Patterns

### Type Assertion in Switch Statements

Do NOT combine cases in switch statements for type assertions:

```go
// WRONG - will panic
case *ast.ForStmt, *ast.RangeStmt:
    complexity++

// CORRECT - separate cases
case *ast.ForStmt:
    complexity++
case *ast.RangeStmt:
    complexity++
```

### HTML Template JSON Escaping

When embedding JSON in HTML templates, use `template.JS()` to prevent HTML escaping of JSON special characters:

```go
// Correct - prevents double-escaping
template.HTML(template.HTMLEscaper(jsonData))
```

### Git Churn Requirements

Churn analysis requires:
- Active git repository (not just files)
- Git commands available in PATH
- For analysis not in git: use `--skip-churn` flag
- Function-level churn is experimental and slower

## Common Issues and Solutions

**Type assertion panics in Go analyzer**
- Symptom: Panic when analyzing Go files with certain syntax
- Cause: Combined case types in switch statement
- Fix: Separate each type into individual case statements

**Git churn not working**
- Symptom: Churn metrics are zero or analysis hangs
- Cause: Not in git repo, git not in PATH, or too many commits
- Fix: Use `--skip-churn` flag, or ensure git is available

**HTML visualization not rendering**
- Symptom: Blank page or JSON visible in browser
- Cause: JSON not properly escaped in HTML template
- Fix: Ensure JSON is wrapped with `template.JS()`

**Performance degradation on large repos**
- Symptom: Analysis takes >1 minute for 100K LOC
- Cause: Churn analysis doing expensive git operations
- Fix: Use `--skip-churn`, or run on SSD-backed storage

**SQLite database grows too large**
- Symptom: `.kaizen/` database file >100MB
- Cause: Accumulation of snapshots over time
- Fix: Use `kaizen history prune --days=30` to remove old snapshots

## Testing Strategy

**Unit Tests**: Each package has corresponding `*_test.go` files
- Test metric calculations with known complexity patterns
- Test parsing for language-specific edge cases
- Minimum 50% coverage for main packages (enforced by codecov)

**Integration Tests**: End-to-end analysis pipeline
- Build binary, run analysis, verify output format
- Test visualization output generation
- Test storage and snapshot retrieval

**CI/CD**: GitHub Actions runs on every PR
- Multi-platform (Linux, macOS, Windows)
- Multi-version (Go 1.21, 1.22, 1.23)
- Linting with `golangci-lint`
- Coverage uploaded to codecov

## Deployment and Releases

- Binary built with `go build` or `go install`
- Cross-platform builds via GitHub Actions
- Artifacts published to releases
- No external dependencies at runtime (except git for churn)

## Performance Characteristics

**Small Project** (10K LOC): ~0.1-0.5s
**Medium Project** (100K LOC): ~1-3s
**Large Project** (1M LOC): ~15-25s
**Enterprise** (10M+ LOC): ~3-5m

Factors affecting speed:
- Churn analysis adds 20-40% overhead (can be skipped)
- Language parser type (Go native < tree-sitter)
- Storage backend (SQLite for historical)
- Filesystem type (SSD > HDD by 2-3x)

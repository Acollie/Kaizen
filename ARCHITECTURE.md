# 🏗️ Kaizen Architecture Guide

Deep dive into how Kaizen is designed, built, and extended.

## Table of Contents

1. [System Overview](#system-overview)
2. [Core Design Patterns](#core-design-patterns)
3. [Project Structure](#project-structure)
4. [Analysis Pipeline](#analysis-pipeline)
5. [Metric Calculations](#metric-calculations)
6. [Storage & Persistence](#storage--persistence)
7. [Language Support](#language-support)
8. [Adding New Languages](#adding-new-languages)
9. [Performance Considerations](#performance-considerations)

---

## System Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Kaizen CLI (Cobra)                      │
│  analyze | visualize | diff | trend | report | history     │
└───────────────┬─────────────────────────────────────────────┘
                │
        ┌───────┴──────┐
        │              │
    ┌───▼────┐    ┌────▼───┐
    │ Config │    │ Churn   │
    │ Parser │    │Analyzer │
    └────────┘    └────┬────┘
                       │
            ┌──────────┴──────────┐
            │                     │
        ┌───▼──────┐      ┌──────▼──┐
        │ Language │      │   Git   │
        │Registry  │      │  Cmds   │
        └───┬──────┘      └─────────┘
            │
    ┌───────┴─────────┐
    │                 │
┌───▼────┬────┬────┬──▼───┐
│   Go   │ Kt │ Sw │ Py.. │
│  ast   │TS  │TS  │ Stub │
└────────┴────┴────┴──────┘
           │
      ┌────▼─────┐
      │ Analysis │
      │ Result   │
      └────┬─────┘
           │
    ┌──────┴──────┐
    │             │
┌───▼──┐      ┌───▼──┐
│SQLite│      │HTML  │
│ DB   │      │  /   │
│      │      │JSON  │
└──────┘      └──────┘
```

### Key Components

| Component | Purpose | Technology |
|-----------|---------|------------|
| **Pipeline** | Orchestrate analysis | Go goroutines |
| **Language Analyzers** | Parse code | go/ast, tree-sitter |
| **Aggregator** | Roll up metrics | Custom calculation |
| **Churn Analyzer** | Git history | Git CLI + parsing |
| **Storage Backend** | Persist results | SQLite + GORM |
| **Visualizations** | Display results | D3.js, HTML/CSS |
| **CLI** | User interface | Cobra framework |

---

## Core Design Patterns

### 1. **Interface-Based Language Analyzers**

All language support follows this interface:

```go
type LanguageAnalyzer interface {
    Name() string
    FileExtensions() []string
    CanAnalyze(filePath string) bool
    AnalyzeFile(filePath string) (*models.FileAnalysis, error)
    IsStub() bool
}
```

**Benefits:**
- ✅ Easy to add new languages
- ✅ No coupling between languages
- ✅ Can run in parallel
- ✅ Simple testing

### 2. **FunctionNode Pattern**

Language-specific function analysis:

```go
type FunctionNode interface {
    CalculateCyclomaticComplexity() int
    CalculateCognitiveComplexity() int
    CalculateNestingDepth() int
}
```

**Benefits:**
- ✅ Consistent complexity metrics across languages
- ✅ Language-specific optimizations
- ✅ AST-aware calculations

### 3. **Storage Backend Interface**

Multiple storage options:

```go
type StorageBackend interface {
    Save(snapshot *AnalysisResult) error
    GetLatest() (*AnalysisResult, error)
    GetTimeSeries(metric string, days int) ([]DataPoint, error)
    // ...
}
```

**Implementations:**
- SQLite (current)
- JSON files (backup)
- Ready for cloud storage (S3, etc.)

### 4. **Configuration Priority**

```
CLI Flags > .kaizen.yaml > .kaizenignore > Defaults
```

Allows flexibility from simple to complex configs.

---

## Project Structure

```
kaizen/
├── cmd/
│   └── kaizen/           # CLI entry point
│       ├── main.go       # Cobra command setup
│       ├── analyze.go    # analyze command
│       ├── visualize.go  # visualize command
│       ├── diff.go       # diff command (new)
│       └── ...
│
├── pkg/
│   ├── models/           # Data structures
│   │   └── models.go     # FileAnalysis, FunctionAnalysis, etc.
│   │
│   ├── analyzer/         # Core analysis engine
│   │   ├── pipeline.go   # Analysis orchestrator
│   │   ├── aggregator.go # Metric aggregation
│   │   ├── interfaces.go # LanguageAnalyzer, FunctionNode
│   │   └── *_test.go     # Tests
│   │
│   ├── languages/        # Multi-language support
│   │   ├── registry.go   # Language registration
│   │   ├── golang/       # Go analyzer (ast-based)
│   │   ├── kotlin/       # Kotlin analyzer (tree-sitter)
│   │   ├── swift/        # Swift analyzer (tree-sitter)
│   │   └── python/       # Python stub
│   │
│   ├── churn/            # Git integration
│   │   ├── analyzer.go   # Calculate churn metrics
│   │   └── *_test.go
│   │
│   ├── storage/          # Persistence layer
│   │   ├── interface.go  # StorageBackend interface
│   │   ├── sqlite.go     # SQLite implementation
│   │   ├── migrations.go # Database schema
│   │   └── *_test.go
│   │
│   ├── visualization/    # Output generation
│   │   ├── html.go       # Interactive treemap
│   │   ├── sankey.go     # Ownership diagrams
│   │   ├── terminal.go   # ASCII output
│   │   └── *_test.go
│   │
│   ├── reports/          # Reporting
│   │   ├── scorer.go     # Grade calculation
│   │   ├── grading.go    # A-F grading
│   │   ├── concerns.go   # Issue detection
│   │   └── *_test.go
│   │
│   ├── ownership/        # CODEOWNERS integration
│   │   ├── parser.go     # Parse CODEOWNERS
│   │   ├── aggregator.go # Team metrics
│   │   ├── reporter.go   # Generate reports
│   │   └── *_test.go
│   │
│   └── trending/         # Historical analysis
│       ├── ascii.go      # Terminal trends
│       ├── html.go       # Interactive charts
│       ├── json.go       # JSON export
│       └── *_test.go
│
├── internal/
│   └── config/           # Configuration loading
│       └── config.go     # .kaizen.yaml parsing
│
├── demo/                 # Demo project
│   └── sample-project/   # Example code
│
├── .github/
│   ├── workflows/        # CI/CD
│   ├── ISSUE_TEMPLATE/   # Issue templates
│   └── PULL_REQUEST_TEMPLATE.md
│
├── CLAUDE.md             # AI guidelines
├── README.md             # Main documentation
├── GUIDE.md              # Usage guide
├── ARCHITECTURE.md       # This file
├── CONTRIBUTING.md       # Contribution guidelines
├── CODE_OF_CONDUCT.md    # Community standards
├── LICENSE               # MIT license
├── SECURITY.md           # Security policy
├── CHANGELOG.md          # Version history
└── go.mod / go.sum       # Dependencies
```

---

## Analysis Pipeline

### Step-by-Step Flow

```
1. Configuration Loading
   └─ Load .kaizen.yaml, .kaizenignore, CLI flags

2. File Discovery
   └─ Walk directory tree, apply ignore patterns

3. Language Detection
   └─ Match extension to analyzer via Registry

4. File Analysis (parallel)
   ├─ Go AST → FunctionAnalysis[]
   ├─ Kotlin Tree-sitter → FunctionAnalysis[]
   ├─ Swift Tree-sitter → FunctionAnalysis[]
   └─ Python → (stub)

5. Churn Analysis (parallel)
   ├─ git log --numstat (file-level)
   └─ git log -L (function-level)

6. Aggregation
   └─ Roll up metrics by folder hierarchy

7. Scoring & Grading
   ├─ Calculate component scores
   ├─ Generate A-F grade
   └─ Detect areas of concern

8. Persistence
   ├─ Save to SQLite
   └─ Save JSON backup (optional)

9. Output
   ├─ Generate visualizations
   ├─ Print to terminal
   └─ Return results
```

### Code: Pipeline Execution

```go
// pkg/analyzer/pipeline.go
func (pipeline *Pipeline) Analyze(options AnalysisOptions) (*AnalysisResult, error) {
    // 1. Discover files
    files := pipeline.discoverFiles(options.RootPath)

    // 2. Analyze in parallel
    results := make(chan *FileAnalysis, len(files))
    for _, file := range files {
        go pipeline.analyzeFile(file, results)
    }

    // 3. Aggregate
    aggregated := pipeline.aggregator.Aggregate(fileAnalyses)

    // 4. Score
    scored := pipeline.score(aggregated)

    return scored, nil
}
```

---

## Metric Calculations

### Cyclomatic Complexity (CC)

**Definition:** Counts linearly independent paths through code

**Formula:** CC = 1 + Σ(decision points)

**Implementation:**

```go
// Count decision points in AST
cyclomatic := 1

ast.Inspect(funcNode, func(n ast.Node) bool {
    switch n.(type) {
    case *ast.IfStmt, *ast.ForStmt, *ast.CaseClause:
        cyclomatic++
    case *ast.BinaryExpr:
        if expr.Op == token.LAND || expr.Op == token.LOR {
            cyclomatic++
        }
    }
    return true
})
```

**Interpretation:**
- 1-5: Low complexity (good)
- 6-10: Moderate (acceptable)
- 11-15: High (review)
- 16+: Very high (refactor)

### Cognitive Complexity

**Definition:** CC with nesting penalty

**Formula:** CC + (nesting_level × bonus) per decision

**Implementation:**

```go
cognitive := 0
nestingLevel := 0

// For each decision point:
// cognitive += 1 + nestingLevel
// If entering nested block:
//   nestingLevel++
```

**Comparison:**

```go
// CC = 3, Cognitive = 6
if a {      // +1 CC, +1 Cognitive
    if b {  // +1 CC, +2 Cognitive (nested)
        if c { // +1 CC, +3 Cognitive (doubly nested)
            do()
        }
    }
}
```

### Halstead Metrics

**Operators:** Keywords, operators (`+`, `-`, `=`, etc.)
**Operands:** Variables, literals

**Metrics:**
- Vocabulary = # unique operators + # unique operands
- Length = total tokens
- Volume = Length × log₂(Vocabulary)
- Difficulty = (unique operators / 2) × (total operands / unique operands)
- Effort = Difficulty × Volume

**Implementation:**

```go
volume := float64(totalTokens) * math.Log2(float64(vocabulary))
difficulty := (float64(uniqueOps) / 2.0) * (float64(totalOps) / float64(uniqueOps))
effort := difficulty * volume
```

### Maintainability Index (MI)

**Definition:** How easy is code to maintain?

**Formula:** `171 - 5.2×ln(HalsteadVolume) - 0.23×CC - 16.2×ln(LOC)`

**Range:** 0-100
- 85-100: Easy to maintain
- 50-84: Moderate
- 0-49: Difficult

**Implementation:**

```go
mi := 171 -
    (5.2 * math.Log(halsteadVolume)) -
    (0.23 * float64(complexity)) -
    (16.2 * math.Log(float64(loc)))

mi = math.Max(0, math.Min(100, mi)) // Clamp to 0-100
```

### Hotspot Detection

**Definition:** High-churn + High-complexity code

**Formula:**
```
Hotspot = (complexity > threshold) AND (churn > threshold)
```

**Thresholds:**
- Complexity > 10
- Churn > 10 commits

**Why:** These are pain points needing immediate attention

---

## Storage & Persistence

### SQLite Schema

```sql
-- Snapshots - each analysis run
snapshots
├── id (PK)
├── analyzed_at (timestamp)
├── overall_score (float)
└── metadata (json)

-- File metrics per snapshot
folder_metrics
├── id (PK)
├── snapshot_id (FK)
├── scope_path (string)
├── file_count (int)
├── avg_complexity (float)
└── ...

-- Function-level metrics
function_metrics
├── id (PK)
├── file_path (string)
├── function_name (string)
├── complexity (int)
├── churn (int)
└── ...

-- Team ownership
file_ownership
├── id (PK)
├── file_path (string)
├── owner (string)
└── ...
```

### Data Persistence

```go
// Single interface for all storage
type StorageBackend interface {
    Save(result *AnalysisResult) (int64, error)
    GetLatest() (*AnalysisResult, error)
    GetLatestSummary() (*SnapshotSummary, error)
    GetTimeSeries(metric string, days int) ([]DataPoint, error)
    GetByID(id int64) (*AnalysisResult, error)
    Prune(keepDays int) error
}

// SQLite implementation
type SQLiteBackend struct {
    db *gorm.DB
}
```

---

## Language Support

### Supported Languages

#### Go ✅ Full Support

**Parser:** `go/ast` (built-in)
- Native Go package
- No external dependencies
- Very reliable

**Analyzer:** `pkg/languages/golang/analyzer.go`
- Uses `ast.Inspect()` for tree walking
- Type switches for AST node handling
- Native support for Go semantics

**Metrics:**
- CC: counts `if`, `for`, `switch case`, `&&`, `||`
- MI: calculated from LOC, CC, volume
- 95%+ coverage

#### Kotlin ✅ Full Support

**Parser:** Tree-sitter (github.com/smacker/go-tree-sitter/kotlin)
- AST-based parsing
- Cursor-based traversal
- Comprehensive node types

**Analyzer:** `pkg/languages/kotlin/analyzer.go`
- Identifies `function_declaration` nodes
- Extracts parameters, complexity
- Handles class/interface definitions

**Metrics:**
- CC: counts `if`, `when`, `for`, `while`, `try-catch`
- MI: calculated similarly to Go
- 90%+ coverage

#### Swift ✅ Full Support

**Parser:** Tree-sitter (github.com/smacker/go-tree-sitter/swift)
- AST-based parsing
- Robust Swift syntax support

**Analyzer:** `pkg/languages/swift/analyzer.go`
- Identifies `function_declaration` nodes
- Extracts parameter count
- Detects types (struct, class, protocol, enum)

**Metrics:**
- CC: counts `if`, `guard`, `for`, `while`, `switch`
- Cognitive: adds nesting penalty
- 90%+ coverage

#### Python 🚧 Stub

**Status:** Ready for implementation
**Parser:** Tree-sitter (available)
**Next Steps:** Follow Swift/Kotlin pattern

---

## Adding New Languages

### Step 1: Create Language Package

```bash
mkdir -p pkg/languages/python
cd pkg/languages/python
```

### Step 2: Implement LanguageAnalyzer Interface

```go
// pkg/languages/python/analyzer.go
package python

import "github.com/smacker/go-tree-sitter"
import "github.com/smacker/go-tree-sitter/python"

type PythonAnalyzer struct {
    language *sitter.Language
}

func NewPythonAnalyzer() analyzer.LanguageAnalyzer {
    return &PythonAnalyzer{
        language: python.GetLanguage(),
    }
}

func (pa *PythonAnalyzer) Name() string { return "Python" }
func (pa *PythonAnalyzer) FileExtensions() []string { return []string{".py"} }
func (pa *PythonAnalyzer) CanAnalyze(filePath string) bool { ... }
func (pa *PythonAnalyzer) IsStub() bool { return false }

func (pa *PythonAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
    // 1. Read source
    sourceBytes, _ := os.ReadFile(filePath)

    // 2. Parse with tree-sitter
    parser := sitter.NewParser()
    parser.SetLanguage(pa.language)
    tree := parser.Parse(nil, sourceBytes)

    // 3. Extract functions
    functions := pa.extractFunctions(tree.RootNode(), sourceBytes)

    // 4. Return analysis
    return &models.FileAnalysis{
        Language: "Python",
        Functions: functions,
        // ...
    }, nil
}

func (pa *PythonAnalyzer) extractFunctions(node *sitter.Node, src []byte) []models.FunctionAnalysis {
    // Recursive AST walk looking for "function_definition" nodes
    // ...
}
```

### Step 3: Implement Complexity Calculation

```go
// pkg/languages/python/function.go
package python

type PythonFunction struct {
    node        *sitter.Node
    sourceBytes []byte
}

func (pf *PythonFunction) CalculateCyclomaticComplexity() int {
    complexity := 1
    // Count: if, elif, except, for, while, and, or, comprehensions
    // ...
    return complexity
}

func (pf *PythonFunction) CalculateCognitiveComplexity() int {
    // Like CC but with nesting penalty
    // ...
}

func (pf *PythonFunction) CalculateNestingDepth() int {
    // Track max indentation level
    // ...
}
```

### Step 4: Register in Registry

```go
// pkg/languages/registry.go
import "github.com/alexcollie/kaizen/pkg/languages/python"

func NewRegistry() *Registry {
    return &Registry{
        analyzers: []analyzer.LanguageAnalyzer{
            golang.NewGoAnalyzer(),
            kotlin.NewKotlinAnalyzer(),
            swift.NewSwiftAnalyzer(),
            python.NewPythonAnalyzer(),  // ADD THIS
        },
    }
}
```

### Step 5: Write Tests

```go
// pkg/languages/python/analyzer_test.go
package python

func TestCanAnalyze(t *testing.T) {
    analyzer := NewPythonAnalyzer()
    assert.True(t, analyzer.CanAnalyze("test.py"))
}

func TestAnalyzeFile(t *testing.T) {
    // Create test.py with sample code
    // Analyze it
    // Check results
}
```

### Step 6: Update Documentation

- Add to README.md language table
- Document any language-specific considerations
- Add examples to GUIDE.md

### Step 7: Submit PR

All done! Your new language will automatically work with:
- ✅ `kaizen analyze`
- ✅ `kaizen visualize`
- ✅ `kaizen diff`
- ✅ `kaizen trend`
- ✅ Team ownership reports

---

## Performance Considerations

### Optimization Strategies

#### 1. Parallel File Analysis

```go
// Process files in parallel
results := make(chan *FileAnalysis, len(files))
for _, file := range files {
    go pipeline.analyzeFile(file, results)
}
```

**Speedup:** Near-linear with CPU cores

#### 2. Skip Expensive Operations

```bash
# Skip git churn analysis (can be slow on large repos)
kaizen analyze --path=. --skip-churn

# Add it later
kaizen analyze --path=.  # Full analysis
```

**Speedup:** 5-10x on large repos

#### 3. Incremental Analysis

**Future feature:** Only re-analyze changed files

#### 4. Caching

**Future feature:** Cache AST parsing results

### Typical Performance

| Project | Files | LOC | Time | Notes |
|---------|-------|-----|------|-------|
| Small | 10 | 1K | 0.1s | Very fast |
| Medium | 100 | 20K | 0.5s | Includes churn |
| Large | 1K | 200K | 3s | Full analysis |
| XL | 10K | 2M | 20s | With git history |

### Profiling

```bash
# Build with profiling
go build -o kaizen ./cmd/kaizen

# Run with CPU profiling
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

---

## Key Design Decisions

### 1. Interface-Based Languages

**Decision:** All languages implement common interface
**Rationale:**
- Easy to add languages without modifying core
- Clear contract for new implementations
- Testable independently

### 2. Tree-Sitter for New Languages

**Decision:** Use tree-sitter instead of language-specific parsers
**Rationale:**
- Consistent across languages
- Reliable AST parsing
- Maintained community parsers

### 3. SQLite for Storage

**Decision:** SQLite instead of other databases
**Rationale:**
- Zero-config (embedded)
- Good for time-series
- File-based (portable)

### 4. Cobra for CLI

**Decision:** Cobra framework for CLI
**Rationale:**
- Professional, battle-tested
- Great help/completion
- Flag management

---

## Future Architecture Improvements

- **Distributed Analysis:** Process massive codebases across machines
- **Incremental Analysis:** Only re-parse changed files
- **Caching Layer:** Cache AST parsing results
- **Custom Metrics:** Plugin system for organization-specific metrics
- **Cloud Storage:** S3/Cloud backend support
- **Web Dashboard:** Real-time monitoring interface

---

## Contributing to Architecture

Want to improve Kaizen's internals?

1. **File Issues** - Discuss major changes first
2. **Benchmarks** - Show performance impact
3. **Tests** - All changes need tests
4. **Documentation** - Update ARCHITECTURE.md
5. **PR** - Reference issue, explain rationale

See [CONTRIBUTING.md](./CONTRIBUTING.md) for details.


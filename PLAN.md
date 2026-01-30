# Kaizen - Code Analysis Tool

## Project Overview

Kaizen is a code analysis tool that reads Go and Kotlin codebases to generate heat maps based on:
- **Code Churn** (from git history)
- **Function Length**
- **Cyclomatic Complexity**
- **Cognitive Complexity**
- And many other code quality metrics

The tool is split into two main components:
1. **Analyzer** - Parses code, calculates metrics, extracts git history
2. **Visualizer** - Generates heat maps showing code health by folder

## Architecture

```
┌─────────────────────────────────────────────────┐
│              CLI Entry Point (Cobra)             │
│  - kaizen analyze                                │
│  - kaizen visualize                              │
└─────────────────┬───────────────────────────────┘
                  │
         ┌────────┴────────┐
         │                 │
    ┌────▼─────┐     ┌────▼──────┐
    │ Analyzer │     │ Visualizer│
    │ Pipeline │     │  Engine   │
    └────┬─────┘     └────▲──────┘
         │                │
         │    ┌───────────┘
         │    │ JSON Results File
         │    │
         └────▼─────────────────────┐
         │  Language Analyzers       │
         │  - Go (full impl)         │
         │  - Kotlin (stub)          │
         └───────────────────────────┘
```

## Directory Structure

```
kaizen/
├── PLAN.md                    # This file
├── README.md                  # User documentation
├── go.mod                     # Go module definition
├── go.sum                     # Dependency checksums
│
├── cmd/
│   ├── analyze/               # CLI command for analysis
│   │   └── main.go
│   └── visualize/             # CLI command for visualization
│       └── main.go
│
├── pkg/
│   ├── analyzer/
│   │   ├── interfaces.go      # Core interfaces
│   │   ├── pipeline.go        # Analysis orchestration
│   │   ├── aggregator.go      # Folder-level aggregation
│   │   └── metrics.go         # Metric calculation utilities
│   │
│   ├── languages/
│   │   ├── registry.go        # Auto-detection of languages
│   │   │
│   │   ├── golang/
│   │   │   ├── analyzer.go    # Go implementation of LanguageAnalyzer
│   │   │   ├── parser.go      # AST parsing using go/ast
│   │   │   ├── function.go    # FunctionNode implementation
│   │   │   └── complexity.go  # Complexity calculations
│   │   │
│   │   └── kotlin/
│   │       ├── analyzer.go    # Kotlin stub (returns "not implemented")
│   │       └── README.md      # How to implement Kotlin support
│   │
│   ├── churn/
│   │   ├── git.go             # Git command execution
│   │   └── analyzer.go        # ChurnAnalyzer implementation
│   │
│   ├── models/
│   │   └── models.go          # All data structures
│   │
│   └── visualization/
│       ├── heatmap.go         # Heat map generation logic
│       ├── html.go            # HTML export with D3.js
│       ├── terminal.go        # Terminal-based colored output
│       └── json.go            # JSON export
│
└── internal/
    └── config/
        └── config.go          # Configuration management
```

## Core Design Principles

### 1. Language Agnostic via Interfaces

The `LanguageAnalyzer` interface allows adding new languages without changing core logic:

```go
type LanguageAnalyzer interface {
    Name() string
    FileExtensions() []string
    CanAnalyze(filePath string) bool
    AnalyzeFile(filePath string) (*models.FileAnalysis, error)
    IsStub() bool  // Returns true for incomplete implementations
}
```

**Kotlin is stubbed** - it implements the interface but returns errors with helpful messages about being unimplemented.

### 2. Separation of Concerns

- **Analyzers** parse code and extract metrics (no git knowledge)
- **Churn Analyzer** handles git operations (no language knowledge)
- **Pipeline** orchestrates analysis and combines results
- **Aggregator** rolls up file metrics to folders
- **Visualizer** only consumes JSON, knows nothing about analysis

### 3. Data Flow

```
Source Code Files → Language Analyzer → FileAnalysis
      +                                        │
Git Repository  → Churn Analyzer ──────────────┘
                                               ↓
                                      AnalysisResult (JSON)
                                               ↓
                                        Visualizer
                                               ↓
                                   Heat Map (HTML/Terminal)
```

## Metrics Collected

### Tier 1 - Essential (MVP)
1. **Cyclomatic Complexity** - Count of decision points
2. **Function Length** - Lines per function
3. **Code Churn** - Git commits, lines changed
4. **Nesting Depth** - Maximum nesting level
5. **Parameter Count** - Number of function parameters
6. **Comment Density** - Comment lines / total lines

### Tier 2 - High Value
1. **Cognitive Complexity** - Nesting-weighted complexity
2. **Hotspots** - High churn + high complexity
3. **Maintainability Index** - Composite maintainability score
4. **Fan-in/Fan-out** - Function call relationships
5. **Halstead Metrics** - Volume, difficulty, effort

### Tier 3 - Future
1. **Code Duplication** - Clone detection
2. **LCOM** - Lack of cohesion of methods
3. **Coupling Metrics** - Afferent/efferent coupling
4. **Test Coverage** - Integration with coverage tools
5. **Temporal Coupling** - Files that change together

## Implementation Phases

### Phase 1: Core Infrastructure ✓ (Current)
- [x] Project structure
- [x] Core interfaces
- [x] Data models
- [ ] Go analyzer with basic metrics
- [ ] Kotlin stub analyzer
- [ ] Language registry
- [ ] Basic metric calculators

### Phase 2: Full Go Support
- [ ] Complete Go AST parsing
- [ ] Cyclomatic complexity calculation
- [ ] Cognitive complexity calculation
- [ ] Halstead metrics
- [ ] All Tier 1 metrics
- [ ] Unit tests for Go analyzer

### Phase 3: Git Integration
- [ ] Git command wrapper
- [ ] File churn calculation
- [ ] Function-level churn (git log -L)
- [ ] Temporal coupling detection
- [ ] Contributor analysis

### Phase 4: Analysis Pipeline
- [ ] Pipeline orchestration
- [ ] File discovery and filtering
- [ ] Parallel processing
- [ ] Progress reporting
- [ ] Folder aggregation
- [ ] Normalization/scoring

### Phase 5: Visualization
- [ ] Terminal output with colors
- [ ] HTML heat map with D3.js treemap
- [ ] Multiple metric views
- [ ] Drill-down capability
- [ ] JSON export

### Phase 6: CLI and UX
- [ ] Cobra CLI setup
- [ ] Configuration file support
- [ ] Filtering options
- [ ] Output formats
- [ ] Documentation

### Phase 7: Kotlin Support (Future)
- [ ] Research tree-sitter or Kotlin parser
- [ ] Implement Kotlin analyzer
- [ ] Kotlin-specific metrics
- [ ] Testing with real Kotlin code

## CLI Usage

```bash
# Analyze current directory, last 30 days of git history
kaizen analyze --path=. --since=30d --output=results.json

# Analyze specific path with custom time range
kaizen analyze --path=/path/to/repo --since=2024-01-01 --output=analysis.json

# Visualize as HTML heat map (complexity metric)
kaizen visualize --input=results.json --metric=complexity --format=html --output=heatmap.html

# Visualize in terminal (churn metric)
kaizen visualize --input=results.json --metric=churn --format=terminal

# Combined: analyze and visualize
kaizen analyze --path=. --visualize --metric=hotspot

# Filter by language
kaizen analyze --path=. --languages=go,kotlin

# Exclude folders
kaizen analyze --path=. --exclude=vendor,node_modules,test
```

## Configuration File

`.kaizen.yaml` (optional):
```yaml
# Analysis settings
analysis:
  since: 90d
  languages:
    - go
    - kotlin
  exclude:
    - vendor
    - node_modules
    - "*_test.go"
    - "*/testdata/*"

# Thresholds for warnings
thresholds:
  cyclomatic_complexity: 10
  cognitive_complexity: 15
  function_length: 50
  nesting_depth: 4
  parameter_count: 5

# Visualization settings
visualization:
  default_metric: hotspot
  color_scheme: red-yellow-green
  show_percentages: true
```

## Language-Specific Details

### Go Implementation
- Uses stdlib `go/ast` and `go/parser`
- Parses AST to extract functions
- Walks AST to count decision points
- Tracks nesting levels during traversal
- Fast and reliable

### Kotlin Stub
- Implements `LanguageAnalyzer` interface
- Returns `IsStub() = true`
- `AnalyzeFile()` returns error: "Kotlin support not yet implemented"
- Provides clear path for future implementation
- Suggests using tree-sitter-kotlin or external parser

### Adding New Languages
1. Create `pkg/languages/<lang>/` directory
2. Implement `LanguageAnalyzer` interface
3. Register in `pkg/languages/registry.go`
4. Add file extensions
5. Implement metric calculations

## Git Churn Analysis

### File-Level Churn
```bash
git log --since=<date> --numstat --follow -- <file>
```
Extracts:
- Total commits
- Lines added
- Lines deleted
- Contributors
- Last modification date

### Function-Level Churn
```bash
git log -L :<funcname>:<file> --since=<date>
```
Tracks changes to specific functions over time.

### Challenges
- Function renames (may break tracking)
- Function moves (need --follow equivalent)
- Multiple functions with same name

## Visualization Approach

### Heat Map Design
- **Treemap** - Folders as rectangles, size = LOC
- **Color** - Based on selected metric
  - Green = good (low complexity/churn)
  - Yellow = moderate
  - Red = problematic (high complexity/churn)

### Metrics Available for Visualization
1. **Complexity** - Average cyclomatic complexity
2. **Cognitive** - Average cognitive complexity
3. **Churn** - Total changes
4. **Hotspot** - Combined churn + complexity (most important)
5. **Maintainability** - Maintainability index
6. **Length** - Average function length

### Interactive Features (HTML)
- Click folder to drill down
- Hover to see details
- Toggle between metrics
- Show top 10 problematic functions

## Testing Strategy

### Unit Tests
- Each analyzer (Go, Kotlin stub)
- Metric calculations
- Git churn parsing
- Aggregation logic

### Integration Tests
- Full pipeline with sample repos
- Real git repositories
- Multiple languages

### Test Data
- `testdata/` directory with sample code
- Known complexity examples
- Git repo with known history

## Performance Considerations

1. **Parallel File Processing**
   - Use worker pool (e.g., 8 workers)
   - Parse files concurrently
   - Combine results

2. **Git Command Optimization**
   - Cache git log results
   - Batch git operations
   - Use `--numstat` for efficiency

3. **Large Repositories**
   - Stream results
   - Progress reporting
   - Memory-efficient aggregation

## Error Handling

- Graceful degradation (skip unparseable files)
- Warn on git errors (continue without churn)
- Report stub languages clearly
- Validate configuration

## Future Enhancements

1. **Language Support**
   - Java
   - Python
   - TypeScript/JavaScript
   - Rust

2. **Advanced Analysis**
   - Code clone detection
   - Architectural metrics
   - Dependency analysis
   - Test coverage integration

3. **CI/CD Integration**
   - GitHub Actions
   - GitLab CI
   - Quality gates
   - Trend analysis over time

4. **Output Formats**
   - PDF reports
   - CSV export
   - SonarQube format
   - GitHub annotations

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/fatih/color` - Terminal colors
- `go/ast`, `go/parser`, `go/token` - Go parsing (stdlib)
- Potential: `github.com/smacker/go-tree-sitter` - For Kotlin

## Success Criteria

1. Can analyze Go codebases accurately
2. Produces actionable heat maps
3. Git churn integration works
4. Fast enough for large repos (< 5 min for 100k LOC)
5. Clear documentation for adding languages
6. Kotlin stub demonstrates extensibility

## Open Questions

1. **Kotlin Parser**: tree-sitter vs. calling `kotlinc`?
2. **Function Churn**: How to handle renames?
3. **Visualization Library**: D3.js vs. Chart.js vs. custom?
4. **Storage**: JSON files vs. SQLite for historical data?
5. **Baseline**: Compare against previous runs?

## References

- [Cyclomatic Complexity - McCabe](https://en.wikipedia.org/wiki/Cyclomatic_complexity)
- [Cognitive Complexity - SonarSource](https://www.sonarsource.com/resources/cognitive-complexity/)
- [Halstead Metrics](https://en.wikipedia.org/wiki/Halstead_complexity_measures)
- [Maintainability Index](https://docs.microsoft.com/en-us/visualstudio/code-quality/code-metrics-values)
- [Code Churn](https://www.pluralsight.com/blog/teams/what-is-code-churn)

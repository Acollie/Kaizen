# Kaizen - Code Quality Analysis Tool

A powerful code analysis tool that measures code quality, complexity, and churn to identify technical debt and hotspots in your codebase. Generates health grades, actionable concerns, and interactive visualizations.

## Features

- **Code Health Grading** - Overall A-F grade with 0-100 score
- **Areas of Concern** - Actionable issues with severity levels and VS Code links
- **Multi-Language Support** - Currently supports Go (Kotlin stubbed for future)
- **Comprehensive Metrics** - Cyclomatic, Cognitive, Halstead, Maintainability Index
- **Git Churn Analysis** - Tracks code change frequency over time
- **Hotspot Detection** - Identifies high-churn + high-complexity code
- **Interactive Visualizations** - Zoomable HTML treemaps with drill-down navigation

## Installation

```bash
# Clone the repository
git clone https://github.com/alexcollie/kaizen
cd kaizen

# Build
go build -o kaizen ./cmd/kaizen

# Or install globally
go install ./cmd/kaizen
```

## Quick Start

```bash
# Analyze current directory
kaizen analyze --path=.

# Generate interactive HTML heat map
kaizen visualize --input=kaizen-results.json --format=html
```

**Example Output:**
```
📋 Code Health Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Overall Grade: A (90/100)

Component Scores:
  Complexity:       ████████░░ 80/100 (good)
  Maintainability:  █████████░ 91/100 (excellent)
  Churn:            ███████░░░ 70/100 (moderate)
  Function Size:    █████████░ 94/100 (excellent)
  Code Structure:   ██████████ 100/100 (excellent)

Areas of Concern (2):

  [CRITICAL] Complexity Hotspots
    These functions average CC:15 with 12 commits each. High complexity
    makes changes error-prone, and frequent changes multiply that risk.
    - pkg/analyzer/pipeline.go:45 (Analyze)
    - pkg/visualization/html.go:123 (GenerateHTML)

  [WARNING] Low Maintainability
    Low scores driven by long functions (avg 51 lines). Break into
    smaller, focused functions to improve readability.
    - pkg/churn/analyzer.go:119 (parseNumstatOutput)
```

---

## Algorithms & Metrics

### Cyclomatic Complexity (CC)

Measures the number of linearly independent paths through code. Introduced by Thomas McCabe in 1976.

**Algorithm:**
```
CC = E - N + 2P

Where:
  E = number of edges in the control flow graph
  N = number of nodes in the control flow graph
  P = number of connected components (usually 1)
```

**Simplified counting method (used by Kaizen):**
```
CC = 1 + count of:
  - if statements
  - else if / else branches
  - for / while loops
  - case clauses in switch
  - catch clauses
  - && and || operators
  - ?: ternary operators
```

**Interpretation:**
| CC | Risk Level | Recommendation |
|----|------------|----------------|
| 1-5 | Low | Simple, easy to test |
| 6-10 | Moderate | Acceptable complexity |
| 11-20 | High | Consider refactoring |
| 21+ | Very High | Must refactor |

---

### Cognitive Complexity

Developed by SonarSource as a more accurate measure of code understandability. Unlike cyclomatic complexity, it penalizes nested structures.

**Algorithm:**
```
Cognitive Complexity = sum of:
  1. +1 for each break in linear flow:
     - if, else if, else
     - switch, case
     - for, while, do-while
     - catch
     - goto, break/continue to label
     - sequences of && or ||
     - recursion

  2. +1 nesting penalty for each level when inside:
     - if, else if, else
     - switch
     - for, while, do-while
     - catch
     - nested functions/lambdas
```

**Example:**
```go
func example(a, b int) int {      // +0
    if a > 0 {                    // +1 (if)
        for i := 0; i < b; i++ {  // +2 (for +1, nesting +1)
            if i > a {            // +3 (if +1, nesting +2)
                return i          // +0 (no increment)
            }
        }
    }
    return 0
}
// Cognitive Complexity = 6
```

---

### Halstead Metrics

Software science metrics developed by Maurice Halstead in 1977. Based on counting operators and operands.

**Definitions:**
```
n1 = number of distinct operators
n2 = number of distinct operands
N1 = total number of operators
N2 = total number of operands
```

**Calculated Metrics:**
```
Vocabulary:        n = n1 + n2
Program Length:    N = N1 + N2
Volume:            V = N × log₂(n)
Difficulty:        D = (n1/2) × (N2/n2)
Effort:            E = D × V
Time to Program:   T = E / 18 seconds
Bugs Delivered:    B = V / 3000
```

**Operators include:** `+`, `-`, `*`, `/`, `=`, `==`, `!=`, `<`, `>`, `&&`, `||`, `if`, `for`, `return`, `func`, `.`, `,`, `(`, `)`, `{`, `}`, etc.

**Operands include:** Variables, constants, literals, function names

---

### Maintainability Index (MI)

Composite metric indicating how maintainable code is. Originally developed at Hewlett-Packard in 1992.

**Formula (used by Kaizen):**
```
MI = 171 - 5.2 × ln(HV) - 0.23 × CC - 16.2 × ln(LOC)

Where:
  HV  = Halstead Volume
  CC  = Cyclomatic Complexity
  LOC = Lines of Code

Result is clamped to 0-100 range.
```

**Interpretation:**
| MI | Rating | Description |
|----|--------|-------------|
| 85-100 | Excellent | Highly maintainable |
| 65-84 | Good | Reasonably maintainable |
| 40-64 | Moderate | Difficult to maintain |
| 0-39 | Poor | Very difficult to maintain |

---

### Churn Analysis

Measures how frequently code changes using git history.

**Algorithm:**
```bash
# File-level churn
git log --numstat --since="90 days ago" -- <file>

# Extracts:
- Total commits touching the file
- Lines added
- Lines deleted
- Unique contributors
- Last modified date
```

**Churn Score Calculation:**
```
ChurnScore = 100 - clamp(avgCommitsPerFunction × 2, 0, 100)

Where higher commits = lower score (more churn = more risk)
```

**Why Churn Matters:**
- Code that changes frequently has more opportunity for bugs
- Combined with complexity, identifies highest-risk code
- Helps prioritize refactoring efforts

---

### Hotspot Detection

A **hotspot** is a function that is both complex AND changes frequently.

**Detection Rule:**
```
IsHotspot = (CyclomaticComplexity > 10) AND (TotalCommits > 10)
```

**Why Hotspots Matter:**
- Complex code is harder to modify correctly
- Frequently changed code has more chances for bugs
- The combination multiplies risk exponentially

---

## Score Report & Grading

### Overall Grade

Kaizen calculates an overall health grade (A-F) based on weighted component scores.

**Component Weights:**
| Component | Weight | What It Measures |
|-----------|--------|------------------|
| Complexity | 25% | Average cyclomatic complexity |
| Maintainability | 25% | Average maintainability index |
| Churn | 20% | Code change frequency |
| Function Size | 15% | % of long/very long functions |
| Code Structure | 15% | Nesting depth, parameters, high CC |

**Grade Thresholds:**
| Grade | Score Range |
|-------|-------------|
| A | 90-100 |
| B | 75-89 |
| C | 60-74 |
| D | 40-59 |
| F | 0-39 |

**Component Score Formulas:**
```
Complexity Score      = 100 - clamp(avgCC × 5, 0, 100)
Maintainability Score = avgMI (already 0-100)
Churn Score          = 100 - clamp(avgCommits × 2, 0, 100)
Function Size Score  = 100 - (longFuncPct × 50 + veryLongFuncPct × 50)
Code Structure Score = 100 - (highNestingPct × 40 + highParamPct × 30 + veryHighCCPct × 30)
```

---

### Areas of Concern

Kaizen automatically detects and reports code issues with severity levels.

**Concern Types:**

| Type | Severity | Trigger |
|------|----------|---------|
| Complexity Hotspot | Critical | CC > 10 AND Churn > 10 commits |
| Large Function + High Churn | Critical | Length > 100 AND Churn > 20 |
| Low Maintainability | Critical/Warning | MI < 20 (critical) or MI < 40 (warning) |
| Deep Nesting | Warning/Info | Depth > 7 (warning) or > 5 (info) |
| Too Many Parameters | Warning/Info | Params > 10 (warning) or > 7 (info) |
| God Function | Warning | Params > 6 AND FanIn > 10 |

**Smart Descriptions:**

Concern descriptions explain *why* the issue exists:
```
[CRITICAL] Critical Maintainability Issues
  Low scores driven by long functions (avg 106 lines), high complexity
  (avg CC: 15.7) and dense code with many operators/operands. Break
  into smaller, focused functions to improve readability.
```

---

## Visualization

### Interactive HTML Treemap

The HTML visualization includes:

- **Grade Circle** - Large A-F grade with score
- **Component Score Bars** - Visual breakdown of each component
- **Zoomable Treemap** - Click to drill down into folders
- **Breadcrumb Navigation** - Click to zoom back out
- **Metric Switching** - Toggle between complexity, churn, hotspot, etc.
- **Concerns Panel** - Collapsible list with VS Code links

**Drill-Down Navigation:**
```
Click "pkg" → zooms into pkg folder
Click "languages" → zooms into languages subfolder
Click breadcrumb "." → zooms back to root
```

**VS Code Integration:**

Clicking a file path in the concerns panel opens it directly in VS Code:
```
vscode://file//path/to/file.go:123
```

### Generate Visualizations

```bash
# Interactive HTML (opens in browser)
kaizen visualize --format=html

# Static SVG
kaizen visualize --format=svg --metric=complexity

# Terminal output
kaizen visualize --format=terminal
```

---

## Configuration

### `.kaizenignore`

Exclude files from analysis (gitignore syntax):

```gitignore
# Dependencies
vendor/
node_modules/

# Generated code
*.pb.go
*.generated.go

# Tests
*_test.go
```

### `.kaizen.yaml`

Full configuration:

```yaml
analysis:
  since: 90d
  languages: [go]
  exclude: [vendor, "*_test.go"]
  skip_churn: false
  max_workers: 8

thresholds:
  cyclomatic_complexity: 10
  cognitive_complexity: 15
  function_length: 50
  maintainability_index: 40
```

---

## CLI Reference

### `kaizen analyze`

```bash
kaizen analyze [flags]

Flags:
  -p, --path string       Path to analyze (default ".")
  -s, --since string      Churn period (default "90d")
  -o, --output string     Output file (default "kaizen-results.json")
  -l, --languages strings Languages to include
  -e, --exclude strings   Patterns to exclude
      --skip-churn        Skip git churn analysis
```

### `kaizen visualize`

```bash
kaizen visualize [flags]

Flags:
  -i, --input string    Input JSON (default "kaizen-results.json")
  -f, --format string   Format: terminal, html, svg (default "terminal")
  -m, --metric string   Metric: complexity, churn, hotspot, etc.
  -o, --output string   Output file
      --open            Auto-open HTML (default true)
```

### `kaizen callgraph`

```bash
kaizen callgraph [flags]

Flags:
  -p, --path string     Path to analyze (default ".")
  -o, --output string   Output file
  -f, --format string   Format: html, svg, json
      --min-calls int   Filter by minimum call count
```

---

## Architecture

```
pkg/
├── analyzer/       # Core analysis pipeline
├── languages/      # Language-specific AST parsers
│   ├── golang/     # Go analyzer (fully implemented)
│   └── kotlin/     # Kotlin stub
├── churn/          # Git history analysis
├── models/         # Data structures
├── reports/        # Score calculation & concerns
└── visualization/  # HTML, SVG, terminal output
```

### Analysis Pipeline

```
Source Files
    ↓ (discover files, apply ignore patterns)
Filtered File List
    ↓ (match file extensions)
Language Analyzer
    ↓ (parse AST, extract functions)
FileAnalysis + FunctionAnalysis[]
    ↓ (git log --numstat)
Churn Metrics
    ↓ (aggregate by folder)
FolderMetrics
    ↓ (calculate percentiles)
Normalized Scores
    ↓ (weighted average)
ScoreReport + Concerns
    ↓ (render)
JSON / HTML / Terminal Output
```

---

## Adding Language Support

1. Create `pkg/languages/<lang>/analyzer.go`
2. Implement `LanguageAnalyzer` interface:

```go
type LanguageAnalyzer interface {
    Name() string
    FileExtensions() []string
    CanAnalyze(filePath string) bool
    AnalyzeFile(filePath string) (*models.FileAnalysis, error)
    IsStub() bool
}
```

3. Register in `pkg/languages/registry.go`

---

## Roadmap

- [x] Code health grading (A-F)
- [x] Areas of concern with explanations
- [x] Zoomable treemap visualization
- [x] VS Code integration
- [ ] Kotlin language support
- [ ] Python, TypeScript, Java support
- [ ] Historical trend analysis
- [ ] CI/CD integration
- [ ] GitHub Actions reporter

---

## Credits

Inspired by:
- [SonarQube](https://www.sonarqube.org/) - Code quality platform
- [Code Climate](https://codeclimate.com/) - Maintainability scoring
- [Code Maat](https://github.com/adamtornhill/code-maat) - Churn analysis
- [Your Code as a Crime Scene](https://pragprog.com/titles/atcrime/your-code-as-a-crime-scene/) - Hotspot concept

## License

MIT

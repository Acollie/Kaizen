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
- **Historical Tracking** - SQLite database for time-series analysis (Phase 1)
- **Trend Analysis** - ASCII, JSON, and HTML charts showing metric evolution (Phase 2)
- **Code Ownership Reports** - Team-based metrics aggregation with CODEOWNERS integration (Phase 3)
- **Ownership Flow Diagrams** - Interactive Sankey diagrams showing team dependencies on shared functions

## Installation

### Quick Install (Recommended)

Install kaizen with shell completion for zsh and fish:

```bash
# Clone the repository
git clone https://github.com/alexcollie/kaizen.git
cd kaizen

# Run the install script
./install.sh
```

The install script will:
- Build the kaizen binary
- Install it to `~/.local/bin` (or `$INSTALL_DIR` if set)
- Set up shell completions for zsh and fish
- Provide next steps for configuration

**Shell Completion:**
- **Zsh**: Completions installed to `~/.local/share/zsh/site-functions/_kaizen`
- **Fish**: Completions installed to `~/.config/fish/completions/kaizen.fish`

After installation, restart your shell or run:
```bash
# Zsh
exec zsh

# Fish
exec fish
```

### Manual Installation

```bash
# Build
go build -o kaizen ./cmd/kaizen

# Install to custom location
mv kaizen /usr/local/bin/

# Or install with Go
go install ./cmd/kaizen
```

## Quick Start

```bash
# Analyze current directory (automatically saves to database)
kaizen analyze --path=.

# Generate interactive HTML heat map
kaizen visualize --format=html

# View analysis history
kaizen history list

# Generate ownership report
kaizen report owners

# View trends over time
kaizen trend overall_score
```

## Common Workflows

### First Time Setup

```bash
# 1. Install kaizen
./install.sh

# 2. Analyze your project
cd /path/to/your/project
kaizen analyze --path=.

# 3. View the results
kaizen visualize --format=html
```

### Daily Development

```bash
# Run analysis before committing
kaizen analyze --path=.

# Quick terminal view of hotspots
kaizen visualize --metric=hotspot

# Check if code quality is improving
kaizen trend overall_score --days=7
```

### Team Health Monitoring

```bash
# Generate ownership report
kaizen report owners --format=html

# Export team metrics for CI/CD
kaizen report owners --format=json --output=team-health.json

# Visualize team dependencies
kaizen sankey --min-owners=3
```

### Refactoring Sessions

```bash
# Before refactoring - create baseline
kaizen analyze --path=. --output=before.json

# After refactoring - compare results
kaizen analyze --path=. --output=after.json

# View improvements
kaizen visualize --input=after.json --format=html

# Check trends
kaizen trend complexity --days=1
```

### CI/CD Integration

```bash
#!/bin/bash
# .github/workflows/code-quality.yml or similar

# Run analysis
kaizen analyze --path=.

# Fail if grade drops below B
SCORE=$(kaizen history show 1 | grep "Score:" | awk '{print $2}')
if (( $(echo "$SCORE < 75" | bc -l) )); then
  echo "❌ Code quality below threshold (Score: $SCORE)"
  exit 1
fi

# Generate reports
kaizen report owners --format=html --output=reports/team-health.html
kaizen visualize --format=html --output=reports/complexity-map.html
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

## Historical Analysis & Trends

### Phase 1: Time-Series Storage

Kaizen automatically stores all analysis results in SQLite database (`.kaizen/kaizen.db`). This enables tracking metrics over time and comparing code quality across versions.

**Basic Usage:**

```bash
# Run analysis (automatically saves to database)
kaizen analyze --path=.

# List all snapshots
kaizen history list

# View specific snapshot
kaizen history show 1

# Prune old snapshots (keep last 90 days)
kaizen history prune --retention=90
```

**Example Output:**
```
ID   Date                 Grade   Score   Files
──────────────────────────────────────────────────
3    2026-01-30 15:36    A       89.2    35
2    2026-01-30 15:35    A       87.5    35
1    2026-01-30 15:00    B       82.1    34
```

---

### Phase 2: Trend Visualization

Track metric evolution over time with multiple output formats.

**Basic Usage:**

```bash
# Run multiple analyses over time
kaizen analyze --path=.
# ... make changes ...
kaizen analyze --path=.
# ... make more changes ...
kaizen analyze --path=.

# View trends (ASCII format, default)
kaizen trend overall_score
kaizen trend complexity
kaizen trend maintainability

# Export trends as JSON
kaizen trend overall_score --format=json --output=trends.json

# Generate interactive HTML chart
kaizen trend complexity --format=html --output=complexity-chart.html
```

**Supported Metrics:**
- `overall_score` - Overall health score
- `complexity` - Average cyclomatic complexity
- `maintainability` - Average maintainability index
- `hotspots` - Number of hotspots detected

**Example ASCII Output:**
```
Complexity Trend (Last 30 days)
─────────────────────────────────
5.2  │
5.0  │      ╱╲
4.8  │    ╱╲  ╲
4.6  │  ╱    ╲  ╲
4.4  │╱        ╲__╲
└────┴──────────────── Time
     Min: 4.2 | Max: 5.2 | Avg: 4.8 | Change: -0.4
```

---

### Phase 3: Code Ownership Reports

Aggregate metrics by team using CODEOWNERS file for team-based accountability.

**Setup:**

First, create a `.github/CODEOWNERS` file:
```
# CODEOWNERS file (GitHub/GitLab format)
# Last matching rule wins (most specific at bottom)

* @maintainers

pkg/storage/ @storage-team
pkg/storage/sqlite.go @db-expert

pkg/analyzer/ @analysis-team
pkg/languages/ @language-team
pkg/languages/golang/ @golang-expert

pkg/visualization/ @ui-team
```

**Basic Usage:**

```bash
# Generate ownership report (ASCII format, default)
kaizen report owners

# Specific snapshot
kaizen report owners 2

# Export as JSON
kaizen report owners --format=json --output=team-metrics.json

# Generate interactive HTML report
kaizen report owners --format=html --output=team-report.html

# Open HTML in browser (automatic)
kaizen report owners --format=html
```

**Example ASCII Output:**
```
👥 Code Ownership Report
═════════════════════════════════════════════════════════════════

Owner              │ Files │ Funcs │ Health  │ Avg Cmplx │ Hotspots
────────────────────┼───────┼───────┼─────────┼───────────┼──────────
@storage-team      │ 4     │ 5     │ 93.1%   │ 2.6       │ 0
@golang-expert     │ 4     │ 38    │ 89.4%   │ 3.7       │ 0
@ui-team           │ 4     │ 31    │ 88.5%   │ 3.9       │ 0
@db-expert         │ 1     │ 20    │ 87.0%   │ 4.9       │ 0
@maintainers       │ 12    │ 59    │ 81.5%   │ 4.4       │ 0
```

**CI/CD Integration Example:**

```bash
#!/bin/bash
# Run analysis and check team health
kaizen analyze --path=.
kaizen report owners --format=json --output=metrics.json

# Fail if any team below health threshold
jq '.owner_metrics[] | select(.overall_health_score < 70)' metrics.json | \
  if [ -s /dev/stdin ]; then
    echo "⚠️  Teams below health threshold!"
    exit 1
  fi
```

**Sankey Diagram - Ownership Flow Visualization:**

```bash
# Generate interactive Sankey diagram showing owner → function dependencies
kaizen sankey

# Adjust threshold (show functions used by 3+ owners)
kaizen sankey --min-owners=3

# Save to specific file
kaizen sankey --output=ownership-flow.html

# Don't open browser automatically
kaizen sankey --open=false
```

**What it shows:**
- Flow from code owners (left) to commonly-used functions (right)
- Width of flow = number of calls
- Identifies shared dependencies across teams
- Highlights collaboration patterns and potential bottlenecks

**Example visualization:**
```
@storage-team ────────▶ models.NewCallGraph (18 calls)
              ────────▶ fmt.Errorf (12 calls)
@analysis-team ───────▶ fmt.Errorf (23 calls)
               ───────▶ filepath.Join (15 calls)
@ui-team ─────────────▶ fmt.Errorf (18 calls)
         ─────────────▶ template.Must (11 calls)
```

The Sankey diagram helps answer questions like:
- Which functions are shared across multiple teams?
- Which teams depend most on common utilities?
- Are there central functions that need extra attention?
- How isolated vs. collaborative are our teams?

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

# Storage configuration (Phase 1)
storage:
  type: sqlite                    # sqlite, json, or both
  path: ./kaizen.db              # Database location
  keep_json_backup: true         # Also save JSON files
  retention_days: 90             # Auto-prune after 90 days
  auto_prune: false              # Prune on each analyze

# Code ownership (Phase 3)
codeowners:
  path: .github/CODEOWNERS       # CODEOWNERS file location
  auto_report: false             # Generate report on analyze
  exclude_owners: []             # Teams to exclude from reports
```

### `.github/CODEOWNERS`

Define team ownership (GitHub/GitLab format):

```
# Catch-all rule (least specific)
* @maintainers

# Specific team assignments
pkg/analyzer/ @analysis-team
pkg/languages/ @language-team
pkg/languages/golang/ @golang-expert

pkg/storage/ @storage-team
pkg/storage/sqlite.go @db-expert

pkg/visualization/ @ui-team

cmd/kaizen/ @cli-team
```

**Rules:**
- CODEOWNERS uses "last match wins" semantics
- Most general patterns should come first
- Most specific patterns should come last
- A file can have multiple owners
- Comments start with `#`

---

## CLI Reference

### `kaizen analyze`

Analyze code quality and save to database.

```bash
kaizen analyze [flags]

Flags:
  -p, --path string       Path to analyze (default ".")
  -s, --since string      Churn period (default "90d")
  -o, --output string     Output file (default "kaizen-results.json")
  -l, --languages strings Languages to include
  -e, --exclude strings   Patterns to exclude
      --skip-churn        Skip git churn analysis

Examples:
  # Analyze current directory (saves to database + JSON file)
  kaizen analyze --path=.
  
  # Analyze specific path without churn
  kaizen analyze --path=./pkg --skip-churn
  
  # Custom output file and time period
  kaizen analyze --since=30d --output=results.json
  
  # Analyze only Go files
  kaizen analyze --languages=go

Note: Results are automatically saved to both:
  - SQLite database (.kaizen/kaizen.db) for historical tracking
  - JSON file (kaizen-results.json) for visualizations
```

**Follow-up commands:**
```bash
kaizen history list                          # View all snapshots
kaizen trend overall_score                   # See score trends
kaizen visualize --format=html               # Generate interactive visualization
kaizen visualize --input=kaizen-results.json # Generate visualization
```

---

### `kaizen history`

Manage analysis snapshots and history.

```bash
kaizen history list [flags]
  List all snapshots with ID, date, grade, and score
  Flags: (none)

kaizen history show <snapshot-id> [flags]
  Display detailed snapshot information
  Flags: (none)

kaizen history prune [flags]
  Delete snapshots older than retention days
  Flags:
    -r, --retention int  Days to retain (default 90)

Examples:
  kaizen history list
  kaizen history show 5
  kaizen history prune --retention=30
```

---

### `kaizen trend`

Visualize metrics over time.

```bash
kaizen trend <metric> [flags]

Metrics:
  overall_score, complexity, maintainability, hotspots, churn
  avg_cyclomatic_complexity, avg_cognitive_complexity, avg_maintainability_index

Flags:
  -d, --days int      Time range in days (default 30)
  -f, --format string Format: ascii, json, html (default "ascii")
  -o, --output string Output file (for json/html)
      --folder string Show trends for specific folder

Examples:
  # View overall score trend in terminal
  kaizen trend overall_score
  
  # View complexity over last 60 days
  kaizen trend complexity --days=60
  
  # Export hotspots trend as HTML chart
  kaizen trend hotspots --format=html --output=hotspots-chart.html
  
  # Track maintainability for specific folder
  kaizen trend maintainability --folder=pkg/analyzer
  
  # Get raw trend data as JSON
  kaizen trend overall_score --format=json --output=trend-data.json

Note: Requires historical data in database from multiple 'kaizen analyze' runs.
```

**Follow-up commands:**
```bash
kaizen report owners                         # Compare with team metrics
kaizen trend <different_metric>              # View another metric
kaizen history list                          # See all snapshots
```

---

### `kaizen report owners`

Generate code ownership report by team.

```bash
kaizen report owners [snapshot-id] [flags]

Flags:
  -c, --codeowners string Path to CODEOWNERS file (auto-detected)
  -f, --format string     Format: ascii, json, html (default "ascii")
  -o, --output string     Output file (for json/html)
      --open              Open HTML in browser (default true)

Examples:
  # Use latest snapshot from database
  kaizen report owners
  
  # Use specific snapshot by ID
  kaizen report owners 5
  
  # Export as JSON or HTML
  kaizen report owners --format=json --output=team-health.json
  kaizen report owners --format=html --output=team-report.html
  
  # Specify custom CODEOWNERS location
  kaizen report owners --codeowners=.gitlab/CODEOWNERS

Note: This command reads snapshots from the database (.kaizen/kaizen.db).
      Run 'kaizen analyze' first to populate the database.
```

**Follow-up commands:**
```bash
# Sort teams by health score
jq '.owner_metrics | sort_by(.overall_health_score)[]' team-health.json

# View snapshot history
kaizen history list

# Compare with trend data
kaizen trend complexity --format=json | jq '.data'
```

---

### `kaizen visualize`

Generate interactive visualizations.

```bash
kaizen visualize [flags]

Flags:
  -i, --input string    Input JSON (default "kaizen-results.json")
  -f, --format string   Format: terminal, html, svg (default "terminal")
  -m, --metric string   Metric: complexity, churn, hotspot, etc.
  -o, --output string   Output file
      --open            Auto-open HTML (default true)

Examples:
  # Generate HTML visualization from latest analysis
  kaizen visualize --format=html
  
  # View complexity in terminal
  kaizen visualize --metric=complexity
  
  # Generate SVG without opening browser
  kaizen visualize --format=svg --output=heatmap.svg --open=false
  
  # Use specific analysis file
  kaizen visualize --input=old-results.json --format=html

Note: By default, uses kaizen-results.json created by 'kaizen analyze'.
```

---

### `kaizen callgraph`

Generate function call graph.

```bash
kaizen callgraph [flags]

Flags:
  -p, --path string     Path to analyze (default ".")
  -o, --output string   Output file
  -f, --format string   Format: html, svg, json
      --min-calls int   Filter by minimum call count

Examples:
  kaizen callgraph --format=html
  kaizen callgraph --path=./pkg/analyzer --format=svg
```

---

### `kaizen sankey`

Generate Sankey diagram showing code ownership flow to common functions.

```bash
kaizen sankey [flags]

Flags:
  -i, --input string      Input analysis file (default "kaizen-results.json")
  -o, --output string     Output HTML file (default "kaizen-sankey.html")
      --min-owners int    Minimum owners calling a function to include it (default 2)
      --min-calls int     Minimum calls to include a function (default 1)
      --open              Open in browser (default true)

Examples:
  kaizen sankey
  kaizen sankey --min-owners=3
  kaizen sankey --output=team-dependencies.html --open=false

Requires:
  - CODEOWNERS file (.github/CODEOWNERS or similar)
  - Analysis results from 'kaizen analyze'
```

**What it visualizes:**
- Left side: Code owners (teams/individuals)
- Right side: Commonly-used functions
- Links: Call relationships (width = call count)
- Helps identify shared dependencies and collaboration patterns

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

## Troubleshooting

### Command Not Found After Installation

If you get `command not found: kaizen` after running the install script:

1. Check if `~/.local/bin` is in your PATH:
   ```bash
   echo $PATH | grep -q "$HOME/.local/bin" && echo "Found" || echo "Not found"
   ```

2. Add to your PATH by adding this to `~/.zshrc` or `~/.bashrc`:
   ```bash
   export PATH="$PATH:$HOME/.local/bin"
   ```

3. Reload your shell:
   ```bash
   source ~/.zshrc  # or source ~/.bashrc
   ```

### Shell Completions Not Working

**Zsh:**
```bash
# Ensure completion directory exists and is in fpath
mkdir -p ~/.local/share/zsh/site-functions
echo $fpath | grep -q "local/share/zsh/site-functions" || echo "Add to fpath in ~/.zshrc"

# Rebuild completion cache
rm -f ~/.zcompdump
compinit
```

**Fish:**
```bash
# Completions should be automatic, but you can check:
set -U fish_complete_path $fish_complete_path ~/.config/fish/completions
```

### "No Snapshots Found" Error

If `kaizen report owners` or `kaizen trend` shows "no snapshots":

1. Run an analysis first:
   ```bash
   kaizen analyze --path=.
   ```

2. Verify database exists:
   ```bash
   ls -la .kaizen/kaizen.db
   ```

3. List snapshots:
   ```bash
   kaizen history list
   ```

### CODEOWNERS Not Found

If ownership reports fail:

1. Create `.github/CODEOWNERS` in your repository root
2. Or specify custom location:
   ```bash
   kaizen report owners --codeowners=.gitlab/CODEOWNERS
   ```

### Visualization Opens Wrong Browser

Set your default browser:

**macOS:**
```bash
# Use Chrome
export BROWSER=/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome

# Use Firefox
export BROWSER=/Applications/Firefox.app/Contents/MacOS/firefox
```

**Linux:**
```bash
export BROWSER=firefox
# or
export BROWSER=google-chrome
```

Add to `~/.zshrc` or `~/.bashrc` to make permanent.

### Database Locked Error

If you get "database is locked":

1. Ensure no other kaizen processes are running:
   ```bash
   ps aux | grep kaizen
   ```

2. If stuck, remove the lock:
   ```bash
   rm .kaizen/kaizen.db-shm .kaizen/kaizen.db-wal
   ```

---

## Roadmap

### Completed ✅
- [x] Code health grading (A-F)
- [x] Areas of concern with explanations
- [x] Zoomable treemap visualization
- [x] VS Code integration
- [x] Historical time-series storage (Phase 1)
- [x] Trend visualization with ASCII/JSON/HTML (Phase 2)
- [x] Code ownership reports with CODEOWNERS integration (Phase 3)
- [x] Sankey diagrams for ownership flow visualization

### In Progress / Planned
- [ ] Kotlin language support
- [ ] Python, TypeScript, Java support
- [ ] Ownership trend analysis (track team metrics over time)
- [ ] CI/CD integration examples
- [ ] GitHub Actions reporter
- [ ] Slack integration
- [ ] PDF report export
- [ ] Performance forecasting

---

## Credits

Inspired by:
- [SonarQube](https://www.sonarqube.org/) - Code quality platform
- [Code Climate](https://codeclimate.com/) - Maintainability scoring
- [Code Maat](https://github.com/adamtornhill/code-maat) - Churn analysis
- [Your Code as a Crime Scene](https://pragprog.com/titles/atcrime/your-code-as-a-crime-scene/) - Hotspot concept

## License

MIT

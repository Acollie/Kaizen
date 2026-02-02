# 📖 Kaizen Usage Guide

Complete guide to using Kaizen for code analysis, from installation to advanced workflows.

## Table of Contents

1. [Installation](#installation)
2. [First Time Setup](#first-time-setup)
3. [Command Reference](#command-reference)
4. [Common Workflows](#common-workflows)
5. [Configuration](#configuration)
6. [Advanced Topics](#advanced-topics)
7. [Troubleshooting](#troubleshooting)

---

## Installation

### Requirements

- **Go 1.21 or higher** - [Install Go](https://golang.org/doc/install)
- **Git** - For churn analysis (optional)

### Option 1: Quick Install (Recommended)

```bash
git clone https://github.com/alexcollie/kaizen.git
cd kaizen
./install.sh
```

This will:
- Build the kaizen binary
- Install to `~/.local/bin` (or custom `$INSTALL_DIR`)
- Set up shell completions for zsh and fish

### Option 2: Using `go install`

```bash
go install github.com/alexcollie/kaizen/cmd/kaizen@latest
```

### Option 3: Manual Build

```bash
git clone https://github.com/alexcollie/kaizen.git
cd kaizen
go build -o kaizen ./cmd/kaizen
mv kaizen /usr/local/bin/
```

### Verify Installation

```bash
kaizen --version
kaizen --help
```

---

## First Time Setup

### 1. Analyze Your First Project

```bash
cd /path/to/your/project
kaizen analyze --path=.
```

This will:
- Scan all supported code files
- Calculate metrics
- Generate health report
- Save snapshot to database

### 2. View the Results

```bash
# Interactive HTML visualization
kaizen visualize --format=html

# Terminal view
kaizen visualize --metric=complexity --format=ascii
```

### 3. Understand the Grades

Your project receives an A-F grade based on:

```
Grade A (90-100):  ✅ Excellent   - Well-maintained, easy to work with
Grade B (75-89):   ✅ Good        - Some technical debt, mostly solid
Grade C (60-74):   ⚠️  Fair        - Notable issues, plan refactoring
Grade D (40-59):   ❌ Poor        - Significant technical debt
Grade F (0-39):    🚨 Critical    - Urgent attention needed
```

### 4. Check Areas of Concern

The report highlights specific issues:

```bash
kaizen analyze --path=. | grep -A 10 "Areas of Concern"
```

Look for:
- 🔴 **High Complexity** - Functions with CC > 10
- 🟡 **Low Maintainability** - MI < 20 indicates hard-to-read code
- 🔴 **Long Functions** - > 50 lines (harder to test)
- 🟠 **Hotspots** - High complexity + High churn (pain points)

---

## Command Reference

### `kaizen analyze`

Run code analysis on a project.

```bash
# Basic analysis
kaizen analyze --path=/path/to/project

# Skip git churn analysis (faster)
kaizen analyze --path=. --skip-churn

# Analyze since specific time
kaizen analyze --path=. --since=2024-01-01

# Save to custom location
kaizen analyze --path=. --output=results.json

# Analyze specific languages only
kaizen analyze --path=. --include-languages=go,kotlin
```

**Flags:**
- `--path` (string) - Directory to analyze (default: ".")
- `--since` (string) - Only analyze commits since date (e.g., "2024-01-01")
- `--skip-churn` (bool) - Skip git churn analysis for speed
- `--output` (string) - Save JSON results to file
- `--include-languages` (strings) - Only analyze specific languages

### `kaizen visualize`

Generate visualizations of analysis results.

```bash
# Interactive HTML heatmap (default)
kaizen visualize --format=html --open

# Use specific metric for coloring
kaizen visualize --metric=complexity
kaizen visualize --metric=churn
kaizen visualize --metric=maintainability

# ASCII art in terminal
kaizen visualize --format=ascii

# Load previous analysis
kaizen visualize --input=results.json --format=html

# Top N folders/files
kaizen visualize --top=10
```

**Metrics:**
- `complexity` - Cyclomatic complexity (default)
- `maintainability` - Maintainability index
- `churn` - Git commit frequency
- `hotspot` - Combination of complexity + churn
- `functions` - Function count
- `comments` - Comment density

### `kaizen diff`

Compare current analysis with last snapshot.

```bash
# Compare with previous snapshot
kaizen diff --path=.

# Show breakdown by team
kaizen diff --path=. --teams

# Save report to file
kaizen diff --path=. --output=diff-report.txt

# Skip churn for faster comparison
kaizen diff --path=. --skip-churn
```

**Flags:**
- `--path` (string) - Directory to analyze
- `--teams` (bool) - Show team-based breakdown (requires CODEOWNERS)
- `--output` (string) - Save report to file
- `--skip-churn` (bool) - Skip git churn analysis
- `--codeowners` (string) - Path to CODEOWNERS file

### `kaizen history`

Manage historical analysis snapshots.

```bash
# List all snapshots
kaizen history list

# Show details of specific snapshot
kaizen history show 1

# Prune old snapshots (keep last 30 days)
kaizen history prune --days=30

# Remove all snapshots
kaizen history prune --days=0
```

### `kaizen trend`

View metric trends over time.

```bash
# Show score trend for last 30 days
kaizen trend overall_score --days=30

# Complexity trend
kaizen trend complexity --days=90

# Export to JSON
kaizen trend overall_score --days=30 --format=json --output=trends.json

# HTML interactive chart
kaizen trend overall_score --days=30 --format=html

# Specific folder
kaizen trend overall_score --days=30 --folder=pkg/analyzer
```

**Available Metrics:**
- `overall_score` - Health score (0-100)
- `complexity` - Average cyclomatic complexity
- `maintainability` - Average maintainability index
- `hotspots` - Number of hotspot functions
- `churn` - Average churn

### `kaizen report owners`

Generate team-based reports using CODEOWNERS.

```bash
# ASCII report
kaizen report owners

# HTML report
kaizen report owners --format=html

# JSON export
kaizen report owners --format=json --output=team-metrics.json

# Specific snapshot
kaizen report owners --snapshot-id=2
```

### `kaizen sankey`

Generate ownership flow diagrams.

```bash
# Basic sankey diagram
kaizen sankey --format=html

# Minimum owners filter
kaizen sankey --min-owners=3

# Minimum call count
kaizen sankey --min-calls=5

# Load from JSON
kaizen sankey --input=results.json
```

### `kaizen callgraph`

Generate function call graphs.

```bash
# For current project
kaizen callgraph --path=.

# For specific file
kaizen callgraph --path=./src/main.go

# JSON output
kaizen callgraph --path=. --format=json

# Filter by minimum calls
kaizen callgraph --path=. --min-calls=5
```

---

## Common Workflows

### Daily Development

Track code quality while actively developing:

```bash
# Morning: Check yesterday's trends
kaizen trend overall_score --days=1

# During day: Quick check of current state
kaizen visualize --metric=hotspot --format=ascii

# Before commit: Compare with yesterday
kaizen diff --path=.

# After refactoring: See improvement
kaizen analyze --path=. && kaizen visualize --format=html
```

### Team Health Monitoring

Regular monitoring for team leads:

```bash
# Weekly report: Team metrics
kaizen report owners --format=html --output=weekly-report.html

# Track trends over quarter
kaizen trend overall_score --days=90

# Identify knowledge silos
kaizen sankey --min-owners=2 --format=html

# Compare with previous week
kaizen diff --path=. --teams
```

### Refactoring Sessions

Before and after metrics:

```bash
# Step 1: Create baseline
kaizen analyze --path=. --output=before.json

# Step 2: Do your refactoring
# (edit files...)

# Step 3: Create after snapshot
kaizen analyze --path=. --output=after.json

# Step 4: Compare results
kaizen visualize --input=before.json --format=html
kaizen visualize --input=after.json --format=html

# Step 5: Check improvements
kaizen diff --path=.
```

### CI/CD Integration

Automated quality checks in CI/CD pipeline:

```bash
#!/bin/bash
# ci-check.sh

# Analyze code
kaizen analyze --path=. --output=analysis.json

# Extract score
SCORE=$(jq '.score_report.overall_score' analysis.json)

# Fail build if quality degraded
if (( $(echo "$SCORE < 75" | bc -l) )); then
    echo "❌ Code quality below threshold: $SCORE"
    kaizen report owners --format=json | jq '.'
    exit 1
fi

echo "✅ Code quality check passed: $SCORE"
```

### Performance Optimization

Finding slow-to-parse code:

```bash
# Analyze with timing
time kaizen analyze --path=.

# Skip churn to isolate AST parsing time
time kaizen analyze --path=. --skip-churn

# Analyze subset
time kaizen analyze --path=./cmd --skip-churn
```

---

## Configuration

### `.kaizenignore`

Similar to `.gitignore`, patterns to exclude from analysis:

```
# Ignore vendor and test directories
vendor/
*_test.go
**/*.generated.go

# Ignore specific packages
pkg/internal/deprecated/

# Negation - include even if previous rule matched
!vendor/important/package/
```

Patterns support:
- `*` - Match anything in directory
- `**` - Match across directories
- `!` - Negation (include even if excluded)
- `#` - Comments

### `.kaizen.yaml`

Main configuration file:

```yaml
# Analysis settings
analysis:
  skip_churn: false
  include_languages:
    - go
    - kotlin
    - swift
  exclude_patterns:
    - "**/vendor/**"
    - "**/*_test.go"

# Visualization settings
visualization:
  color_scheme: "nordic"  # nordic, default
  metrics:
    - complexity
    - maintainability
    - churn

# Storage settings
storage:
  type: "sqlite"  # sqlite, json, or both
  retention_days: 90
  auto_prune: false

# Thresholds for concerns
thresholds:
  max_cyclomatic_complexity: 10
  max_cognitive_complexity: 15
  min_maintainability_index: 20
  max_function_length: 50
  max_nesting_depth: 4
```

### `.github/CODEOWNERS`

Define team ownership for team-based reporting:

```
# Default owner
* @maintainers

# Specific teams own specific areas
/cmd/kaizen @cli-team
/pkg/analyzer @analysis-team
/pkg/visualization @ui-team
/pkg/languages @language-team
```

---

## Advanced Topics

### Churn Analysis

Understanding code change frequency:

```bash
# View churn for each file
kaizen analyze --path=. | grep "churn"

# High churn indicates:
# - Frequently modified code
# - Potential instability
# - Knowledge concentration

# Hotspots = High Complexity + High Churn
# These are priority refactoring targets
```

### Metric Calculations

#### Cyclomatic Complexity

Counts decision points:

```go
func example(x int) {
    if x > 0 {           // +1
        if x > 10 {      // +1
            do()
        }
    }
    for i := 0; i < 10; i++ {  // +1
        work()
    }
    // CC = 1 (base) + 3 = 4
}
```

#### Cognitive Complexity

Adds nesting penalty:

```go
if x > 0 {              // +1
    if y > 0 {          // +2 (nesting level 1)
        if z > 0 {      // +3 (nesting level 2)
            do()
        }
    }
}
// Cognitive = 6 (vs CC = 4)
```

#### Maintainability Index

Formula: `171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)`

- 100 = Easy to maintain
- 50 = Moderate difficulty
- 0 = Hard to maintain

### Performance Tuning

Optimize analysis for large codebases:

```bash
# Skip churn for 10x speedup (on large repos)
kaizen analyze --path=. --skip-churn

# Add churn later for historical data
kaizen analyze --path=. # Full analysis

# Analyze subset for quick check
kaizen analyze --path=./cmd

# Parallel analysis (future feature)
# kaizen analyze --path=. --max-workers=8
```

### Adding Custom Languages

See [ARCHITECTURE.md](./ARCHITECTURE.md#adding-languages) for details on:
1. Implementing the LanguageAnalyzer interface
2. Using tree-sitter for AST parsing
3. Registering in the language registry

---

## Troubleshooting

### Issue: "not a git repository"

**Error:** `Error: X/Y is not a git repository`

**Solution:** Git churn analysis requires a git repo:

```bash
# Skip churn analysis
kaizen analyze --path=. --skip-churn

# Or initialize git
cd /path/to/project
git init
kaizen analyze --path=.
```

### Issue: "no analyzer found"

**Error:** `no analyzer found for file extension: .xyz`

**Solution:** Language not supported. Check supported languages:

```bash
kaizen --help | grep "Supports"
```

Supported: Go, Kotlin, Swift, Python (stub)

### Issue: "database is locked"

**Error:** `database is locked`

**Solution:** Another kaizen process is running:

```bash
# Check for running processes
pgrep kaizen

# Remove lock files
rm .kaizen/kaizen.db-shm .kaizen/kaizen.db-wal

# Retry
kaizen analyze --path=.
```

### Issue: "slow analysis"

**Cause:** Git churn analysis can be slow on large repos

**Solution:**

```bash
# Skip churn for quick analysis
kaizen analyze --path=. --skip-churn

# Or limit git history
kaizen analyze --path=. --since=2024-01-01

# Analyze subset
kaizen analyze --path=./cmd
```

### Issue: "tree-sitter error"

**Error:** `failed to parse Kotlin/Swift file`

**Solution:** Tree-sitter parsing error (usually syntax error in code):

```bash
# Verify code is valid
kotlin -script file.kt  # For Kotlin
swiftc -parse file.swift  # For Swift

# Skip problematic file
# Add to .kaizenignore
```

### Getting Help

```bash
# Show command help
kaizen analyze --help

# Show all commands
kaizen --help

# Check documentation
cat GUIDE.md      # This file
cat ARCHITECTURE.md  # Technical details

# Report issues
# https://github.com/alexcollie/kaizen/issues
```

---

## Examples by Use Case

### 📊 Executive Dashboard

```bash
# Create weekly report
kaizen analyze --path=.
kaizen report owners --format=json --output=weekly.json

# Extract key metrics
jq '.score_report | {score: .overall_score, grade: .overall_grade}' weekly.json

# View trends
kaizen trend overall_score --days=90 --format=json
```

### 🔧 Individual Developer

```bash
# Morning standup
kaizen trend overall_score --days=1
kaizen visualize --metric=hotspot --format=ascii

# Before PR
kaizen diff --path=.

# After refactoring
kaizen analyze --path=.
kaizen visualize --format=html --open
```

### 👥 Team Lead

```bash
# Team ownership visualization
kaizen report owners --format=html
kaizen sankey --format=html

# Find knowledge silos
kaizen sankey --min-owners=1 | grep "solo"

# Track team productivity
kaizen trend overall_score --days=30
```

### 🚀 DevOps/Platform

```bash
# CI/CD integration
kaizen analyze --path=. --output=analysis.json
kaizen report owners --format=json > team-metrics.json

# Export to monitoring
curl -X POST /metrics -d @team-metrics.json

# Trending dashboard
kaizen trend overall_score --days=90 --format=json
```

---

## Next Steps

- 📖 **Deep Dive:** Read [ARCHITECTURE.md](./ARCHITECTURE.md)
- 🔧 **Contribute:** See [CONTRIBUTING.md](./CONTRIBUTING.md)
- 💬 **Discuss:** Join [GitHub Discussions](https://github.com/alexcollie/kaizen/discussions)
- 🐛 **Report:** [Create an issue](https://github.com/alexcollie/kaizen/issues)


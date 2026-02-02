# Kaizen Demo

This directory contains a complete demonstration of Kaizen's capabilities, including sample projects, example outputs, and screenshots.

## Quick Start

Run the automated demo:

```bash
cd demo
./run-demo.sh
```

This will:
1. Analyze the sample project
2. Generate all visualization types
3. Display results in your terminal
4. Open HTML visualizations in your browser

## What's Included

### 📁 Sample Project (`sample-project/`)

A realistic Go codebase demonstrating various code quality scenarios:
- Simple, well-written functions (low complexity)
- Complex functions with high cyclomatic complexity
- Nested logic with high cognitive complexity
- Functions with varying lengths
- Mix of good and problematic code patterns

Perfect for understanding how Kaizen analyzes real code.

### 📊 Example Outputs (`outputs/`)

Pre-generated analysis results:
- `sample-heatmap.html` - Interactive treemap visualization
- `sample-trends.html` - Historical trend charts
- `sample-ownership.html` - Team ownership Sankey diagram
- `sample-analysis.json` - Raw analysis data

### 📸 Screenshots (`screenshots/`)

High-quality visualizations showing Kaizen's features:
1. `01-code-health-grade.png` - Overall code health score (A-F)
2. `02-interactive-heatmap.png` - Treemap with drill-down
3. `03-areas-of-concern.png` - Problematic code detection
4. `04-trend-analysis.png` - Historical metric tracking
5. `05-ownership-sankey.png` - Team dependency flows
6. `06-terminal-output.png` - Beautiful CLI output

## Use Cases

### Finding Hotspots in Legacy Code

```bash
# Analyze a legacy codebase
kaizen analyze --path=/path/to/legacy-app

# Look for hotspots: high complexity + high churn
# These are the riskiest areas that change frequently
```

**What to look for:**
- Red/orange boxes in the heatmap (high complexity)
- Functions marked as "Hotspots" in the output
- Areas of concern list with severity ratings

### Tracking Refactoring Progress

```bash
# Initial baseline
kaizen analyze --path=. --output=before.json

# ... refactor some code ...

# After refactoring
kaizen analyze --path=. --output=after.json

# View trends over time
kaizen trends --path=.
```

**Metrics to track:**
- Cyclomatic complexity trends (should decrease)
- Maintainability index (should increase)
- Number of hotspots (should decrease)

### Team Ownership Analysis

```bash
# Generate ownership report (requires CODEOWNERS file)
kaizen owners --path=. --format=html --output=ownership.html
```

**Insights you'll get:**
- Which teams own the most complex code
- Cross-team dependencies via Sankey diagram
- Distribution of code ownership
- Team-level quality metrics

### CI/CD Integration

Add to your GitHub Actions workflow:

```yaml
- name: Code Quality Check
  run: |
    go install github.com/alexcollie/kaizen/cmd/kaizen@latest
    kaizen analyze --path=. --skip-churn --output=kaizen.json

    # Fail if maintainability is too low
    # (Custom script to parse JSON and check thresholds)
```

## Features Demonstrated

### 1. Code Health Grading

Kaizen assigns an overall grade (A-F) based on:
- **A (90-100)**: Excellent code quality
- **B (80-89)**: Good code quality
- **C (70-79)**: Acceptable quality, some improvements needed
- **D (60-69)**: Below average, refactoring recommended
- **F (<60)**: Poor quality, immediate attention required

Grade components:
- Complexity score (40%)
- Maintainability score (30%)
- Churn score (20%)
- Coverage score (10%, if available)

### 2. Interactive Visualizations

**Treemap Heatmap:**
- Size = lines of code
- Color = selected metric (complexity, churn, maintainability)
- Click to drill down into folders
- Hover for detailed metrics

**Trend Charts:**
- Time-series data showing metric evolution
- Multiple metrics on same chart
- ASCII version for terminal, HTML for browser
- Identify improving or degrading areas

**Sankey Diagrams:**
- Shows code ownership flows between teams
- Ribbon width = amount of code
- Colored by team
- Highlights cross-team dependencies

### 3. Areas of Concern

Automatically detects problematic code:
- **Critical**: Complexity > 20, immediate attention
- **High**: Complexity > 15, should refactor soon
- **Medium**: Complexity > 10, monitor closely
- **Low**: Complexity > 5, consider simplifying

### 4. Hotspot Detection

Functions that are both:
- High complexity (>10 cyclomatic complexity)
- High churn (>10 commits)

These are risky because:
- Complex code is harder to modify safely
- Frequent changes increase bug likelihood
- Combination creates technical debt

### 5. Multiple Metrics

**Cyclomatic Complexity:**
- Counts decision points (if, for, switch, &&, ||)
- Industry standard for complexity
- Threshold: >10 is concerning

**Cognitive Complexity:**
- Penalizes nesting more than cyclomatic
- Better reflects human understanding
- More intuitive for code review

**Halstead Metrics:**
- Based on operators and operands
- Estimates program volume and difficulty
- Good for comparing similar functions

**Maintainability Index:**
- Formula: 171 - 5.2*ln(HV) - 0.23*CC - 16.2*ln(LOC)
- Clamped to 0-100 scale
- >80 is maintainable, <20 needs work

## Running Your Own Analysis

### Basic Analysis

```bash
# Build kaizen
cd ..
go build -o kaizen ./cmd/kaizen

# Analyze your project
./kaizen analyze --path=/path/to/your/project

# Generate heatmap
./kaizen visualize --input=kaizen-results.json --format=html --open=true
```

### Advanced Options

```bash
# Skip git churn analysis (faster)
./kaizen analyze --path=. --skip-churn

# Custom output file
./kaizen analyze --path=. --output=my-analysis.json

# Configure via .kaizen.yaml
cat > .kaizen.yaml <<EOF
ignore:
  - "vendor/**"
  - "*_test.go"
thresholds:
  complexity: 15
  churn: 20
EOF

./kaizen analyze --path=.
```

### Viewing Results

```bash
# Interactive HTML heatmap
./kaizen visualize --input=kaizen-results.json --format=html --open=true

# ASCII heatmap (for terminal)
./kaizen visualize --input=kaizen-results.json --format=ascii

# JSON output (for scripting)
./kaizen visualize --input=kaizen-results.json --format=json
```

## Tips for Best Results

1. **Run from repository root**: Git churn analysis works best from the repo root
2. **Use .kaizenignore**: Exclude vendor/, test files, generated code
3. **Track over time**: Run regularly to see trends
4. **Set realistic thresholds**: Adjust based on your team's standards
5. **Combine metrics**: Look at complexity + churn together for hotspots
6. **Review areas of concern**: Start refactoring with the highest severity

## Screenshot Guide

The screenshots in this demo show:

### 01-code-health-grade.png
Overall health grade with component breakdown. Notice how different metrics contribute to the final score.

### 02-interactive-heatmap.png
Treemap visualization showing the full codebase. Larger boxes = more code, redder color = higher complexity.

### 03-areas-of-concern.png
List of functions that need attention, sorted by severity. Each entry shows file location, complexity, and why it's concerning.

### 04-trend-analysis.png
Historical data showing how metrics change over time. Look for upward trends (improving) or downward trends (degrading).

### 05-ownership-sankey.png
Team ownership flow diagram. Wide ribbons indicate large codebases, colors represent teams.

### 06-terminal-output.png
Beautiful CLI output with colors, progress bars, and clear formatting.

## Next Steps

- Read the main [README.md](../README.md) for full documentation
- Check [CONTRIBUTING.md](../CONTRIBUTING.md) to add new language support
- Review [CLAUDE.md](../CLAUDE.md) for architecture details
- Star the repo if you find Kaizen useful!

## Questions?

- **Issues**: [GitHub Issues](https://github.com/alexcollie/kaizen/issues)
- **Discussions**: [GitHub Discussions](https://github.com/alexcollie/kaizen/discussions)
- **Documentation**: [README.md](../README.md)

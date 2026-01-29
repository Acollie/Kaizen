# Kaizen - Code Analysis Tool

A powerful code analysis tool that measures code quality, complexity, and churn to help you identify technical debt and hotspots in your codebase.

## Features

- 🔍 **Multi-Language Support**: Currently supports Go (Kotlin stubbed for future implementation)
- 📊 **Comprehensive Metrics**:
  - Cyclomatic Complexity
  - Cognitive Complexity
  - Function Length
  - Halstead Metrics
  - Maintainability Index
- 📈 **Git Churn Analysis**: Tracks code changes over time
- 🔥 **Hotspot Detection**: Identifies high-churn + high-complexity code
- 🗺️ **Visual Heat Maps**:
  - Interactive HTML treemaps with D3.js
  - Terminal-based colored visualization
  - Multiple metric views with button switching
  - Hover tooltips with detailed information
- 🎯 **Actionable Insights**: Pinpoints areas needing refactoring

## Installation

```bash
# Clone the repository
git clone https://github.com/alexcollie/kaizen
cd kaizen

# Build
go build -o kaizen ./cmd/kaizen

# Or install globally
go install ./cmd/kaizen

# Kaizen is now available as 'kaizen' command
kaizen --help
```

## Shell Completion

Kaizen supports auto-completion for Bash, Zsh, Fish, and PowerShell!

### Fish Shell

```bash
# Load completions for current session
kaizen completion fish | source

# Install permanently
kaizen completion fish > ~/.config/fish/completions/kaizen.fish

# Restart your shell or run:
source ~/.config/fish/config.fish
```

### Bash

```bash
# Load completions for current session
source <(kaizen completion bash)

# Install permanently (Linux)
kaizen completion bash > /etc/bash_completion.d/kaizen

# Install permanently (macOS with Homebrew bash-completion)
kaizen completion bash > $(brew --prefix)/etc/bash_completion.d/kaizen
```

### Zsh

```bash
# Load completions for current session
source <(kaizen completion zsh)

# Install permanently
kaizen completion zsh > "${fpath[1]}/_kaizen"

# Restart your shell
exec zsh
```

### PowerShell

```powershell
# Load completions for current session
kaizen completion powershell | Out-String | Invoke-Expression

# Install permanently
kaizen completion powershell > kaizen.ps1
# Add to your $PROFILE
```

**What you get with completion:**
- Auto-complete commands: `kaizen an<TAB>` → `kaizen analyze`
- Auto-complete flags: `kaizen analyze --<TAB>` → shows all available flags
- Suggestions for metric names, formats, etc.
- Works across all commands and subcommands

## Quick Start

### Analyze a codebase

```bash
# Analyze current directory
kaizen analyze --path=. --since=30d

# Analyze with specific options
kaizen analyze \
  --path=/path/to/repo \
  --since=90d \
  --output=results.json \
  --languages=go \
  --exclude=vendor,test
```

### Visualize results

```bash
# Interactive HTML heat map (opens in browser)
kaizen visualize --input=results.json --format=html

# Static SVG heat map (perfect for documentation/presentations)
kaizen visualize --input=results.json --format=svg

# Terminal heat map
kaizen visualize --input=results.json --format=terminal --metric=hotspot

# SVG with custom dimensions
kaizen visualize --format=svg --metric=complexity --svg-width=1600 --svg-height=1000

# Different metrics (works with all formats)
kaizen visualize --format=html --metric=complexity
kaizen visualize --format=svg --metric=churn --output=churn-map.svg
kaizen visualize --format=terminal --metric=maintainability

# Generate HTML without opening browser
kaizen visualize --format=html --open=false --output=custom-heatmap.html
```

## Metrics Explained

### Cyclomatic Complexity
Measures the number of independent paths through code. Higher values indicate more complex code.
- **1-5**: Simple, easy to test
- **6-10**: Moderate complexity
- **11-20**: High complexity, consider refactoring
- **>20**: Very high complexity, refactor recommended

### Cognitive Complexity
Similar to cyclomatic but penalizes nested structures more heavily. Better predictor of code understandability.

### Maintainability Index
Composite metric (0-100) based on complexity, volume, and lines of code. Higher is better.
- **>80**: Excellent
- **60-80**: Good
- **40-60**: Moderate
- **<40**: Poor maintainability

### Hotspots
Functions with both:
- High cyclomatic complexity (>10)
- High churn (>10 commits)

These are the highest priority for refactoring.

## Architecture

Kaizen is designed to be extensible and language-agnostic:

```
pkg/
├── analyzer/       # Core analysis engine
├── languages/      # Language-specific parsers
│   ├── golang/     # Go analyzer (fully implemented)
│   └── kotlin/     # Kotlin stub (for future)
├── churn/          # Git history analysis
├── models/         # Data structures
└── visualization/  # Output rendering
```

See [PLAN.md](PLAN.md) for detailed architecture documentation.

## Adding Language Support

To add support for a new language:

1. Create `pkg/languages/<lang>/analyzer.go`
2. Implement the `LanguageAnalyzer` interface
3. Register in `pkg/languages/registry.go`
4. See `pkg/languages/kotlin/README.md` for detailed guide

## Configuration

Kaizen supports two configuration files that work together:

### `.kaizenignore` - Ignore Patterns

Similar to `.gitignore`, this file specifies patterns to exclude from analysis:

```gitignore
# Dependencies
vendor/
node_modules/
third_party/

# Build outputs
dist/
build/
*.exe

# Test files
*_test.go
**/*_test.py

# Generated code
*.pb.go
*.generated.go
generated/

# Documentation
docs/
*.md

# Specific directories
/path/to/exclude/
```

**Pattern Syntax:**
- `vendor/` - Matches folder and all contents
- `*.go` - Matches all .go files
- `*_test.go` - Matches files ending with _test.go
- `/absolute/path` - Matches from project root
- `**/*.test.js` - Matches in any directory
- `!important.go` - Negation (don't ignore this file)
- `# comment` - Comments start with #

### `.kaizen.yaml` - Full Configuration

For more advanced configuration, create `.kaizen.yaml`:

```yaml
# Analysis settings
analysis:
  since: 90d                    # Time range for churn
  languages:                    # Languages to analyze
    - go
  exclude:                      # Additional exclude patterns
    - vendor
    - "*_test.go"
  skip_churn: false            # Skip git analysis
  max_workers: 8               # Parallel workers

# Metric thresholds for warnings
thresholds:
  cyclomatic_complexity: 10
  cognitive_complexity: 15
  function_length: 50
  nesting_depth: 4
  parameter_count: 5
  maintainability_index: 60

# Visualization settings
visualization:
  default_metric: hotspot       # Default metric to show
  color_scheme: red-yellow-green
  show_percentages: true
  auto_open_browser: true
```

**Priority:**
- CLI flags override everything
- `.kaizen.yaml` overrides defaults
- `.kaizenignore` patterns are always applied
- Both files are optional

**Example:**
```bash
# Copy example configuration
cp .kaizen.yaml.example .kaizen.yaml

# Edit for your project
vim .kaizen.yaml

# Analyze (will use config automatically)
kaizen analyze
```

## CLI Reference

### `kaizen analyze`

Analyze a codebase and generate metrics.

**Flags:**
- `--path, -p`: Path to analyze (default: `.`)
- `--since, -s`: Analyze churn since (e.g., `30d`, `2024-01-01`)
- `--output, -o`: Output file (default: `kaizen-results.json`)
- `--languages, -l`: Languages to include (default: all)
- `--exclude, -e`: Patterns to exclude (default: `vendor,node_modules,*_test.go`)
- `--skip-churn`: Skip git churn analysis

**Examples:**
```bash
# Basic analysis
kaizen analyze

# Last 6 months
kaizen analyze --since=180d

# Only Go files, exclude tests
kaizen analyze --languages=go --exclude=*_test.go,vendor
```

### `kaizen visualize`

Visualize analysis results with interactive heat maps, static SVG exports, or terminal output.

**Flags:**
- `--input, -i`: Input JSON file (default: `kaizen-results.json`)
- `--format, -f`: Output format - `terminal`, `html`, or `svg` (default: `terminal`)
- `--metric, -m`: Metric to visualize - `complexity`, `cognitive`, `churn`, `hotspot`, `length`, `maintainability` (default: `hotspot`)
- `--output, -o`: HTML/SVG output file (default: `kaizen-heatmap.html` or `kaizen-heatmap.svg`)
- `--svg-width`: SVG width in pixels (default: `1200`)
- `--svg-height`: SVG height in pixels (default: `800`)
- `--open`: Auto-open HTML in browser (default: `true`)
- `--limit, -l`: Number of top hotspots to show (terminal only, default: `10`)

**Examples:**
```bash
# Interactive HTML heat map (opens in browser)
kaizen visualize --format=html

# Static SVG for embedding in docs/presentations
kaizen visualize --format=svg --metric=complexity

# SVG with custom dimensions
kaizen visualize --format=svg --svg-width=1600 --svg-height=1000

# Terminal heat map
kaizen visualize --format=terminal --metric=hotspot

# HTML without auto-opening
kaizen visualize --format=html --open=false --output=my-report.html
```

**HTML Features:**
- 🎨 Interactive D3.js treemap visualization
- 🔘 Button controls to switch between metrics
- 🖱️ Hover tooltips with detailed stats
- 🎨 Color-coded by score (green=good, yellow=moderate, red=needs attention)
- 📊 Summary statistics at the top
- 💾 Self-contained HTML file (works offline)

**SVG Features:**
- 📄 Static treemap perfect for embedding
- 🎨 Color-coded folders by metric score
- 📏 Customizable dimensions
- 🖼️ Works in browsers, image viewers, and documentation
- 💾 Lightweight vector format (scales without quality loss)
- 📊 Includes title, legend, and summary stats
- 🔖 Tooltip data embedded in SVG (viewable in supporting tools)

## Output Format

Results are saved as JSON:

```json
{
  "repository": "/path/to/repo",
  "analyzed_at": "2024-01-29T10:00:00Z",
  "files": [
    {
      "path": "pkg/analyzer/pipeline.go",
      "language": "Go",
      "total_lines": 150,
      "functions": [
        {
          "name": "Analyze",
          "cyclomatic_complexity": 12,
          "cognitive_complexity": 15,
          "length": 45,
          "is_hotspot": true
        }
      ]
    }
  ],
  "folder_stats": {
    "pkg/analyzer": {
      "average_complexity": 8.5,
      "hotspot_count": 3
    }
  }
}
```

## Use Cases

### Identify Refactoring Candidates
Find hotspots - code that's both complex and frequently changed:
```bash
kaizen analyze && kaizen visualize --metric=hotspot
```

### Track Technical Debt
Run periodically and track metrics over time:
```bash
kaizen analyze --output=analysis-$(date +%Y%m%d).json
```

### Pre-commit Hook
Fail builds on high complexity:
```bash
#!/bin/bash
kaizen analyze --output=tmp.json
# Parse JSON and check thresholds
```

### Code Review Tool
Analyze PR changes before review:
```bash
kaizen analyze --since=7d --output=pr-analysis.json
```

## Roadmap

- [x] **HTML heat map visualization with D3.js** ✅ Complete!
- [ ] Kotlin language support
- [ ] Python, Java, TypeScript support
- [ ] Code duplication detection
- [ ] Coupling and cohesion metrics
- [ ] Historical trend analysis
- [ ] CI/CD integration (GitHub Actions, GitLab CI)
- [ ] SonarQube format export

## Contributing

Contributions welcome! See [PLAN.md](PLAN.md) for architecture details.

## License

MIT

## Credits

Inspired by:
- [SonarQube](https://www.sonarqube.org/)
- [Code Maat](https://github.com/adamtornhill/code-maat)
- [Your Code as a Crime Scene](https://pragprog.com/titles/atcrime/your-code-as-a-crime-scene/)

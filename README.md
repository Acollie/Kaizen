# 🏔️ Kaizen - Code Quality Analysis Tool

[![Build Status](https://github.com/acollie/kaizen/workflows/CI/badge.svg)](https://github.com/acollie/kaizen/actions)
[![codecov](https://codecov.io/github/Acollie/Kaizen/graph/badge.svg?token=9V3XZY7JAF)](https://codecov.io/github/Acollie/Kaizen)
[![Go Report Card](https://goreportcard.com/badge/github.com/collie/kaizen)](https://goreportcard.com/report/github.com/acollie/kaizen)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org/)
[![Latest Release](https://img.shields.io/github/v/release/acollie/kaizen)](https://github.com/acollie/kaizen/releases)

**Continuous improvement starts with visibility.** Kaizen is a powerful code analysis tool that measures code quality, complexity, and churn to identify technical debt and hotspots in your codebase. It generates health grades, actionable concerns, and beautiful interactive visualizations.

📖 **[Usage Guide](./GUIDE.md)** | 🏗️ **[Architecture](./ARCHITECTURE.md)** | 📊 **[Self-Analysis Report](./ANALYSIS_REPORT.md)** | 🎨 **[Interactive Heatmap](./ANALYSIS.html)** | 🚀 **[Quick Start](#quick-start)**

---

## ✨ Key Features

### 🎯 Code Health Analysis
- **A-F Health Grades** with 0-100 score for overall codebase health
- **5 Component Scores**: Complexity, Maintainability, Churn, Function Size, Code Structure
- **Areas of Concern** - Automatically detect issues with severity levels and actionable recommendations

### 📈 Comprehensive Metrics
- **Cyclomatic Complexity** - Counts decision points in code paths
- **Cognitive Complexity** - Penalizes nested structures more heavily
- **Halstead Metrics** - Volume, Difficulty, Effort based on operator/operand counts
- **Maintainability Index** - Industry-standard MI score (0-100)
- **Hotspot Detection** - Identifies high-churn + high-complexity "pain points"

### 🌍 Multi-Language Support
- ✅ **Go** (Full support)
- ✅ **Kotlin** (Full support via tree-sitter)
- ✅ **Swift** (Full support via tree-sitter)
- 🚧 **Python** (Stub for future implementation)
- 🔄 Easy to extend with new languages

### 📊 Visualization & Reporting
- **Interactive HTML Treemaps** - Drill-down navigation with color-coded metrics
- **Sankey Diagrams** - Visualize code ownership and team dependencies
- **Terminal ASCII Charts** - Quick insights without opening browser
- **Historical Trends** - Track metrics over time with trend analysis
- **Ownership Reports** - Aggregate metrics by team using CODEOWNERS

### 🛡️ CI Quality Gate
- **Blast-Radius Detection** - Warns when modified functions have high fan-in (many callers)
- **Exit Codes for CI** - `0` = clean, `2` = blast-radius concerns detected
- **Branch Diffing** - Compares current branch against a base branch (default: main)
- **Text or JSON Output** - Machine-readable format for pipeline integration

### 🔗 Call Graph Analysis
- **Function Call Graph** - Interactive D3.js force-directed graph showing who calls whom
- **Fan-In / Fan-Out** - Identify heavily-depended-on functions and coupling
- **Filterable** - Set minimum call count to reduce noise
- **Multiple Formats** - HTML, SVG, or JSON output

### 🔄 Git Integration
- **Automatic Churn Analysis** - Track how frequently code changes
- **Time-Series Database** - SQLite storage for historical snapshots
- **Trend Comparison** - View metric evolution with diff reporting
- **Team Metrics** - Understand ownership patterns and dependencies

---

---

## ✅ Quality Assurance

### Test Coverage
All code changes are tested across multiple Go versions (1.21, 1.22, 1.23) and operating systems (Linux, macOS, Windows):

- **Unit Tests**: 50+ test files covering analyzers, metrics, and language parsers
- **Integration Tests**: End-to-end analysis and visualization pipeline validation
- **Coverage Target**: 50%+ coverage on main branch with automated codecov checks
- **CI Pipeline**: Automated testing, linting, and build verification on every PR

### Build Status
- [![Tests](https://github.com/acollie/kaizen/actions/workflows/ci.yml/badge.svg)](https://github.com/acollie/kaizen/actions/workflows/ci.yml)
- [![codecov](https://codecov.io/github/Acollie/Kaizen/graph/badge.svg?token=9V3XZY7JAF)](https://codecov.io/github/Acollie/Kaizen)
- [![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org/)

---

## 📊 Self-Analysis & Code Quality

Kaizen practices what it preaches! The project analyzes itself to ensure quality standards.

### 🎓 Latest Self-Assessment

**Grade: B (88/100)** | **Status:** ✅ Healthy Production Codebase

- **Overall Health:** Excellent - Grade B indicates well-maintained code with minor improvement opportunities
- **Code Structure:** Perfect (100/100) - Modular, clean architecture
- **Maintainability:** Strong (86/100) - Easy to understand and modify
- **Complexity:** Good (78/100) - Well-balanced decision logic
- **Function Sizing:** Excellent (92/100) - Average function length just 27 lines

### 📈 Key Metrics

| Metric | Value | Status |
|--------|-------|--------|
| Files Analyzed | 47 | ✅ |
| Total Functions | 350 | ✅ |
| Avg Cyclomatic Complexity | 4.5 | ✅ Excellent |
| Avg Function Length | 27 lines | ✅ Excellent |
| Hotspots Detected | 0 | ✅ Perfect |
| Avg Maintainability Index | 86.4 | ✅ Good |

### 📚 Resources

- **📋 [Detailed Analysis Report](./ANALYSIS_REPORT.md)** - Executive summary, findings, recommendations
- **🎨 [Interactive Heatmap](./ANALYSIS.html)** - Drill-down visualization of complexity and metrics
- **🎨 [Heatmap Treemap](./kaizen-heatmap-tool.html)** - Interactive complexity treemap (generated by kaizen visualize)
- **👥 [Ownership Report](./kaizen-ownership-report.html)** - Team metrics and code ownership (generated by kaizen report)
- **📊 [Raw Analysis Data](./kaizen-self-analysis.json)** - Full metrics in JSON format

---

## 🚀 Quick Start

### Installation

```bash
# Clone and install
git clone https://github.com/acollie/kaizen.git
cd kaizen
./install.sh

# Verify installation
kaizen --version
```

See [Installation Guide](./GUIDE.md#installation) for more options.

### First Analysis

```bash
# Analyze your project
cd /path/to/your/project
kaizen analyze --path=.

# View the health report
kaizen visualize --format=html

# Check code health summary
kaizen report owners
```

### Common Commands

```bash
# View analysis results as interactive heatmap
kaizen visualize --metric=complexity --format=html --open

# CI quality gate — fail on high blast-radius changes
kaizen check --base=main --path=.

# Generate function call graph
kaizen callgraph --path=. --format=html

# Compare with previous analysis
kaizen diff --path=.

# Track trends over time
kaizen trend overall_score --days=30

# Generate team ownership report
kaizen report owners --format=html

# Generate code ownership Sankey diagram
kaizen sankey --input=kaizen-results.json

# View analysis history
kaizen history list
```

**→ See [Usage Guide](./GUIDE.md) for detailed examples**

---

## 📊 What It Analyzes

### Per-File Metrics
- Total/Code/Comment/Blank lines
- Import count and density
- Duplication percentage

### Per-Function Metrics
- Length and parameter count
- **Cyclomatic Complexity** (CC)
- **Cognitive Complexity** (nested decision penalty)
- **Nesting Depth** (max indentation level)
- **Halstead Metrics** (Volume, Difficulty, Effort)
- **Maintainability Index** (0-100 readability score)
- **Fan-in/Fan-out** (coupling metrics)

### Code Health Scoring
```
Grade A (90-100):  ✅ Excellent - Well-maintained code
Grade B (75-89):   ✅ Good     - Minor improvements suggested
Grade C (60-74):   ⚠️  Fair     - Consider refactoring
Grade D (40-59):   ❌ Poor     - Significant technical debt
Grade F (0-39):    🚨 Critical - Urgent refactoring needed
```

### Automatic Issue Detection
- 🔴 **High Complexity** - Functions with CC > 10 (error-prone)
- 🟡 **Low Maintainability** - MI < 20 (hard to understand)
- 🔴 **Long Functions** - > 50 lines (harder to test)
- 🔴 **Deep Nesting** - > 4 levels (confusing logic)
- 🟡 **High Churn** - > 10 commits (unstable code)
- 🟠 **Hotspots** - High complexity + High churn (pain points)

---

## 🎨 Visualization Examples

### Interactive Heatmap
Drill-down treemap showing complexity and churn at a glance:
- **Color intensity** = Metric severity (red = worse)
- **Box size** = Code volume (larger = more code)
- **Click to explore** = Navigate folder hierarchy
- **Hover for details** = See exact metrics

### Ownership Sankey Diagram
See how code ownership flows to shared functions:
- **Team → Functions** - Understand dependencies
- **Flow width** = Call frequency
- **Identify bottlenecks** - Shared dependencies across teams

### Terminal ASCII Charts
Quick trends without opening browser:
```
Score Trend (30 days):
100 │
 90 │     ╱╲
 80 │   ╱    ╲╱╲
 70 │  ╱        ╲
 60 │╱
    └─────────────────
```

---

## 📖 Full Documentation

### Getting Started
- [Installation & Setup](./GUIDE.md#installation)
- [First Time Usage](./GUIDE.md#first-time-setup)
- [Configuration](./GUIDE.md#configuration)

### Usage Guides
- [Daily Development Workflow](./GUIDE.md#daily-development)
- [Team Health Monitoring](./GUIDE.md#team-health-monitoring)
- [CI/CD Integration](./GUIDE.md#cicd-integration)
- [All Commands Reference](./GUIDE.md#command-reference)

### Technical Deep-Dives
- [Architecture Overview](./ARCHITECTURE.md)
- [How Metrics Are Calculated](./ARCHITECTURE.md#metric-calculations)
- [Database Schema](./ARCHITECTURE.md#storage)
- [Extending with New Languages](./ARCHITECTURE.md#adding-languages)

### Advanced Topics
- [Configuration Files](./GUIDE.md#configuration)
- [Git Churn Analysis](./GUIDE.md#churn-analysis)
- [Performance Tuning](./GUIDE.md#performance)
- [Troubleshooting](./GUIDE.md#troubleshooting)

---

## 💡 Use Cases

### 🔧 For Developers
- Identify complexity hotspots before code review
- Refactor with confidence using metrics baseline
- Quick feedback on code quality improvements

### 👥 For Teams
- Track code health trends over time
- Identify team knowledge silos (ownership patterns)
- Prioritize technical debt paydown
- Measure refactoring impact

### 🏗️ For Leadership
- Executive health dashboard showing trend line
- Identify risky components before incidents
- Quantify technical debt and improvement ROI
- Make data-driven architecture decisions

### 🚀 For CI/CD
- **`kaizen check`** - Fail builds when modified functions have high blast-radius (exit code 2)
- Track quality progression across releases
- Catch complexity regressions early
- Export metrics as JSON for external dashboards

---

## 🔧 Command Reference

| Command | Description |
|---------|-------------|
| `kaizen analyze` | Analyze a codebase and generate metrics (JSON output) |
| `kaizen visualize` | Generate interactive heatmaps (HTML, SVG, or terminal) |
| `kaizen check` | CI quality gate — warn on high blast-radius function changes |
| `kaizen callgraph` | Generate interactive function call graph (D3.js force-directed) |
| `kaizen sankey` | Generate Sankey diagram of code ownership flow |
| `kaizen diff` | Compare current analysis with previous snapshot |
| `kaizen trend` | Visualize metric trends over time (ASCII, HTML, or JSON) |
| `kaizen report owners` | Generate code ownership report |
| `kaizen history list` | List all stored analysis snapshots |
| `kaizen history show` | Display detailed snapshot information |
| `kaizen history prune` | Remove old snapshots |

---

## 🏗️ Architecture

Kaizen uses a **modular, language-agnostic architecture** built on:

- **Interface-based language analyzers** - Easy to add new languages
- **Tree-sitter AST parsing** - Accurate syntax understanding (Kotlin, Swift)
- **Go's `ast` package** - Native support for Go analysis
- **SQLite time-series database** - Efficient historical tracking
- **Cobra CLI framework** - Professional command structure

### Supported Languages

| Language | Status | Parser | Coverage |
|----------|--------|--------|----------|
| Go | ✅ Full | go/ast | 95%+ |
| Kotlin | ✅ Full | tree-sitter | 90%+ |
| Swift | ✅ Full | tree-sitter | 90%+ |
| Python | 🚧 Stub | tree-sitter | Ready for implementation |

**→ See [Architecture Guide](./ARCHITECTURE.md) for technical details**

---

## 📊 Performance

Real-world analysis on standard hardware (M1 MacBook Pro, macOS 14.2):

| Project | Files | LOC | Analysis Time | Output Size |
|---------|-------|-----|---|---|
| **Kaizen** (itself) | 47 | ~20K | 0.2s | 274KB |
| **Kubernetes** | 16,614 | 3.2M | 25s | 58MB |

**Estimated performance scaling:**

| Project Size | Estimated Time | Notes |
|--------------|---|---|
| 10K LOC | ~0.1-0.5s | Small library/module |
| 100K LOC | ~1-3s | Medium project |
| 1M LOC | ~15-25s | Large monorepo (like Kubernetes) |
| 10M+ LOC | ~3-5m | Enterprise codebase |

**Factors affecting analysis speed:**
- **Churn analysis** - Git operations add 20-40% overhead (disable with `--skip-churn` for speed)
- **Language mix** - Go (native `ast` package) is fastest; tree-sitter analyzers (Kotlin, Swift) are slightly slower
- **Filesystem I/O** - SSD significantly faster than HDD
- **CPU cores** - Current implementation is single-threaded; multi-core systems show ~80% faster wall-clock time due to background I/O
- **Code complexity** - Complex ASTs take longer to parse but impact is minimal

**Pro tips for faster analysis:**
- Use `--skip-churn` for initial analysis, add historical data later
- Run on SSD-backed storage for 2-3x faster I/O
- Exclude large binary directories with `.kaizenignore`

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](./CONTRIBUTING.md) for:
- Code style guide
- Testing requirements
- Pull request process
- Development setup

### Adding a New Language

1. Create `pkg/languages/swift/` directory
2. Implement `LanguageAnalyzer` interface
3. Register in `pkg/languages/registry.go`
4. Add tests
5. Submit PR

See [Adding Languages](./ARCHITECTURE.md#adding-languages) for detailed guide.

### Reporting Issues

Found a bug? Have a suggestion?
- 🐛 [Report Bug](https://github.com/acollie/kaizen/issues/new?template=bug_report.md)
- 💡 [Request Feature](https://github.com/acollie/kaizen/issues/new?template=feature_request.md)

---

## 📋 Requirements

- **Go 1.21+** - Build and run requirements
- **Git** - For churn analysis (optional with `--skip-churn`)
- **Tree-sitter libraries** - For Kotlin/Swift (auto-installed)
- **~100MB** - Disk space for database storage

---

## 📜 License

MIT License - see [LICENSE](./LICENSE) file for details.

## 🙏 Acknowledgments

Built with:
- [go-tree-sitter](https://github.com/smacker/go-tree-sitter) - Multi-language AST parsing
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [D3.js](https://d3js.org/) - Visualization
- [GORM](https://gorm.io/) - Database ORM

---

## 🎯 Roadmap

### Planned Features
- [x] 📊 Web dashboard for team health monitoring
- [ ] 🔌 GitHub/GitLab integration (automatic PR comments)
- [ ] 📈 Advanced trend prediction
- [ ] 🐍 Complete Python analyzer
- [ ] 🦀 Rust analyzer
- [ ] 📱 TypeScript/JavaScript analyzer
- [ ] ☕ Java analyzer

### Quality Improvements
- [ ] Performance optimization for massive codebases (100M+ LOC)
- [ ] Better error messages and recovery
- [ ] Parallel analysis for multi-core systems
- [ ] Incremental analysis (only changed files)

---

## 📞 Support

- 📖 [Full Documentation](./GUIDE.md)
- 🏗️ [Architecture Guide](./ARCHITECTURE.md)
- 💬 [GitHub Discussions](https://github.com/acollie/kaizen/discussions)
- 🐛 [Report Issue](https://github.com/acollie/kaizen/issues)

---

<div align="center">

Made with ❤️ for continuous improvement

[⭐ Star us on GitHub](https://github.com/acollie/kaizen) | [📬 Follow on Twitter](https://twitter.com/kaizencode) | [💼 LinkedIn](https://linkedin.com/in/acollie)

</div>

# 🏔️ Kaizen - Code Quality Analysis Tool

[![Build Status](https://github.com/acollie/kaizen/workflows/CI/badge.svg)](https://github.com/acollie/kaizen/actions)
[![codecov](https://codecov.io/github/Acollie/Kaizen/graph/badge.svg?token=9V3XZY7JAF)](https://codecov.io/github/Acollie/Kaizen)
[![Go Report Card](https://goreportcard.com/badge/github.com/acollie/kaizen)](https://goreportcard.com/report/github.com/acollie/kaizen)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org/)
[![Latest Release](https://img.shields.io/github/v/release/acollie/kaizen)](https://github.com/acollie/kaizen/releases)

**Continuous improvement starts with visibility.** Kaizen analyzes code quality, complexity, and churn to identify technical debt and hotspots in your codebase. It generates health grades, actionable concerns, and interactive visualizations.

---

## ✨ Features

- 🎯 **A-F Health Grades** with 0-100 scores across complexity, maintainability, churn, function size, and code structure
- 📈 **Cyclomatic & Cognitive Complexity**, Halstead Metrics, Maintainability Index, and hotspot detection
- 🌍 **Multi-Language** — Go (native AST), Python, Kotlin & Swift (tree-sitter)
- 🎨 **Interactive Visualizations** — HTML treemaps, Sankey diagrams, call graphs, terminal charts
- 🛡️ **CI Quality Gate** — blast-radius detection with exit codes for pipelines
- 🤖 **GitHub PR Action** — automatic PR comments with score deltas, hotspot tracking, and call graph diffs
- 📊 **Historical Tracking** — SQLite snapshots with trend analysis and diff reporting
- 👥 **Code Ownership** — CODEOWNERS-based team metrics and reports

---

## 📊 Self-Analysis — Kaizen on Kaizen

Kaizen practices what it preaches. Here are the results from analyzing its own codebase:

> **Grade B** — **88/100** ✅

| Metric | Value | Status |
|--------|-------|--------|
| 📁 Files Analyzed | 53 | |
| ⚙️ Total Functions | 394 | |
| 🧠 Avg Cyclomatic Complexity | 4.5 | ✅ Excellent |
| 🧩 Avg Cognitive Complexity | 4.9 | ✅ Excellent |
| 📏 Avg Function Length | 27 lines | ✅ Excellent |
| 🔧 Avg Maintainability Index | 86.8 | ✅ Good |
| 🔥 Hotspots | 0 | ✅ Perfect |

**Component Scores:**

| Component | Score | Rating |
|-----------|-------|--------|
| 🧠 Complexity | 77/100 | Good |
| 🔧 Maintainability | 87/100 | Good |
| 📏 Function Size | 92/100 | Excellent |
| 🏗️ Code Structure | 100/100 | Excellent |

### 🏥 Code Health Grades

| Grade | Score | Meaning |
|-------|-------|---------|
| 🟢 **A** | 90–100 | Excellent — well-maintained, low risk |
| 🟢 **B** | 75–89 | Good — minor improvements suggested |
| 🟡 **C** | 60–74 | Fair — consider refactoring |
| 🔴 **D** | 40–59 | Poor — significant technical debt |
| 🚨 **F** | 0–39 | Critical — urgent refactoring needed |

### 🔍 Automatic Issue Detection

| Issue | Threshold | Why It Matters |
|-------|-----------|----------------|
| 🔴 High Complexity | CC > 10 | Error-prone, hard to test |
| 🟡 Low Maintainability | MI < 20 | Hard to understand and modify |
| 🔴 Long Functions | > 50 lines | Harder to test and review |
| 🔴 Deep Nesting | > 4 levels | Confusing control flow |
| 🟡 High Churn | > 10 commits | Unstable, frequently changing |
| 🟠 Hotspots | High CC + High churn | Top priority for refactoring |

---

## 🤖 GitHub Action — PR Analysis

Add Kaizen to any repository to get automatic code quality comments on every pull request.

### ⚡ Quick Setup

Create `.github/workflows/kaizen.yml` in your repository:

```yaml
name: Kaizen PR Analysis

on:
  pull_request:
    branches: [main]

jobs:
  kaizen:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
      contents: read
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: acollie/kaizen@v0.1.0-beta
        with:
          path: "."
          base-branch: main
```

Every PR will receive a comment showing score changes, complexity metrics, and hotspot status. The comment is updated in-place on each push — never duplicated.

👉 See [full Action docs](#-inputs) for all inputs, outputs, and examples.

---

## 🚀 Quick Start

### 📦 Installation

```bash
git clone https://github.com/acollie/kaizen.git
cd kaizen
./install.sh

kaizen --version
```

### 🔬 First Analysis

```bash
cd /path/to/your/project
kaizen analyze --path=.
kaizen visualize --format=html
kaizen report owners
```

### 💻 Common Commands

```bash
# 🎨 Interactive heatmap
kaizen visualize --metric=complexity --format=html --open

# 🛡️ CI quality gate
kaizen check --base=main --path=.

# 🔗 Function call graph
kaizen callgraph --path=. --format=html

# 🔗 Call graph filtered to changed functions only
kaizen callgraph --path=. --base=main --format=svg

# 📈 Compare with previous analysis
kaizen diff --path=.

# 📊 Track trends over time
kaizen trend overall_score --days=30

# 👥 Team ownership report
kaizen report owners --format=html

# 🔄 Code ownership Sankey diagram
kaizen sankey --input=kaizen-results.json

# 📋 Analysis history
kaizen history list
```

### 🔧 Command Reference

| Command | Description |
|---------|-------------|
| `kaizen analyze` | 🔬 Analyze a codebase and generate metrics (JSON output) |
| `kaizen visualize` | 🎨 Generate interactive heatmaps (HTML, SVG, or terminal) |
| `kaizen check` | 🛡️ CI quality gate — warn on high blast-radius function changes |
| `kaizen callgraph` | 🔗 Generate function call graph (HTML, SVG, or JSON) |
| `kaizen pr-comment` | 🤖 Generate a GitHub PR comment from base vs head analysis |
| `kaizen sankey` | 🔄 Generate Sankey diagram of code ownership flow |
| `kaizen diff` | 📈 Compare current analysis with previous snapshot |
| `kaizen trend` | 📊 Visualize metric trends over time (ASCII, HTML, or JSON) |
| `kaizen report owners` | 👥 Generate code ownership report |
| `kaizen history list` | 📋 List all stored analysis snapshots |
| `kaizen history show` | 🔍 Display detailed snapshot information |
| `kaizen history prune` | 🗑️ Remove old snapshots |

---

## 🤖 GitHub Action — Full Reference

### 📥 Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `path` | Directory to analyze | `.` |
| `base-branch` | Branch to compare against | `main` |
| `skip-churn` | Skip git churn analysis for faster runs | `true` |
| `github-token` | GitHub token for posting PR comments | `${{ github.token }}` |
| `fail-on-regression` | Fail the action if the score drops | `false` |
| `languages` | Comma-separated list of languages to include | all |
| `include-callgraph` | Include SVG call graph of changed functions (Go only) | `false` |

### 📤 Outputs

| Output | Description |
|--------|-------------|
| `score-delta` | Numeric score change between base and head |
| `grade` | Current grade letter (A-F) |
| `has-concerns` | Whether blast-radius concerns were found |

### 🔧 Full Example

```yaml
name: Kaizen PR Analysis

on:
  pull_request:
    branches: [main, develop]

jobs:
  kaizen:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
      contents: read
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: acollie/kaizen@v0.1.0-beta
        id: kaizen
        with:
          path: "."
          base-branch: main
          skip-churn: "true"
          fail-on-regression: "true"
          languages: "go,kotlin"
          include-callgraph: "true"

      - name: Check results
        run: |
          echo "Score delta: ${{ steps.kaizen.outputs.score-delta }}"
          echo "Grade: ${{ steps.kaizen.outputs.grade }}"
          echo "Has concerns: ${{ steps.kaizen.outputs.has-concerns }}"
```

### 💬 What the PR Comment Shows

- 🎯 **Grade and Score** — overall health grade (A-F) with numeric score out of 100
- 📈 **Score Delta** — how much the score changed compared to the base branch
- 📊 **Metrics Table** — overall score, avg complexity, maintainability, hotspot count, function count with deltas
- 🔥 **Hotspot Changes** — new hotspots introduced, hotspots fixed, and persistent hotspots
- 💥 **Blast-Radius Warnings** — functions with high fan-in (many callers) that were modified
- 🔗 **Call Graph** (optional) — SVG artifact showing changed functions and their callers/callees

### 📌 Pinning a Version

```yaml
      # Pin to a specific release (recommended)
      - uses: acollie/kaizen@v0.1.0-beta

      # Or track the latest on main (may include breaking changes)
      - uses: acollie/kaizen@main
```

### 🔑 Using with a Custom Token

The default `${{ github.token }}` works for most cases. If you need to trigger other workflows from the comment, use a Personal Access Token or GitHub App token:

```yaml
      - uses: acollie/kaizen@v0.1.0-beta
        with:
          github-token: ${{ secrets.KAIZEN_TOKEN }}
```

---

## 🎯 Roadmap

### 🚀 Planned Features
- [x] 📊 Web dashboard for team health monitoring
- [x] 🤖 GitHub integration (automatic PR comments via reusable Action)
- [x] 🐍 Complete Python analyzer with tree-sitter AST parsing
- [ ] 📈 Advanced trend prediction
- [ ] 🦀 Rust analyzer
- [ ] 📱 TypeScript/JavaScript analyzer
- [ ] ☕ Java analyzer

### 🔧 Quality Improvements
- [ ] ⚡ Performance optimization for massive codebases (100M+ LOC)
- [ ] 💬 Better error messages and recovery
- [ ] 🧵 Parallel analysis for multi-core systems
- [ ] 🔄 Incremental analysis (only changed files)

---

## 🏗️ How It Works

### 🧱 Architecture

Kaizen uses a modular, language-agnostic architecture:

- 🔌 **Interface-based language analyzers** — easy to add new languages
- 🌳 **Tree-sitter AST parsing** — accurate syntax understanding (Python, Kotlin, Swift)
- 🐹 **Go's `ast` package** — native support for Go analysis
- 💾 **SQLite time-series database** — efficient historical tracking
- ⌨️ **Cobra CLI framework** — professional command structure

📖 See [Architecture Guide](./ARCHITECTURE.md) for technical details.

### 🌍 Supported Languages

| Language | Status | Parser | Coverage |
|----------|--------|--------|----------|
| 🐹 Go | ✅ Full | go/ast | 95%+ |
| 🐍 Python | ✅ Full | tree-sitter | 90%+ |
| 🟣 Kotlin | ✅ Full | tree-sitter | 90%+ |
| 🍎 Swift | ✅ Full | tree-sitter | 90%+ |

### 📏 What It Analyzes

**Per-File:** lines of code, import count, duplication percentage

**Per-Function:** length, parameter count, cyclomatic complexity, cognitive complexity, nesting depth, Halstead metrics, maintainability index, fan-in/fan-out

### 🎨 Visualizations

🗺️ **Interactive Heatmap** — drill-down treemap with color-coded metrics. Color intensity = severity, box size = code volume, click to explore, hover for details.

🔗 **Call Graph** — D3.js force-directed graph showing function call relationships. Node size = fan-in, color = complexity. Filter by `--base` to show only changed functions.

🔄 **Sankey Diagram** — code ownership flow showing team-to-function dependencies. Flow width = call frequency.

📟 **Terminal Charts** — quick ASCII trend lines without opening a browser.

### ⚡ Performance

| Project Size | Estimated Time |
|--------------|----------------|
| 10K LOC | ~0.1-0.5s |
| 100K LOC | ~1-3s |
| 1M LOC | ~15-25s |
| 10M+ LOC | ~3-5m |

💡 Use `--skip-churn` to disable git operations (adds 20-40% overhead). Run on SSD for 2-3x faster I/O. Exclude large directories with `.kaizenignore`.

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](./CONTRIBUTING.md) for code style, testing requirements, and pull request process.

### 🌍 Adding a New Language

1. Create `pkg/languages/<lang>/` directory
2. Implement the `LanguageAnalyzer` interface
3. Register in `pkg/languages/registry.go`
4. Add tests
5. Submit PR

📖 See [Adding Languages](./ARCHITECTURE.md#adding-languages) for the detailed guide.

### 🐛 Reporting Issues

- 🐛 [Report Bug](https://github.com/acollie/kaizen/issues/new?template=bug_report.md)
- 💡 [Request Feature](https://github.com/acollie/kaizen/issues/new?template=feature_request.md)

---

## ✅ Quality Assurance

All code changes are tested across Go versions 1.21, 1.22, and 1.23 on Linux:

- 🧪 **Unit Tests**: 50+ test files covering analyzers, metrics, and language parsers
- 🔗 **Integration Tests**: end-to-end analysis and visualization pipeline validation
- 📊 **Coverage Target**: 50%+ on main branch with automated codecov checks
- 🔄 **CI Pipeline**: automated testing, linting (`golangci-lint`), and build verification on every PR

[![Tests](https://github.com/acollie/kaizen/actions/workflows/ci.yml/badge.svg)](https://github.com/acollie/kaizen/actions/workflows/ci.yml) [![codecov](https://codecov.io/github/Acollie/Kaizen/graph/badge.svg?token=9V3XZY7JAF)](https://codecov.io/github/Acollie/Kaizen)

---

## 📋 Requirements

- 🐹 **Go 1.21+**
- 🔀 **Git** (optional — use `--skip-churn` without it)
- 🌳 **Tree-sitter libraries** for Kotlin/Swift (auto-installed)

---

## 📖 Documentation

- 📖 [Usage Guide](./GUIDE.md) — installation, configuration, daily workflow
- 🏗️ [Architecture Guide](./ARCHITECTURE.md) — internals, metrics, extending
- 📊 [Self-Analysis Report](./ANALYSIS_REPORT.md) — Kaizen analyzing itself
- 💬 [GitHub Discussions](https://github.com/acollie/kaizen/discussions)

---

## 📜 License

MIT License — see [LICENSE](./LICENSE) for details.

## 🙏 Acknowledgments

Built with [go-tree-sitter](https://github.com/smacker/go-tree-sitter), [Cobra](https://github.com/spf13/cobra), [D3.js](https://d3js.org/), and [GORM](https://gorm.io/).

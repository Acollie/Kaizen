# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Upgrade notes
- **Go nesting depth is now measured correctly.** Earlier versions reported a
  `NestingDepth` of about 0 for almost every Go function. Snapshots stored
  before this release under-report nesting, so the first `kaizen diff` (or
  trend) against an older snapshot can show nesting-depth regressions with no
  code change. Take a fresh snapshot to reset the baseline.
- **Go 1.25 is now the minimum** Go version for building Kaizen.
- **Call-graph node keys can be directory-qualified.** Keys stay `pkg.Func` and
  `pkg.Type.Method`, except when two directories declare the same package name
  (for example two `main` packages): then both are qualified with their
  directory, e.g. `cmd/kaizen:main.main`, instead of one silently overwriting
  the other.

### Added
- `kaizen review <pr-number|pr-url>`: fetches a GitHub pull request with `gh`,
  ranks its changed functions by risk (complexity, fan-in, churn, size,
  security-sensitive code), runs an optional AI review, and opens a local
  interactive UI from which the review can be posted back to GitHub. A PR URL
  is reviewed in the URL's repository and must be run from a clone of it.
- Call graph: callers/callees index, method calls resolved via receivers and
  unique method names (marked inferred), and hidden/vendor/testdata
  directories skipped.
- `--version` flag and Homebrew formula.
- Full Kotlin language support with tree-sitter AST parsing
- Code ownership reports with CODEOWNERS integration
- Interactive Sankey diagrams for team dependencies
- Historical trend tracking with SQLite database
- Multiple visualization formats (HTML, ASCII, JSON)
- Code health grading system (A-F scale)
- Areas of concern detection for problematic code
- Halstead complexity metrics
- Cognitive complexity analysis
- Maintainability index calculation

### Changed
- Upgraded Kotlin analyzer from regex-based to tree-sitter AST parsing

### Fixed
- Component scores showing 0/100 in heatmap visualization
- Areas of concern items not displaying in heatmap

## [0.1.0] - 2025-01-31

### Added
- Initial public release
- Go language analyzer with full AST parsing
- Cyclomatic complexity calculation
- Git churn analysis (file-level and function-level)
- Interactive HTML treemap visualizations with D3.js
- Nordic warm color palette design
- Configuration system (.kaizen.yaml, .kaizenignore)
- CLI with analyze, visualize, trends, and owners commands
- SQLite database for historical metrics tracking
- ASCII and HTML trend charts
- Function-level metrics extraction
- Hotspot detection (high complexity + high churn)
- Installation script (install.sh)
- Comprehensive documentation (README.md, CLAUDE.md)

### Technical Details
- Built with Go 1.21+
- Interface-based language analyzer architecture
- Support for custom ignores patterns
- Configurable complexity thresholds
- Percentile-based metric normalization

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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

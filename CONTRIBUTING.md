# Contributing to Kaizen

First off, thank you for considering contributing to Kaizen! It's people like you that make Kaizen such a great tool for code quality analysis.

## Code of Conduct

Please review our [Code of Conduct](CODE_OF_CONDUCT.md) before participating in this project. We are committed to fostering an open and welcoming environment for all contributors.

## Development Setup

### Prerequisites

- **Go 1.21 or higher** - [Install Go](https://golang.org/doc/install)
- **Git** - For version control and churn analysis features
- A code editor (VS Code, GoLand, Vim, etc.)

### Building from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/alexcollie/kaizen.git
   cd kaizen
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Build the binary:**
   ```bash
   go build -o kaizen ./cmd/kaizen
   ```

4. **Run the binary:**
   ```bash
   ./kaizen --help
   ```

### Running Tests

Run the full test suite:
```bash
go test ./...
```

Run tests with verbose output:
```bash
go test -v ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Dogfooding - Analyzing Kaizen Itself

A great way to understand how Kaizen works is to run it on its own codebase:

```bash
# Build the binary
go build -o kaizen ./cmd/kaizen

# Analyze the kaizen project itself
./kaizen analyze --path=. --skip-churn --output=kaizen-analysis.json

# Generate HTML visualization
./kaizen visualize --input=kaizen-analysis.json --format=html --output=kaizen-heatmap.html --open=false
```

## How to Contribute

### Reporting Bugs

If you find a bug, please create an issue using the bug report template. Include:

- **Environment information**: OS, Go version, Kaizen version
- **Steps to reproduce**: Clear, minimal steps to reproduce the issue
- **Expected behavior**: What you expected to happen
- **Actual behavior**: What actually happened
- **Logs/screenshots**: Any relevant error messages or screenshots
- **Sample code**: If applicable, a minimal code example that triggers the bug

### Suggesting Features

We welcome feature suggestions! Please create an issue using the feature request template. Include:

- **Problem description**: What problem does this feature solve?
- **Proposed solution**: How would you like to see it implemented?
- **Alternatives considered**: What other approaches did you consider?
- **Additional context**: Any other context, mockups, or examples

### Submitting Pull Requests

1. **Fork the repository** and create your branch from `main`:
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes** following our code style guidelines (see below)

3. **Write or update tests** for your changes:
   - All new functions should have unit tests
   - Bug fixes should include a test that would have caught the bug
   - Aim for high test coverage on new code

4. **Update documentation**:
   - Update README.md if adding user-facing features
   - Update CLAUDE.md if changing architecture
   - Add code comments for complex logic
   - Update CHANGELOG.md under `[Unreleased]`

5. **Ensure all tests pass**:
   ```bash
   go test ./...
   go vet ./...
   ```

6. **Commit your changes** using conventional commits (see below)

7. **Push to your fork**:
   ```bash
   git push origin feature/my-new-feature
   ```

8. **Open a Pull Request** using the PR template

## Code Style Guidelines

We follow clean code principles inspired by Uncle Bob's recommendations:

### General Principles

- **Avoid single-character variable names**: Use descriptive names like `fileCount` instead of `n`
- **Prefer smaller methods**: Functions should do one thing well
- **Function length**: Aim for functions under 50 lines; 100 lines is acceptable but not preferred
- **Meaningful names**: Names should reveal intent - `getUserByEmail` not `getUsr`
- **Comments**: Explain *why*, not *what* - the code should be self-documenting

### Go-Specific Style

- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use `gofmt` and `goimports` for formatting
- Run `go vet` to catch common mistakes
- Follow Go naming conventions:
  - `camelCase` for private functions/variables
  - `PascalCase` for exported functions/variables
  - Acronyms stay uppercase: `HTTPServer` not `HttpServer`

### Example

**Bad:**
```go
func p(f string) (*models.FileAnalysis, error) {
    // This function parses a file
    d, err := os.ReadFile(f)
    if err != nil {
        return nil, err
    }
    // ... 150 lines of parsing logic ...
}
```

**Good:**
```go
func parseFile(filePath string) (*models.FileAnalysis, error) {
    content, err := readFileContent(filePath)
    if err != nil {
        return nil, fmt.Errorf("reading file: %w", err)
    }

    ast := parseAST(content)
    functions := extractFunctions(ast)
    metrics := calculateMetrics(functions)

    return buildAnalysis(filePath, functions, metrics), nil
}

func readFileContent(filePath string) ([]byte, error) {
    return os.ReadFile(filePath)
}

func parseAST(content []byte) *ast.File {
    // 20-30 lines of focused AST parsing
}
```

## Adding New Language Analyzers

Kaizen uses an **interface-based architecture** for language support. Adding a new language is straightforward:

### 1. Understand the Interface

All language analyzers implement the `LanguageAnalyzer` interface in `pkg/analyzer/interfaces.go`:

```go
type LanguageAnalyzer interface {
    Name() string
    FileExtensions() []string
    CanAnalyze(filePath string) bool
    AnalyzeFile(filePath string) (*models.FileAnalysis, error)
    IsStub() bool
}
```

### 2. Create Your Analyzer

Create a new directory: `pkg/languages/<language>/`

Example structure:
```
pkg/languages/python/
├── analyzer.go      # Main analyzer implementation
├── function.go      # FunctionNode implementation
├── metrics.go       # Complexity calculators
└── analyzer_test.go # Unit tests
```

### 3. Implement the Interface

**Minimal example (`analyzer.go`):**

```go
package python

import (
    "github.com/alexcollie/kaizen/pkg/analyzer"
    "github.com/alexcollie/kaizen/pkg/models"
)

type PythonAnalyzer struct{}

func NewPythonAnalyzer() analyzer.LanguageAnalyzer {
    return &PythonAnalyzer{}
}

func (p *PythonAnalyzer) Name() string {
    return "Python"
}

func (p *PythonAnalyzer) FileExtensions() []string {
    return []string{".py"}
}

func (p *PythonAnalyzer) CanAnalyze(filePath string) bool {
    return strings.HasSuffix(filePath, ".py")
}

func (p *PythonAnalyzer) IsStub() bool {
    return false // Set to true if not fully implemented
}

func (p *PythonAnalyzer) AnalyzeFile(filePath string) (*models.FileAnalysis, error) {
    // 1. Read file
    content, err := os.ReadFile(filePath)
    if err != nil {
        return nil, err
    }

    // 2. Parse AST (use tree-sitter, python parser library, etc.)
    ast := parsePythonAST(content)

    // 3. Extract functions
    functions := extractFunctions(ast)

    // 4. Calculate metrics for each function
    var functionAnalyses []models.FunctionAnalysis
    for _, fn := range functions {
        functionAnalyses = append(functionAnalyses, models.FunctionAnalysis{
            Name:                  fn.Name,
            StartLine:             fn.StartLine,
            EndLine:               fn.EndLine,
            Length:                fn.Length,
            CyclomaticComplexity:  calculateCyclomaticComplexity(fn),
            CognitiveComplexity:   calculateCognitiveComplexity(fn),
            HalsteadVolume:        calculateHalsteadVolume(fn),
        })
    }

    // 5. Return file analysis
    return &models.FileAnalysis{
        FilePath:  filePath,
        Language:  "Python",
        Functions: functionAnalyses,
    }, nil
}
```

### 4. Register Your Analyzer

Add it to `pkg/languages/registry.go`:

```go
func NewRegistry() *Registry {
    return &Registry{
        analyzers: []analyzer.LanguageAnalyzer{
            golang.NewGoAnalyzer(),
            kotlin.NewKotlinAnalyzer(),
            python.NewPythonAnalyzer(), // Add your analyzer here
        },
    }
}
```

### 5. Write Tests

Create `analyzer_test.go`:

```go
func TestPythonAnalyzer_AnalyzeFile(t *testing.T) {
    analyzer := NewPythonAnalyzer()

    // Test basic function
    result, err := analyzer.AnalyzeFile("testdata/simple.py")
    require.NoError(t, err)
    assert.Equal(t, "Python", result.Language)
    assert.Len(t, result.Functions, 1)

    // Test complexity calculation
    assert.Greater(t, result.Functions[0].CyclomaticComplexity, 1)
}
```

### 6. Testing Expectations

- **Unit tests required** for all analyzers
- Test files should be in `testdata/` directory
- Cover edge cases: empty files, no functions, complex functions
- Test complexity calculations with known values
- Test error handling: invalid syntax, missing files

### Reference Implementations

- **Full implementation**: `pkg/languages/golang/` - Complete with AST parsing
- **Stub implementation**: `pkg/languages/kotlin/` - Shows interface structure

## Testing Guidelines

### Test Naming Conventions

- Test files: `*_test.go`
- Test functions: `TestFunctionName_Scenario`
- Example: `TestGoAnalyzer_AnalyzeFile_WithComplexFunction`

### Test Structure

Follow the AAA pattern (Arrange, Act, Assert):

```go
func TestCalculateCyclomaticComplexity(t *testing.T) {
    // Arrange
    code := `
    func example(x int) int {
        if x > 0 {
            return x
        }
        return 0
    }`
    function := parseFunction(code)

    // Act
    complexity := calculateCyclomaticComplexity(function)

    // Assert
    assert.Equal(t, 2, complexity) // 1 base + 1 if statement
}
```

### Using Test Fixtures

Place test files in `testdata/` directories:

```
pkg/languages/python/
├── analyzer.go
├── analyzer_test.go
└── testdata/
    ├── simple.py
    ├── complex.py
    └── edge_cases.py
```

Reference in tests:
```go
result, err := analyzer.AnalyzeFile("testdata/simple.py")
```

### Mocking

For external dependencies (git, filesystem), use interfaces and mocks:

```go
type GitClient interface {
    GetCommitHistory(path string) ([]Commit, error)
}

// In tests
mockGit := &MockGitClient{
    commits: []Commit{{Author: "test"}},
}
```

## Commit Message Conventions

We use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring (no feature change or bug fix)
- `perf`: Performance improvement
- `test`: Adding or updating tests
- `chore`: Maintenance tasks (dependencies, build scripts)
- `ci`: CI/CD changes

### Examples

```
feat(python): Add Python language analyzer

Implements full AST parsing for Python files using tree-sitter.
Calculates cyclomatic, cognitive, and Halstead metrics.

Closes #42
```

```
fix(golang): Fix panic in complexity calculation

Added nil check for empty function bodies.

Fixes #123
```

```
docs: Update CONTRIBUTING.md with testing guidelines
```

```
chore(deps): Bump golang.org/x/tools to v0.18.0
```

## Pull Request Process

1. **Ensure CI passes**: All tests, linting, and builds must pass
2. **Update documentation**: README, CLAUDE.md, CHANGELOG.md as needed
3. **Get review**: At least one maintainer must approve
4. **Squash commits**: We prefer clean history - squash related commits
5. **Merge**: Maintainers will merge once approved

### PR Checklist

Before submitting, verify:

- [ ] All tests pass (`go test ./...`)
- [ ] Code follows style guidelines
- [ ] New code has tests
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Commit messages follow conventions
- [ ] No breaking changes (or clearly documented)
- [ ] PR description is clear and complete

## Questions?

- **General questions**: Open a [GitHub Discussion](https://github.com/alexcollie/kaizen/discussions)
- **Bug reports**: Open an [Issue](https://github.com/alexcollie/kaizen/issues/new?template=bug_report.md)
- **Feature requests**: Open an [Issue](https://github.com/alexcollie/kaizen/issues/new?template=feature_request.md)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

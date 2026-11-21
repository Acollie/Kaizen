package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// addedEdit is the edit of a function in path made only of the given added lines
func addedEdit(path string, lines ...string) functionEdit {
	hunkLines := make([]string, len(lines))
	for index, line := range lines {
		hunkLines[index] = "+" + line
	}
	file := &FileDiff{Path: path, Hunks: []Hunk{hunkOf(hunkLines...)}}
	return analyzeFunctionEdit(file, 1, len(lines))
}

func TestBehaviourChangeFactor(t *testing.T) {
	testCases := []struct {
		name   string
		path   string
		lines  []string
		units  int
		reason string
	}{
		{
			name:  "branches and an exec.Command",
			path:  "a.go",
			lines: []string{"if a {", "} else if b {", "for _, x := range xs {", "switch y {", `out, err := exec.Command("git", "log").Output()`},
			units: 6, reason: "behaviour change: adds 4 branches and an exec.Command",
		},
		{
			name:  "error returns, goroutine, defer and panic",
			path:  "a.go",
			lines: []string{"return nil, err", `return fmt.Errorf("bad: %w", err)`, "go worker(ch)", "defer mu.Unlock()", `panic("unreachable")`},
			units: 6, reason: "behaviour change: adds 2 error returns, a goroutine, a defer and a panic",
		},
		{
			name:  "plain return is not an error return",
			path:  "a.go",
			lines: []string{"return total, nil", "x := compute()"},
			units: 0,
		},
		{
			name:  "keywords inside strings and comments do not count",
			path:  "a.go",
			lines: []string{`log.Print("if for switch go defer")`, "x := 1 // if this panics, return err"},
			units: 0,
		},
		{
			name:  "Go block comment and doc prose do not count",
			path:  "a.go",
			lines: []string{"/*", "If this fails we panic.", "Go and defer are words here.", "*/", "x := 1 /* if */"},
			units: 0,
		},
		{
			name:  "Python hash comment and docstring do not count",
			path:  "a.py",
			lines: []string{`"""Fetch the file.`, `If this fails we panic or raise.`, `"""`, `x = 1  # If this fails we panic`},
			units: 0,
		},
		{
			name:  "Kotlin KDoc does not count",
			path:  "a.kt",
			lines: []string{"/**", " * If the call fails we throw, when retries run out.", " */"},
			units: 0,
		},
		{
			name:  "single branch scores but is not named",
			path:  "a.go",
			lines: []string{"if ok {"},
			units: 1,
		},
		{
			name:  "python keywords, raise is an error return",
			path:  "a.py",
			lines: []string{"elif value:", "while running:", "raise ValueError(msg)"},
			units: 3, reason: "behaviour change: adds 2 branches and an error return",
		},
		{
			name:  "kotlin when and throw",
			path:  "a.kt",
			lines: []string{"when (state) {", `throw IllegalStateException("x")`},
			units: 2, reason: "behaviour change: adds a branch and an error return",
		},
		{
			name: "selectors, assignments and keys are not keywords",
			path: "a.go",
			lines: []string{"ok := re.Match(x)", "rows := db.Select(q)", "matched, _ := filepath.Match(p, n)",
				`logger.Panic("x")`, "cfg := Config{Go: true}"},
			units: 0,
		},
		{
			name:  "python selectors and assignments",
			path:  "a.py",
			lines: []string{"found = re.match(pattern, text)", "match = 3", "self.raise_on_error = True"},
			units: 0,
		},
		{
			name:  "kotlin val named when",
			path:  "a.kt",
			lines: []string{"val when = 3"},
			units: 0,
		},
		{
			name:  "unknown language keeps the union of keywords",
			path:  "",
			lines: []string{"elif value:", "when (state) {", "guard let x = y else {"},
			units: 3, reason: "behaviour change: adds 3 branches",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			edit := addedEdit(testCase.path, testCase.lines...)
			change := countBehaviourChange(edit.unpairedAdded, edit.languageSyntax())
			assert.Equal(t, testCase.units, change.units())
			factor := behaviourChangeFactor(edit)
			assert.Equal(t, testCase.reason, factor.reason)
			assert.InDelta(t, WeightBehaviourChange*ramp(float64(testCase.units), 0, BehaviourChangeFullUnits), factor.contribution, 1e-9)
		})
	}
}

func TestReturnsError(t *testing.T) {
	testCases := []struct {
		line        string
		errorReturn bool
	}{
		{"return err", true},
		{"return nil, err", true},
		{`return fmt.Errorf("parse %s: %w", name, err)`, true},
		{`return errors.New("closed")`, true},
		{"return nil, wrapErr(err)", true},
		{"return result, parseErr", true},
		{"return ErrClosed", true},
		{"return &ParseError{Line: line}", true},
		{"if err != nil { return err }", true},
		{`return fmt.Errorf("fetch %s: %w",`, true},
		{"return nil, fmt.Errorf(", true},
		{"return err == nil", false},
		{"return errorCount", false},
		{"return len(errs), nil", false},
		{"return err.Error()", false},
		{"return errors.Is(err, io.EOF)", false},
		{"return total, nil", false},
		{"return isError", false},
		{`return fmt.Errorf("x") == nil`, false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.line, func(t *testing.T) {
			change := countBehaviourChange([]string{testCase.line}, goSyntax)
			assert.Equal(t, testCase.errorReturn, change.errorReturns == 1, "%+v", change)
		})
	}
}

func TestDecisionLogicFactor(t *testing.T) {
	testCases := []struct {
		name     string
		function string
		path     string
		lines    []string
		signals  int
		label    string
	}{
		{
			name: "withholding predicate", function: "shouldWithhold", path: "a.go",
			lines:   []string{`if strings.HasSuffix(path, ".min.js") {`, "return true", "}", "return false"},
			signals: 2, label: "shouldWithhold",
		},
		{
			name: "decision-named assignment", function: "markFiles", path: "a.go",
			lines:   []string{"file.Generated = true", "generated[file.Path] = true"},
			signals: 2, label: "file.Generated",
		},
		{
			name: "skip flag from content", function: "process", path: "a.go",
			lines:   []string{`skip := strings.Contains(content, "@generated")`},
			signals: 1, label: "skip",
		},
		{
			name: "returns a decision value", function: "load", path: "a.go",
			lines:   []string{"return isMinified(file)"},
			signals: 1, label: "isMinified",
		},
		{
			name: "empty reason keeps the file", function: "generatedReason", path: "a.go",
			lines:   []string{"if !bundleExtensions[ext] {", `return ""`, "}"},
			signals: 1, label: "generatedReason",
		},
		{
			name: "python trust check", function: "is_trusted_host", path: "a.py",
			lines:   []string{`return host.endswith(".corp.example.com")`},
			signals: 1, label: "is_trusted_host",
		},
		{
			name: "regex match feeding a boolean return", function: "check", path: "a.go",
			lines:   []string{"if pattern.MatchString(path) {", "return true", "}"},
			signals: 1, label: "pattern.MatchString",
		},
		{
			name: "plain error handling does not fire", function: "Run", path: "a.go",
			lines: []string{"if err != nil {", "return err", "}", "return nil"},
		},
		{
			name: "decision word only in a comment does not fire", function: "Run", path: "a.go",
			lines: []string{"x := compute() // skip the generated files", "return x"},
		},
		{
			name: "contains without a yes-or-no answer does not fire", function: "render", path: "a.go",
			lines: []string{`if strings.Contains(message, "x") {`, "out = append(out, message)", "}"},
		},
		{
			name: "decision-named helper returning a position does not fire", function: "skipLiteral", path: "a.go",
			lines: []string{"return position"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			function := &FunctionRisk{Name: testCase.function, File: testCase.path}
			factor := decisionLogicFactor(function, addedEdit(testCase.path, testCase.lines...))
			assert.InDelta(t, WeightDecisionLogic*ramp(float64(testCase.signals), 0, decisionFullSignals), factor.contribution, 1e-9)
			if testCase.signals == 0 {
				assert.Empty(t, factor.reason)
				return
			}
			assert.Equal(t, "decides what is skipped or trusted ("+testCase.label+"): check it can't be gamed by PR content", factor.reason)
		})
	}
}

func TestBehaviourChangeIgnoresRewrittenLines(t *testing.T) {
	// The if is a reformat of an existing statement, so it adds no branch
	file := &FileDiff{Hunks: []Hunk{hunkOf(" func A() {", "-	if a>b {", "+	if a > b {", " }")}}
	edit := analyzeFunctionEdit(file, 1, 4)
	assert.Zero(t, behaviourChangeFactor(edit).contribution)
}

func TestBehaviourChangeRanksNewLogicAboveMechanicalRewrite(t *testing.T) {
	mechanicalLines := []string{" func Format() string {"}
	for range 10 {
		mechanicalLines = append(mechanicalLines, `-	sb.WriteString(fmt.Sprintf("%s\n", row))`)
	}
	for range 10 {
		mechanicalLines = append(mechanicalLines, `+	fmt.Fprintf(&sb, "%s\n", row)`)
	}
	behaviourLines := []string{
		" func Handle() error {",
		"+	if err := validate(input); err != nil {",
		"+		return fmt.Errorf(\"validate: %w\", err)",
		"+	}",
		"+	for _, item := range input.Items {",
		"+		if item.Expired() {",
		"+			continue",
		"+		}",
		"+	}",
	}
	files := []FileDiff{
		{Path: "cmd/format.go", Hunks: []Hunk{hunkOf(mechanicalLines...)}},
		{Path: "pkg/handle.go", Hunks: []Hunk{hunkOf(behaviourLines...)}},
	}
	functions := []FunctionRisk{
		{ID: "main.Format", Name: "Format", File: "cmd/format.go", StartLine: 1, EndLine: 11, LinesAdded: 10, LinesRemoved: 10, Metrics: simpleMetrics(17, 29, 4, 20)},
		{ID: "pkg.Handle", Name: "Handle", File: "pkg/handle.go", StartLine: 1, EndLine: 9, LinesAdded: 8, Metrics: simpleMetrics(5, 6, 3, 60)},
	}
	functions[0].Metrics.Length = 90
	functions[1].Metrics.Length = 30
	ScoreFunctionsWithDiff(functions, files, 5)
	assert.Greater(t, functions[1].RiskScore, functions[0].RiskScore)
	assert.Contains(t, functions[1].RiskReasons, "behaviour change: adds 3 branches and an error return")
}

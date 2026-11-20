package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBehaviourChangeFactor(t *testing.T) {
	testCases := []struct {
		name   string
		lines  []string
		units  int
		reason string
	}{
		{
			name:  "branches and an exec.Command",
			lines: []string{"if a {", "} else if b {", "for _, x := range xs {", "switch y {", `out, err := exec.Command("git", "log").Output()`},
			units: 6, reason: "behaviour change: adds 4 branches and an exec.Command",
		},
		{
			name:  "error returns, goroutine, defer and panic",
			lines: []string{"return nil, err", `return fmt.Errorf("bad: %w", err)`, "go worker(ch)", "defer mu.Unlock()", `panic("unreachable")`},
			units: 6, reason: "behaviour change: adds 2 error returns, a goroutine, a defer and a panic",
		},
		{
			name:  "plain return is not an error return",
			lines: []string{"return total, nil", "x := compute()"},
			units: 0,
		},
		{
			name:  "keywords inside strings and comments do not count",
			lines: []string{`log.Print("if for switch go defer")`, "x := 1 // if this panics, return err"},
			units: 0,
		},
		{
			name:  "single branch scores but is not named",
			lines: []string{"if ok {"},
			units: 1,
		},
		{
			name:  "python and kotlin keywords",
			lines: []string{"elif value:", "while running:", "raise ValueError(msg)", "when (state) {"},
			units: 4, reason: "behaviour change: adds 3 branches and a panic",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			change := countBehaviourChange(testCase.lines)
			assert.Equal(t, testCase.units, change.units())
			factor := behaviourChangeFactor(functionEdit{unpairedAdded: testCase.lines})
			assert.Equal(t, testCase.reason, factor.reason)
			assert.InDelta(t, WeightBehaviourChange*ramp(float64(testCase.units), 0, BehaviourChangeFullUnits), factor.contribution, 1e-9)
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

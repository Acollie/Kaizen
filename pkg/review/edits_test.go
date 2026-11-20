package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteScore(t *testing.T) {
	testCases := []struct {
		name    string
		removed string
		added   string
		rewrite bool
	}{
		{"Sprintf to Fprintf", `sb.WriteString(fmt.Sprintf("%s: %d\n", name, count))`, `fmt.Fprintf(&sb, "%s: %d\n", name, count)`, true},
		{"reformat", `if x==nil{`, `if x == nil {`, true},
		{"rename", `total := sumValues(items)`, `sum := addValues(entries)`, true},
		{"string literal change", `log.Printf("starting %s", name)`, `log.Printf("booting %s", name)`, true},
		{"comment only change", `return value // old note`, `return value // new note`, true},
		{"comparison change", `if index > limit {`, `if index >= limit {`, false},
		{"arithmetic change", `total := left + right`, `total := left - right`, false},
		{"number change", `retries := 3`, `retries := 5`, false},
		{"swapped arguments", `copyFile(source, destination)`, `copyFile(destination, source)`, false},
		{"nil to error", `return result, nil`, `return nil, err`, false},
		{"new condition", `if ok {`, `if ok && !expired {`, false},
		{"unrelated statement", `defer file.Close()`, `values = append(values, item)`, false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, rewrite := rewriteScore(shapeOf(testCase.removed), shapeOf(testCase.added))
			assert.Equal(t, testCase.rewrite, rewrite)
		})
	}
}

func TestTokenizeLineMergesQualifiedNames(t *testing.T) {
	var texts []string
	for _, token := range tokenizeLine(`if err := pkg.Do(a.b, "x") // trailing`) {
		texts = append(texts, token.text)
	}
	assert.Equal(t, []string{"if", "err", ":=", "pkg.do", "(", "a.b", ",", "str", ")"}, texts)
}

// hunkOf builds a hunk from "-old" / "+new" / " context" lines starting at head line 1
func hunkOf(lines ...string) Hunk {
	hunk := Hunk{NewStart: 1}
	oldLine, newLine := 1, 1
	for _, line := range lines {
		switch line[0] {
		case '-':
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineRemoved, OldLine: oldLine, Text: line[1:]})
			oldLine++
		case '+':
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineAdded, NewLine: newLine, Text: line[1:]})
			newLine++
		default:
			hunk.Lines = append(hunk.Lines, DiffLine{Kind: LineContext, OldLine: oldLine, NewLine: newLine, Text: line[1:]})
			oldLine++
			newLine++
		}
	}
	return hunk
}

func TestAnalyzeFunctionEdit(t *testing.T) {
	testCases := []struct {
		name       string
		lines      []string
		mechanical bool
		rewritten  int
		changed    int
		unpaired   int
	}{
		{
			name: "fmt.Fprintf rewrite is mechanical",
			lines: []string{
				" func Report() string {",
				`-	sb.WriteString(fmt.Sprintf("a %d\n", one))`,
				`-	sb.WriteString(fmt.Sprintf("b %s\n", two))`,
				`+	fmt.Fprintf(&sb, "a %d\n", one)`,
				`+	fmt.Fprintf(&sb, "b %s\n", two)`,
				" }",
			},
			mechanical: true, rewritten: 4, changed: 4,
		},
		{
			name: "rewrite plus new branch is not mechanical",
			lines: []string{
				" func Report() string {",
				`-	sb.WriteString(fmt.Sprintf("a %d\n", one))`,
				`+	fmt.Fprintf(&sb, "a %d\n", one)`,
				`+	if one > 3 {`,
				`+		return ""`,
				`+	}`,
				" }",
			},
			mechanical: false, rewritten: 2, changed: 5, unpaired: 3,
		},
		{
			name:       "new lines only are not mechanical",
			lines:      []string{" func A() {", "+	run()", "+	stop()", " }"},
			mechanical: false, changed: 2, unpaired: 2,
		},
		{
			name:       "condition change is not mechanical",
			lines:      []string{" func A() {", "-	if a > b {", "+	if a >= b {", " }"},
			mechanical: false, changed: 2, unpaired: 1,
		},
		{
			name:       "blank lines are ignored",
			lines:      []string{" func A() {", "-	x:=1", "+	x := 1", "+", " }"},
			mechanical: true, rewritten: 2, changed: 2,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			file := &FileDiff{Hunks: []Hunk{hunkOf(testCase.lines...)}}
			edit := analyzeFunctionEdit(file, 1, 20)
			assert.Equal(t, testCase.mechanical, edit.isMechanical())
			assert.Equal(t, testCase.rewritten, edit.rewritten)
			assert.Equal(t, testCase.changed, edit.changed)
			assert.Len(t, edit.unpairedAdded, testCase.unpaired)
		})
	}
}

func TestAnalyzeFunctionEditStaysInRange(t *testing.T) {
	file := &FileDiff{Hunks: []Hunk{hunkOf(" a", "-old()", "+new()", " b", " c", "+outside()")}}
	edit := analyzeFunctionEdit(file, 1, 3)
	assert.Equal(t, []string{"new()"}, edit.added)
	assert.Equal(t, 2, edit.changed)
	assert.Empty(t, analyzeFunctionEdit(nil, 1, 3).added)
}

func TestMechanicalEditIsDiscounted(t *testing.T) {
	lines := []string{" func FormatReport() string {"}
	for range 8 {
		lines = append(lines, `-	sb.WriteString(fmt.Sprintf("row %d\n", value))`)
	}
	for range 8 {
		lines = append(lines, `+	fmt.Fprintf(&sb, "row %d\n", value)`)
	}
	lines = append(lines, " }")
	files := []FileDiff{{Path: "cmd/report.go", Hunks: []Hunk{hunkOf(lines...)}}}
	function := FunctionRisk{ID: "main.FormatReport", Name: "FormatReport", File: "cmd/report.go", StartLine: 1, EndLine: 10,
		LinesAdded: 8, LinesRemoved: 8, Metrics: simpleMetrics(18, 29, 4, 15)}
	function.Metrics.Length = 40

	plain := []FunctionRisk{function}
	ScoreFunctions(plain, 5)
	withDiff := []FunctionRisk{function}
	ScoreFunctionsWithDiff(withDiff, files, 5)

	assert.InDelta(t, plain[0].RiskScore*MechanicalEditDiscount, withDiff[0].RiskScore, 0.2)
	assert.Contains(t, withDiff[0].RiskReasons, "mechanical edit (16 of 16 changed lines are rewrites of the same statement): complexity weighted down")
}

func TestMechanicalEditKeepsFanInAndSecurity(t *testing.T) {
	files := []FileDiff{{Path: "pkg/run.go", Hunks: []Hunk{hunkOf(" func Run() {", "-	out:=exec.Command(name)", "+	out := exec.Command(name)", " }")}}}
	functions := []FunctionRisk{{ID: "pkg.Run", Name: "Run", File: "pkg/run.go", StartLine: 1, EndLine: 4, FanIn: 15, LinesAdded: 1, LinesRemoved: 1}}
	ScoreFunctionsWithDiff(functions, files, 5)
	require.NotEmpty(t, functions[0].RiskReasons)
	assert.Equal(t, WeightFanIn+WeightSecurity, functions[0].RiskScore, "fan-in and security are not discounted, and a reformat adds no behaviour change")
}

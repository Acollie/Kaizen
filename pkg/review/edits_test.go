package review

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goShape is the comparison shape of one line of Go
func goShape(line string) lineShape {
	return shapeOf(cleanLines([]string{line}, goSyntax)[0], goSyntax)
}

func TestRewriteScore(t *testing.T) {
	testCases := []struct {
		name    string
		removed string
		added   string
		rewrite bool
	}{
		{"Sprintf to Fprintf", `sb.WriteString(fmt.Sprintf("%s: %d\n", name, count))`, `fmt.Fprintf(&sb, "%s: %d\n", name, count)`, true},
		{"Sprintf to Fprintf, one argument", `sb.WriteString(fmt.Sprintf("x %d", n))`, `fmt.Fprintf(&sb, "x %d", n)`, true},
		{"reformat", `if x==nil{`, `if x == nil {`, true},
		{"string literal change", `log.Printf("starting %s", name)`, `log.Printf("booting %s", name)`, true},
		{"comment only change", `return value // old note`, `return value // new note`, true},
		{"block comment only change", `return value /* old */`, `return value /* new */`, true},
		// A rename changes every identifier, so it is no longer treated as
		// mechanical: the identifiers must mostly match too
		{"rename", `total := sumValues(items)`, `sum := addValues(entries)`, false},
		{"comparison change", `if index > limit {`, `if index >= limit {`, false},
		{"equality flipped", `if role == admin {`, `if role != admin {`, false},
		{"boolean flipped", `enabled := false`, `enabled := true`, false},
		{"arithmetic change", `total := left + right`, `total := left - right`, false},
		{"number change", `retries := 3`, `retries := 5`, false},
		{"swapped arguments", `copyFile(source, destination)`, `copyFile(destination, source)`, false},
		{"nil to error", `return result, nil`, `return nil, err`, false},
		{"new condition", `if ok {`, `if ok && !expired {`, false},
		{"unrelated statement", `defer file.Close()`, `values = append(values, item)`, false},
		{"log becomes delete", `log.Printf(path)`, `os.RemoveAll(path)`, false},
		{"called predicate changes", `if isAdmin(user) {`, `if isGuest(user) {`, false},
		{"URL swap", `endpoint := "https://api.example.com/v1"`, `endpoint := "http://attacker.example.com/v1"`, false},
		{"URL swap in a long call", `response, err := client.Get(ctx, "https://api.example.com", options, timeout)`,
			`response, err := client.Get(ctx, "http://attacker.example.com", options, timeout)`, false},
		{"gains a side effect", `data, err := load(path)`, `data, err := os.WriteFile(path)`, false},
		{"method on another receiver", `cache.Remove(key)`, `os.Remove(key)`, false},
		{"function moved to a package", `err = openInBrowser(path)`, `err = visualization.OpenInBrowser(path)`, true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, rewrite := rewriteScore(goShape(testCase.removed), goShape(testCase.added))
			assert.Equal(t, testCase.rewrite, rewrite)
		})
	}
}

func tokenTexts(tokens []lineToken) []string {
	texts := []string{}
	for _, token := range tokens {
		texts = append(texts, token.text)
	}
	return texts
}

func TestTokenizeLineMergesQualifiedNames(t *testing.T) {
	line := cleanLines([]string{`if err := pkg.Do(a.b, "x") // trailing`}, goSyntax)[0]
	assert.Equal(t, []string{"if", "err", ":=", "pkg.do", "(", "a.b", ",", "str", ")"}, tokenTexts(tokenizeLine(line, goSyntax)))
}

func TestKeywordsUsedAsNamesAreIdentifiers(t *testing.T) {
	testCases := []struct {
		name     string
		syntax   *languageSyntax
		line     string
		keywords []string
	}{
		{"method named match", pythonSyntax, `if re.match(pattern, text):`, []string{"if"}},
		{"method named Select", goSyntax, `rows := db.Select(query)`, nil},
		{"filepath.Match", goSyntax, `ok, err := filepath.Match(pattern, name)`, nil},
		{"logger.Panic", goSyntax, `logger.Panic("boom")`, nil},
		{"Kotlin val named when", kotlinSyntax, `val when = 3`, []string{"val"}},
		{"variable named go", goSyntax, `go = 3`, nil},
		{"Go struct key", goSyntax, `config := Config{Go: true, select: 1}`, []string{"true"}},
		{"object key", javascriptSyntax, `const options = { if: 1, default: 2 }`, []string{"const", "default"}},
		{"Python else is a keyword", pythonSyntax, `else:`, []string{"else"}},
		{"Go default is a keyword", goSyntax, `default:`, []string{"default"}},
		{"Go case true is a keyword", goSyntax, `case true:`, []string{"case", "true"}},
		{"Go keywords are case-sensitive", goSyntax, `If := Go(Defer)`, nil},
		{"Python goroutine-like word is not a keyword", pythonSyntax, `defer(go)`, nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var keywords []string
			for _, token := range tokenizeLine(testCase.line, testCase.syntax) {
				if token.kind == tokenKeyword {
					keywords = append(keywords, token.value)
				}
			}
			assert.Equal(t, testCase.keywords, keywords)
		})
	}
}

func TestCommentScanner(t *testing.T) {
	testCases := []struct {
		name   string
		path   string
		lines  []string
		output []string
	}{
		{"Go line comment", "a.go", []string{`x := 1 // if this fails we panic`}, []string{`x := 1 `}},
		{"Go inline block comment", "a.go", []string{`x := /* if */ 1`}, []string{`x :=   1`}},
		{"Go block comment across lines", "a.go",
			[]string{`x := 1 /* start`, ` if this fails we panic`, ` end */ y := 2`},
			[]string{`x := 1 `, ``, ` y := 2`}},
		{"Javadoc continuation lines", "a.kt",
			[]string{`/**`, ` * If this fails we throw.`, ` */`, `fun f() {`},
			[]string{``, ``, ``, `fun f() {`}},
		{"continuation line out of context", "a.ts", []string{` * if it panics we throw`}, []string{``}},
		{"Python hash comment", "a.py", []string{`x = 1  # If this fails we panic`}, []string{`x = 1  `}},
		{"Python // is division", "a.py", []string{`half = total // 2`}, []string{`half = total // 2`}},
		{"Python docstring", "a.py",
			[]string{`    """Return the value.`, `    If this fails we raise.`, `    """`, `    return value`},
			[]string{`    "Return the value."`, `"    If this fails we raise."`, `"    "`, `    return value`}},
		{"hash in a string is kept", "a.py", []string{`tag = "#if" # note`}, []string{`tag = "#if" `}},
		{"slashes in a string are kept", "a.go", []string{`url := "https://x.io" // note`}, []string{`url := "https://x.io" `}},
		{"Go raw string across lines", "a.go",
			[]string{"prompt := `Review this.", "If this fails we panic.", "`"},
			[]string{`prompt := "Review this."`, `"If this fails we panic."`, `""`}},
		{"hunk starting inside a raw string", "a.go",
			[]string{"`, width, height))", "x := 1 // if"},
			[]string{`"", width, height))`, `x := 1 `}},
		{"lone closing backtick", "a.go", []string{"\t`", "if ok {"}, []string{"\"\t\"", "if ok {"}},
		{"shell hash comment", "a.sh", []string{`echo "$#" # if count`}, []string{`echo "$#" `}},
		{"shell ${#x} is not a comment", "a.sh", []string{`n=${#items[@]}`}, []string{`n=${#items[@]}`}},
		{"YAML comment", "ci.yml", []string{`if: always() # if failed`}, []string{`if: always() `}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.output, cleanLines(testCase.lines, syntaxFor(testCase.path, "")))
		})
	}
}

func TestSyntaxFor(t *testing.T) {
	assert.Same(t, goSyntax, syntaxFor("pkg/a.go", ""))
	assert.Same(t, pythonSyntax, syntaxFor("tool", "Python"))
	assert.Same(t, javascriptSyntax, syntaxFor("web/src/App.tsx", ""))
	assert.Same(t, shellSyntax, syntaxFor("Dockerfile", ""))
	assert.Same(t, defaultSyntax, syntaxFor("README", ""))
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
			name:       "comment-only lines are ignored",
			lines:      []string{" func A() {", "-	// old note", "+	// if this fails we panic", "-	x:=1", "+	x := 1", " }"},
			mechanical: true, rewritten: 2, changed: 2,
		},
		{
			name:       "log line becomes a delete",
			lines:      []string{" func A() {", "-	log.Printf(path)", "+	os.RemoveAll(path)", " }"},
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

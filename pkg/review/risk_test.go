package review

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/models"
)

func simpleMetrics(cyclomatic, cognitive, nesting int, maintainability float64) *models.FunctionAnalysis {
	return &models.FunctionAnalysis{
		CyclomaticComplexity: cyclomatic,
		CognitiveComplexity:  cognitive,
		NestingDepth:         nesting,
		MaintainabilityIndex: maintainability,
		HalsteadVolume:       100,
	}
}

func TestScoreFunctionsTable(t *testing.T) {
	testCases := []struct {
		name            string
		function        FunctionRisk
		minScore        float64
		maxScore        float64
		reasonFragments []string
	}{
		{
			name:     "trivial change scores near zero",
			function: FunctionRisk{ID: "app.Tiny", Name: "Tiny", File: "app/tiny.go", LinesAdded: 1, Metrics: simpleMetrics(1, 0, 1, 90)},
			minScore: 0,
			maxScore: 1,
		},
		{
			name:            "notable cyclomatic complexity",
			function:        FunctionRisk{ID: "app.Branchy", Name: "Branchy", File: "app/x.go", Metrics: simpleMetrics(12, 0, 1, 90)},
			minScore:        9,
			maxScore:        10,
			reasonFragments: []string{"cyclomatic complexity 12 (notable)"},
		},
		{
			name:            "high fan-in",
			function:        FunctionRisk{ID: "app.Shared", Name: "Shared", File: "app/x.go", FanIn: 14, Metrics: simpleMetrics(1, 0, 0, 90)},
			minScore:        18,
			maxScore:        19,
			reasonFragments: []string{"fan-in 14: changes ripple to many callers"},
		},
		{
			name:            "critical approximate fan-in",
			function:        FunctionRisk{ID: "a.py:shared", Name: "shared", File: "a.py", FanIn: 20, FanInApproximate: true},
			minScore:        20,
			maxScore:        20,
			reasonFragments: []string{"fan-in 20 (approximate): critical blast radius"},
		},
		{
			name: "everything maxed",
			function: FunctionRisk{
				ID: "auth.Login", Name: "Login", File: "auth/login.go", FanIn: 30, IsNew: true,
				LinesAdded: 100, LinesRemoved: 10,
				Metrics: &models.FunctionAnalysis{
					CyclomaticComplexity: 25, CognitiveComplexity: 40, NestingDepth: 7,
					MaintainabilityIndex: 5, HalsteadVolume: 900, Churn: &models.ChurnMetric{TotalCommits: 30},
				},
			},
			minScore: 100,
			maxScore: 100,
			reasonFragments: []string{
				"cyclomatic complexity 25 (high)", "cognitive complexity 40", "nesting depth 7",
				"fan-in 30", "churn 30 commits", "110 lines changed (+100/-10)",
				"maintainability index 5 (low)", "new function with cyclomatic complexity 25",
				"security-sensitive: name or path mentions \"login\"",
			},
		},
		{
			name:            "security keyword in camelCase name",
			function:        FunctionRisk{ID: "app.validateTokenExpiry", Name: "validateTokenExpiry", File: "app/x.go"},
			minScore:        10,
			maxScore:        10,
			reasonFragments: []string{"mentions \"token\""},
		},
		{
			name:     "author is not auth",
			function: FunctionRisk{ID: "app.AuthorName", Name: "AuthorName", File: "app/x.go"},
			minScore: 0,
			maxScore: 0,
		},
		{
			name:     "missing MI is ignored",
			function: FunctionRisk{ID: "x.swift:f", Name: "f", File: "x.swift", Metrics: &models.FunctionAnalysis{CyclomaticComplexity: 1}},
			minScore: 0,
			maxScore: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			functions := []FunctionRisk{testCase.function}
			ScoreFunctions(functions, 5)
			scored := functions[0]

			assert.GreaterOrEqual(t, scored.RiskScore, testCase.minScore)
			assert.LessOrEqual(t, scored.RiskScore, testCase.maxScore)
			assert.NotNil(t, scored.RiskReasons)
			if len(testCase.reasonFragments) == 0 {
				assert.Empty(t, scored.RiskReasons)
			}
			for _, fragment := range testCase.reasonFragments {
				assert.True(t, containsFragment(scored.RiskReasons, fragment), "missing reason %q in %v", fragment, scored.RiskReasons)
			}
		})
	}
}

func TestScoreFunctionsReasonsOrderedByContribution(t *testing.T) {
	functions := []FunctionRisk{{
		ID: "app.F", Name: "F", File: "app/f.go", FanIn: 15,
		Metrics: simpleMetrics(10, 0, 0, 90),
	}}
	ScoreFunctions(functions, 5)
	require.Len(t, functions[0].RiskReasons, 2)
	assert.Contains(t, functions[0].RiskReasons[0], "fan-in 15")
	assert.Contains(t, functions[0].RiskReasons[1], "cyclomatic complexity 10")
}

func TestScoreFunctionsWithDiffSecurityLines(t *testing.T) {
	functions := []FunctionRisk{{ID: "app.Run", Name: "Run", File: "app/run.go", StartLine: 1, EndLine: 10}}
	files := []FileDiff{{
		Path: "app/run.go",
		Hunks: []Hunk{{Lines: []DiffLine{
			{Kind: LineAdded, NewLine: 5, Text: `out, err := exec.Command("sh", "-c", input).Output()`},
			{Kind: LineAdded, NewLine: 20, Text: "outside the function"},
		}}},
	}}

	ScoreFunctionsWithDiff(functions, files, 5)
	// security in full, plus the behaviour-change factor for a new side effect (2 units)
	assert.Equal(t, WeightSecurity+WeightBehaviourChange*2/BehaviourChangeFullUnits, functions[0].RiskScore)
	assert.Contains(t, functions[0].RiskReasons[0], "exec.Command")
	assert.Contains(t, functions[0].RiskReasons, "behaviour change: adds an exec.Command")

	// The same pattern outside the function's range does not count
	files[0].Hunks[0].Lines[0].NewLine = 50
	ScoreFunctionsWithDiff(functions, files, 5)
	assert.Equal(t, 0.0, functions[0].RiskScore)
}

func TestScoreFunctionsFocus(t *testing.T) {
	functions := []FunctionRisk{
		{ID: "a", FanIn: 15},                            // 20
		{ID: "b", FanIn: 8},                             // 10
		{ID: "c", FanIn: 15},                            // 20, ties with a
		{ID: "d", Metrics: simpleMetrics(20, 0, 0, 90)}, // 20
		{ID: "e"}, // 0
	}

	ScoreFunctions(functions, 2)
	focused := []string{}
	for _, function := range functions {
		if function.Focus {
			focused = append(focused, function.ID)
		}
	}
	// Ties are broken by ID, and b (score 10) is below MinFocusScore anyway
	assert.Equal(t, []string{"a", "c"}, focused)

	ScoreFunctions(functions, 0) // default focus count
	focusCount := 0
	for _, function := range functions {
		if function.Focus {
			focusCount++
		}
	}
	assert.Equal(t, 3, focusCount, "only scores >= MinFocusScore are focused")
}

func TestSplitIdentifierWords(t *testing.T) {
	assert.Equal(t, []string{"parse", "json", "token"}, splitIdentifierWords("parseJSONToken"))
	assert.Equal(t, []string{"json", "parser"}, splitIdentifierWords("JSONParser"))
	assert.Equal(t, []string{"pkg", "auth", "login", "go"}, splitIdentifierWords("pkg/auth/login.go"))
	assert.Equal(t, []string{"get", "user", "by", "id"}, splitIdentifierWords("get_user_by_id"))
}

func TestWeightsSumToHundred(t *testing.T) {
	total := WeightCyclomatic + WeightCognitive + WeightNesting + WeightFanIn + WeightChurn +
		WeightLinesChanged + WeightLowMaintainability + WeightNewComplex + WeightSecurity
	assert.InDelta(t, 100.0, total, 0.001)
}

func containsFragment(reasons []string, fragment string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, fragment) {
			return true
		}
	}
	return false
}

func TestIsTestFile(t *testing.T) {
	cases := map[string]bool{
		"internal/config/validation_test.go":                      true,
		"pkg/app/test_views.py":                                   true,
		"pkg/app/views_test.py":                                   true,
		"app/src/CartTest.kt":                                     true,
		"Sources/CartTests.swift":                                 true,
		"tests/fixtures/helpers.go":                               true,
		"pkg/reviewui/testdata/worktree/pkg/analyzer/pipeline.go": true,
		"web/src/fixtures/review.ts":                              true,
		"web/src/__fixtures__/pr.json":                            true,
		"web/src/__snapshots__/App.test.tsx.snap":                 true,
		"pkg/fixture/loader.go":                                   false,
		"pkg/testdatagen/gen.go":                                  false,
		"internal/config/config.go":                               false,
		"pkg/testing/assert.go":                                   false,
		"cmd/contest/main.go":                                     false,
	}
	for path, expected := range cases {
		assert.Equal(t, expected, IsTestFile(path), path)
	}
}

func TestScoreFunctionsWeightsDownTestCode(t *testing.T) {
	functions := []FunctionRisk{
		{ID: "prod", File: "internal/config/config.go", LinesAdded: 166, Metrics: &models.FunctionAnalysis{CyclomaticComplexity: 6}},
		{ID: "test", File: "internal/config/config_test.go", LinesAdded: 166, Metrics: &models.FunctionAnalysis{CyclomaticComplexity: 6}},
	}
	ScoreFunctions(functions, 5)

	assert.Less(t, functions[1].RiskScore, functions[0].RiskScore)
	assert.Contains(t, functions[1].RiskReasons[len(functions[1].RiskReasons)-1], "test code")
}

func TestAssignFocusGuaranteesTopFewModerateFunctions(t *testing.T) {
	functions := []FunctionRisk{
		{ID: "high", RiskScore: 21.8},
		{ID: "moderate", RiskScore: 14.7},
		{ID: "lower", RiskScore: 9.4},
		{ID: "fourth", RiskScore: 7.7},
		{ID: "trivial", RiskScore: 3.0},
	}
	assignFocus(functions, 5)

	focused := map[string]bool{}
	for _, function := range functions {
		focused[function.ID] = function.Focus
	}
	assert.Equal(t, map[string]bool{"high": true, "moderate": true, "lower": true, "fourth": false, "trivial": false}, focused)
}

func TestScoreFileRanksNonCodeFiles(t *testing.T) {
	workflow, workflowReasons := ScoreFile(FileDiff{Path: ".github/workflows/ci.yml", Additions: 4}, false)
	action, _ := ScoreFile(FileDiff{Path: "action.yml", Additions: 120, Deletions: 30}, false)
	script, _ := ScoreFile(FileDiff{Path: "scripts/release.sh", Additions: 5}, false)
	manifest, _ := ScoreFile(FileDiff{Path: "go.mod", Additions: 2}, false)
	readme, readmeReasons := ScoreFile(FileDiff{Path: "README.md", Additions: 3}, false)
	source, _ := ScoreFile(FileDiff{Path: "pkg/app/app.go", Additions: 400}, true)

	assert.Equal(t, FileRiskWorkflow, workflow)
	assert.Contains(t, workflowReasons[0], "CI workflow")
	assert.Greater(t, action, FileRiskWorkflow, "large unanalysable change adds size risk")
	assert.Equal(t, FileRiskScript, script)
	assert.Equal(t, FileRiskDependencies, manifest)
	attributes, attributeReasons := ScoreFile(FileDiff{Path: ".gitattributes", Additions: 1}, false)
	assert.Equal(t, FileRiskGitattributes, attributes)
	assert.Contains(t, attributeReasons[0], "-diff")
	assert.Zero(t, readme)
	assert.Empty(t, readmeReasons)
	assert.Zero(t, source, "source files with functions are scored through their functions")
}

func complexMetrics() *models.FunctionAnalysis {
	return &models.FunctionAnalysis{
		Length: 94, CyclomaticComplexity: 17, CognitiveComplexity: 20, NestingDepth: 3,
		MaintainabilityIndex: 30, HalsteadVolume: 100,
	}
}

func TestScoreFunctionsScalesComplexityByChangeShare(t *testing.T) {
	testCases := []struct {
		name          string
		linesAdded    int
		linesRemoved  int
		isNew         bool
		expectedScore float64
		smallEdit     bool
	}{
		{name: "3-line touch drops to a quarter", linesAdded: 3, linesRemoved: 3, expectedScore: 6.6, smallEdit: true},
		{name: "partial edit scales proportionally", linesAdded: 12, linesRemoved: 8, expectedScore: 13.0, smallEdit: true},
		{name: "large edit keeps full complexity", linesAdded: 50, linesRemoved: 10, expectedScore: 33.1},
		{name: "new function keeps full complexity", linesAdded: 3, isNew: true, expectedScore: 29.4},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			functions := []FunctionRisk{{
				ID: "app.FormatReport", Name: "FormatReport", File: "app/report.go", IsNew: testCase.isNew,
				LinesAdded: testCase.linesAdded, LinesRemoved: testCase.linesRemoved, Metrics: complexMetrics(),
			}}
			ScoreFunctions(functions, 5)

			assert.InDelta(t, testCase.expectedScore, functions[0].RiskScore, 0.15)
			changed := testCase.linesAdded + testCase.linesRemoved
			reason := fmt.Sprintf("small edit (%d of 94 lines): complexity weighted down", changed)
			assert.Equal(t, testCase.smallEdit, containsFragment(functions[0].RiskReasons, reason), "%v", functions[0].RiskReasons)
		})
	}
}

func TestChangeShareDoesNotScaleFanInOrSecurity(t *testing.T) {
	functions := []FunctionRisk{{
		ID: "app.Run", Name: "Run", File: "app/run.go", FanIn: 15, LinesAdded: 1,
		Metrics: &models.FunctionAnalysis{Length: 100, CyclomaticComplexity: 1},
	}}
	ScoreFunctions(functions, 5)
	assert.Equal(t, WeightFanIn, functions[0].RiskScore)
	assert.False(t, containsFragment(functions[0].RiskReasons, "small edit"), "no intrinsic risk, nothing to weight down")
}

func TestSecurityLinePatterns(t *testing.T) {
	testCases := []struct {
		name    string
		line    string
		pattern string
	}{
		{"filepath.Join is not security-sensitive", `path := filepath.Join(root, name)`, ""},
		{"word in a description string", `return 0, []string{"runs with repository secrets and write tokens"}`, ""},
		{"word in a raw string", "help := `paste your secret token here`", ""},
		{"word in a comment", `count++ // the secret sauce`, ""},
		{"escaped quote stays inside the literal", `message := "say \"secret\" twice"`, ""},
		{"process execution", `command := exec.CommandContext(ctx, name, args...)`, "exec.CommandContext"},
		{"syscall exec", `return syscall.Exec(binary, args, env)`, "syscall.Exec"},
		{"outbound HTTP post", `response, err := http.Post(url, "application/json", body)`, "http.Post"},
		{"outbound request", `request, err := http.NewRequestWithContext(ctx, "POST", url, nil)`, "http.NewRequest"},
		{"client Do", `response, err := client.Do(request)`, ".Do( on an HTTP client"},
		{"http.DefaultClient Do", `response, err := http.DefaultClient.Do(request)`, ".Do( on an HTTP client"},
		{"once.Do is not HTTP", `once.Do(setup)`, ""},
		{"gh api through a runner", `runner.Run(ctx, dir, "gh", "api", endpoint, "--method", "POST")`, `"gh", "api"`},
		{"gh api in a shell string", `script := "gh api repos/x --method POST"`, "gh api"},
		{"listener", `listener, err := net.Listen("tcp", address)`, "net.Listen"},
		{"server", `log.Fatal(http.ListenAndServe(":8080", nil))`, "ListenAndServe"},
		{"file write", `err := os.WriteFile(path, data, 0o600)`, "os.WriteFile"},
		{"recursive delete", `defer os.RemoveAll(directory)`, "os.RemoveAll"},
		{"environment", `os.Setenv("HOME", home)`, "os.Setenv"},
		{"trusted HTML", `return template.HTML(rendered)`, "template.HTML"},
		{"React raw HTML", `<div dangerouslySetInnerHTML={{ __html: body }} />`, "dangerouslySetInnerHTML"},
		{"DOM raw HTML", `element.innerHTML = body;`, "innerHTML"},
		{"eval", `const result = eval(source);`, "eval("},
		{"retrieval( is not eval", `const result = retrieval(source);`, ""},
		{"import path", `import "os/exec"`, "os/exec"},
		{"AI subprocess permissions", `"--permission-mode", "dontAsk",`, "--permission-mode"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.pattern, firstSecurityPattern(testCase.line))
		})
	}
}

func TestScoreFunctionsSecurityReasonNamesTheToken(t *testing.T) {
	functions := []FunctionRisk{{ID: "github.Client.PostReview", Name: "PostReview", File: "pkg/github/post.go", StartLine: 1, EndLine: 5}}
	files := []FileDiff{{Path: "pkg/github/post.go", Hunks: []Hunk{{Lines: []DiffLine{
		{Kind: LineAdded, NewLine: 3, Text: `output, err := client.runner().RunWithInput(ctx, dir, input, "gh", "api", endpoint, "--method", "POST")`},
	}}}}}
	ScoreFunctionsWithDiff(functions, files, 5)
	assert.Equal(t, WeightSecurity, functions[0].RiskScore)
	assert.Equal(t, []string{`security-sensitive: changed lines use "\"gh\", \"api\""`}, functions[0].RiskReasons)
}

func TestFanInReasonCountsCallSitesAndCallers(t *testing.T) {
	functions := []FunctionRisk{
		{ID: "a", FanIn: 36, CallerCount: 11},
		{ID: "b", FanIn: 6, CallerCount: 1, FanInApproximate: true},
	}
	ScoreFunctions(functions, 5)
	assert.Contains(t, functions[0].RiskReasons, "fan-in 36 call sites from 11 callers: critical blast radius, many callers depend on this")
	assert.Contains(t, functions[1].RiskReasons, "fan-in 6 call sites from 1 caller (approximate): changes ripple to many callers")
}

func TestScoreFileFlagsAgentInstructionFiles(t *testing.T) {
	paths := []string{
		"CLAUDE.md", "pkg/review/CLAUDE.md", "AGENTS.md", ".cursorrules",
		".github/copilot-instructions.md", ".claude/settings.json", ".claude/agents/reviewer.md",
	}
	for _, path := range paths {
		score, reasons := ScoreFile(FileDiff{Path: path, Additions: 2}, false)
		assert.Equal(t, FileRiskAgentInstructions, score, path)
		require.NotEmpty(t, reasons, path)
		assert.Equal(t, "AI agent instructions: can steer AI tools (including this review); read it yourself", reasons[0], path)
	}

	docs, _ := ScoreFile(FileDiff{Path: "docs/claude-integration.md", Additions: 2}, false)
	assert.Zero(t, docs)
}

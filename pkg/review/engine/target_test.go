package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolTarget(t *testing.T) {
	reviewContext := liveContext(t)
	toolbox := tools.New(reviewContext)
	worktree := reviewContext.WorktreePath
	cases := []struct {
		name, input, target, kind string
	}{
		{"mcp__kaizen__get_function", `{"id":"example.com/calc.Add"}`, "example.com/calc.Add", review.TargetFunction},
		{"mcp__kaizen__get_callers", `{"id":"Add"}`, "example.com/calc.Add", review.TargetFunction},
		{"get_callees", `{"id":"other.Fn"}`, "other.Fn", review.TargetFunction},
		{"mcp__kaizen__get_file_diff", `{"path":"./calc/calc.go"}`, "calc/calc.go", review.TargetFile},
		{"read_lines", `{"path":"docs/a.md","start":1,"end":3}`, "docs/a.md", review.TargetFile},
		{"Read", `{"file_path":"` + filepath.Join(worktree, "calc", "calc.go") + `"}`, "calc/calc.go", review.TargetFile},
		{"Read", `{"file_path":"calc/calc.go"}`, "calc/calc.go", review.TargetFile},
		{"Read", `{"file_path":"/etc/hosts"}`, "", ""},
		{"Read", `{"file_path":"../outside.go"}`, "", ""},
		{"Grep", `{"pattern":"x","path":"` + worktree + `"}`, "", ""},
		{"Glob", `{"pattern":"**/*.go"}`, "", ""},
		{"mcp__kaizen__report_finding", `{"file":"calc/calc.go"}`, "", ""},
		{"mcp__kaizen__get_function", `{"id":""}`, "", ""},
		{"mcp__kaizen__get_function", `{"id":`, "", ""},
	}
	for _, testCase := range cases {
		target, kind := toolTarget(testCase.name, json.RawMessage(testCase.input), worktree, toolbox)
		assert.Equal(t, testCase.target, target, testCase.name+" "+testCase.input)
		assert.Equal(t, testCase.kind, kind, testCase.name+" "+testCase.input)
	}
	target, _ := toolTarget("get_callers", json.RawMessage(`{"id":"Add"}`), worktree, nil)
	assert.Equal(t, "Add", target, "without a toolbox the id is passed through")
}

func TestSummarizeReadRange(t *testing.T) {
	assert.Equal(t, "Read(a.go:4-13)", summarizeToolCall("Read", json.RawMessage(`{"file_path":"/w/a.go","offset":4,"limit":10}`), "/w"))
	assert.Equal(t, "Read(a.go:1-20)", summarizeToolCall("Read", json.RawMessage(`{"file_path":"/w/a.go","limit":20}`), "/w"))
	assert.Equal(t, "Read(a.go:30-)", summarizeToolCall("Read", json.RawMessage(`{"file_path":"/w/a.go","offset":30}`), "/w"))
}

// targetsFixture writes subagent_targets.jsonl with the worktree path filled in
func targetsFixture(t *testing.T, worktree string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "subagent_targets.jsonl"))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "targets.jsonl")
	filled := strings.ReplaceAll(string(content), "__WORKTREE__", worktree)
	require.NoError(t, os.WriteFile(path, []byte(filled), 0o600))
	return path
}

func TestCLIEngineSubagentTargetsAndLiveNormalisation(t *testing.T) {
	reviewContext := liveContext(t)
	reviewContext.Files[0].OldPath = "calc/old_calc.go"
	reviewContext.Files[0].Status = review.FileRenamed
	engine := fakeEngine(t, "")
	t.Setenv(fakeClaudeFixture, targetsFixture(t, reviewContext.WorktreePath))
	recorder := &eventRecorder{}
	_, err := engine.Run(context.Background(), reviewContext, recorder.emit)
	require.NoError(t, err)

	byID := map[string]review.ToolCallData{}
	var order []review.ToolCallData
	for _, event := range recorder.ofType(review.EventToolCall) {
		call := event.Data.(review.ToolCallData)
		order = append(order, call)
		byID[call.Summary] = call
	}
	expect := map[string][3]string{ // summary -> agent, target, kind
		"get_risk_ranking()":                                {"lead", "", ""},
		"get_function(Add)":                                 {"hotspot-reviewer", "example.com/calc.Add", review.TargetFunction},
		"get_callers(example.com/calc.Add)":                 {"hotspot-reviewer", "example.com/calc.Add", review.TargetFunction},
		"get_callees(example.com/calc.Unchanged)":           {"hotspot-reviewer", "example.com/calc.Unchanged", review.TargetFunction},
		"get_file_diff(./calc/old_calc.go)":                 {"hotspot-reviewer", "calc/calc.go", review.TargetFile},
		"read_lines(calc/calc_test.go:1-40)":                {"hotspot-reviewer", "calc/calc_test.go", review.TargetFile},
		"Read(calc/calc.go:4-13)":                           {"hotspot-reviewer", "calc/calc.go", review.TargetFile},
		"Read(/etc/hosts)":                                  {"hotspot-reviewer", "", ""},
		`Glob("**/*.go")`:                                   {"hotspot-reviewer", "", ""},
		"report_finding(warning ./calc/calc.go:5 Dot path)": {"hotspot-reviewer", "", ""},
	}
	for summary, want := range expect {
		call, ok := byID[summary]
		require.True(t, ok, "missing tool call %s in %v", summary, order)
		assert.Equal(t, want[0], call.Agent, summary)
		assert.Equal(t, want[1], call.Target, summary)
		assert.Equal(t, want[2], call.TargetKind, summary)
	}
	for _, call := range order {
		if call.Tool == "Grep" {
			assert.Empty(t, call.Target)
		}
	}

	// Live findings are canonicalised like the MCP tool does, so they land inline and dedup
	live := recorder.ofType(review.EventFinding)
	require.Len(t, live, 2, "the third report duplicates the first once its path is canonical")
	first := live[0].Data.(review.Finding)
	second := live[1].Data.(review.Finding)
	assert.Equal(t, "calc/calc.go", first.File)
	assert.Zero(t, first.EndLine)
	assert.Equal(t, "Open calc/calc.go:5", first.Verify)
	assert.Equal(t, "calc/calc.go", second.File, "a renamed file's old path maps to the new path")
	assert.InDelta(t, 0.4, second.Confidence, 1e-9)

	statuses := messagesOf(recorder.ofType(review.EventStatus))
	assert.Contains(t, statuses, "Claude Code (credentials: ANTHROPIC_API_KEY)")
	assertContainsPrefix(t, statuses, "Warning: Claude Code is using an API key (ANTHROPIC_API_KEY)")
}

func TestClaudeEnvironmentStripsAPICredentials(t *testing.T) {
	environ := []string{
		"HOME=/Users/dev", "PATH=/usr/bin", "ANTHROPIC_API_KEY=sk-ant-x", "ANTHROPIC_AUTH_TOKEN=tok",
		"ANTHROPIC_BASE_URL=https://proxy", "ANTHROPIC_MODEL=opus", "CLAUDE_CONFIG_DIR=/c", "EMPTY=",
	}
	got := claudeEnvironment(environ)
	assert.Equal(t, []string{"HOME=/Users/dev", "PATH=/usr/bin", "ANTHROPIC_MODEL=opus", "CLAUDE_CONFIG_DIR=/c", "EMPTY="}, got)
	for _, entry := range got {
		name, _, _ := strings.Cut(entry, "=")
		assert.NotContains(t, cliStrippedEnv, name)
	}
}

func TestCLIEngineDoesNotPassAPIKeyToClaude(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-should-not-leak")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "should-not-leak")
	engine := fakeEngine(t, "subagent_findings.jsonl")
	envPath := filepath.Join(t.TempDir(), "env.txt")
	t.Setenv(fakeClaudeRecordEnv, envPath)
	_, err := engine.Run(context.Background(), liveContext(t), nil)
	require.NoError(t, err)
	content, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "should-not-leak")
	assert.Contains(t, string(content), "HOME=")
}

func TestReportCredentials(t *testing.T) {
	cases := map[string]bool{"none": false, "/login managed key": false, "ANTHROPIC_API_KEY": true, "apiKeyHelper": true}
	for source, warns := range cases {
		recorder := &eventRecorder{}
		parser := newStreamParser(recorder.emit, "")
		parser.reportCredentials(source)
		statuses := messagesOf(recorder.ofType(review.EventStatus))
		assert.Equal(t, warns, len(statuses) == 2, source)
	}
	recorder := &eventRecorder{}
	newStreamParser(recorder.emit, "").reportCredentials("")
	assert.Empty(t, recorder.snapshot())
}

func TestBudgetAndAuthErrors(t *testing.T) {
	err := missingOutputError(&streamResult{Subtype: "error_max_budget_usd", IsError: true, CostUSD: 10.02})
	assert.ErrorContains(t, err, "spending cap")
	assert.ErrorContains(t, err, "$10.02")

	err = missingOutputError(&streamResult{Subtype: "success", IsError: true, Text: "Invalid API key · Please run /login"})
	assert.ErrorContains(t, err, `Run "claude"`)
	assert.NotContains(t, err.Error(), "export ANTHROPIC_API_KEY")

	exit := errors.New("exit status 1")
	assert.ErrorContains(t, claudeExitError(exit, "Not logged in · Please run /login", nil), "log in")
	assert.ErrorContains(t, claudeExitError(exit, "", &streamResult{Subtype: "error_max_budget_usd", IsError: true}), "spending cap")
	plain := claudeExitError(exit, "boom", nil)
	assert.ErrorContains(t, plain, "claude exited with an error")
	assert.ErrorContains(t, plain, "boom")

	recorder := &eventRecorder{}
	parser := newStreamParser(recorder.emit, "")
	parser.handleLine([]byte(`{"type":"result","subtype":"error_max_budget_usd","is_error":true,"total_cost_usd":10.5}`))
	assertContainsPrefix(t, messagesOf(recorder.ofType(review.EventStatus)), "Review stopped: reached the spending cap")
}

func TestCLIEngineDefaultBudget(t *testing.T) {
	config, err := (&CLIEngine{ClaudePath: "/bin/true", KaizenPath: "/bin/true"}).resolved()
	require.NoError(t, err)
	assert.InDelta(t, DefaultMaxBudgetUSD, config.MaxBudgetUSD, 1e-9)
	args, err := config.buildArgs("/tmp/context.json", review.PullRequest{Number: 1})
	require.NoError(t, err)
	assertFlag(t, args, "--max-budget-usd", "10")
}

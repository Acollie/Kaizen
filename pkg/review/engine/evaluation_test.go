package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
)

// Tests for the changes driven by the blind evaluation rounds: the evasion
// and sweep reviewers, the confidence scale, and the defects the judges found.

func TestEvasionReviewerIsDefinedAndAlwaysDispatched(t *testing.T) {
	definition, ok := subagentDefinitions("opus", "")[agentEvasion]
	require.True(t, ok)
	assert.Equal(t, subagentTools, definition.Tools)
	for _, phrase := range []string{"hostile PR author", "cache keys", "is safe / generated / minified / vendored / test / mechanical / trivial", "construction works on paper", "cite the lines", "config or environment inheritance"} {
		assert.Contains(t, definition.Prompt, phrase)
	}
	lead := leadPrompt(review.PullRequest{Number: 4})
	assert.Contains(t, lead, "One "+agentEvasion+", on every review")
	assert.Contains(t, apiSystemPrompt, "Evasion pass, on every review")
	assert.Contains(t, apiSystemPrompt, "construction works on paper")
}

func TestSecurityReviewerChecksInheritedEnvironment(t *testing.T) {
	prompt := subagentDefinitions("opus", "")[agentSecurity].Prompt
	for _, phrase := range []string{"git hooks", "core.hooksPath", "credential helpers", "environment variables", "Decisions made from attacker-controlled content"} {
		assert.Contains(t, prompt, phrase)
	}
}

func TestCoverageSweep(t *testing.T) {
	for name, definition := range subagentDefinitions("opus", "") {
		assert.Contains(t, definition.Prompt, `"FILES READ:"`, name)
	}
	lead := leadPrompt(review.PullRequest{Number: 4})
	assert.Contains(t, lead, "dispatch ONE "+agentSweep)
	assert.Contains(t, lead, "at most "+strconv.Itoa(maxSweepFiles))
	assert.Contains(t, lead, "If nothing meaningful is uncovered, do not dispatch it")
	assert.Contains(t, lead, "which meaningful changed files no reviewer read")
	assert.NotContains(t, lead, "Look briefly at anything")
	assert.Contains(t, sweepPrompt, "(at most "+strconv.Itoa(maxSweepFiles)+")")
}

func TestWorkerModelAppliesToBreadthReviewersOnly(t *testing.T) {
	definitions := subagentDefinitions("opus", "sonnet")
	for _, name := range []string{agentPackage, agentSweep, agentEvasion} {
		assert.Equal(t, "sonnet", definitions[name].Model, name)
	}
	for _, name := range []string{agentHotspot, agentSecurity, agentImpact} {
		assert.Equal(t, "opus", definitions[name].Model, name)
	}
	for name, definition := range subagentDefinitions("opus", "") {
		assert.Equal(t, "opus", definition.Model, name)
	}

	config, err := (&CLIEngine{ClaudePath: "/bin/true", KaizenPath: "/bin/true", WorkerModel: "sonnet"}).resolved()
	require.NoError(t, err)
	args, err := config.buildArgs("/tmp/context.json", review.PullRequest{Number: 1})
	require.NoError(t, err)
	assert.Contains(t, flagValue(args, "--agents"), `"model":"sonnet"`)
	assertFlag(t, args, "--model", "opus")
}

func TestNonZeroExitKeepsStructuredReview(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "subagent_findings.jsonl"))
	require.NoError(t, err)
	fixture := filepath.Join(t.TempDir(), "budget.jsonl")
	require.NoError(t, os.WriteFile(fixture, source, 0o600))
	engine := fakeEngine(t, "")
	t.Setenv(fakeClaudeFixture, fixture)
	t.Setenv(fakeClaudeExit, "1")
	t.Setenv(fakeClaudeStderr, "Error: exceeded --max-budget-usd")

	recorder := &eventRecorder{}
	result, err := engine.Run(context.Background(), liveContext(t), recorder.emit)
	require.NoError(t, err)
	assert.Equal(t, review.VerdictRequestChanges, result.Verdict)
	require.Len(t, result.Findings, 2)
	statuses := messagesOf(recorder.ofType(review.EventStatus))
	assertContainsPrefix(t, statuses, "Warning: claude exited with an error after producing the review")
	assert.Len(t, recorder.ofType(review.EventResult), 1)
}

func TestNonZeroExitWithoutStructuredReviewFails(t *testing.T) {
	engine := fakeEngine(t, "")
	t.Setenv(fakeClaudeExit, "1")
	_, err := engine.Run(context.Background(), liveContext(t), nil)
	assert.ErrorContains(t, err, "claude exited with an error")
}

func TestRestatedFindingReplacesAndReEmits(t *testing.T) {
	set := newFindingSet()
	assert.True(t, set.add(liveFinding("f-1", 5, "Add subtracts")))
	restated := liveFinding("f-1", 6, "Add subtracts instead of adding")
	restated.Severity = review.SeverityCritical
	assert.True(t, set.add(restated), "a restated finding is a change")
	require.Len(t, set.list(), 1)
	assert.Equal(t, restated, set.list()[0])
	// The new location is the dedup key now; the old one is free
	assert.False(t, set.add(liveFinding("f-2", 6, "add subtracts INSTEAD of adding")))
	assert.True(t, set.add(liveFinding("f-3", 5, "Add subtracts")))

	recorder := &eventRecorder{}
	parser := newStreamParser(recorder.emit, "")
	for _, body := range []string{"first", "second"} {
		parser.acceptFinding(pendingFinding{finding: review.Finding{File: "calc/calc.go", Line: 5, Title: "T", Body: body}, agent: "hotspot-reviewer"},
			contentBlock{Content: []byte(`"recorded f-9"`)})
	}
	live := recorder.ofType(review.EventFinding)
	require.Len(t, live, 2, "the restatement is emitted live")
	assert.Equal(t, "second", live[1].Data.(review.Finding).Body)
}

func TestExitSignalsIgnoreModelProse(t *testing.T) {
	exit := errors.New("exit status 1")
	prose := &streamResult{Subtype: "success", Text: "The budget check returns (401) when the token expires."}
	err := claudeExitError(exit, "boom", prose)
	assert.ErrorContains(t, err, "claude exited with an error")
	assert.NotContains(t, err.Error(), "spending cap")
	assert.NotContains(t, err.Error(), "log in")
	assert.False(t, isBudgetStop(prose))

	budget := claudeExitError(exit, "stopped: --max-budget-usd reached", nil)
	assert.ErrorContains(t, budget, "spending cap")
	assert.ErrorContains(t, budget, "claude stderr:\nstopped: --max-budget-usd reached", "the stderr tail is kept")
	assert.ErrorContains(t, claudeExitError(exit, "", &streamResult{Subtype: "error_max_budget_usd", IsError: true}), "spending cap")
	assert.ErrorContains(t, claudeExitError(exit, "", &streamResult{Subtype: "success", IsError: true, Text: "Invalid API key · Please run /login"}), "log in")

	recorder := &eventRecorder{}
	parser := newStreamParser(recorder.emit, "")
	parser.handleLine([]byte(`{"type":"result","subtype":"success","result":"Raise the budget in config.go","total_cost_usd":1}`))
	assert.Empty(t, recorder.ofType(review.EventStatus))
}

func TestSelectValidatesEffortAndBudget(t *testing.T) {
	options := SelectOptions{LookPath: fakeLookPath(true), Getenv: fakeGetenv(nil)}
	options.CLI.Effort = "hihg"
	_, err := Select(SelectCLI, options)
	assert.ErrorContains(t, err, `unknown effort "hihg"`)

	options.CLI.Effort = "XHigh"
	options.CLI.MaxBudgetUSD = -1
	_, err = Select(SelectCLI, options)
	assert.ErrorContains(t, err, "must not be negative")

	options.CLI.MaxBudgetUSD = 0
	selected, err := Select(SelectCLI, options)
	require.NoError(t, err)
	config, err := selected.(*CLIEngine).resolved()
	require.NoError(t, err)
	assert.Equal(t, "xhigh", config.Effort)
	assert.InDelta(t, DefaultMaxBudgetUSD, config.MaxBudgetUSD, 1e-9)

	_, err = (&CLIEngine{ClaudePath: "/bin/true", KaizenPath: "/bin/true", MaxBudgetUSD: -2}).resolved()
	assert.ErrorContains(t, err, "must not be negative")
}

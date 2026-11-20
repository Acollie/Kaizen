package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

func TestToolDefinitionsWireFormat(t *testing.T) {
	definitions := toolDefinitions()
	require.Len(t, definitions, len(tools.Names())+1)
	names := map[string]bool{}
	for _, definition := range definitions {
		require.NotNil(t, definition.OfTool)
		names[definition.OfTool.Name] = true
		encoded, err := json.Marshal(definition)
		require.NoError(t, err)
		var wire map[string]any
		require.NoError(t, json.Unmarshal(encoded, &wire))
		assert.Equal(t, true, wire["strict"], definition.OfTool.Name)
		assert.Equal(t, true, wire["eager_input_streaming"], definition.OfTool.Name)
		assert.NotEmpty(t, wire["description"])
		schema := wire["input_schema"].(map[string]any)
		assert.Equal(t, "object", schema["type"])
		assert.Equal(t, false, schema["additionalProperties"], definition.OfTool.Name)
	}
	assert.True(t, names[toolSubmitReview])
	assert.True(t, names[tools.ToolReportFinding])
}

func TestRunnerParams(t *testing.T) {
	engine := (&APIEngine{}).resolved()
	params := engine.runnerParams(review.PullRequest{Number: 3}, engine.MaxIterations)
	assert.Equal(t, defaultAPIMaxIterations, params.MaxIterations)
	assert.True(t, params.DisableThread)
	assert.Equal(t, []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}, params.Betas)

	encoded, err := json.Marshal(params.BetaMessageNewParams)
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))
	assert.Equal(t, "claude-opus-5-5", wire["model"])
	assert.Equal(t, map[string]any{"type": "adaptive", "display": "summarized"}, wire["thinking"])
	assert.Equal(t, map[string]any{"effort": "high"}, wire["output_config"])
	assert.Equal(t, map[string]any{"type": "auto"}, wire["tool_choice"])
	assert.Equal(t, "default", wire["fallbacks"])
	system := wire["system"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{"type": "ephemeral"}, system["cache_control"])
	assert.Contains(t, system["text"], "submit_review")
}

func TestAPIToolHandlersValidateAndRecord(t *testing.T) {
	recorder := &eventRecorder{}
	run := newAPIRun(liveContext(t), recorder.emit)
	betaTools, err := run.buildTools()
	require.NoError(t, err)
	byName := map[string]anthropic.BetaTool{}
	for _, betaTool := range betaTools {
		byName[betaTool.Name()] = betaTool
	}
	ctx := context.Background()

	// Truncated or invalid input never reaches the tool
	_, err = byName[tools.ToolReadLines].Execute(ctx, json.RawMessage(`{"path":"calc/calc.go","start":1`))
	assert.ErrorContains(t, err, "not valid JSON")
	_, err = byName[tools.ToolReportFinding].Execute(ctx, json.RawMessage(`{"severity":"critical"}`))
	assert.ErrorContains(t, err, "missing required field")

	content, err := byName[tools.ToolGetFunction].Execute(ctx, json.RawMessage(`{"id":"example.com/calc.Add"}`))
	require.NoError(t, err)
	require.Len(t, content, 1)
	assert.Contains(t, content[0].OfText.Text, "return a - b")

	finding := `{"severity":"critical","category":"correctness","file":"calc/calc.go","line":5,"title":"Add subtracts","body":"Total is negated.","confidence":0.9}`
	content, err = byName[tools.ToolReportFinding].Execute(ctx, json.RawMessage(finding))
	require.NoError(t, err)
	assert.Contains(t, content[0].OfText.Text, "recorded f-")
	live := recorder.ofType(review.EventFinding)
	require.Len(t, live, 1)
	assert.Equal(t, "lead", live[0].Data.(review.Finding).Source)

	calls := recorder.ofType(review.EventToolCall)
	assert.Equal(t, "get_function(example.com/calc.Add)", calls[2].Data.(review.ToolCallData).Summary)

	_, err = byName[toolSubmitReview].Execute(ctx, json.RawMessage(`{"summary":"s","verdict":"maybe","findings":[]}`))
	assert.ErrorContains(t, err, "must be one of")
	assert.Nil(t, run.submission())

	submit := `{"summary":"Broken add.","verdict":"request_changes","findings":[` + finding + `]}`
	_, err = byName[toolSubmitReview].Execute(ctx, json.RawMessage(submit))
	require.NoError(t, err)

	result, err := run.finish("claude-opus-5-5")
	require.NoError(t, err)
	assert.Equal(t, APIEngineName, result.Engine)
	assert.Equal(t, review.VerdictRequestChanges, result.Verdict)
	require.Len(t, result.Findings, 1, "the submitted copy and the live finding are the same finding")
	assert.Equal(t, live[0].Data.(review.Finding).ID, result.Findings[0].ID)
	assert.Len(t, recorder.ofType(review.EventResult), 1)
}

func TestAPIFinishWithoutSubmission(t *testing.T) {
	run := newAPIRun(liveContext(t), nil)
	_, err := run.finish("m")
	assert.ErrorContains(t, err, "without calling submit_review")
}

func TestCheckStop(t *testing.T) {
	run := newAPIRun(liveContext(t), nil)
	refusal := &anthropic.BetaMessage{StopReason: anthropic.BetaStopReasonRefusal}
	refusal.StopDetails.Category = "cyber"
	refusal.StopDetails.Explanation = "declined"
	assert.ErrorContains(t, run.checkStop(refusal), `declined to continue the review (category "cyber"): declined`)
	assert.ErrorContains(t, run.checkStop(&anthropic.BetaMessage{StopReason: anthropic.BetaStopReasonMaxTokens}), "ran out of output tokens")
	assert.NoError(t, run.checkStop(&anthropic.BetaMessage{StopReason: anthropic.BetaStopReasonEndTurn}))
	assert.NoError(t, run.checkStop(nil))

	run.submitted = &ReviewOutput{Verdict: "approve"}
	assert.NoError(t, run.checkStop(&anthropic.BetaMessage{StopReason: anthropic.BetaStopReasonMaxTokens}), "a submitted review stands")
}

func TestFriendlyAPIError(t *testing.T) {
	cases := map[int]string{
		401: "set ANTHROPIC_API_KEY",
		403: "lacks permission",
		429: "rate limit",
		529: "unavailable (529)",
		400: "anthropic API error (400)",
	}
	for status, expected := range cases {
		apiErr := &anthropic.Error{StatusCode: status, Request: &http.Request{}, Response: &http.Response{StatusCode: status}}
		err := friendlyAPIError(apiErr)
		assert.ErrorContains(t, err, expected, status)
		assert.True(t, errors.As(err, new(*anthropic.Error)))
	}
	assert.ErrorContains(t, friendlyAPIError(errors.New("dial tcp: refused")), "anthropic API: dial tcp")
}

func TestDeltaBufferChunks(t *testing.T) {
	recorder := &eventRecorder{}
	buffer := &deltaBuffer{emit: recorder.emit}
	var delta anthropic.BetaRawContentBlockDeltaUnion
	require.NoError(t, json.Unmarshal([]byte(`{"type":"thinking_delta","thinking":"Checking callers. "}`), &delta))
	buffer.addDelta(delta)
	require.NoError(t, json.Unmarshal([]byte(`{"type":"text_delta","text":"Done."}`), &delta))
	buffer.addDelta(delta)
	require.NoError(t, json.Unmarshal([]byte(`{"type":"input_json_delta","partial_json":"{\"id\""}`), &delta))
	buffer.addDelta(delta)
	assert.Empty(t, recorder.ofType(review.EventThinking))
	buffer.flush()
	thinking := recorder.ofType(review.EventThinking)
	require.Len(t, thinking, 1)
	assert.Equal(t, review.TextData{Text: "Checking callers. Done.", Agent: "lead"}, thinking[0].Data)
}

func TestAPIEngineRequiresWorktree(t *testing.T) {
	_, err := (&APIEngine{}).Run(context.Background(), &review.ReviewContext{}, nil)
	assert.ErrorContains(t, err, "no worktree path")
}

func TestReviewOutputSchemaIsStrictCompatible(t *testing.T) {
	var schema map[string]any
	require.NoError(t, json.Unmarshal(ReviewOutputSchema(), &schema))
	assert.Equal(t, false, schema["additionalProperties"])
	findings := schema["properties"].(map[string]any)["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	assert.Equal(t, false, items["additionalProperties"])
	assert.NotContains(t, string(ReviewOutputSchema()), "$ref")
}

func TestAssembleResultVerdictAndDedup(t *testing.T) {
	reviewContext := liveContext(t)
	live := newFindingSet()
	live.add(review.Finding{ID: "f-1", File: "calc/calc.go", Line: 5, Title: "Bug", Source: "impact-analyst",
		Severity: review.SeverityWarning, Category: review.CategoryCorrectness, Body: "b"})
	output := &ReviewOutput{Summary: " s ", Verdict: "APPROVE", Findings: []tools.FindingInput{
		{Severity: "critical", Category: "correctness", File: "calc/calc.go", Line: 5, Title: "bug", Body: "lead body", Confidence: 0.9},
		{Severity: "critical", Category: "correctness", File: "calc/calc.go", Line: 5, Title: "BUG", Body: "dup", Confidence: 0.9},
	}}
	result := assembleResult(output, live, tools.New(reviewContext), nil)
	assert.Equal(t, "s", result.Summary)
	assert.Equal(t, review.VerdictApprove, result.Verdict)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, "f-1", result.Findings[0].ID)
	assert.Equal(t, "lead body", result.Findings[0].Body)
	assert.Equal(t, "impact-analyst", result.Findings[0].Source)

	assert.Equal(t, review.VerdictComment, normalizeVerdict("lgtm"))
	empty := assembleResult(nil, newFindingSet(), tools.New(reviewContext), nil)
	assert.NotNil(t, empty.Findings)
}

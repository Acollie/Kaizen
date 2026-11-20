package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

func liveFinding(id string, line int, title string) review.Finding {
	return review.Finding{ID: id, File: "calc/calc.go", Line: line, Title: title, Source: "hotspot-reviewer",
		Severity: review.SeverityWarning, Category: review.CategoryCorrectness, Body: "b"}
}

func TestAssembleResultDropsWithdrawnFindings(t *testing.T) {
	reviewContext := liveContext(t)
	live := newFindingSet()
	live.add(liveFinding("f-keep", 5, "Kept bug"))
	live.add(liveFinding("f-gone", 6, "False alarm"))
	live.add(liveFinding("f-gone2", 7, "Also wrong"))
	output := &ReviewOutput{Summary: "s", Verdict: "comment", Findings: []tools.FindingInput{
		{ID: "f-keep", Severity: "warning", Category: "correctness", File: "calc/calc.go", Line: 5, Title: "Kept bug", Body: "lead", Confidence: 0.8},
	}}
	recorder := &eventRecorder{}
	result := assembleResult(output, live, tools.New(reviewContext), recorder.emit)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, "f-keep", result.Findings[0].ID)
	assert.Equal(t, "lead", result.Findings[0].Body)
	assert.Contains(t, messagesOf(recorder.ofType(review.EventStatus)),
		`Lead reviewer withdrew 2 findings: "False alarm", "Also wrong"`)
}

func TestAssembleResultEmptyStructuredOutputWithdrawsAll(t *testing.T) {
	reviewContext := liveContext(t)
	live := newFindingSet()
	live.add(liveFinding("f-gone", 6, "False alarm"))
	recorder := &eventRecorder{}
	result := assembleResult(&ReviewOutput{Verdict: "approve"}, live, tools.New(reviewContext), recorder.emit)
	assert.Empty(t, result.Findings)
	assert.NotNil(t, result.Findings)
	assert.Contains(t, messagesOf(recorder.ofType(review.EventStatus)), `Lead reviewer withdrew 1 finding: "False alarm"`)
}

func TestAssembleResultKeepsLiveFindingWhenStructuredCopyIsInvalid(t *testing.T) {
	reviewContext := liveContext(t)
	live := newFindingSet()
	live.add(liveFinding("f-keep", 5, "Kept bug"))
	output := &ReviewOutput{Verdict: "comment", Findings: []tools.FindingInput{
		{ID: "f-keep", Severity: "bogus", Category: "correctness", File: "calc/calc.go", Line: 5, Title: "Kept bug", Body: "lead", Confidence: 0.8},
	}}
	recorder := &eventRecorder{}
	result := assembleResult(output, live, tools.New(reviewContext), recorder.emit)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, "b", result.Findings[0].Body)
	for _, message := range messagesOf(recorder.ofType(review.EventStatus)) {
		assert.NotContains(t, message, "withdrew")
	}
}

func TestAssembleResultWithoutStructuredOutputKeepsLiveFindings(t *testing.T) {
	reviewContext := liveContext(t)
	live := newFindingSet()
	live.add(liveFinding("f-1", 5, "One"))
	live.add(liveFinding("f-2", 6, "Two"))
	recorder := &eventRecorder{}
	result := assembleResult(nil, live, tools.New(reviewContext), recorder.emit)
	assert.Len(t, result.Findings, 2)
	assert.Empty(t, recorder.ofType(review.EventStatus))
}

func TestStructuredPromptsSayOmittedFindingsAreWithdrawn(t *testing.T) {
	for name, text := range map[string]string{
		"lead":   leadPrompt(review.PullRequest{Number: 1}),
		"api":    apiSystemPrompt,
		"submit": descriptionSubmitReview,
		"schema": string(ReviewOutputSchema()),
	} {
		assert.Contains(t, text, "withdrawn", name)
	}
}

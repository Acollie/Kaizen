package github

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
)

func postTestPR() review.PullRequest {
	return review.PullRequest{Number: 42, Owner: "acme", Repo: "widgets", HeadSHA: "head111"}
}

// postTestFiles has one hunk covering head lines 10-14 (11 and 12 added) and a
// second hunk covering head lines 30-31
func postTestFiles() []review.FileDiff {
	return []review.FileDiff{{
		Path: "pkg/app/run.go",
		Hunks: []review.Hunk{
			{NewStart: 10, Lines: []review.DiffLine{
				{Kind: review.LineContext, OldLine: 10, NewLine: 10, Text: "a"},
				{Kind: review.LineRemoved, OldLine: 11, Text: "old"},
				{Kind: review.LineAdded, NewLine: 11, Text: "b"},
				{Kind: review.LineAdded, NewLine: 12, Text: "c"},
				{Kind: review.LineContext, OldLine: 12, NewLine: 13, Text: "d"},
				{Kind: review.LineContext, OldLine: 13, NewLine: 14, Text: "e"},
			}},
			{NewStart: 30, Lines: []review.DiffLine{
				{Kind: review.LineAdded, NewLine: 30, Text: "f"},
				{Kind: review.LineContext, OldLine: 29, NewLine: 31, Text: "g"},
			}},
		},
	}}
}

func finding(id string, line, endLine int) review.Finding {
	return review.Finding{
		ID: id, Severity: review.SeverityWarning, Category: review.CategoryCorrectness,
		File: "pkg/app/run.go", Line: line, EndLine: endLine, Title: "Title " + id, Body: "Body " + id,
	}
}

// aiNote is the attribution line on an AI finding with no source
const aiNote = "<sub>AI-assisted finding (kaizen review), checked by the reviewer</sub>"

func TestBuildReviewPayloadSplitsInlineAndBody(t *testing.T) {
	request := review.PostRequest{
		Summary: "Looks mostly good.",
		Findings: []review.Finding{
			finding("added", 11, 0),
			finding("context", 14, 0),
			finding("range", 11, 13),
			finding("outside", 20, 0),
			finding("cross-hunk", 13, 30),
			finding("file-level", 0, 0),
			{ID: "other-file", Severity: review.SeverityCritical, Category: review.CategorySecurity, File: "other.go", Line: 11, Title: "Elsewhere"},
		},
	}

	payload, inlineCount, bodyCount := buildReviewPayload(postTestPR(), postTestFiles(), request)
	assert.Equal(t, 3, inlineCount)
	assert.Equal(t, 4, bodyCount)
	assert.Equal(t, "head111", payload.CommitID)
	assert.Equal(t, "COMMENT", payload.Event)

	require.Len(t, payload.Comments, 3)
	assert.Equal(t, reviewComment{Path: "pkg/app/run.go", Line: 11, Side: "RIGHT", Body: "**[warning · correctness] Title added**\n\nBody added\n\n" + aiNote}, payload.Comments[0])
	assert.Equal(t, 14, payload.Comments[1].Line)
	assert.Zero(t, payload.Comments[1].StartLine)

	ranged := payload.Comments[2]
	assert.Equal(t, 11, ranged.StartLine)
	assert.Equal(t, "RIGHT", ranged.StartSide)
	assert.Equal(t, 13, ranged.Line)

	assert.True(t, strings.HasPrefix(payload.Body, "Looks mostly good.\n\n### Other findings\n"))
	assert.Contains(t, payload.Body, "- **[warning · correctness] Title outside** (`pkg/app/run.go:20`)")
	assert.Contains(t, payload.Body, "(`pkg/app/run.go:13-30`)")
	assert.Contains(t, payload.Body, "Title file-level** (`pkg/app/run.go`)")
	assert.Contains(t, payload.Body, "- **[critical · security] Elsewhere** (`other.go:11`)")
	assert.Contains(t, payload.Body, "  Body outside")
	assert.NotContains(t, payload.Body, "Title added")
}

func TestBuildReviewPayloadSuggestionBlock(t *testing.T) {
	withSuggestion := finding("fix", 11, 12)
	withSuggestion.Suggestion = "b2\nc2\n"
	request := review.PostRequest{Findings: []review.Finding{withSuggestion}}

	payload, _, _ := buildReviewPayload(postTestPR(), postTestFiles(), request)
	require.Len(t, payload.Comments, 1)
	assert.Equal(t,
		"**[warning · correctness] Title fix**\n\nBody fix\n\n```suggestion\nb2\nc2\n```\n\n"+aiNote,
		payload.Comments[0].Body)
	assert.Equal(t, defaultReviewBody, payload.Body, "empty summary gets a default body")
}

func TestBuildReviewPayloadSuggestionWithBackticks(t *testing.T) {
	withSuggestion := finding("fence", 11, 0)
	withSuggestion.Suggestion = "doc := \"```go\""
	payload, _, _ := buildReviewPayload(postTestPR(), postTestFiles(), review.PostRequest{Findings: []review.Finding{withSuggestion}})
	assert.Contains(t, payload.Comments[0].Body, "````suggestion\n")
	assert.True(t, strings.HasSuffix(payload.Comments[0].Body, "\n````\n\n"+aiNote))
}

func TestPostReview(t *testing.T) {
	runner := (&fakeRunner{}).respond("gh api", `{"id": 1, "html_url": "https://github.com/acme/widgets/pull/42#pullrequestreview-1"}`, nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}
	request := review.PostRequest{Summary: "Summary", Findings: []review.Finding{finding("a", 11, 0), finding("b", 99, 0)}}

	result, err := client.PostReview(context.Background(), postTestPR(), postTestFiles(), request)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/acme/widgets/pull/42#pullrequestreview-1", result.URL)
	assert.Equal(t, 1, result.InlineComments)
	assert.Equal(t, 1, result.BodyFindings)

	call := runner.lastCall()
	assert.Equal(t, "gh api repos/acme/widgets/pulls/42/reviews --method POST --input -", call.commandLine())
	var sent map[string]any
	require.NoError(t, json.Unmarshal(call.Input, &sent))
	assert.Equal(t, "head111", sent["commit_id"])
	assert.Equal(t, "COMMENT", sent["event"])
	comments := sent["comments"].([]any)
	require.Len(t, comments, 1)
	comment := comments[0].(map[string]any)
	assert.Equal(t, "RIGHT", comment["side"])
	assert.NotContains(t, comment, "start_line")
}

func TestPostReviewErrors(t *testing.T) {
	client := &Client{Runner: &fakeRunner{}}
	_, err := client.PostReview(context.Background(), review.PullRequest{Number: 1}, nil, review.PostRequest{})
	assert.ErrorContains(t, err, "missing owner")

	failure := &CommandError{Name: "gh", Stderr: "gh: Bad credentials (HTTP 401)", Err: errors.New("exit status 1")}
	client = &Client{Runner: (&fakeRunner{}).respond("gh api", "", failure)}
	_, err = client.PostReview(context.Background(), postTestPR(), nil, review.PostRequest{Summary: "x"})
	assert.ErrorIs(t, err, ErrGHNotAuthenticated)
}

func userComment(line int, body string) review.Finding {
	return review.Finding{
		ID: "c-1", Severity: review.SeveritySuggestion, Category: review.CategoryDesign,
		File: "pkg/app/run.go", Line: line, Body: body, Confidence: 1, Source: "you",
	}
}

func TestUserCommentsArePostedAsWritten(t *testing.T) {
	request := review.PostRequest{Findings: []review.Finding{
		userComment(11, "Can we name this `runOnce`?"),
		userComment(0, "Overall this reads well."),
	}}
	payload, inlineCount, bodyCount := buildReviewPayload(postTestPR(), postTestFiles(), request)
	require.Equal(t, 1, inlineCount)
	require.Equal(t, 1, bodyCount)

	assert.Equal(t, "Can we name this `runOnce`?", payload.Comments[0].Body, "no heading, no attribution")
	assert.NotContains(t, payload.Body, "**[")
	assert.NotContains(t, payload.Body, "AI-assisted")
	assert.Contains(t, payload.Body, "- (`pkg/app/run.go`)\n\n  Overall this reads well.")

	prLevel := userComment(0, "First line\nSecond line")
	prLevel.File = ""
	assert.Equal(t, "- First line\n  Second line\n", formatBodyFinding(prLevel))
}

func TestAIFindingsCarryAttribution(t *testing.T) {
	sourced := finding("a", 11, 0)
	sourced.Source = "security-reviewer"
	payload, _, _ := buildReviewPayload(postTestPR(), postTestFiles(), review.PostRequest{Findings: []review.Finding{sourced, finding("b", 99, 0)}})

	require.Len(t, payload.Comments, 1)
	assert.True(t, strings.HasSuffix(payload.Comments[0].Body,
		"\n\n<sub>AI-assisted finding (kaizen review · security-reviewer), checked by the reviewer</sub>"))
	assert.Contains(t, payload.Body, "  Body b\n\n  "+aiNote)
}

// unresolvableLine is what gh prints when GitHub rejects an inline comment's line
func unresolvableLine() ([]byte, error) {
	stdout := `{"message":"Unprocessable Entity","errors":["Line could not be resolved"],"status":"422"}`
	return []byte(stdout), &CommandError{Name: "gh", Stderr: "gh: Unprocessable Entity (HTTP 422)", Err: errors.New("exit status 1")}
}

// sequenceRunner answers successive calls from a list of responses
type sequenceRunner struct {
	fakeRunner
	outputs [][]byte
	errs    []error
}

func (runner *sequenceRunner) RunWithInput(ctx context.Context, dir string, input []byte, name string, args ...string) ([]byte, error) {
	_, _ = runner.fakeRunner.RunWithInput(ctx, dir, input, name, args...)
	call := len(runner.fakeRunner.commandLines()) - 1
	return runner.outputs[call], runner.errs[call]
}

func TestPostReviewFallsBackToBodyOn422(t *testing.T) {
	failedOutput, failure := unresolvableLine()
	runner := &sequenceRunner{
		outputs: [][]byte{failedOutput, []byte(`{"html_url": "https://github.com/acme/widgets/pull/42#pullrequestreview-2"}`)},
		errs:    []error{failure, nil},
	}
	client := &Client{Runner: runner, RepoDir: "/repo"}
	request := review.PostRequest{Summary: "Summary", Findings: []review.Finding{finding("a", 11, 0), finding("b", 99, 0)}}

	result, err := client.PostReview(context.Background(), postTestPR(), postTestFiles(), request)
	require.NoError(t, err)
	assert.True(t, result.FallbackToBody)
	assert.Equal(t, 0, result.InlineComments)
	assert.Equal(t, 2, result.BodyFindings)
	assert.Equal(t, "https://github.com/acme/widgets/pull/42#pullrequestreview-2", result.URL)

	require.Len(t, runner.calls, 2, "retried exactly once")
	var first, retry reviewPayload
	require.NoError(t, json.Unmarshal(runner.calls[0].Input, &first))
	require.NoError(t, json.Unmarshal(runner.calls[1].Input, &retry))
	assert.Len(t, first.Comments, 1)
	assert.Empty(t, retry.Comments)
	assert.Contains(t, retry.Body, "Title a** (`pkg/app/run.go:11`)")
	assert.Contains(t, retry.Body, "Title b** (`pkg/app/run.go:99`)")
}

func TestPostReviewDoesNotRetryOtherErrors(t *testing.T) {
	failure := &CommandError{Name: "gh", Stderr: "gh: Validation Failed (HTTP 422): commit_sha is not part of the pull request", Err: errors.New("exit status 1")}
	runner := &sequenceRunner{outputs: [][]byte{nil, nil}, errs: []error{failure, nil}}
	client := &Client{Runner: runner}
	_, err := client.PostReview(context.Background(), postTestPR(), postTestFiles(), review.PostRequest{Findings: []review.Finding{finding("a", 11, 0)}})
	require.Error(t, err)
	assert.Len(t, runner.calls, 1)
}

func TestPostReviewFallbackFailureIsReported(t *testing.T) {
	failedOutput, failure := unresolvableLine()
	runner := &sequenceRunner{outputs: [][]byte{failedOutput, failedOutput}, errs: []error{failure, failure}}
	client := &Client{Runner: runner}
	_, err := client.PostReview(context.Background(), postTestPR(), postTestFiles(), review.PostRequest{Findings: []review.Finding{finding("a", 11, 0)}})
	require.Error(t, err)
	assert.Len(t, runner.calls, 2, "only one retry")
}

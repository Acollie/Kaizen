package review

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewCacheRoundTrip(t *testing.T) {
	root := t.TempDir()
	pr := PullRequest{Number: 4, HeadSHA: "5066c07d1234567890abcdef"}
	path := ReviewCachePath(root, pr)
	assert.Equal(t, filepath.Join(root, ".kaizen", "reviews", "pr-4-5066c07d1234.json"), path)

	events := []Event{
		{Seq: 1, Type: EventStatus, Data: StatusData{Message: "start"}},
		{Seq: 2, Type: EventFinding, Data: Finding{ID: "f-1", Title: "bug"}},
		{Seq: 3, Type: EventResult, Data: ReviewResult{Verdict: VerdictComment}},
		{Seq: 4, Type: EventDone},
	}
	require.NoError(t, SaveReviewCache(path, pr, "claude-cli", events))

	cached, err := LoadReviewCache(path, pr, "claude-cli")
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, "claude-cli", cached.Engine)
	require.Len(t, cached.Events, 3, "done is not cached")
	assert.Equal(t, EventResult, cached.Events[2].Type)
}

func TestReviewCacheMissesForNewHeadOrIncompleteReview(t *testing.T) {
	root := t.TempDir()
	pr := PullRequest{Number: 4, HeadSHA: "aaaaaaaaaaaaaaaa"}
	path := ReviewCachePath(root, pr)

	missing, err := LoadReviewCache(path, pr, "claude-cli")
	require.NoError(t, err)
	assert.Nil(t, missing)

	require.NoError(t, SaveReviewCache(path, pr, "claude-cli", []Event{{Type: EventStatus}}))
	incomplete, err := LoadReviewCache(path, pr, "claude-cli")
	require.NoError(t, err)
	assert.Nil(t, incomplete, "a review without a result is not reused")

	require.NoError(t, SaveReviewCache(path, pr, "claude-cli", []Event{{Type: EventResult}}))
	pushed := PullRequest{Number: 4, HeadSHA: "bbbbbbbbbbbbbbbb"}
	stale, err := LoadReviewCache(path, pushed, "claude-cli")
	require.NoError(t, err)
	assert.Nil(t, stale, "a new head commit gets a new review")

	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))
	_, err = LoadReviewCache(path, pr, "claude-cli")
	assert.Error(t, err)
}

func TestReviewCacheMissesForDifferentEngine(t *testing.T) {
	root := t.TempDir()
	pr := PullRequest{Number: 4, HeadSHA: "aaaaaaaaaaaaaaaa"}
	path := ReviewCachePath(root, pr)
	require.NoError(t, SaveReviewCache(path, pr, "claude-cli", []Event{{Type: EventResult}}))

	other, err := LoadReviewCache(path, pr, "anthropic-api")
	require.NoError(t, err)
	assert.Nil(t, other, "--engine=api must not replay a claude-cli review")

	same, err := LoadReviewCache(path, pr, "claude-cli")
	require.NoError(t, err)
	assert.NotNil(t, same)
}

func TestSaveReviewCacheIgnoresItself(t *testing.T) {
	root := t.TempDir()
	pr := PullRequest{Number: 4, HeadSHA: "aaaaaaaaaaaaaaaa"}
	require.NoError(t, SaveReviewCache(ReviewCachePath(root, pr), pr, "claude-cli", []Event{{Type: EventResult}}))

	content, err := os.ReadFile(filepath.Join(root, ".kaizen", ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "*\n", string(content))
}

func TestSaveReviewCacheKeepsExistingGitignore(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".kaizen"), 0o755))
	ignorePath := filepath.Join(root, ".kaizen", ".gitignore")
	require.NoError(t, os.WriteFile(ignorePath, []byte("reviews/\n"), 0o644))

	pr := PullRequest{Number: 4, HeadSHA: "aaaaaaaaaaaaaaaa"}
	require.NoError(t, SaveReviewCache(ReviewCachePath(root, pr), pr, "claude-cli", []Event{{Type: EventResult}}))

	content, err := os.ReadFile(ignorePath)
	require.NoError(t, err)
	assert.Equal(t, "reviews/\n", string(content))
}

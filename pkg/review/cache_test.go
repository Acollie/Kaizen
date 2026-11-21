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
	path := ReviewCachePath(root, pr, "claude-cli")
	assert.Equal(t, filepath.Join(root, ".kaizen", "reviews", "pr-4-5066c07d1234-"+cacheKeyHash("claude-cli")+".json"), path)
	assert.Equal(t, filepath.Join(root, ".kaizen", "reviews", "pr-4-5066c07d1234.json"), ReviewCachePath(root, pr, ""))

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
	path := ReviewCachePath(root, pr, "claude-cli")

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
	path := ReviewCachePath(root, pr, "claude-cli")
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
	require.NoError(t, SaveReviewCache(ReviewCachePath(root, pr, "claude-cli"), pr, "claude-cli", []Event{{Type: EventResult}}))

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
	require.NoError(t, SaveReviewCache(ReviewCachePath(root, pr, "claude-cli"), pr, "claude-cli", []Event{{Type: EventResult}}))

	content, err := os.ReadFile(ignorePath)
	require.NoError(t, err)
	assert.Equal(t, "reviews/\n", string(content))
}

// Reviews made with different settings get different files and never overwrite each other
func TestReviewCacheKeysDoNotCollide(t *testing.T) {
	root := t.TempDir()
	pr := PullRequest{Number: 5, HeadSHA: "aaaaaaaaaaaaaaaa"}
	highKey, lowKey := "claude-cli model= effort=high focus=5", "claude-cli model= effort=low focus=5"
	highPath, lowPath := ReviewCachePath(root, pr, highKey), ReviewCachePath(root, pr, lowKey)
	assert.NotEqual(t, highPath, lowPath)

	require.NoError(t, SaveReviewCache(highPath, pr, highKey, []Event{{Type: EventResult}}))
	require.NoError(t, SaveReviewCache(lowPath, pr, lowKey, []Event{{Type: EventResult}}))

	high, err := LoadReviewCache(highPath, pr, highKey)
	require.NoError(t, err)
	require.NotNil(t, high)
	low, err := LoadReviewCache(lowPath, pr, lowKey)
	require.NoError(t, err)
	require.NotNil(t, low)
}

// A cache saved under the old settings-free name still replays for its own settings
func TestReviewCacheReadsLegacyName(t *testing.T) {
	root := t.TempDir()
	pr := PullRequest{Number: 5, HeadSHA: "abcdef1234567890"}
	key := "claude-cli model= effort=high focus=5"
	require.NoError(t, SaveReviewCache(ReviewCachePath(root, pr, ""), pr, key, []Event{{Type: EventResult}}))

	cached, err := LoadReviewCache(ReviewCachePath(root, pr, key), pr, key)
	require.NoError(t, err)
	assert.NotNil(t, cached)

	other, err := LoadReviewCache(ReviewCachePath(root, pr, "anthropic-api"), pr, "anthropic-api")
	require.NoError(t, err)
	assert.Nil(t, other, "the legacy file only replays for the settings that made it")
}

package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cacheVersion is bumped when the cached event format changes incompatibly
const cacheVersion = 1

// CachedReview is a finished review's event stream, saved so reopening the same PR
// head replays it instead of paying for a new AI run
type CachedReview struct {
	Version   int       `json:"version"`
	PRNumber  int       `json:"pr_number"`
	HeadSHA   string    `json:"head_sha"`
	Engine    string    `json:"engine"`
	CreatedAt time.Time `json:"created_at"`
	Events    []Event   `json:"events"`
}

// ReviewCachePath is where the review of a PR at its current head commit is cached:
// <repoRoot>/.kaizen/reviews/pr-<number>-<head sha prefix>.json
func ReviewCachePath(repoRoot string, pr PullRequest) string {
	head := pr.HeadSHA
	if len(head) > 12 {
		head = head[:12]
	}
	return filepath.Join(repoRoot, ".kaizen", "reviews", fmt.Sprintf("pr-%d-%s.json", pr.Number, head))
}

// SaveReviewCache writes a finished review. EventDone is dropped: the replayer emits its own.
func SaveReviewCache(path string, pr PullRequest, engine string, events []Event) error {
	kept := make([]Event, 0, len(events))
	for _, event := range events {
		if event.Type != EventDone {
			kept = append(kept, event)
		}
	}
	cached := CachedReview{
		Version:   cacheVersion,
		PRNumber:  pr.Number,
		HeadSHA:   pr.HeadSHA,
		Engine:    engine,
		CreatedAt: time.Now(),
		Events:    kept,
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// LoadReviewCache reads a cached review. It returns (nil, nil) when there is no cache
// for this PR head, and an error only for a cache that exists but cannot be used.
func LoadReviewCache(path string, pr PullRequest) (*CachedReview, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cached CachedReview
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("reading cached review %s: %w", path, err)
	}
	if cached.Version != cacheVersion || cached.HeadSHA != pr.HeadSHA || !hasResult(cached.Events) {
		return nil, nil
	}
	return &cached, nil
}

func hasResult(events []Event) bool {
	for _, event := range events {
		if event.Type == EventResult {
			return true
		}
	}
	return false
}

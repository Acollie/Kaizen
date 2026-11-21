package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// ReviewCachePath is where the review of a PR at its current head commit, made with
// the settings identified by cacheKey (engine, model, effort, focus), is cached:
// <repoRoot>/.kaizen/reviews/pr-<number>-<head sha prefix>-<key hash>.json. Different
// settings get different files, so they never overwrite each other. An empty
// cacheKey gives the settings-free name pr-<number>-<head sha prefix>.json, which
// older versions used for every review.
func ReviewCachePath(repoRoot string, pr PullRequest, cacheKey string) string {
	head := pr.HeadSHA
	if len(head) > 12 {
		head = head[:12]
	}
	name := fmt.Sprintf("pr-%d-%s", pr.Number, head)
	if cacheKey != "" {
		name += "-" + cacheKeyHash(cacheKey)
	}
	return filepath.Join(repoRoot, ".kaizen", "reviews", name+".json")
}

// cacheKeyHash is a short, stable hash of a cache key for file names
func cacheKeyHash(cacheKey string) string {
	sum := sha256.Sum256([]byte(cacheKey))
	return hex.EncodeToString(sum[:])[:8]
}

// keyedCacheName matches a keyed cache file name, capturing its settings-free prefix
var keyedCacheName = regexp.MustCompile(`^(pr-\d+-[^-]*)-[0-9a-f]{8}\.json$`)

// legacyCachePath is the settings-free name older versions saved a keyed path's
// review under, or "" when path is not a keyed cache path
func legacyCachePath(path string) string {
	match := keyedCacheName.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return ""
	}
	return filepath.Join(filepath.Dir(path), match[1]+".json")
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
	if err := ensureCacheDir(filepath.Dir(path)); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// ensureCacheDir creates the reviews directory. When the .kaizen directory above it
// (often inside someone else's repository) has no .gitignore, one holding "*" is
// written so the cache ignores itself and never shows up in git status.
func ensureCacheDir(reviewsDir string) error {
	if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
		return err
	}
	kaizenDir := filepath.Dir(reviewsDir)
	if filepath.Base(kaizenDir) != ".kaizen" {
		return nil
	}
	ignorePath := filepath.Join(kaizenDir, ".gitignore")
	if _, err := os.Lstat(ignorePath); !errors.Is(err, os.ErrNotExist) {
		return nil // keep any existing .gitignore
	}
	return os.WriteFile(ignorePath, []byte("*\n"), 0o644)
}

// LoadReviewCache reads a cached review. It returns (nil, nil) when there is no cache
// for this PR head and engine (so --engine=api never replays a claude-cli review), and
// an error only for a cache that exists but cannot be used.
func LoadReviewCache(path string, pr PullRequest, engine string) (*CachedReview, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// Older versions saved every review under the settings-free name; the engine
		// check below still only replays it for the settings that produced it
		legacy := legacyCachePath(path)
		if legacy == "" {
			return nil, nil
		}
		path = legacy
		data, err = os.ReadFile(path)
	}
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
	if cached.Version != cacheVersion || cached.HeadSHA != pr.HeadSHA || cached.Engine != engine || !hasResult(cached.Events) {
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

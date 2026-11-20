package reviewui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/alexcollie/kaizen/pkg/review"
)

//go:embed testdata/fixture_context.json
var demoContextJSON []byte

//go:embed testdata/worktree
var demoWorktreeFiles embed.FS

const demoWorktreeRoot = "testdata/worktree"

// LoadDemoContext returns the fixture review context used by `kaizen review-demo`.
// The fixture worktree is extracted to a temporary directory so context
// expansion works from any working directory; call cleanup to remove it.
func LoadDemoContext() (reviewContext *review.ReviewContext, cleanup func(), err error) {
	reviewContext = &review.ReviewContext{}
	if err := json.Unmarshal(demoContextJSON, reviewContext); err != nil {
		return nil, nil, fmt.Errorf("reviewui: parse demo fixture: %w", err)
	}

	worktree, err := os.MkdirTemp("", "kaizen-review-demo-")
	if err != nil {
		return nil, nil, fmt.Errorf("reviewui: create demo worktree: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(worktree) }

	if err := extractDemoWorktree(worktree); err != nil {
		cleanup()
		return nil, nil, err
	}
	reviewContext.WorktreePath = worktree
	return reviewContext, cleanup, nil
}

// extractDemoWorktree copies the embedded fixture worktree into destination
func extractDemoWorktree(destination string) error {
	return fs.WalkDir(demoWorktreeFiles, demoWorktreeRoot, func(embeddedPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(demoWorktreeRoot, embeddedPath)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := demoWorktreeFiles.ReadFile(embeddedPath)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

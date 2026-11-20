package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
)

func TestCheckoutCommandsAndIdempotentCleanup(t *testing.T) {
	runner := (&fakeRunner{}).respond("git merge-base", "mb999\n", nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}
	pr := &review.PullRequest{Number: 42, BaseRef: "main", BaseSHA: "base000", HeadSHA: "head111"}

	worktree, cleanup, err := client.Checkout(context.Background(), pr)
	require.NoError(t, err)
	assert.Equal(t, "mb999", pr.MergeBase)
	assert.Contains(t, filepath.Base(worktree), "kaizen-review-42-")

	require.NoError(t, cleanup())
	require.NoError(t, cleanup())

	assert.Equal(t, []string{
		"git fetch --no-tags origin pull/42/head main",
		"git worktree add --detach " + worktree + " head111",
		"git merge-base base000 head111",
		"git worktree remove --force " + worktree,
	}, runner.commandLines(), "cleanup runs git worktree remove exactly once")

	_, statErr := os.Stat(worktree)
	assert.True(t, os.IsNotExist(statErr))
}

func TestCheckoutFailures(t *testing.T) {
	failure := &CommandError{Name: "git", Stderr: "fatal: couldn't find remote ref pull/42/head", Err: errors.New("exit status 128")}

	runner := (&fakeRunner{}).respond("git fetch", "", failure)
	client := &Client{Runner: runner}
	_, _, err := client.Checkout(context.Background(), &review.PullRequest{Number: 42, BaseSHA: "b", HeadSHA: "h"})
	assert.ErrorContains(t, err, "couldn't find remote ref")

	_, _, err = client.Checkout(context.Background(), &review.PullRequest{Number: 42})
	assert.ErrorContains(t, err, "missing head or base SHA")

	// A failed merge-base removes the worktree it just created
	runner = (&fakeRunner{}).respond("git merge-base", "", failure)
	client = &Client{Runner: runner}
	_, _, err = client.Checkout(context.Background(), &review.PullRequest{Number: 7, BaseSHA: "b", HeadSHA: "h"})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(runner.lastCall().commandLine(), "git worktree remove --force"))
}

func TestDiffCommand(t *testing.T) {
	runner := (&fakeRunner{}).respond("git diff", "diff --git a/x b/x\n", nil)
	client := &Client{Runner: runner}

	diff, err := client.Diff(context.Background(), "/tmp/wt", "mb", "head", 0)
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/x b/x\n", diff)
	assert.Equal(t, "git diff mb head --unified=0 -M", runner.lastCall().commandLine())
	assert.Equal(t, "/tmp/wt", runner.lastCall().Dir)
}

// TestCheckoutWithRealGit exercises fetch, worktree add, merge-base and cleanup
// against a local "origin" that exposes refs/pull/<n>/head like GitHub does
func TestCheckoutWithRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	origin := t.TempDir()
	gitRun(t, origin, "init", "-q", "-b", "main")
	writeFile(t, origin, "a.txt", "one\n")
	gitRun(t, origin, "add", ".")
	gitRun(t, origin, "commit", "-q", "-m", "base")
	baseSHA := strings.TrimSpace(gitRun(t, origin, "rev-parse", "HEAD"))

	gitRun(t, origin, "checkout", "-q", "-b", "feature")
	writeFile(t, origin, "a.txt", "one\ntwo\n")
	gitRun(t, origin, "commit", "-q", "-am", "head")
	headSHA := strings.TrimSpace(gitRun(t, origin, "rev-parse", "HEAD"))
	gitRun(t, origin, "update-ref", "refs/pull/5/head", headSHA)
	gitRun(t, origin, "checkout", "-q", "main")

	clone := filepath.Join(t.TempDir(), "clone")
	gitRun(t, filepath.Dir(clone), "clone", "-q", "--single-branch", "--branch", "main", origin, clone)

	client := NewClient(clone)
	pr := &review.PullRequest{Number: 5, BaseRef: "main", BaseSHA: baseSHA, HeadSHA: headSHA}
	worktree, cleanup, err := client.Checkout(ctx, pr)
	require.NoError(t, err)
	assert.Equal(t, baseSHA, pr.MergeBase)

	content, err := os.ReadFile(filepath.Join(worktree, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "one\ntwo\n", string(content))

	diff, err := client.Diff(ctx, worktree, pr.MergeBase, pr.HeadSHA, 3)
	require.NoError(t, err)
	assert.Contains(t, diff, "+two")

	require.NoError(t, cleanup())
	require.NoError(t, cleanup())
	_, statErr := os.Stat(worktree)
	assert.True(t, os.IsNotExist(statErr))
	assert.NotContains(t, gitRun(t, clone, "worktree", "list"), worktree)
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-c", "user.email=test@example.com", "-c", "user.name=Test User"}, args...)
	command := exec.Command("git", fullArgs...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, output)
	return string(output)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

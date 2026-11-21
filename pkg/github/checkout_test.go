package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
)

func TestCheckoutCommandsAndIdempotentCleanup(t *testing.T) {
	runner := (&fakeRunner{}).respond(hardenedGit+" merge-base", "mb999\n", nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}
	pr := &review.PullRequest{Number: 42, BaseRef: "main", BaseSHA: "base000", HeadSHA: "head111"}

	worktree, cleanup, err := client.Checkout(context.Background(), pr)
	require.NoError(t, err)
	assert.Equal(t, "mb999", pr.MergeBase)
	assert.Contains(t, filepath.Base(worktree), "kaizen-review-42-")

	require.NoError(t, cleanup())
	require.NoError(t, cleanup())

	assert.Equal(t, []string{
		hardenedGit + " -c credential.interactive=false fetch --no-tags -- origin pull/42/head main",
		hardenedGit + " worktree add --detach -- " + worktree + " head111",
		hardenedGit + " merge-base base000 head111",
		hardenedGit + " worktree remove --force -- " + worktree,
	}, runner.commandLines(), "cleanup runs git worktree remove exactly once")

	_, statErr := os.Stat(worktree)
	assert.True(t, os.IsNotExist(statErr))
}

func TestCheckoutFailures(t *testing.T) {
	failure := &CommandError{Name: "git", Stderr: "fatal: couldn't find remote ref pull/42/head", Err: errors.New("exit status 128")}

	runner := (&fakeRunner{}).respond(hardenedGit+" -c credential.interactive=false fetch", "", failure)
	client := &Client{Runner: runner}
	_, _, err := client.Checkout(context.Background(), &review.PullRequest{Number: 42, BaseSHA: "b", HeadSHA: "h"})
	assert.ErrorContains(t, err, "couldn't find remote ref")

	_, _, err = client.Checkout(context.Background(), &review.PullRequest{Number: 42})
	assert.ErrorContains(t, err, "missing head or base SHA")

	// A failed merge-base removes the worktree it just created
	runner = (&fakeRunner{}).respond(hardenedGit+" merge-base", "", failure)
	client = &Client{Runner: runner}
	_, _, err = client.Checkout(context.Background(), &review.PullRequest{Number: 7, BaseSHA: "b", HeadSHA: "h"})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(runner.lastCall().commandLine(), hardenedGit+" worktree remove --force"))
}

func TestDiffCommand(t *testing.T) {
	runner := (&fakeRunner{}).respond(hardenedGit+" --attr-source=mb diff", "diff --git a/x b/x\n", nil)
	client := &Client{Runner: runner}

	diff, err := client.Diff(context.Background(), "/tmp/wt", "mb", "head", 0)
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/x b/x\n", diff)
	assert.Equal(t, hardenedGit+" --attr-source=mb diff mb head --unified=0 --no-ext-diff --no-color --no-textconv "+
		"--src-prefix=a/ --dst-prefix=b/ --diff-algorithm=myers -M --inter-hunk-context=0", runner.lastCall().commandLine())
	assert.Equal(t, "/tmp/wt", runner.lastCall().Dir)
}

func TestDiffFallsBackWithoutAttrSourceOnOldGit(t *testing.T) {
	runner := (&fakeRunner{}).
		respond(hardenedGit+" --attr-source=mb diff", "", errors.New("unknown option: --attr-source=mb")).
		respond(hardenedGit+" diff", "diff --git a/x b/x\n", nil)
	client := &Client{Runner: runner}

	diff, err := client.Diff(context.Background(), "/tmp/wt", "mb", "head", 3)
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/x b/x\n", diff)
	assert.True(t, strings.HasPrefix(runner.lastCall().commandLine(), hardenedGit+" diff mb head --unified=3"))
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

// A PR must not be able to hide its own code changes with .gitattributes, and the
// user's git config (colour, external diff tools, no-prefix) must not change the diff.
func TestDiffIgnoresPRGitattributesAndUserDiffConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q", "-b", "main")
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, repo, "near.txt", numberedLines(12, -1, -1))
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "base")
	baseSHA := strings.TrimSpace(gitRun(t, repo, "rev-parse", "HEAD"))

	writeFile(t, repo, ".gitattributes", "*.go -diff\n")
	writeFile(t, repo, "main.go", "package main\n\nfunc main() { evil() }\n")
	// Two changes four lines apart: separate hunks at --unified=0
	writeFile(t, repo, "near.txt", numberedLines(12, 3, 8))
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "hide it")
	headSHA := strings.TrimSpace(gitRun(t, repo, "rev-parse", "HEAD"))

	// Hostile user config: colour always on, an external diff that would fail, no
	// prefixes, and inter-hunk context that would merge nearby hunks
	gitRun(t, repo, "config", "color.ui", "always")
	gitRun(t, repo, "config", "diff.external", "false")
	gitRun(t, repo, "config", "diff.noprefix", "true")
	gitRun(t, repo, "config", "diff.interHunkContext", "10")

	diff, err := NewClient(repo).Diff(ctx, repo, baseSHA, headSHA, 3)
	require.NoError(t, err)
	assert.Contains(t, diff, "+func main() { evil() }", "the Go change must be visible despite *.go -diff")
	assert.NotContains(t, diff, "Binary files")
	assert.NotContains(t, diff, "\x1b[", "no ANSI colour codes")
	assert.Contains(t, diff, "diff --git a/main.go b/main.go")

	mappingDiff, err := NewClient(repo).Diff(ctx, repo, baseSHA, headSHA, 0)
	require.NoError(t, err)
	nearDiff := mappingDiff[strings.Index(mappingDiff, "diff --git a/near.txt"):]
	assert.Equal(t, 2, strings.Count(nearDiff, "\n@@ "), "diff.interHunkContext must not merge nearby hunks:\n%s", nearDiff)
}

// numberedLines returns "line 1\n".."line N\n", with the given lines (1-based) changed
func numberedLines(count int, changed ...int) string {
	var builder strings.Builder
	for lineNumber := 1; lineNumber <= count; lineNumber++ {
		suffix := ""
		for _, changedLine := range changed {
			if changedLine == lineNumber {
				suffix = " changed"
			}
		}
		builder.WriteString("line " + strconv.Itoa(lineNumber) + suffix + "\n")
	}
	return builder.String()
}

const upstreamURL = "https://github.com/acme/widgets.git"

// hardenedGit is how every checkout-related git command starts
var hardenedGit = "git -c core.hooksPath=" + disabledHooksPath()

func TestCheckoutFetchesFromBaseRepositoryURL(t *testing.T) {
	runner := (&fakeRunner{}).respond(hardenedGit+" merge-base", "mb999\n", nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}
	pr := &review.PullRequest{Number: 42, Owner: "acme", Repo: "widgets", BaseRef: "main", BaseSHA: "base000", HeadSHA: "head111"}

	_, cleanup, err := client.Checkout(context.Background(), pr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })
	assert.Equal(t, hardenedGit+" -c credential.interactive=false fetch --no-tags -- "+upstreamURL+" pull/42/head main", runner.commandLines()[0])
	assert.NotContains(t, strings.Join(runner.commandLines(), "\n"), "fetch --no-tags -- origin", "origin is only a fallback")
}

func TestBaseRepoURL(t *testing.T) {
	assert.Equal(t, upstreamURL, baseRepoURL(review.PullRequest{Owner: "acme", Repo: "widgets"}))
	assert.Equal(t, "https://ghe.example.com/team/svc.git",
		baseRepoURL(review.PullRequest{URL: "https://ghe.example.com/team/svc/pull/7", Owner: "acme", Repo: "widgets"}),
		"the PR URL wins, so Enterprise hosts work")
	assert.Equal(t, upstreamURL, baseRepoURL(review.PullRequest{URL: "not a url", Owner: "acme", Repo: "widgets"}))
	assert.Empty(t, baseRepoURL(review.PullRequest{}))
}

func TestCheckoutFallsBackToOrigin(t *testing.T) {
	failure := &CommandError{Name: "git", Stderr: "fatal: unable to access 'https://github.com/acme/widgets.git/': Could not resolve host", Err: errors.New("exit status 128")}
	runner := (&fakeRunner{}).
		respond(hardenedGit+" -c credential.interactive=false fetch --no-tags -- https:", "", failure).
		respond(hardenedGit+" merge-base", "mb\n", nil)
	client := &Client{Runner: runner}
	pr := &review.PullRequest{Number: 42, Owner: "acme", Repo: "widgets", BaseRef: "main", BaseSHA: "b", HeadSHA: "h"}

	_, cleanup, err := client.Checkout(context.Background(), pr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })
	assert.Equal(t, hardenedGit+" -c credential.interactive=false fetch --no-tags -- origin pull/42/head main", runner.commandLines()[1])
}

func TestCheckoutAuthFailureMentionsSetupGit(t *testing.T) {
	authFailure := &CommandError{Name: "git", Stderr: "remote: Repository not found.\nfatal: repository 'https://github.com/acme/widgets.git/' not found", Err: errors.New("exit status 128")}
	originFailure := &CommandError{Name: "git", Stderr: "fatal: couldn't find remote ref pull/42/head", Err: errors.New("exit status 128")}
	runner := (&fakeRunner{}).
		respond(hardenedGit+" -c credential.interactive=false fetch --no-tags -- https:", "", authFailure).
		respond(hardenedGit+" -c credential.interactive=false fetch --no-tags -- origin", "", originFailure)
	client := &Client{Runner: runner}
	pr := &review.PullRequest{Number: 42, Owner: "acme", Repo: "widgets", BaseSHA: "b", HeadSHA: "h"}

	_, _, err := client.Checkout(context.Background(), pr)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFetchAuth)
	assert.ErrorContains(t, err, "gh auth setup-git")
	assert.ErrorContains(t, err, "couldn't find remote ref", "the origin failure is reported too")

	// A non-auth failure does not suggest gh auth setup-git
	runner = (&fakeRunner{}).respond("git", "", originFailure)
	_, _, err = (&Client{Runner: runner}).Checkout(context.Background(), pr)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrFetchAuth)
	assert.NotContains(t, err.Error(), "gh auth setup-git")
}

// TestCheckoutFromUpstreamWhenOriginIsAFork reproduces a contributor's clone: origin
// is their fork (no PR refs), the PR lives on the upstream repository. The upstream
// URL is redirected to a local bare repository with url.<path>.insteadOf.
func TestCheckoutFromUpstreamWhenOriginIsAFork(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	work := t.TempDir()
	source := filepath.Join(work, "source")
	gitRun(t, work, "init", "-q", "-b", "main", source)
	writeFile(t, source, "a.txt", "one\n")
	gitRun(t, source, "add", ".")
	gitRun(t, source, "commit", "-q", "-m", "base")
	baseSHA := strings.TrimSpace(gitRun(t, source, "rev-parse", "HEAD"))
	gitRun(t, source, "checkout", "-q", "-b", "feature")
	writeFile(t, source, "a.txt", "one\ntwo\n")
	gitRun(t, source, "commit", "-q", "-am", "head")
	headSHA := strings.TrimSpace(gitRun(t, source, "rev-parse", "HEAD"))
	gitRun(t, source, "checkout", "-q", "main")

	// upstream: bare, exposes refs/pull/9/head like GitHub
	upstream := filepath.Join(work, "upstream.git")
	gitRun(t, work, "clone", "-q", "--bare", source, upstream)
	gitRun(t, upstream, "update-ref", "refs/pull/9/head", headSHA)

	// fork: has main only, no PR refs and not the feature commit
	fork := filepath.Join(work, "fork.git")
	gitRun(t, work, "clone", "-q", "--bare", "--no-local", "--single-branch", "--branch", "main", source, fork)

	clone := filepath.Join(work, "clone")
	gitRun(t, work, "clone", "-q", fork, clone)
	gitRun(t, clone, "remote", "add", "upstream", upstream)
	gitRun(t, clone, "config", "url."+upstream+".insteadOf", upstreamURL)

	client := NewClient(clone)
	pr := &review.PullRequest{Number: 9, Owner: "acme", Repo: "widgets", BaseRef: "main", BaseSHA: baseSHA, HeadSHA: headSHA}
	worktree, cleanup, err := client.Checkout(ctx, pr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })
	assert.Equal(t, baseSHA, pr.MergeBase)
	content, err := os.ReadFile(filepath.Join(worktree, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "one\ntwo\n", string(content))

	// Before the fix, origin was the only source and the fetch failed
	_, originErr := client.git(ctx, clone, "fetch", "--no-tags", "origin", "pull/9/head")
	assert.Error(t, originErr, "the fork really has no PR ref")
}

// The user's git hooks (core.hooksPath=.husky and the like) must not run while the
// untrusted PR is fetched and checked out: a post-checkout hook would run with the
// PR's files as its working directory
func TestCheckoutDoesNotRunUserHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell hooks")
	}
	ctx := context.Background()
	origin := t.TempDir()
	gitRun(t, origin, "init", "-q", "-b", "main")
	writeFile(t, origin, "a.txt", "one\n")
	gitRun(t, origin, "add", ".")
	gitRun(t, origin, "commit", "-q", "-m", "base")
	baseSHA := strings.TrimSpace(gitRun(t, origin, "rev-parse", "HEAD"))
	writeFile(t, origin, "a.txt", "one\ntwo\n")
	gitRun(t, origin, "commit", "-q", "-am", "head")
	headSHA := strings.TrimSpace(gitRun(t, origin, "rev-parse", "HEAD"))
	gitRun(t, origin, "update-ref", "refs/pull/3/head", headSHA)
	gitRun(t, origin, "reset", "-q", "--hard", baseSHA)

	clone := filepath.Join(t.TempDir(), "clone")
	gitRun(t, filepath.Dir(clone), "clone", "-q", origin, clone)
	markers := t.TempDir()
	hooksDir := filepath.Join(clone, ".husky")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	for _, hook := range []string{"post-checkout", "reference-transaction"} {
		script := "#!/bin/sh\npwd > '" + filepath.Join(markers, hook) + "'\n"
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, hook), []byte(script), 0o755))
	}
	gitRun(t, clone, "config", "core.hooksPath", ".husky")

	// Sanity check: without the hardening, the hook really runs
	probe := filepath.Join(t.TempDir(), "probe")
	gitRun(t, clone, "worktree", "add", "-q", "--detach", probe, baseSHA)
	require.FileExists(t, filepath.Join(markers, "post-checkout"))
	gitRun(t, clone, "worktree", "remove", "--force", probe)
	require.NoError(t, os.RemoveAll(markers))
	require.NoError(t, os.MkdirAll(markers, 0o755))

	client := NewClient(clone)
	pr := &review.PullRequest{Number: 3, BaseRef: "main", BaseSHA: baseSHA, HeadSHA: headSHA}
	worktree, cleanup, err := client.Checkout(ctx, pr)
	require.NoError(t, err)
	_, err = client.Diff(ctx, worktree, pr.MergeBase, pr.HeadSHA, 3)
	require.NoError(t, err)
	require.NoError(t, cleanup())

	entries, err := os.ReadDir(markers)
	require.NoError(t, err)
	assert.Empty(t, entries, "no user hook may run during checkout")
}

func TestCheckoutCommandsAreHardened(t *testing.T) {
	runner := (&fakeRunner{}).respond(hardenedGit+" merge-base", "mb\n", nil)
	client := &Client{Runner: runner}
	pr := &review.PullRequest{Number: 1, BaseRef: "main", BaseSHA: "b", HeadSHA: "h"}
	_, cleanup, err := client.Checkout(context.Background(), pr)
	require.NoError(t, err)
	require.NoError(t, cleanup())
	_, err = client.Diff(context.Background(), "/wt", "mb", "h", 0)
	require.NoError(t, err)

	for _, call := range runner.calls {
		assert.True(t, strings.HasPrefix(call.commandLine(), hardenedGit+" "), call.commandLine())
		assert.Contains(t, call.Env, "GIT_LFS_SKIP_SMUDGE=1", call.commandLine())
		assert.Contains(t, call.Env, "GIT_TERMINAL_PROMPT=0", call.commandLine())
	}
}

func TestCheckoutRejectsOptionLikeRefs(t *testing.T) {
	for _, pr := range []review.PullRequest{
		{Number: 1, BaseRef: "--upload-pack=touch /tmp/x", BaseSHA: "b", HeadSHA: "h"},
		{Number: 1, BaseRef: "main", BaseSHA: "-b", HeadSHA: "h"},
		{Number: 1, BaseRef: "main", BaseSHA: "b", HeadSHA: "--output=/tmp/x"},
	} {
		runner := &fakeRunner{}
		_, _, err := (&Client{Runner: runner}).Checkout(context.Background(), &pr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "starts with '-'")
		assert.Empty(t, runner.commandLines(), "nothing runs")
	}

	_, err := (&Client{Runner: &fakeRunner{}}).Diff(context.Background(), "/wt", "--output=x", "h", 0)
	assert.Error(t, err)
}

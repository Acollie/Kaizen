package github

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alexcollie/kaizen/pkg/review"
)

// defaultRemote is the remote PR refs are fetched from
const defaultRemote = "origin"

// Checkout fetches the PR head (via refs/pull/<n>/head, so forks work) and
// base, creates a detached worktree at the head SHA in a new temporary
// directory, and sets pr.MergeBase. The returned cleanup removes the worktree;
// it is safe to call more than once and ignores the caller's context.
func (client *Client) Checkout(ctx context.Context, pr *review.PullRequest) (string, func() error, error) {
	if pr == nil || pr.HeadSHA == "" || pr.BaseSHA == "" {
		return "", nil, fmt.Errorf("checkout: pull request is missing head or base SHA")
	}
	if err := client.fetchPRRefs(ctx, pr); err != nil {
		return "", nil, err
	}

	worktreePath, err := os.MkdirTemp("", fmt.Sprintf("kaizen-review-%d-", pr.Number))
	if err != nil {
		return "", nil, fmt.Errorf("create worktree directory: %w", err)
	}

	if _, err := client.git(ctx, client.repoDir(), "worktree", "add", "--detach", worktreePath, pr.HeadSHA); err != nil {
		_ = os.RemoveAll(worktreePath)
		return "", nil, fmt.Errorf("create worktree for PR #%d: %w", pr.Number, err)
	}
	cleanup := client.worktreeCleanup(worktreePath)

	mergeBase, err := client.git(ctx, client.repoDir(), "merge-base", pr.BaseSHA, pr.HeadSHA)
	if err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("compute merge base: %w", err)
	}
	pr.MergeBase = strings.TrimSpace(string(mergeBase))

	return worktreePath, cleanup, nil
}

// fetchPRRefs fetches the PR head ref and the base branch from the remote
func (client *Client) fetchPRRefs(ctx context.Context, pr *review.PullRequest) error {
	refs := []string{"pull/" + strconv.Itoa(pr.Number) + "/head"}
	if pr.BaseRef != "" {
		refs = append(refs, pr.BaseRef)
	}
	args := append([]string{"fetch", "--no-tags", defaultRemote}, refs...)
	if _, err := client.git(ctx, client.repoDir(), args...); err != nil {
		return fmt.Errorf("fetch PR #%d: %w", pr.Number, err)
	}
	return nil
}

// worktreeCleanup returns an idempotent function that removes the worktree
func (client *Client) worktreeCleanup(worktreePath string) func() error {
	var once sync.Once
	var cleanupErr error
	return func() error {
		once.Do(func() {
			_, err := client.git(context.Background(), client.repoDir(), "worktree", "remove", "--force", worktreePath)
			if removeErr := os.RemoveAll(worktreePath); err == nil && removeErr != nil {
				err = removeErr
			}
			cleanupErr = err
		})
		return cleanupErr
	}
}

// diffFlags make the diff independent of the user's git config, so it always parses
// the same way and matches the hunks GitHub shows (inline comments must land in them):
// no external diff tools, colour, textconv, mnemonic/no prefixes or custom algorithms.
var diffFlags = []string{
	"--no-ext-diff", "--no-color", "--no-textconv",
	"--src-prefix=a/", "--dst-prefix=b/", "--diff-algorithm=myers", "-M",
}

// Diff returns the diff between mergeBase and headSHA with N lines of context.
// Attributes come from the merge base (--attr-source), so a PR cannot hide its own
// changes by adding e.g. "*.go -diff" to .gitattributes. Gits older than 2.40 lack
// --attr-source; for them the diff is retried without it.
func (client *Client) Diff(ctx context.Context, worktree, mergeBase, headSHA string, unified int) (string, error) {
	diffArgs := append([]string{"diff", mergeBase, headSHA, "--unified=" + strconv.Itoa(unified)}, diffFlags...)
	output, err := client.git(ctx, worktree, append([]string{"--attr-source=" + mergeBase}, diffArgs...)...)
	if err != nil && isUnknownOptionError(err) {
		output, err = client.git(ctx, worktree, diffArgs...)
	}
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(output), nil
}

// isUnknownOptionError reports whether git rejected an option it does not know
func isUnknownOptionError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "unknown option") || strings.Contains(message, "usage: git")
}

// git runs a git subcommand in dir
func (client *Client) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return client.runner().Run(ctx, dir, "git", args...)
}

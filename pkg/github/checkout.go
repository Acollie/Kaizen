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

// Diff returns `git diff <mergeBase> <headSHA> --unified=N -M` run inside the worktree
func (client *Client) Diff(ctx context.Context, worktree, mergeBase, headSHA string, unified int) (string, error) {
	output, err := client.git(ctx, worktree, "diff", mergeBase, headSHA, "--unified="+strconv.Itoa(unified), "-M")
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(output), nil
}

// git runs a git subcommand in dir
func (client *Client) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return client.runner().Run(ctx, dir, "git", args...)
}

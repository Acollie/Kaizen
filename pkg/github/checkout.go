package github

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alexcollie/kaizen/pkg/review"
)

// defaultRemote is the fallback remote PR refs are fetched from when fetching
// from the base repository's URL fails
const defaultRemote = "origin"

// ErrFetchAuth wraps a fetch from the base repository that failed for lack of credentials
var ErrFetchAuth = errors.New("could not authenticate to fetch the pull request; " +
	"run `gh auth setup-git` so git uses your GitHub CLI login")

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

// fetchPRRefs fetches the PR head ref and the base branch. They are fetched from
// the base repository's URL, because the PR lives there: in a clone where gh
// resolves another remote (origin is a fork, upstream is the real repo),
// refs/pull/<n>/head does not exist on origin. If the URL fetch fails, origin is
// tried as before. Private repositories need git credentials for the URL, which
// `gh auth setup-git` provides; an auth failure says so.
func (client *Client) fetchPRRefs(ctx context.Context, pr *review.PullRequest) error {
	refs := []string{"pull/" + strconv.Itoa(pr.Number) + "/head"}
	if pr.BaseRef != "" {
		refs = append(refs, pr.BaseRef)
	}

	repoURL := baseRepoURL(*pr)
	if repoURL == "" {
		return client.fetchFromOrigin(ctx, pr.Number, refs)
	}
	// Never prompt for a password: the fallback to origin (or the error) is better than a hang
	urlArgs := append([]string{"-c", "credential.interactive=false", "fetch", "--no-tags", repoURL}, refs...)
	_, urlErr := client.git(ctx, client.repoDir(), urlArgs...)
	if urlErr == nil {
		return nil
	}

	originErr := client.fetchFromOrigin(ctx, pr.Number, refs)
	if originErr == nil {
		return nil
	}
	if isAuthFailure(urlErr) {
		return fmt.Errorf("fetch PR #%d from %s: %w (%v); fetching from %s also failed: %v",
			pr.Number, repoURL, ErrFetchAuth, urlErr, defaultRemote, originErr)
	}
	return fmt.Errorf("fetch PR #%d from %s: %v; fetching from %s also failed: %w", pr.Number, repoURL, urlErr, defaultRemote, originErr)
}

// fetchFromOrigin fetches refs from the default remote
func (client *Client) fetchFromOrigin(ctx context.Context, number int, refs []string) error {
	args := append([]string{"fetch", "--no-tags", defaultRemote}, refs...)
	if _, err := client.git(ctx, client.repoDir(), args...); err != nil {
		return fmt.Errorf("fetch PR #%d: %w", number, err)
	}
	return nil
}

// baseRepoURL is the clone URL of the repository the PR targets. It comes from the
// PR's own URL (https://<host>/<owner>/<repo>/pull/<n>, so GitHub Enterprise hosts
// work), else from Owner/Repo on github.com; "" when neither is known.
func baseRepoURL(pr review.PullRequest) string {
	if parsed, err := url.Parse(pr.URL); err == nil && parsed.Scheme == "https" && parsed.Host != "" {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(segments) >= 4 && segments[2] == "pull" && segments[0] != "" && segments[1] != "" {
			return fmt.Sprintf("https://%s/%s/%s.git", parsed.Host, segments[0], segments[1])
		}
	}
	if pr.Owner != "" && pr.Repo != "" {
		return fmt.Sprintf("https://github.com/%s/%s.git", pr.Owner, pr.Repo)
	}
	return ""
}

// isAuthFailure reports whether a git fetch failed for lack of credentials. GitHub
// answers "Repository not found" for private repositories without them.
func isAuthFailure(err error) bool {
	lower := strings.ToLower(errorDetail(err))
	for _, marker := range []string{
		"authentication failed", "could not read username", "could not read password",
		"terminal prompts disabled", "repository not found", "permission denied",
		"http 401", "http 403", "the requested url returned error: 403",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
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

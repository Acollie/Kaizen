package github

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Sentinel errors for the failure modes a user can act on. Errors returned by
// Client wrap one of these where applicable, so callers can use errors.Is.
var (
	ErrNotGitRepo         = errors.New("not a git repository (run kaizen review from inside a clone)")
	ErrGHNotInstalled     = errors.New("the GitHub CLI (gh) is not installed; see https://cli.github.com")
	ErrGHNotAuthenticated = errors.New("the GitHub CLI is not authenticated; run `gh auth login`")
	ErrPRNotFound         = errors.New("pull request not found")
	ErrRepoMismatch       = errors.New("the local clone is not the pull request's repository")
)

// classifyGHError maps a failed gh invocation onto a sentinel error when the
// cause is recognisable, keeping the original error for detail
func classifyGHError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: %v", ErrGHNotInstalled, err)
	}

	lower := strings.ToLower(errorDetail(err))
	switch {
	case strings.Contains(lower, "not a git repository"):
		return fmt.Errorf("%w: %v", ErrNotGitRepo, err)
	case strings.Contains(lower, "gh auth login"),
		strings.Contains(lower, "not logged in"),
		strings.Contains(lower, "authentication required"),
		strings.Contains(lower, "bad credentials"),
		strings.Contains(lower, "http 401"):
		return fmt.Errorf("%w: %v", ErrGHNotAuthenticated, err)
	case strings.Contains(lower, "could not resolve to a pullrequest"),
		strings.Contains(lower, "no pull requests found"),
		strings.Contains(lower, "http 404"):
		return fmt.Errorf("%w: %v", ErrPRNotFound, err)
	}
	return err
}

// errorDetail returns stderr for command errors, else the error text
func errorDetail(err error) string {
	var commandError *CommandError
	if errors.As(err, &commandError) {
		return commandError.Stderr + " " + commandError.Error()
	}
	return err.Error()
}

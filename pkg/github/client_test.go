package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const prViewJSON = `{
  "number": 42,
  "title": "Add review command",
  "body": "Adds kaizen review",
  "author": {"login": "octocat"},
  "url": "https://github.com/acme/widgets/pull/42",
  "baseRefName": "main",
  "headRefName": "feat/review",
  "baseRefOid": "base000",
  "headRefOid": "head111",
  "additions": 120,
  "deletions": 7,
  "files": [
    {"path": "cmd/main.go", "additions": 100, "deletions": 5},
    {"path": "README.md", "additions": 20, "deletions": 2}
  ]
}`

const repoViewJSON = `{"owner": {"login": "acme"}, "name": "widgets"}`

func commandFailure(stderr string) error {
	return &CommandError{Name: "gh", Args: []string{"pr", "view"}, Stderr: stderr, Err: errors.New("exit status 1")}
}

func TestFetchPR(t *testing.T) {
	runner := (&fakeRunner{}).
		respond("gh pr view 42", prViewJSON, nil).
		respond("gh repo view", repoViewJSON, nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}

	pullRequest, files, err := client.FetchPR(context.Background(), 42)
	require.NoError(t, err)

	assert.Equal(t, 42, pullRequest.Number)
	assert.Equal(t, "Add review command", pullRequest.Title)
	assert.Equal(t, "octocat", pullRequest.Author)
	assert.Equal(t, "acme", pullRequest.Owner)
	assert.Equal(t, "widgets", pullRequest.Repo)
	assert.Equal(t, "main", pullRequest.BaseRef)
	assert.Equal(t, "feat/review", pullRequest.HeadRef)
	assert.Equal(t, "base000", pullRequest.BaseSHA)
	assert.Equal(t, "head111", pullRequest.HeadSHA)
	assert.Equal(t, 120, pullRequest.Additions)
	assert.Equal(t, 7, pullRequest.Deletions)
	assert.Equal(t, 2, pullRequest.FileCount)
	assert.Empty(t, pullRequest.MergeBase)
	require.Len(t, files, 2)
	assert.Equal(t, PRFile{Path: "cmd/main.go", Additions: 100, Deletions: 5}, files[0])

	assert.Equal(t, []string{
		"git rev-parse --is-inside-work-tree",
		"gh pr view 42 --json " + prViewFields,
		"gh repo view --json owner,name",
	}, runner.commandLines())
	assert.Equal(t, "/repo", runner.lastCall().Dir)
}

func TestFetchPRErrors(t *testing.T) {
	testCases := []struct {
		name     string
		runner   *fakeRunner
		expected error
	}{
		{
			name:     "not a git repo",
			runner:   (&fakeRunner{}).respond("git rev-parse", "", commandFailure("fatal: not a git repository (or any of the parent directories): .git")),
			expected: ErrNotGitRepo,
		},
		{
			name:     "gh not installed",
			runner:   (&fakeRunner{}).respond("gh pr view", "", &CommandError{Name: "gh", Err: fmt.Errorf("exec: %w", exec.ErrNotFound)}),
			expected: ErrGHNotInstalled,
		},
		{
			name:     "gh not authenticated",
			runner:   (&fakeRunner{}).respond("gh pr view", "", commandFailure("To get started with GitHub CLI, please run:  gh auth login")),
			expected: ErrGHNotAuthenticated,
		},
		{
			name:     "PR not found",
			runner:   (&fakeRunner{}).respond("gh pr view", "", commandFailure("GraphQL: Could not resolve to a PullRequest with the number of 999. (repository.pullRequest)")),
			expected: ErrPRNotFound,
		},
		{
			name:     "repo view unauthenticated",
			runner:   (&fakeRunner{}).respond("gh pr view", prViewJSON, nil).respond("gh repo view", "", commandFailure("HTTP 401: Bad credentials")),
			expected: ErrGHNotAuthenticated,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := &Client{Runner: testCase.runner, RepoDir: "/repo"}
			_, _, err := client.FetchPR(context.Background(), 999)
			require.Error(t, err)
			assert.ErrorIs(t, err, testCase.expected)
		})
	}
}

func TestFetchPRUnclassifiedErrorKeepsStderr(t *testing.T) {
	runner := (&fakeRunner{}).respond("gh pr view", "", commandFailure("something odd happened"))
	client := &Client{Runner: runner}
	_, _, err := client.FetchPR(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "something odd happened")
	assert.NotErrorIs(t, err, ErrPRNotFound)
}

func TestFetchPRBadJSON(t *testing.T) {
	runner := (&fakeRunner{}).respond("gh pr view", "not json", nil)
	client := &Client{Runner: runner}
	_, _, err := client.FetchPR(context.Background(), 1)
	assert.ErrorContains(t, err, "parse gh pr view output")
}

func TestCommandErrorIncludesStderr(t *testing.T) {
	commandError := &CommandError{Name: "git", Args: []string{"diff"}, Stderr: "fatal: bad revision\n", Err: errors.New("exit status 128")}
	assert.Equal(t, "git diff: exit status 128: fatal: bad revision", commandError.Error())
}

func TestExecRunnerCapturesStderrAndStdin(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	runner := ExecRunner{}

	output, err := runner.RunWithInput(context.Background(), "", []byte("hello"), "sh", "-c", "cat")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(output))

	_, err = runner.Run(context.Background(), "", "sh", "-c", "echo oops >&2; exit 3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oops")

	_, err = runner.Run(context.Background(), "", "definitely-not-a-real-binary-kaizen")
	assert.ErrorIs(t, classifyGHError(err), ErrGHNotInstalled)
}

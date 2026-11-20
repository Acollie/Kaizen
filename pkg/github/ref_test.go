package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePRRef(t *testing.T) {
	testCases := []struct {
		input    string
		expected PRRef
	}{
		{input: "42", expected: PRRef{Number: 42}},
		{input: "#42", expected: PRRef{Number: 42}},
		{input: "https://github.com/acme/widgets/pull/42", expected: PRRef{Host: "github.com", Owner: "acme", Repo: "widgets", Number: 42}},
		{input: "https://github.com/acme/widgets/pull/42/files", expected: PRRef{Host: "github.com", Owner: "acme", Repo: "widgets", Number: 42}},
		{input: "github.com/acme/widgets/pull/7", expected: PRRef{Host: "github.com", Owner: "acme", Repo: "widgets", Number: 7}},
		{input: "https://ghe.example.com/team/svc/pull/3", expected: PRRef{Host: "ghe.example.com", Owner: "team", Repo: "svc", Number: 3}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			ref, err := ParsePRRef(testCase.input)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, ref)
		})
	}
}

func TestParsePRRefRejectsInvalid(t *testing.T) {
	for _, input := range []string{"", "abc", "0", "-3", "#", "https://github.com/acme/widgets/issues/4", "https://github.com/acme/pull/4", "https://github.com/acme/widgets/pull/x"} {
		_, err := ParsePRRef(input)
		assert.Error(t, err, input)
	}
}

func TestPRRefRepoFlag(t *testing.T) {
	assert.Equal(t, "acme/widgets", PRRef{Host: "github.com", Owner: "acme", Repo: "widgets"}.RepoFlag())
	assert.Equal(t, "acme/widgets", PRRef{Owner: "acme", Repo: "widgets"}.RepoFlag())
	assert.Equal(t, "ghe.example.com/acme/widgets", PRRef{Host: "ghe.example.com", Owner: "acme", Repo: "widgets"}.RepoFlag())
}

func TestFetchPRRefUsesURLRepository(t *testing.T) {
	runner := (&fakeRunner{}).
		respond("gh pr view 42", prViewJSON, nil).
		respond("gh repo view acme/widgets", `{"owner": {"login": "Acme"}, "name": "widgets"}`, nil).
		respond("gh repo view", `{"owner": {"login": "acme"}, "name": "Widgets"}`, nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}

	ref := PRRef{Host: "github.com", Owner: "acme", Repo: "widgets", Number: 42}
	pullRequest, _, err := client.FetchPRRef(context.Background(), ref)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"git rev-parse --is-inside-work-tree",
		"gh repo view --json owner,name",
		"gh pr view 42 --json " + prViewFields + " --repo acme/widgets",
		"gh repo view acme/widgets --json owner,name",
	}, runner.commandLines())
	// Owner/Repo (what PostReview posts to) come from the URL's repository
	assert.Equal(t, "Acme", pullRequest.Owner)
	assert.Equal(t, "widgets", pullRequest.Repo)
}

func TestFetchPRRefRejectsCloneOfAnotherRepository(t *testing.T) {
	runner := (&fakeRunner{}).
		respond("gh repo view", `{"owner": {"login": "me"}, "name": "kaizen"}`, nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}

	ref := PRRef{Host: "github.com", Owner: "acme", Repo: "widgets", Number: 42}
	_, _, err := client.FetchPRRef(context.Background(), ref)
	require.ErrorIs(t, err, ErrRepoMismatch)
	assert.ErrorContains(t, err, "this is a clone of me/kaizen; the PR is in acme/widgets; run from a clone of acme/widgets")
	for _, line := range runner.commandLines() {
		assert.NotContains(t, line, "gh pr view", "must not fetch the PR before rejecting the clone")
	}
}

func TestFetchPRNumberStaysLocal(t *testing.T) {
	runner := (&fakeRunner{}).
		respond("gh pr view 42", prViewJSON, nil).
		respond("gh repo view", repoViewJSON, nil)
	client := &Client{Runner: runner, RepoDir: "/repo"}

	ref, err := ParsePRRef("#42")
	require.NoError(t, err)
	_, _, err = client.FetchPRRef(context.Background(), ref)
	require.NoError(t, err)
	for _, line := range runner.commandLines() {
		assert.NotContains(t, line, "--repo")
	}
}

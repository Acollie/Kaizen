package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review"
)

// prViewFields are the fields requested from `gh pr view --json`
const prViewFields = "number,title,body,author,url,baseRefName,headRefName,baseRefOid,headRefOid,additions,deletions,files"

// Client talks to GitHub through gh and to the local clone through git
type Client struct {
	Runner  CommandRunner // nil means ExecRunner
	RepoDir string        // the local clone; "" means the current directory
}

// NewClient returns a client using the real gh and git binaries
func NewClient(repoDir string) *Client {
	return &Client{Runner: ExecRunner{}, RepoDir: repoDir}
}

// PRFile is one file from `gh pr view --json files`
type PRFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// prViewResponse mirrors the JSON emitted by `gh pr view --json ...`
type prViewResponse struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	URL         string   `json:"url"`
	BaseRefName string   `json:"baseRefName"`
	HeadRefName string   `json:"headRefName"`
	BaseRefOid  string   `json:"baseRefOid"`
	HeadRefOid  string   `json:"headRefOid"`
	Additions   int      `json:"additions"`
	Deletions   int      `json:"deletions"`
	Files       []PRFile `json:"files"`
}

// repoViewResponse mirrors `gh repo view --json owner,name`
type repoViewResponse struct {
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	Name string `json:"name"`
}

func (client *Client) runner() CommandRunner {
	if client.Runner == nil {
		return ExecRunner{}
	}
	return client.Runner
}

func (client *Client) repoDir() string {
	if client.RepoDir == "" {
		return "."
	}
	return client.RepoDir
}

// FetchPR loads PR metadata for a PR in the local clone's repository. MergeBase
// is left empty; Checkout fills it in.
func (client *Client) FetchPR(ctx context.Context, number int) (*review.PullRequest, []PRFile, error) {
	return client.FetchPRRef(ctx, PRRef{Number: number})
}

// FetchPRRef loads PR metadata and the repository owner/name. A reference that
// names its repository (a PR URL) is fetched from that repository with --repo,
// and RepoDir must be a clone of it, because checkout and diff run locally.
// The owner/name it returns is what PostReview posts to.
func (client *Client) FetchPRRef(ctx context.Context, ref PRRef) (*review.PullRequest, []PRFile, error) {
	if err := client.ensureGitRepo(ctx); err != nil {
		return nil, nil, err
	}

	var repoSpec string
	var repoArgs []string
	if ref.HasRepo() {
		repoSpec = ref.RepoFlag()
		repoArgs = []string{"--repo", repoSpec}
		if err := client.ensureCloneOf(ctx, ref); err != nil {
			return nil, nil, err
		}
	}

	view, err := client.viewPR(ctx, ref.Number, repoArgs)
	if err != nil {
		return nil, nil, err
	}
	// The PR's repository, canonical as GitHub spells it; PostReview posts here
	owner, repo, err := client.repoIdentity(ctx, repoSpec)
	if err != nil {
		return nil, nil, err
	}
	return newPullRequest(view, owner, repo), view.Files, nil
}

// viewPR runs `gh pr view <n> --json ...` with optional --repo arguments
func (client *Client) viewPR(ctx context.Context, number int, repoArgs []string) (*prViewResponse, error) {
	args := append([]string{"pr", "view", strconv.Itoa(number), "--json", prViewFields}, repoArgs...)
	output, err := client.runner().Run(ctx, client.repoDir(), "gh", args...)
	if err != nil {
		return nil, fmt.Errorf("fetch PR #%d: %w", number, classifyGHError(err))
	}
	var view prViewResponse
	if err := json.Unmarshal(output, &view); err != nil {
		return nil, fmt.Errorf("parse gh pr view output: %w", err)
	}
	return &view, nil
}

// ensureCloneOf fails unless the local clone's repository (as gh resolves it) is
// the one the PR reference names
func (client *Client) ensureCloneOf(ctx context.Context, ref PRRef) error {
	localOwner, localRepo, err := client.repoIdentity(ctx, "")
	if err != nil {
		return err
	}
	if strings.EqualFold(localOwner, ref.Owner) && strings.EqualFold(localRepo, ref.Repo) {
		return nil
	}
	local := localOwner + "/" + localRepo
	target := ref.Owner + "/" + ref.Repo
	return fmt.Errorf("%w: this is a clone of %s; the PR is in %s; run from a clone of %s (or pass --repo <path>)",
		ErrRepoMismatch, local, target, target)
}

func newPullRequest(view *prViewResponse, owner, repo string) *review.PullRequest {
	return &review.PullRequest{
		Number:    view.Number,
		Title:     view.Title,
		Body:      view.Body,
		Author:    view.Author.Login,
		URL:       view.URL,
		Owner:     owner,
		Repo:      repo,
		BaseRef:   view.BaseRefName,
		HeadRef:   view.HeadRefName,
		BaseSHA:   view.BaseRefOid,
		HeadSHA:   view.HeadRefOid,
		Additions: view.Additions,
		Deletions: view.Deletions,
		FileCount: len(view.Files),
	}
}

// repoIdentity returns the owner login and repository name of the current repo,
// or of repoSpec ([HOST/]OWNER/REPO) when it is set. `gh repo view` has no
// --repo flag; it takes the repository as a positional argument.
func (client *Client) repoIdentity(ctx context.Context, repoSpec string) (string, string, error) {
	args := []string{"repo", "view"}
	if repoSpec != "" {
		args = append(args, repoSpec)
	}
	args = append(args, "--json", "owner,name")
	output, err := client.runner().Run(ctx, client.repoDir(), "gh", args...)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository: %w", classifyGHError(err))
	}
	var view repoViewResponse
	if err := json.Unmarshal(output, &view); err != nil {
		return "", "", fmt.Errorf("parse gh repo view output: %w", err)
	}
	if view.Owner.Login == "" || view.Name == "" {
		return "", "", fmt.Errorf("gh repo view returned no owner/name")
	}
	return view.Owner.Login, view.Name, nil
}

// ensureGitRepo fails with ErrNotGitRepo when RepoDir is not inside a git work tree
func (client *Client) ensureGitRepo(ctx context.Context) error {
	_, err := client.runner().Run(ctx, client.repoDir(), "git", "rev-parse", "--is-inside-work-tree")
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("git is not installed: %w", err)
	}
	return fmt.Errorf("%w: %v", ErrNotGitRepo, err)
}

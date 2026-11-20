package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"

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

// FetchPR loads PR metadata and the repository owner/name. MergeBase is left
// empty; Checkout fills it in.
func (client *Client) FetchPR(ctx context.Context, number int) (*review.PullRequest, []PRFile, error) {
	if err := client.ensureGitRepo(ctx); err != nil {
		return nil, nil, err
	}

	output, err := client.runner().Run(ctx, client.repoDir(), "gh", "pr", "view", strconv.Itoa(number), "--json", prViewFields)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch PR #%d: %w", number, classifyGHError(err))
	}
	var view prViewResponse
	if err := json.Unmarshal(output, &view); err != nil {
		return nil, nil, fmt.Errorf("parse gh pr view output: %w", err)
	}

	owner, repo, err := client.repoIdentity(ctx)
	if err != nil {
		return nil, nil, err
	}

	pullRequest := &review.PullRequest{
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
	return pullRequest, view.Files, nil
}

// repoIdentity returns the owner login and repository name of the current repo
func (client *Client) repoIdentity(ctx context.Context) (string, string, error) {
	output, err := client.runner().Run(ctx, client.repoDir(), "gh", "repo", "view", "--json", "owner,name")
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

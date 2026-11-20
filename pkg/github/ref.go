package github

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// defaultHost is the GitHub host gh assumes when --repo carries no host
const defaultHost = "github.com"

// PRRef identifies a pull request. Owner and Repo are empty for a bare number,
// which means "the PR in the local clone's repository".
type PRRef struct {
	Host   string // "" or github.com for github.com
	Owner  string
	Repo   string
	Number int
}

// HasRepo reports whether the reference names its repository explicitly
func (ref PRRef) HasRepo() bool {
	return ref.Owner != "" && ref.Repo != ""
}

// RepoFlag is the value for gh's --repo flag: OWNER/REPO, or HOST/OWNER/REPO
// for a host other than github.com
func (ref PRRef) RepoFlag() string {
	if ref.Host == "" || strings.EqualFold(ref.Host, defaultHost) {
		return ref.Owner + "/" + ref.Repo
	}
	return ref.Host + "/" + ref.Owner + "/" + ref.Repo
}

// ParsePRRef accepts "123", "#123" or a PR URL like
// https://github.com/owner/repo/pull/123 (extra path segments such as /files are ignored)
func ParsePRRef(reference string) (PRRef, error) {
	trimmed := strings.TrimSpace(reference)
	if strings.Contains(trimmed, "/") {
		return parsePRURL(trimmed)
	}
	number, err := parsePRNumber(strings.TrimPrefix(trimmed, "#"))
	if err != nil {
		return PRRef{}, fmt.Errorf("invalid pull request %q: expected a number like 42 or a PR URL", reference)
	}
	return PRRef{Number: number}, nil
}

// parsePRURL parses https://<host>/<owner>/<repo>/pull/<n>[/...]
func parsePRURL(reference string) (PRRef, error) {
	invalid := fmt.Errorf("invalid pull request URL %q: expected https://github.com/<owner>/<repo>/pull/<number>", reference)
	if !strings.Contains(reference, "://") {
		reference = "https://" + reference
	}
	parsed, err := url.Parse(reference)
	if err != nil || parsed.Host == "" {
		return PRRef{}, invalid
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 4 || segments[0] == "" || segments[1] == "" || segments[2] != "pull" {
		return PRRef{}, invalid
	}
	number, err := parsePRNumber(segments[3])
	if err != nil {
		return PRRef{}, invalid
	}
	return PRRef{
		Host:   parsed.Host,
		Owner:  segments[0],
		Repo:   strings.TrimSuffix(segments[1], ".git"),
		Number: number,
	}, nil
}

func parsePRNumber(text string) (int, error) {
	number, err := strconv.Atoi(text)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("not a positive number: %q", text)
	}
	return number, nil
}

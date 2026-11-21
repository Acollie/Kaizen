package churn

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

// GitChurnAnalyzer implements the ChurnAnalyzer interface using git commands
type GitChurnAnalyzer struct {
	repoPath string
}

// NewGitChurnAnalyzer creates a new git churn analyzer
func NewGitChurnAnalyzer(repoPath string) *GitChurnAnalyzer {
	return &GitChurnAnalyzer{
		repoPath: repoPath,
	}
}

// IsGitRepository checks if the path is in a git repository
func (analyzer *GitChurnAnalyzer) IsGitRepository(repoPath string) bool {
	command := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	command.Dir = repoPath
	err := command.Run()
	return err == nil
}

// GetFileChurn analyzes churn for a specific file
func (analyzer *GitChurnAnalyzer) GetFileChurn(filePath string, since time.Time) (*models.ChurnMetric, error) {
	// Check if we're in a git repository
	if !analyzer.IsGitRepository(analyzer.repoPath) {
		return nil, fmt.Errorf("not a git repository: %s", analyzer.repoPath)
	}

	// Make path relative to repo root
	gitRoot, relPath, err := analyzer.getRelativePath(filePath)
	if err != nil {
		return nil, err
	}

	// Get numstat data: lines added/deleted per commit
	sinceStr := since.Format("2006-01-02")
	command := exec.Command("git", "log",
		fmt.Sprintf("--since=%s", sinceStr),
		"--numstat",
		"--follow",
		"--format=%H|%an|%ae|%ad",
		"--date=iso",
		"--", relPath)
	command.Dir = gitRoot

	output, err := command.Output()
	if err != nil {
		// File might not exist in git history
		return &models.ChurnMetric{}, nil
	}

	return analyzer.parseNumstatOutput(string(output))
}

// getRelativePath returns the git root and filePath relative to it. git commands
// must run in that root: git resolves paths against its working directory, so a
// root-relative path run from a subdirectory (kaizen analyze --path=pkg/x) misses.
func (analyzer *GitChurnAnalyzer) getRelativePath(filePath string) (string, string, error) {
	command := exec.Command("git", "rev-parse", "--show-toplevel")
	command.Dir = analyzer.repoPath
	output, err := command.Output()
	if err != nil {
		return "", "", err
	}

	gitRoot := resolvePath(strings.TrimSpace(string(output)))
	relPath, err := filepath.Rel(gitRoot, resolvePath(filePath))
	if err != nil {
		return "", "", err
	}

	return gitRoot, filepath.ToSlash(relPath), nil
}

// resolvePath makes a path absolute and symlink-free where possible, so a file
// under /var/... and a git root reported as /private/var/... (macOS) still relate
func resolvePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// parseNumstatOutput parses the output of git log --numstat
func (analyzer *GitChurnAnalyzer) parseNumstatOutput(output string) (*models.ChurnMetric, error) {
	lines := strings.Split(output, "\n")

	metric := &models.ChurnMetric{
		Contributors: []string{},
	}

	authorSet := make(map[string]bool)
	var lastModified time.Time

	currentCommit := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a commit line (contains |)
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				currentCommit = parts[0]
				authorName := parts[1]
				commitDate := parts[3]

				metric.TotalCommits++

				// Track unique contributors
				if !authorSet[authorName] {
					authorSet[authorName] = true
					metric.Contributors = append(metric.Contributors, authorName)
				}

				// Track last modified
				parsedDate, err := time.Parse("2006-01-02 15:04:05 -0700", commitDate)
				if err == nil {
					if lastModified.IsZero() || parsedDate.After(lastModified) {
						lastModified = parsedDate
					}
				}
			}
		} else {
			// This is a numstat line: <added> <deleted> <filename>
			fields := strings.Fields(line)
			if len(fields) >= 2 && currentCommit != "" {
				added, err1 := strconv.Atoi(fields[0])
				deleted, err2 := strconv.Atoi(fields[1])

				if err1 == nil && err2 == nil {
					metric.LinesAdded += added
					metric.LinesDeleted += deleted
				}
			}
		}
	}

	metric.TotalChanges = metric.LinesAdded + metric.LinesDeleted
	metric.LastModified = lastModified
	metric.AuthorCount = len(metric.Contributors)

	// Calculate average days between changes
	if metric.TotalCommits > 1 {
		daysSince := time.Since(lastModified).Hours() / 24
		metric.AverageChurnBy = daysSince / float64(metric.TotalCommits)
	}

	return metric, nil
}

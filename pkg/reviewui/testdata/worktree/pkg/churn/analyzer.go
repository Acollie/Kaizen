// Package churn measures how often files and functions change using git history.
package churn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

// maxGitAttempts bounds how many times a git command is retried
const maxGitAttempts = 4

// Analyzer reads churn metrics from a git repository
type Analyzer struct {
	repoPath string
	since    time.Time
	cache    *churnCache
	backoff  time.Duration
}

// NewAnalyzer creates a churn analyzer rooted at repoPath
func NewAnalyzer(repoPath string, since time.Time) *Analyzer {
	return &Analyzer{
		repoPath: repoPath,
		since:    since,
		cache:    newChurnCache(512),
		backoff:  200 * time.Millisecond,
	}
}

// GetFileChurn returns commit, addition and deletion counts for one file
func (analyzer *Analyzer) GetFileChurn(ctx context.Context, filePath string) (*models.ChurnMetric, error) {
	relativePath, err := filepath.Rel(analyzer.repoPath, filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", filePath, err)
	}
	relativePath = filepath.ToSlash(relativePath)

	if cached, ok := analyzer.cache.Get(relativePath); ok {
		return cached, nil
	}

	args := []string{
		"log", "--numstat", "--format=%H|%an|%at",
		"--since=" + analyzer.since.Format(time.RFC3339),
		"--", relativePath,
	}
	output, err := analyzer.runGitWithRetry(ctx, args...)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return &models.ChurnMetric{}, nil
	}

	metric, err := parseNumstat(output)
	if err != nil {
		return nil, fmt.Errorf("parse churn for %s: %w", relativePath, err)
	}
	if metric.TotalCommits > 0 {
		metric.ChurnScore = normalizeChurnScore(metric.TotalChanges, metric.TotalCommits)
	}

	analyzer.cache.Put(relativePath, metric)
	return metric, nil
}

// runGitWithRetry runs a git command, retrying transient failures with exponential backoff
func (analyzer *Analyzer) runGitWithRetry(ctx context.Context, args ...string) ([]byte, error) {
	var lastErr error
	delay := analyzer.backoff

	for attempt := 0; attempt < maxGitAttempts; attempt++ {
		command := exec.CommandContext(ctx, "git", args...)
		command.Dir = analyzer.repoPath

		output, err := command.Output()
		if err == nil {
			return output, nil
		}
		lastErr = err

		if !isTransientGitError(err) {
			break
		}

		time.Sleep(delay)
		delay *= 2
	}

	return nil, fmt.Errorf("git %s failed after retries: %w", args[0], lastErr)
}

// isTransientGitError reports whether a git failure is worth retrying (lock contention, I/O hiccups)
func isTransientGitError(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	stderr := string(exitErr.Stderr)
	return strings.Contains(stderr, "index.lock") ||
		strings.Contains(stderr, "Resource temporarily unavailable")
}

// parseNumstat folds `git log --numstat` output into a churn metric
func parseNumstat(output []byte) (*models.ChurnMetric, error) {
	metric := &models.ChurnMetric{}
	authors := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if header := strings.Split(line, "|"); len(header) == 3 {
			metric.TotalCommits++
			authors[header[1]] = true
			if timestamp, err := strconv.ParseInt(header[2], 10, 64); err == nil {
				commitTime := time.Unix(timestamp, 0)
				if commitTime.After(metric.LastModified) {
					metric.LastModified = commitTime
				}
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] == "-" {
			continue // binary files report "-" for both counts
		}
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		metric.LinesAdded += added
		metric.LinesDeleted += deleted
	}

	metric.TotalChanges = metric.LinesAdded + metric.LinesDeleted
	metric.AuthorCount = len(authors)
	for author := range authors {
		metric.Contributors = append(metric.Contributors, author)
	}
	return metric, scanner.Err()
}

// normalizeChurnScore maps raw change volume onto a 0-100 scale
func normalizeChurnScore(totalChanges, commits int) float64 {
	score := float64(totalChanges) / float64(commits) * 2.5
	if score > 100 {
		return 100
	}
	return score
}

// GetFunctionChurn counts commits touching one function using `git log -L` (experimental)
func (analyzer *Analyzer) GetFunctionChurn(ctx context.Context, filePath, functionName string) (int, error) {
	relativePath, err := filepath.Rel(analyzer.repoPath, filePath)
	if err != nil {
		return 0, err
	}
	command := exec.CommandContext(ctx, "git", "log", "--format=%H", "-L",
		fmt.Sprintf(":%s:%s", functionName, filepath.ToSlash(relativePath)))
	command.Dir = analyzer.repoPath

	output, err := command.Output()
	if err != nil {
		if isTransientGitError(err) {
			return 0, fmt.Errorf("git busy, try again: %w", err)
		}
		return 0, nil
	}

	commits := 0
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) == 40 {
			commits++
		}
	}
	return commits, nil
}

package churn

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

// functionLogCommitMarker starts the per-commit line of `git log -L` output. Diff
// lines start with "+", "-", " ", "@@" or "diff", so a line starting with the
// marker can only be the commit header, whatever the function's code contains.
const functionLogCommitMarker = "kaizen-commit:"

// gitDateLayout is how --date=iso formats commit dates
const gitDateLayout = "2006-01-02 15:04:05 -0700"

// GetFunctionChurn analyzes churn for a function found by name. It is supported for
// Go files only: the function's current line range is located with go/parser (the
// first declaration with that name, method or not) and passed to
// GetFunctionChurnInRange. Errors mean churn is unknown; callers must not treat
// them as zero churn.
func (analyzer *GitChurnAnalyzer) GetFunctionChurn(filePath string, functionName string, since time.Time) (*models.ChurnMetric, error) {
	if !analyzer.IsGitRepository(analyzer.repoPath) {
		return nil, fmt.Errorf("not a git repository: %s", analyzer.repoPath)
	}
	startLine, endLine, err := findGoFunctionRange(filePath, functionName)
	if err != nil {
		return nil, err
	}
	return analyzer.GetFunctionChurnInRange(filePath, startLine, endLine, since)
}

// GetFunctionChurnInRange counts the commits since `since` that touched lines
// startLine..endLine (1-based, inclusive, as they are at HEAD) of filePath, using
// `git log -L <start>,<end>:<file>`, which follows the range back through history.
// Any git failure is returned as an error: churn is then unknown, never zero.
func (analyzer *GitChurnAnalyzer) GetFunctionChurnInRange(filePath string, startLine, endLine int, since time.Time) (*models.ChurnMetric, error) {
	if startLine < 1 || endLine < startLine {
		return nil, fmt.Errorf("invalid line range %d-%d for %s", startLine, endLine, filePath)
	}
	if !analyzer.IsGitRepository(analyzer.repoPath) {
		return nil, fmt.Errorf("not a git repository: %s", analyzer.repoPath)
	}
	gitRoot, relPath, err := analyzer.getRelativePath(filePath)
	if err != nil {
		return nil, err
	}

	command := exec.Command("git", "log",
		fmt.Sprintf("-L%d,%d:%s", startLine, endLine, relPath),
		"--since="+since.Format("2006-01-02"),
		"--format="+functionLogCommitMarker+"%H|%an|%ae|%ad",
		"--date=iso",
		"--no-color", "--no-ext-diff")
	command.Dir = gitRoot

	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git log -L %d,%d:%s: %w: %s", startLine, endLine, relPath, err, strings.TrimSpace(stderr.String()))
	}
	return analyzer.parseFunctionLogOutput(string(output))
}

// findGoFunctionRange returns the line range of the first function or method
// declared with the given name in a Go file
func findGoFunctionRange(filePath, functionName string) (int, int, error) {
	if filepath.Ext(filePath) != ".go" {
		return 0, 0, fmt.Errorf("function churn by name is only supported for Go files: %s", filePath)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.SkipObjectResolution)
	if err != nil {
		return 0, 0, fmt.Errorf("parse %s: %w", filePath, err)
	}
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Name.Name != functionName {
			continue
		}
		// Pos is the func keyword, after any doc comment
		return fileSet.Position(function.Pos()).Line, fileSet.Position(function.End()).Line, nil
	}
	return 0, 0, fmt.Errorf("function %s not found in %s", functionName, filePath)
}

// parseFunctionLogOutput parses `git log -L` output: one marker line per commit,
// followed by that commit's diff of the tracked range. Only lines inside @@ hunks
// count as added or removed, so ---/+++ headers are never counted.
func (analyzer *GitChurnAnalyzer) parseFunctionLogOutput(output string) (*models.ChurnMetric, error) {
	metric := &models.ChurnMetric{Contributors: []string{}}
	authorSet := make(map[string]bool)
	inHunk := false

	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, functionLogCommitMarker):
			inHunk = false
			recordFunctionCommit(metric, authorSet, strings.TrimPrefix(line, functionLogCommitMarker))
		case strings.HasPrefix(line, "diff --git "):
			inHunk = false
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && strings.HasPrefix(line, "+"):
			metric.LinesAdded++
		case inHunk && strings.HasPrefix(line, "-"):
			metric.LinesDeleted++
		}
	}

	metric.TotalChanges = metric.LinesAdded + metric.LinesDeleted
	metric.AuthorCount = len(metric.Contributors)
	return metric, nil
}

// recordFunctionCommit adds one "<sha>|<author>|<email>|<date>" commit to metric
func recordFunctionCommit(metric *models.ChurnMetric, authorSet map[string]bool, header string) {
	parts := strings.SplitN(header, "|", 4)
	if len(parts) < 4 {
		return
	}
	metric.TotalCommits++
	authorName := parts[1]
	if !authorSet[authorName] {
		authorSet[authorName] = true
		metric.Contributors = append(metric.Contributors, authorName)
	}
	if parsedDate, err := time.Parse(gitDateLayout, strings.TrimSpace(parts[3])); err == nil {
		if parsedDate.After(metric.LastModified) {
			metric.LastModified = parsedDate
		}
	}
}

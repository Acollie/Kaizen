package reviewui

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review"
)

const (
	maxLinesPerRequest = 500
	maxServedFileBytes = 8 << 20
)

// FileLine is one numbered line of a head-side file
type FileLine struct {
	Number int    `json:"n"`
	Text   string `json:"text"`
}

// FileSlice is the /api/file response: a window of lines plus the file's total line count
type FileSlice struct {
	Lines []FileLine `json:"lines"`
	Total int        `json:"total"`
}

var errPathRejected = errors.New("path rejected")

// allowedFilePaths lists the repo-relative files the UI may read: files in the
// PR (except deletions, which have no head version) and the files of callers
// and callees of changed functions.
func allowedFilePaths(reviewContext *review.ReviewContext) map[string]bool {
	allowed := make(map[string]bool)
	for _, file := range reviewContext.Files {
		if file.Status != review.FileDeleted && !file.Binary {
			allowed[cleanRelativePath(file.Path)] = true
		}
	}
	for _, function := range reviewContext.Functions {
		allowed[cleanRelativePath(function.File)] = true
		for _, related := range append(append([]review.FunctionRef{}, function.Callers...), function.Callees...) {
			if !related.External && related.File != "" {
				allowed[cleanRelativePath(related.File)] = true
			}
		}
	}
	delete(allowed, "")
	return allowed
}

// cleanRelativePath normalises a slash-separated relative path, returning "" for anything suspicious
func cleanRelativePath(relativePath string) string {
	if relativePath == "" || strings.ContainsAny(relativePath, "\\\x00") ||
		strings.HasPrefix(relativePath, "/") || filepath.IsAbs(relativePath) {
		return ""
	}
	for _, segment := range strings.Split(relativePath, "/") {
		if segment == ".." {
			return ""
		}
	}
	cleaned := path.Clean(relativePath)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

// resolveInsideWorktree maps a relative path to a regular file inside the
// worktree, following symlinks and rejecting anything that resolves outside it.
func resolveInsideWorktree(worktreePath, relativePath string) (string, error) {
	cleaned := cleanRelativePath(relativePath)
	if cleaned == "" || worktreePath == "" {
		return "", errPathRejected
	}
	root, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		return "", fmt.Errorf("%w: worktree unavailable", errPathRejected)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(cleaned)))
	if err != nil {
		return "", fmt.Errorf("%w: file not found", errPathRejected)
	}
	if !isWithin(root, resolved) {
		return "", fmt.Errorf("%w: escapes the worktree", errPathRejected)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxServedFileBytes {
		return "", fmt.Errorf("%w: not a readable regular file", errPathRejected)
	}
	return resolved, nil
}

// isWithin reports whether target is root or a descendant of it
func isWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// readFileLines splits a file into lines without trailing newline or carriage return
func readFileLines(fullPath string) ([]string, error) {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(content), "\n")
	if text == "" {
		return []string{}, nil
	}
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines, nil
}

// sliceLines returns lines from..to (1-based, inclusive), clamped to the file
// and capped at maxLinesPerRequest. A zero from means 1; a zero to means as many as allowed.
func sliceLines(lines []string, from, to int) FileSlice {
	total := len(lines)
	if from < 1 {
		from = 1
	}
	if to == 0 || to > from+maxLinesPerRequest-1 {
		to = from + maxLinesPerRequest - 1
	}
	if to > total {
		to = total
	}
	slice := FileSlice{Lines: []FileLine{}, Total: total}
	for number := from; number <= to; number++ {
		slice.Lines = append(slice.Lines, FileLine{Number: number, Text: lines[number-1]})
	}
	return slice
}

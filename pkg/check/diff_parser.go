package check

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DiffHunk represents a changed section in a diff
type DiffHunk struct {
	FilePath string // repo-relative head-side path, from the +++ b/ line (or the diff --git header)
	NewStart int    // 1-based start line in the new file
	NewCount int    // number of changed lines (0 = deletion-only hunk, skip it)
}

// hunkHeaderRegex parses @@ -old_start[,old_count] +new_start[,new_count] @@
var hunkHeaderRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)?\s\+(\d+)(?:,(\d+))?\s@@`)

// RunGitDiff shells out to git and returns the unified diff output
func RunGitDiff(repoPath, baseBranch string) (string, error) {
	command := exec.Command("git", "diff", fmt.Sprintf("%s...HEAD", baseBranch), "--unified=0")
	command.Dir = repoPath

	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	return string(output), nil
}

// ParseDiffOutput parses unified diff output into structured hunks. The file of each
// hunk comes from the "+++ b/<path>" line (git C-quoting undone; /dev/null means the
// file was deleted and has no head-side hunks), falling back to the diff --git header
// for diffs without one. Every diff --git header resets the current file, so a header
// that cannot be parsed never leaves hunks attributed to the previous file.
func ParseDiffOutput(diffText string) ([]DiffHunk, error) {
	if diffText == "" {
		return nil, nil
	}

	var hunks []DiffHunk
	var currentFile string
	inHeader := false

	for _, line := range strings.Split(diffText, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			currentFile = headerNewPath(strings.TrimPrefix(line, "diff --git "))
			inHeader = true
		case inHeader && strings.HasPrefix(line, "+++ "):
			currentFile = sidePath(strings.TrimPrefix(line, "+++ "), "b/")
		case inHeader && strings.HasPrefix(line, "Binary files"):
			currentFile = ""
		case strings.HasPrefix(line, "@@"):
			inHeader = false
			if hunk, ok := parseHunkHeader(line, currentFile); ok {
				hunks = append(hunks, hunk)
			}
		}
	}

	return hunks, nil
}

// parseHunkHeader turns an @@ line into a hunk of filePath. Pure deletions
// (new count 0) and hunks without a known file are skipped.
func parseHunkHeader(line, filePath string) (DiffHunk, bool) {
	if filePath == "" {
		return DiffHunk{}, false
	}
	match := hunkHeaderRegex.FindStringSubmatch(line)
	if len(match) < 2 {
		return DiffHunk{}, false
	}
	newStart, err := strconv.Atoi(match[1])
	if err != nil {
		return DiffHunk{}, false
	}
	newCount := 1 // default when the count is omitted
	if len(match) >= 3 && match[2] != "" {
		if count, countErr := strconv.Atoi(match[2]); countErr == nil {
			newCount = count
		}
	}
	if newCount == 0 {
		return DiffHunk{}, false
	}
	return DiffHunk{FilePath: filePath, NewStart: newStart, NewCount: newCount}, true
}

// headerNewPath extracts the b/ path from what follows "diff --git ". It handles
// quoted paths and unquoted paths with spaces; "" when it cannot tell.
func headerNewPath(rest string) string {
	if strings.HasSuffix(rest, "\"") {
		opening := strings.LastIndex(rest[:len(rest)-1], " \"")
		if opening < 0 {
			return ""
		}
		return sidePath(rest[opening+1:], "b/")
	}

	// Identical paths, "a/P b/P": the split is exactly in the middle
	if (len(rest)-1)%2 == 0 {
		half := (len(rest) - 1) / 2
		left, right := rest[:half], rest[half+1:]
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") && left[2:] == right[2:] {
			return right[2:]
		}
	}

	separator := strings.LastIndex(rest, " b/")
	if separator < 0 {
		return ""
	}
	return rest[separator+3:]
}

// sidePath unquotes a path from a ---/+++ line or header and strips its side
// prefix. It returns "" for /dev/null.
func sidePath(rawPath, sidePrefix string) string {
	// git appends a tab after names containing spaces
	path := unquoteGitPath(strings.TrimRight(rawPath, "\t\r"))
	if path == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(path, sidePrefix)
}

// unquoteGitPath decodes a C-style quoted path as git emits it ("a/x\ty", octal
// escapes for non-ASCII bytes), leaving unquoted paths alone
func unquoteGitPath(path string) string {
	if len(path) < 2 || !strings.HasPrefix(path, "\"") || !strings.HasSuffix(path, "\"") {
		return path
	}
	if unquoted, err := strconv.Unquote(path); err == nil {
		return unquoted
	}
	return path[1 : len(path)-1]
}

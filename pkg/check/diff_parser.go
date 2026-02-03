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
	FilePath  string // repo-relative, from the b/ side of diff --git
	NewStart  int    // 1-based start line in the new file
	NewCount  int    // number of changed lines (0 = deletion-only hunk, skip it)
}

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

// ParseDiffOutput parses unified diff output into structured hunks
func ParseDiffOutput(diffText string) ([]DiffHunk, error) {
	if diffText == "" {
		return nil, nil
	}

	var hunks []DiffHunk
	var currentFile string

	// Regex to parse @@ -old_start[,old_count] +new_start[,new_count] @@
	hunkHeaderRegex := regexp.MustCompile(`^@@ -\d+(?:,\d+)?\s\+(\d+)(?:,(\d+))?\s@@`)

	lines := strings.Split(diffText, "\n")
	for _, line := range lines {
		// Parse file path from "diff --git a/... b/..." line
		if strings.HasPrefix(line, "diff --git a/") {
			// Extract path from "b/" side (handles renames)
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// The last part is "b/<path>"
				bPart := parts[len(parts)-1]
				if strings.HasPrefix(bPart, "b/") {
					currentFile = bPart[2:] // Strip "b/" prefix
				}
			}
			continue
		}

		// Skip binary file markers
		if strings.HasPrefix(line, "Binary files") {
			currentFile = ""
			continue
		}

		// Parse hunk header
		if strings.HasPrefix(line, "@@") {
			if currentFile == "" {
				continue // Skip if no file context
			}

			match := hunkHeaderRegex.FindStringSubmatch(line)
			if len(match) >= 2 {
				newStart, err := strconv.Atoi(match[1])
				if err != nil {
					continue
				}

				newCount := 1 // Default if count is missing
				if len(match) >= 3 && match[2] != "" {
					count, err := strconv.Atoi(match[2])
					if err == nil {
						newCount = count
					}
				}

				// Skip hunks that represent pure deletions (newCount == 0)
				if newCount > 0 {
					hunks = append(hunks, DiffHunk{
						FilePath: currentFile,
						NewStart: newStart,
						NewCount: newCount,
					})
				}
			}
		}
	}

	return hunks, nil
}

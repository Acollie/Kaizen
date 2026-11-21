package review

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// hunkHeaderPattern matches "@@ -old_start[,old_count] +new_start[,new_count] @@ header"
var hunkHeaderPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ ?(.*)$`)

// LanguageForPath maps a file extension to the language name used across Kaizen,
// or "" when the language is not supported
func LanguageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".kt", ".kts":
		return "Kotlin"
	case ".swift":
		return "Swift"
	default:
		return ""
	}
}

// ParseUnifiedDiff parses the output of `git diff -M --unified=N` into one
// FileDiff per file. Line numbers on each DiffLine are 1-based; the side a line
// does not exist on is 0. LineCount and RiskScore are left for the caller.
func ParseUnifiedDiff(diffText string) ([]FileDiff, error) {
	parser := &unifiedDiffParser{}
	for lineNumber, line := range strings.Split(diffText, "\n") {
		if err := parser.consume(line); err != nil {
			return nil, fmt.Errorf("diff line %d: %w", lineNumber+1, err)
		}
	}
	parser.finishFile()
	return parser.files, nil
}

// unifiedDiffParser is a line-at-a-time state machine over git diff output
type unifiedDiffParser struct {
	files        []FileDiff
	current      *FileDiff
	hunk         *Hunk
	oldRemaining int // lines still expected on the old side of the open hunk
	newRemaining int // lines still expected on the new side of the open hunk
	oldLine      int // next old-side line number
	newLine      int // next new-side line number
}

// consume routes one line of diff text to the right handler
func (parser *unifiedDiffParser) consume(line string) error {
	if strings.HasPrefix(line, "diff --git ") {
		parser.startFile(line)
		return nil
	}
	if parser.current == nil {
		return nil // preamble before the first file, e.g. from `git show`
	}
	if parser.hunkOpen() {
		parser.consumeHunkLine(line)
		return nil
	}
	if strings.HasPrefix(line, "\\") && parser.hunk != nil {
		return nil // "\ No newline at end of file" right after the last line of a hunk
	}
	if strings.HasPrefix(line, "@@") {
		return parser.startHunk(line)
	}
	parser.consumeHeaderLine(line)
	return nil
}

// hunkOpen reports whether the current hunk still expects body lines
func (parser *unifiedDiffParser) hunkOpen() bool {
	return parser.hunk != nil && (parser.oldRemaining > 0 || parser.newRemaining > 0)
}

// startFile closes the previous file and opens a new one from a "diff --git" line
func (parser *unifiedDiffParser) startFile(line string) {
	parser.finishFile()
	oldPath, newPath := parseDiffGitPaths(strings.TrimPrefix(line, "diff --git "))
	parser.current = &FileDiff{
		Path:    newPath,
		OldPath: oldPath,
		Status:  FileModified,
		Hunks:   []Hunk{},
	}
}

// finishFile appends the open file (if any) to the result
func (parser *unifiedDiffParser) finishFile() {
	parser.closeHunk()
	if parser.current == nil {
		return
	}
	file := parser.current
	if file.Status == FileDeleted && file.Path == "" {
		file.Path = file.OldPath
	}
	if file.OldPath == "" {
		file.OldPath = file.Path
	}
	file.Language = LanguageForPath(file.Path)
	parser.files = append(parser.files, *file)
	parser.current = nil
}

// consumeHeaderLine handles the extended header lines between "diff --git" and the first hunk
func (parser *unifiedDiffParser) consumeHeaderLine(line string) {
	file := parser.current
	switch {
	case strings.HasPrefix(line, "new file mode"):
		file.Status = FileAdded
	case strings.HasPrefix(line, "deleted file mode"):
		file.Status = FileDeleted
	case strings.HasPrefix(line, "rename from "):
		file.Status = FileRenamed
		file.OldPath = unquoteGitPath(strings.TrimPrefix(line, "rename from "))
	case strings.HasPrefix(line, "rename to "):
		file.Status = FileRenamed
		file.Path = unquoteGitPath(strings.TrimPrefix(line, "rename to "))
	case strings.HasPrefix(line, "Binary files "), strings.HasPrefix(line, "GIT binary patch"):
		file.Binary = true
	case strings.HasPrefix(line, "--- "):
		if path := stripSidePrefix(strings.TrimPrefix(line, "--- "), "a/"); path != "" {
			file.OldPath = path
		}
	case strings.HasPrefix(line, "+++ "):
		if path := stripSidePrefix(strings.TrimPrefix(line, "+++ "), "b/"); path != "" {
			file.Path = path
		}
	}
}

// startHunk opens a hunk from an "@@ ... @@" header line
func (parser *unifiedDiffParser) startHunk(line string) error {
	parser.closeHunk()
	match := hunkHeaderPattern.FindStringSubmatch(line)
	if match == nil {
		return fmt.Errorf("malformed hunk header %q", line)
	}

	hunk := &Hunk{
		OldStart: atoiOrDefault(match[1], 0),
		OldCount: atoiOrDefault(match[2], 1),
		NewStart: atoiOrDefault(match[3], 0),
		NewCount: atoiOrDefault(match[4], 1),
		Header:   strings.TrimSpace(match[5]),
		Lines:    []DiffLine{},
	}

	parser.hunk = hunk
	parser.oldRemaining = hunk.OldCount
	parser.newRemaining = hunk.NewCount
	parser.oldLine = hunk.OldStart
	parser.newLine = hunk.NewStart
	return nil
}

// consumeHunkLine records one body line of the open hunk
func (parser *unifiedDiffParser) consumeHunkLine(line string) {
	if strings.HasPrefix(line, "\\") {
		return // "\ No newline at end of file" between the removed and added sides
	}

	prefix, text := splitHunkLine(line)
	switch prefix {
	case '+':
		parser.appendLine(DiffLine{Kind: LineAdded, NewLine: parser.newLine, Text: text})
		parser.current.Additions++
		parser.newLine++
		parser.newRemaining--
	case '-':
		parser.appendLine(DiffLine{Kind: LineRemoved, OldLine: parser.oldLine, Text: text})
		parser.current.Deletions++
		parser.oldLine++
		parser.oldRemaining--
	default:
		parser.appendLine(DiffLine{Kind: LineContext, OldLine: parser.oldLine, NewLine: parser.newLine, Text: text})
		parser.oldLine++
		parser.newLine++
		parser.oldRemaining--
		parser.newRemaining--
	}
}

// appendLine adds a line to the open hunk
func (parser *unifiedDiffParser) appendLine(diffLine DiffLine) {
	parser.hunk.Lines = append(parser.hunk.Lines, diffLine)
}

// closeHunk moves the open hunk (if any) onto the current file
func (parser *unifiedDiffParser) closeHunk() {
	if parser.hunk == nil || parser.current == nil {
		parser.hunk = nil
		return
	}
	parser.current.Hunks = append(parser.current.Hunks, *parser.hunk)
	parser.hunk = nil
	parser.oldRemaining = 0
	parser.newRemaining = 0
}

// splitHunkLine separates the +/-/space prefix from the content. An empty line
// is treated as an empty context line (some tools strip the trailing space).
func splitHunkLine(line string) (byte, string) {
	if line == "" {
		return ' ', ""
	}
	return line[0], line[1:]
}

// parseDiffGitPaths extracts the a/ and b/ paths from the rest of a "diff --git" line
func parseDiffGitPaths(rest string) (string, string) {
	if strings.HasPrefix(rest, "\"") {
		return parseQuotedDiffGitPaths(rest)
	}

	// Common case: identical paths, "a/P b/P", so the split is exactly in the middle
	if (len(rest)-1)%2 == 0 {
		half := (len(rest) - 1) / 2
		left, right := rest[:half], rest[half+1:]
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") && left[2:] == right[2:] {
			return left[2:], right[2:]
		}
	}

	separator := strings.LastIndex(rest, " b/")
	if separator < 0 {
		return "", ""
	}
	return strings.TrimPrefix(rest[:separator], "a/"), rest[separator+3:]
}

// parseQuotedDiffGitPaths handles headers where git quoted the paths because
// they contain special characters
func parseQuotedDiffGitPaths(rest string) (string, string) {
	closing := strings.Index(rest[1:], "\" ")
	if closing < 0 {
		return "", ""
	}
	oldPath := stripSidePrefix(rest[:closing+2], "a/")
	newPath := stripSidePrefix(strings.TrimSpace(rest[closing+3:]), "b/")
	return oldPath, newPath
}

// stripSidePrefix unquotes a path from a ---/+++ line and removes its a/ or b/
// prefix. It returns "" for /dev/null.
func stripSidePrefix(rawPath, sidePrefix string) string {
	path := unquoteGitPath(strings.TrimRight(rawPath, "\t"))
	if path == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(path, sidePrefix)
}

// unquoteGitPath decodes a C-style quoted path as git emits it, leaving unquoted paths alone
func unquoteGitPath(path string) string {
	if len(path) < 2 || !strings.HasPrefix(path, "\"") || !strings.HasSuffix(path, "\"") {
		return path
	}
	if unquoted, err := strconv.Unquote(path); err == nil {
		return unquoted
	}
	return path[1 : len(path)-1]
}

// atoiOrDefault parses a number, returning fallback for an empty or invalid string
func atoiOrDefault(text string, fallback int) int {
	if text == "" {
		return fallback
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return fallback
	}
	return value
}

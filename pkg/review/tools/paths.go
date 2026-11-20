package tools

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxFileBytes bounds how much of a single worktree file a tool will read
	maxFileBytes = 4 << 20
	// maxLineBytes bounds a single source line in tool output
	maxLineBytes = 500
)

var errPathRejected = errors.New("path rejected")

// rootDir returns the worktree path with symlinks resolved, so containment
// checks compare like with like (macOS /var vs /private/var, for example).
func (t *Toolbox) rootDir() (string, error) {
	t.rootOnce.Do(func() {
		if t.Context == nil || t.Context.WorktreePath == "" {
			t.rootErr = errors.New("review context has no worktree path")
			return
		}
		absolute, err := filepath.Abs(t.Context.WorktreePath)
		if err != nil {
			t.rootErr = fmt.Errorf("resolve worktree path: %w", err)
			return
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			t.rootErr = fmt.Errorf("resolve worktree path: %w", err)
			return
		}
		t.root = resolved
	})
	return t.root, t.rootErr
}

// cleanRelative validates a model-supplied repo-relative path without touching the filesystem
func cleanRelative(relative string) (string, error) {
	if strings.TrimSpace(relative) == "" {
		return "", fmt.Errorf("%w: empty path", errPathRejected)
	}
	if strings.ContainsRune(relative, 0) {
		return "", fmt.Errorf("%w: path contains a NUL byte", errPathRejected)
	}
	slashed := strings.ReplaceAll(relative, "\\", "/")
	if strings.HasPrefix(slashed, "/") || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return "", fmt.Errorf("%w: %q is absolute; use a repo-relative path", errPathRejected, relative)
	}
	for _, segment := range strings.Split(slashed, "/") {
		if segment == ".." {
			return "", fmt.Errorf("%w: %q contains '..'", errPathRejected, relative)
		}
	}
	cleaned := filepath.Clean(filepath.FromSlash(slashed))
	firstSegment := strings.SplitN(filepath.ToSlash(cleaned), "/", 2)[0]
	if firstSegment == ".git" {
		return "", fmt.Errorf("%w: the .git directory is not readable", errPathRejected)
	}
	return cleaned, nil
}

// resolvePath maps a repo-relative path to an absolute path inside the worktree.
// It rejects absolute paths, '..' segments, the .git directory, and symlinks
// whose target lies outside the worktree.
func (t *Toolbox) resolvePath(relative string) (string, error) {
	cleaned, err := cleanRelative(relative)
	if err != nil {
		return "", err
	}
	root, err := t.rootDir()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, cleaned))
	if err != nil {
		return "", fmt.Errorf("file %q not found in the PR head checkout", relative)
	}
	if !isWithin(root, resolved) {
		return "", fmt.Errorf("%w: %q resolves outside the worktree", errPathRejected, relative)
	}
	return resolved, nil
}

// isWithin reports whether target is root or a descendant of it
func isWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// readFileLines reads a worktree file as lines (without newlines). It refuses
// directories and files larger than maxFileBytes.
func (t *Toolbox) readFileLines(relative string) ([]string, error) {
	resolved, err := t.resolvePath(relative)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", relative, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%q is a directory", relative)
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("%q is too large to read (%d bytes)", relative, info.Size())
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", relative, err)
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxFileBytes)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %q: %w", relative, err)
	}
	return lines, nil
}

// truncateLine shortens an over-long source line for display
func truncateLine(text string) string {
	if len(text) <= maxLineBytes {
		return text
	}
	return strings.ToValidUTF8(text[:maxLineBytes], "") + " …[line truncated]"
}

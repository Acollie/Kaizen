package engine

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

// toolRead is Claude Code's built-in file reader
const toolRead = "Read"

// toolTarget names what a tool call examined, so the UI can show which
// changed code the AI opened and which it never looked at. Kaizen's function
// tools target a function id; get_file_diff, read_lines and Read target a
// repo-relative path. Searches (Grep, Glob) and everything else have no single
// target and return empty strings. toolbox, when set, resolves bare function
// names and renamed paths to the PR's canonical spelling.
func toolTarget(name string, input json.RawMessage, worktreePath string, toolbox *tools.Toolbox) (target, kind string) {
	var arguments toolArguments
	if err := json.Unmarshal(input, &arguments); err != nil {
		return "", ""
	}
	switch strings.TrimPrefix(name, mcpToolPrefix) {
	case tools.ToolGetFunction, tools.ToolGetCallers, tools.ToolGetCallees:
		return functionTarget(arguments.ID, toolbox)
	case tools.ToolGetFileDiff, tools.ToolReadLines:
		return fileTarget(arguments.Path, toolbox)
	case toolRead:
		return fileTarget(repoRelative(worktreePath, arguments.FilePath), toolbox)
	default:
		return "", ""
	}
}

func functionTarget(id string, toolbox *tools.Toolbox) (string, string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ""
	}
	if toolbox != nil {
		id = toolbox.CanonicalFunctionID(id)
	}
	return id, review.TargetFunction
}

func fileTarget(path string, toolbox *tools.Toolbox) (string, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ""
	}
	if toolbox != nil {
		path = toolbox.CanonicalPath(path)
	}
	return path, review.TargetFile
}

// repoRelative makes a Read path relative to the worktree. A path outside the
// worktree is not part of the PR and yields "".
func repoRelative(worktreePath, path string) string {
	relative := relativeTo(worktreePath, strings.TrimSpace(path))
	if relative == "" || filepath.IsAbs(relative) {
		return ""
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return ""
	}
	return relative
}

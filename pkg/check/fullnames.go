package check

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
)

// MatchCallGraphNames matches changed functions to call-graph node keys
// (package.Function or package.Receiver.Function). Each changed function
// contributes at most one key; the result is deduplicated and keeps the order
// of the changed functions. Functions with no matching node are skipped.
func MatchCallGraphNames(changed []ChangedFunction, graph *models.CallGraph, repoPath string) []string {
	if graph == nil {
		return nil
	}

	var fullNames []string
	seen := make(map[string]bool)

	for _, changedFunc := range changed {
		fullName := MatchCallGraphName(changedFunc, graph, repoPath)
		if fullName == "" || seen[fullName] {
			continue
		}
		seen[fullName] = true
		fullNames = append(fullNames, fullName)
	}

	return fullNames
}

// MatchCallGraphName returns the call-graph key for one changed function, or ""
// when no node matches. A node matches when it is not external, its file is the
// changed function's file (compared repo-relative, falling back to a suffix
// match), its name matches, and for methods its key ends in Receiver.Name.
// When several nodes match, the one whose line equals the function's start line
// wins, then the lexically smallest key, so the result is deterministic.
func MatchCallGraphName(changedFunc ChangedFunction, graph *models.CallGraph, repoPath string) string {
	if graph == nil {
		return ""
	}

	changedPath := RepoRelativePath(changedFunc.FilePath, repoPath)
	var candidates []string

	for fullName, node := range graph.Nodes {
		if node.IsExternal || node.Name != changedFunc.Name {
			continue
		}
		if !sameSourceFile(RepoRelativePath(node.File, repoPath), changedPath) {
			continue
		}
		if changedFunc.HasReceiver && !strings.HasSuffix(fullName, "."+changedFunc.ReceiverType+"."+changedFunc.Name) {
			continue
		}
		if node.Line == changedFunc.StartLine {
			return fullName
		}
		candidates = append(candidates, fullName)
	}

	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	return candidates[0]
}

// RepoRelativePath converts a path under repoPath to a slash-separated
// repo-relative path. Paths outside repoPath are returned cleaned but otherwise unchanged.
func RepoRelativePath(path, repoPath string) string {
	if path == "" {
		return ""
	}

	cleanPath := filepath.Clean(path)
	cleanRepo := filepath.Clean(repoPath)

	if cleanRepo != "." && cleanRepo != "" {
		prefix := cleanRepo + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, prefix) {
			cleanPath = strings.TrimPrefix(cleanPath, prefix)
		} else if filepath.IsAbs(cleanPath) && filepath.IsAbs(cleanRepo) {
			if relative, err := filepath.Rel(cleanRepo, cleanPath); err == nil && !strings.HasPrefix(relative, "..") {
				cleanPath = relative
			}
		}
	}

	return filepath.ToSlash(cleanPath)
}

// sameSourceFile reports whether two normalised paths name the same file. Paths
// are compared exactly after cleaning: a fixture copy such as
// pkg/x/testdata/worktree/pkg/analyzer/pipeline.go is a different file from
// pkg/analyzer/pipeline.go. Only when one side is absolute and the other relative
// (a path outside the repo root against a repo-relative one) may the relative path
// match the absolute path's tail, and then only on a whole path-segment boundary.
func sameSourceFile(nodePath, changedPath string) bool {
	if nodePath == "" || changedPath == "" {
		return false
	}
	cleanNode := filepath.ToSlash(filepath.Clean(nodePath))
	cleanChanged := filepath.ToSlash(filepath.Clean(changedPath))
	if cleanNode == cleanChanged {
		return true
	}

	nodeAbsolute, changedAbsolute := isAbsoluteSlashPath(cleanNode), isAbsoluteSlashPath(cleanChanged)
	switch {
	case nodeAbsolute && !changedAbsolute:
		return hasPathSuffix(cleanNode, cleanChanged)
	case changedAbsolute && !nodeAbsolute:
		return hasPathSuffix(cleanChanged, cleanNode)
	}
	return false
}

// isAbsoluteSlashPath reports whether a slash-separated path is absolute on any platform
func isAbsoluteSlashPath(path string) bool {
	return strings.HasPrefix(path, "/") || filepath.IsAbs(filepath.FromSlash(path))
}

// hasPathSuffix reports whether relative names the trailing path segments of absolute
func hasPathSuffix(absolute, relative string) bool {
	if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return false
	}
	return strings.HasSuffix(absolute, "/"+relative)
}

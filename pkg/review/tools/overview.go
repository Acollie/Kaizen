package tools

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review"
)

const (
	// maxOverviewGroups is the most package groups get_package_overview returns;
	// deeper directories are merged into their parents until the count fits
	maxOverviewGroups = 20
	// maxOverviewFiles caps the files listed per group
	maxOverviewFiles = 12
	rootGroupLabel   = "(repository root)"
)

// PackageOverviewInput is the (empty) input of get_package_overview
type PackageOverviewInput struct{}

// packageGroup aggregates the changed files of one directory (or directory subtree)
type packageGroup struct {
	directory     string
	files         []string
	testFiles     int
	functions     int
	focus         int
	linesChanged  int
	nonTestLines  int
	maxRisk       float64
	maxRiskSource string
}

// GetPackageOverview lists the PR's changed packages, grouped by directory, so
// the lead reviewer can spread coverage beyond the top-ranked functions
func (t *Toolbox) GetPackageOverview(_ PackageOverviewInput) (string, error) {
	if t.Context == nil {
		return "", fmt.Errorf("no review context loaded")
	}
	groups := buildPackageGroups(t.Context.Files, t.Context.Functions)
	var builder strings.Builder
	fmt.Fprintf(&builder, "Changed packages (%d groups from %d files, riskiest first). Directories nested deeper are merged into their parent (shown as dir/...).\n",
		len(groups), len(t.Context.Files))
	for _, group := range groups {
		writePackageGroup(&builder, group)
	}
	if len(groups) == 0 {
		builder.WriteString("  (no changed files)\n")
	}
	return builder.String(), nil
}

func writePackageGroup(builder *strings.Builder, group *packageGroup) {
	label := group.directory
	if label == "" {
		label = rootGroupLabel
	}
	fmt.Fprintf(builder, "\n%s: %d files, %d changed functions, %d lines changed (%d non-test), max risk %.0f, %d FOCUS",
		label, len(group.files), group.functions, group.linesChanged, group.nonTestLines, group.maxRisk, group.focus)
	if group.maxRiskSource != "" {
		fmt.Fprintf(builder, " (riskiest: %s)", group.maxRiskSource)
	}
	if group.testFiles == len(group.files) {
		builder.WriteString(" [tests only]")
	}
	builder.WriteString("\n")
	for index, file := range group.files {
		if index == maxOverviewFiles {
			fmt.Fprintf(builder, "  …and %d more\n", len(group.files)-index)
			break
		}
		fmt.Fprintf(builder, "  %s\n", file)
	}
}

// buildPackageGroups groups files by directory, collapses the deepest groups
// into their parents until at most maxOverviewGroups remain, and sorts them by
// risk, then size
func buildPackageGroups(files []review.FileDiff, functions []review.FunctionRisk) []*packageGroup {
	directories := map[string]string{} // file path -> group directory
	for _, file := range files {
		directories[file.Path] = fileDirectory(file.Path)
	}
	collapseDirectories(directories, maxOverviewGroups)

	groups := map[string]*packageGroup{}
	groupFor := func(filePath string) *packageGroup {
		directory, known := directories[filePath]
		if !known {
			return nil
		}
		group, exists := groups[directory]
		if !exists {
			group = &packageGroup{directory: directory}
			groups[directory] = group
		}
		return group
	}
	for _, file := range files {
		addFileToGroup(groupFor(file.Path), file)
	}
	for _, function := range functions {
		addFunctionToGroup(groupFor(function.File), function)
	}
	return sortedGroups(groups)
}

func addFileToGroup(group *packageGroup, file review.FileDiff) {
	changed := file.Additions + file.Deletions
	group.files = append(group.files, file.Path)
	group.linesChanged += changed
	if review.IsTestFile(file.Path) {
		group.testFiles++
	} else {
		group.nonTestLines += changed
	}
	if file.RiskScore > group.maxRisk {
		group.maxRisk = file.RiskScore
		group.maxRiskSource = file.Path
	}
}

func addFunctionToGroup(group *packageGroup, function review.FunctionRisk) {
	if group == nil {
		return
	}
	group.functions++
	if function.Focus {
		group.focus++
	}
	if function.RiskScore > group.maxRisk {
		group.maxRisk = function.RiskScore
		group.maxRiskSource = function.ID
	}
}

func sortedGroups(groups map[string]*packageGroup) []*packageGroup {
	sorted := make([]*packageGroup, 0, len(groups))
	for _, group := range groups {
		sort.Strings(group.files)
		sorted = append(sorted, group)
	}
	sort.Slice(sorted, func(left, right int) bool {
		if sorted[left].maxRisk != sorted[right].maxRisk {
			return sorted[left].maxRisk > sorted[right].maxRisk
		}
		if sorted[left].nonTestLines != sorted[right].nonTestLines {
			return sorted[left].nonTestLines > sorted[right].nonTestLines
		}
		return sorted[left].directory < sorted[right].directory
	})
	return sorted
}

// fileDirectory is the slash-separated directory of a repo-relative path, "" at the root
func fileDirectory(filePath string) string {
	directory := path.Dir(strings.ReplaceAll(filePath, "\\", "/"))
	if directory == "." || directory == "/" {
		return ""
	}
	return directory
}

// collapseDirectories merges the deepest directories into their parents, one
// level at a time, until there are at most limit distinct groups or every
// group is at the top level. A merged group is labelled "dir/...".
func collapseDirectories(directories map[string]string, limit int) {
	for distinctCount(directories) > limit {
		deepest := 0
		for _, directory := range directories {
			deepest = max(deepest, directoryDepth(directory))
		}
		if deepest <= 1 {
			return
		}
		merged := map[string]bool{}
		for filePath, directory := range directories {
			if directoryDepth(directory) == deepest {
				directories[filePath] = parentGroup(directory)
				merged[directories[filePath]] = true
			}
		}
		// files directly in a parent join its merged subtree group
		for filePath, directory := range directories {
			if merged[directory+"/..."] {
				directories[filePath] = directory + "/..."
			}
		}
	}
}

func distinctCount(directories map[string]string) int {
	distinct := map[string]bool{}
	for _, directory := range directories {
		distinct[directory] = true
	}
	return len(distinct)
}

// directoryDepth counts path segments, ignoring a trailing "/..." marker
func directoryDepth(directory string) int {
	trimmed := strings.TrimSuffix(directory, "/...")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}

// parentGroup returns the parent of a directory group, marked as a merged subtree
func parentGroup(directory string) string {
	trimmed := strings.TrimSuffix(directory, "/...")
	parent := path.Dir(trimmed)
	if parent == "." {
		return trimmed + "/..."
	}
	return parent + "/..."
}

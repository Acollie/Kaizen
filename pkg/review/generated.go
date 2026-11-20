package review

import (
	"path/filepath"
	"strings"
)

// minifiedLineLength is the line length above which a changed file is treated as
// minified/generated output (hand-written code essentially never has such lines)
const minifiedLineLength = 500

// generatedFileNames are lockfiles and similar machine-written files
var generatedFileNames = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"poetry.lock": true, "cargo.lock": true, "gemfile.lock": true, "podfile.lock": true,
	"composer.lock": true, "npm-shrinkwrap.json": true,
}

// markGeneratedFiles flags lockfiles and minified files. Their diffs are dropped from
// the context (nobody reviews them line by line, and they would waste the AI's
// context), but the files stay listed with a reason so nothing is silently hidden.
// Detection is by name and content, never by attributes the PR itself could set.
func markGeneratedFiles(files []FileDiff) map[string]bool {
	generated := make(map[string]bool)
	for index := range files {
		file := &files[index]
		reason := generatedReason(*file)
		if reason == "" {
			continue
		}
		generated[file.Path] = true
		file.Generated = true
		file.Hunks = nil
		file.RiskReasons = append(file.RiskReasons, reason)
	}
	return generated
}

func generatedReason(file FileDiff) string {
	base := strings.ToLower(filepath.Base(file.Path))
	switch {
	case generatedFileNames[base]:
		return "lockfile: diff not shown or sent to the AI; check dependency changes in the manifest"
	case strings.Contains(base, ".min."):
		return "minified file: diff not shown or sent to the AI"
	case hasMinifiedLine(file):
		return "generated or minified (very long lines): diff not shown or sent to the AI"
	}
	return ""
}

func hasMinifiedLine(file FileDiff) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind == LineAdded && len(line.Text) > minifiedLineLength {
				return true
			}
		}
	}
	return false
}

package review

import (
	"path/filepath"
	"strings"
)

// Minified-content thresholds. Hand-written files (e.g. prompts in raw strings)
// can have a few long lines, so content detection requires that nearly every added
// line is long and at least one is very long. Only bundle-type files qualify at
// all; anything else stays a normal, reviewable file.
const (
	longLineLength        = 500  // a line this long counts towards the minified share
	veryLongLineLength    = 2000 // at least one added line must be longer than this
	minifiedLongLineShare = 0.8  // share of added lines (or of added characters) that must be long
)

// generatedFileRisk is the file risk a withheld file gets at least, so it is not
// ranked with untouched-looking files at 0 and still draws a reviewer's eye
const generatedFileRisk = 15

// Reasons shown in the UI for files whose diff is withheld
const (
	generatedLockfileNote = "lockfile: diff not shown or sent to the AI; check dependency changes in the manifest"
	generatedMinNameNote  = "minified file: diff not shown or sent to the AI"
	generatedMinifiedNote = "minified content (mostly very long lines): diff not shown or sent to the AI"
)

// generatedFileNames are lockfiles and similar machine-written files
var generatedFileNames = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"poetry.lock": true, "cargo.lock": true, "gemfile.lock": true, "podfile.lock": true,
	"composer.lock": true, "npm-shrinkwrap.json": true,
}

// bundleExtensions are the file types build tools emit minified. Only these are
// ever withheld by name (.min.) or content: a source file (.go, .py, .ts, .sh, ...)
// named auth.min.go or full of long lines is still code someone must review, and
// withholding it would hide it from both the reviewer and the AI.
var bundleExtensions = map[string]bool{
	".js": true, ".css": true, ".mjs": true, ".cjs": true, ".map": true, ".json": true,
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
		file.RiskScore = max(file.RiskScore, generatedFileRisk)
	}
	return generated
}

// generatedReason is the UI note for a file whose diff is withheld, or "" to keep it.
// Lockfiles go by exact name; everything else must be a bundle-type file first.
func generatedReason(file FileDiff) string {
	base := strings.ToLower(filepath.Base(file.Path))
	if generatedFileNames[base] {
		return generatedLockfileNote
	}
	if !bundleExtensions[filepath.Ext(base)] {
		return ""
	}
	if strings.Contains(base, ".min.") {
		return generatedMinNameNote
	}
	if isMinifiedContent(file) {
		return generatedMinifiedNote
	}
	return ""
}

// isMinifiedContent reports whether the added lines look like build output: at
// least 80% of them are long and one is very long. The 80% is met by line count or
// by characters, because bundlers keep short license-comment lines
// (" * @license React") between their megabyte-long code lines.
func isMinifiedContent(file FileDiff) bool {
	addedLines, longLines, addedChars, longChars, longest := 0, 0, 0, 0, 0
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != LineAdded {
				continue
			}
			length := len(line.Text)
			addedLines++
			addedChars += length
			if length > longLineLength {
				longLines++
				longChars += length
			}
			longest = max(longest, length)
		}
	}
	if addedLines == 0 || longest <= veryLongLineLength {
		return false
	}
	mostlyLong := float64(longLines) >= minifiedLongLineShare*float64(addedLines) ||
		float64(longChars) >= minifiedLongLineShare*float64(addedChars)
	return mostlyLong
}

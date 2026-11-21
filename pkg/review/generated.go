package review

import (
	"path/filepath"
	"sort"
	"strings"
)

// Minified-content thresholds. Hand-written files (e.g. prompts in raw strings, or a
// one-line package.json padded with spaces) can have a few long lines, so content
// detection requires that nearly all added characters sit on long lines AND that
// the file is big: many added lines, or one line no human writes by hand.
const (
	longLineLength        = 500   // a line this long counts towards the minified share
	hugeLineLength        = 10000 // one line this long is enough size on its own
	minMinifiedLines      = 20    // otherwise at least this many lines must be added
	minifiedLongLineShare = 0.8   // share of added characters that must be on long lines
)

// generatedFileRisk is the file risk a withheld file gets at least, so it is not
// ranked with untouched-looking files at 0 and still draws a reviewer's eye
const generatedFileRisk = 15

// Reasons shown in the UI for files whose diff is withheld or deliberately shown
const (
	generatedLockfileNote    = "lockfile: diff not shown or sent to the AI; check dependency changes in the manifest"
	generatedBuildOutputNote = "build output: diff not shown or sent to the AI"
	generatedMinifiedNote    = "minified content (mostly very long lines): diff not shown or sent to the AI"
	sideEffectShownNote      = "withheld-looking file contains side-effect code: shown in full"
	sideEffectWithheldPrefix = "withheld build output contains side-effect code ("
	sideEffectWithheldSuffix = "): read it yourself before merging"
)

// generatedFileNames are lockfiles and similar machine-written files
var generatedFileNames = map[string]bool{
	"go.sum": true, "package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"poetry.lock": true, "cargo.lock": true, "gemfile.lock": true, "podfile.lock": true,
	"composer.lock": true, "npm-shrinkwrap.json": true,
}

// bundleExtensions are the file types build tools emit minified. Only these are
// ever withheld: a source file (.go, .py, .ts, .sh, ...) full of long lines is still
// code someone must review, and withholding it would hide it from the reviewer and the AI.
var bundleExtensions = map[string]bool{
	".js": true, ".css": true, ".mjs": true, ".cjs": true, ".map": true, ".json": true,
}

// neverWithheldNames are manifests and config files: small, hand-edited and able to
// run code (install scripts, build plugins), so their diff is always shown
var neverWithheldNames = map[string]bool{
	"package.json": true, "composer.json": true,
}

// buildOutputDirs are directories whose bundle-type files are build or vendored output
var buildOutputDirs = []string{
	"dist/", "build/", "out/", "static/assets/", "public/assets/", "vendor/", ".next/", "node_modules/",
}

// sideEffectTokens mark code that does something when installed, built or loaded:
// runs commands, reads secrets, writes files or talks to the network. A file that
// would be withheld but contains one is shown (or flagged, for build output).
var sideEffectTokens = []string{
	"exec", "child_process", "spawn", "eval(", "postinstall", "preinstall", "curl", "wget",
	"| sh", "http://", "https://", `require("child_process")`, "process.env", "fs.write", "fetch(",
}

// markGeneratedFiles flags lockfiles, build output and minified files. Their diffs are
// dropped from the context (nobody reviews them line by line, and they would waste the
// AI's context), but the files stay listed with a reason so nothing is silently hidden.
// A file that only looks withheld-worthy but carries side-effect code is shown in full.
// Detection is by name, path and content, never by attributes the PR itself could set.
func markGeneratedFiles(files []FileDiff) map[string]bool {
	generated := make(map[string]bool)
	for index := range files {
		file := &files[index]
		decision := classifyGenerated(*file)
		if decision.withhold {
			generated[file.Path] = true
			file.Generated = true
			file.Hunks = nil
		}
		if len(decision.reasons) > 0 {
			file.RiskReasons = append(file.RiskReasons, decision.reasons...)
			file.RiskScore = max(file.RiskScore, generatedFileRisk)
		}
	}
	return generated
}

// generatedDecision is whether a file's diff is withheld, and the reasons shown for it
type generatedDecision struct {
	withhold bool
	reasons  []string
}

// generatedReason is the UI note for a file whose diff is withheld, or "" to keep it
func generatedReason(file FileDiff) string {
	decision := classifyGenerated(file)
	if !decision.withhold {
		return ""
	}
	return decision.reasons[0]
}

// classifyGenerated decides whether to withhold a file. Lockfiles go by exact name.
// Manifests and config are always shown. Other bundle-type files are withheld when
// they sit in a build output directory or are truly minified; side-effect code in
// them is shown in full, or flagged when the file is build output.
func classifyGenerated(file FileDiff) generatedDecision {
	slashed := strings.ToLower(filepath.ToSlash(file.Path))
	base := filepath.Base(slashed)
	if generatedFileNames[base] {
		return generatedDecision{withhold: true, reasons: []string{generatedLockfileNote}}
	}
	extension := filepath.Ext(base)
	inBuildDir := isInBuildOutputDir(slashed)
	if !bundleExtensions[extension] || isNeverWithheld(base, extension, inBuildDir) {
		return generatedDecision{}
	}

	var note string
	switch {
	case inBuildDir:
		note = generatedBuildOutputNote
	case isMinifiedContent(file):
		note = generatedMinifiedNote
	default:
		return generatedDecision{}
	}

	tokens := sideEffectTokensIn(file)
	if len(tokens) == 0 {
		return generatedDecision{withhold: true, reasons: []string{note}}
	}
	if inBuildDir {
		flag := sideEffectWithheldPrefix + strings.Join(tokens, ", ") + sideEffectWithheldSuffix
		return generatedDecision{withhold: true, reasons: []string{note, flag}}
	}
	return generatedDecision{reasons: []string{sideEffectShownNote + " (" + strings.Join(tokens, ", ") + ")"}}
}

// isNeverWithheld reports manifests and config files: package.json and the like,
// dependency manifests, *.config.*, tsconfig*.json, and JSON outside build output
func isNeverWithheld(base, extension string, inBuildDir bool) bool {
	if neverWithheldNames[base] || dependencyManifests[base] {
		return true
	}
	if strings.Contains(base, ".config.") {
		return true
	}
	if strings.HasPrefix(base, "tsconfig") && extension == ".json" {
		return true
	}
	return extension == ".json" && !inBuildDir
}

// isInBuildOutputDir reports whether a lower-case, slash-separated path is inside a
// build or vendored output directory at any depth
func isInBuildOutputDir(slashed string) bool {
	rooted := "/" + slashed
	for _, dir := range buildOutputDirs {
		if strings.Contains(rooted, "/"+dir) {
			return true
		}
	}
	return false
}

// isMinifiedContent reports whether the added lines look like build output: at least
// 80% of the added characters are on lines over 500 characters, and the addition is
// large (20+ lines, or one line over 10k characters). Characters rather than lines
// are counted because bundlers keep short license-comment lines between their
// megabyte-long code lines.
func isMinifiedContent(file FileDiff) bool {
	addedLines, addedChars, longChars, longest := 0, 0, 0, 0
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != LineAdded {
				continue
			}
			length := len(line.Text)
			addedLines++
			addedChars += length
			if length > longLineLength {
				longChars += length
			}
			longest = max(longest, length)
		}
	}
	if addedChars == 0 {
		return false
	}
	mostlyLong := float64(longChars) >= minifiedLongLineShare*float64(addedChars)
	large := addedLines >= minMinifiedLines || longest > hugeLineLength
	return mostlyLong && large
}

// sideEffectTokensIn returns the side-effect tokens found in a file's added lines,
// sorted and deduplicated
func sideEffectTokensIn(file FileDiff) []string {
	found := make(map[string]bool)
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != LineAdded {
				continue
			}
			lower := strings.ToLower(line.Text)
			for _, token := range sideEffectTokens {
				if !found[token] && strings.Contains(lower, strings.ToLower(token)) {
					found[token] = true
				}
			}
		}
	}
	tokens := make([]string, 0, len(found))
	for token := range found {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

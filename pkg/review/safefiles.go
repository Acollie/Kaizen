package review

import (
	"fmt"
	"os"
	"path/filepath"
)

// MaxAnalyzableFileBytes caps the size of a changed file Kaizen will read and parse.
// Larger files are listed in the review but not analyzed.
const MaxAnalyzableFileBytes = 2 << 20

// FileRiskSymlink is the risk of a PR that adds or changes a symlink: it can point
// reviewers and tools at content outside the change, so it always deserves a look.
const FileRiskSymlink = 30.0

// quarantineUnsafeFiles inspects every changed file in the PR checkout without
// following links and returns the repo-relative paths that must not be read: symlinks,
// non-regular files (devices, FIFOs) and oversized files. PR content is untrusted, and
// a link to /dev/zero or a FIFO would otherwise hang or exhaust memory in the
// analyzers. Each quarantined file is marked Binary (so nothing reads its lines) and
// gets a risk reason explaining why it was not analyzed.
func quarantineUnsafeFiles(files []FileDiff, root string) map[string]bool {
	quarantined := make(map[string]bool)
	for index := range files {
		file := &files[index]
		if file.Status == FileDeleted {
			continue
		}
		reason, score := unsafeFileReason(root, file.Path)
		if reason == "" {
			continue
		}
		quarantined[file.Path] = true
		file.Binary = true
		file.Hunks = nil
		file.RiskScore = max(file.RiskScore, score)
		file.RiskReasons = append(file.RiskReasons, reason)
	}
	return quarantined
}

// unsafeFileReason explains why a changed file must not be read, or returns "" if it is safe
func unsafeFileReason(root, relativePath string) (string, float64) {
	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	info, err := os.Lstat(fullPath)
	if err != nil {
		return "", 0
	}
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		target, _ := os.Readlink(fullPath)
		return fmt.Sprintf("symlink to %q: not followed or analyzed; check where it points", target), FileRiskSymlink
	case !mode.IsRegular():
		return "not a regular file (device, pipe or socket): not analyzed", FileRiskSymlink
	case info.Size() > MaxAnalyzableFileBytes:
		return fmt.Sprintf("%d MB file: too large to analyze", info.Size()>>20), 0
	}
	return "", 0
}

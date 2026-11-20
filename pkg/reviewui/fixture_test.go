package reviewui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcollie/kaizen/pkg/review"
)

// The fixture's hunks must agree with the worktree files, or context expansion
// would splice the wrong lines into the diff.
func TestFixtureHunksMatchWorktree(t *testing.T) {
	reviewContext := loadFixture(t)
	if len(reviewContext.Files) < 4 || len(reviewContext.Functions) < 8 || len(reviewContext.Graph.Nodes) < 20 {
		t.Fatalf("fixture too small: %d files, %d functions, %d nodes",
			len(reviewContext.Files), len(reviewContext.Functions), len(reviewContext.Graph.Nodes))
	}
	for _, file := range reviewContext.Files {
		lines, err := readFileLines(filepath.Join(reviewContext.WorktreePath, file.Path))
		if err != nil {
			t.Fatalf("%s: %v", file.Path, err)
		}
		if len(lines) != file.LineCount {
			t.Errorf("%s: line_count %d, file has %d", file.Path, file.LineCount, len(lines))
		}
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == review.LineRemoved {
					continue
				}
				if got := lines[line.NewLine-1]; got != line.Text {
					t.Errorf("%s:%d fixture %q, worktree %q", file.Path, line.NewLine, line.Text, got)
				}
			}
		}
	}
}

func TestFixtureGraphIsConsistent(t *testing.T) {
	reviewContext := loadFixture(t)
	nodes := make(map[string]bool)
	for _, node := range reviewContext.Graph.Nodes {
		nodes[node.ID] = true
	}
	for _, edge := range reviewContext.Graph.Edges {
		if !nodes[edge.From] || !nodes[edge.To] {
			t.Errorf("edge %s -> %s references a missing node", edge.From, edge.To)
		}
	}
	allowed := allowedFilePaths(reviewContext)
	for _, function := range reviewContext.Functions {
		if !nodes[function.ID] {
			t.Errorf("function %s has no graph node", function.ID)
		}
		for _, caller := range function.Callers {
			if _, err := os.Stat(filepath.Join(reviewContext.WorktreePath, caller.File)); err != nil || !allowed[caller.File] {
				t.Errorf("caller file %s unavailable", caller.File)
			}
		}
	}
}

func TestLoadDemoContextExtractsWorktree(t *testing.T) {
	reviewContext, cleanup, err := LoadDemoContext()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if reviewContext.PR.Number != 142 {
		t.Errorf("pr number %d", reviewContext.PR.Number)
	}
	for _, file := range reviewContext.Files {
		if _, err := resolveInsideWorktree(reviewContext.WorktreePath, file.Path); err != nil {
			t.Errorf("%s not extracted: %v", file.Path, err)
		}
	}
	worktree := reviewContext.WorktreePath
	cleanup()
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Error("cleanup did not remove the demo worktree")
	}
}

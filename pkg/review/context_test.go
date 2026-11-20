package review

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureBaseCore = `package app

func Compute(value int) int {
	return value * 2
}
`

const fixtureHeadCore = `package app

import "strconv"

func Compute(value int) int {
	if value < 0 {
		return 0
	}
	for index := 0; index < value; index++ {
		if index%2 == 0 && value > 10 {
			value += normalize(index)
		} else if index%3 == 0 {
			value--
		}
	}
	return value * 2
}

func normalize(input int) int {
	if input > 100 {
		return 100
	}
	_ = strconv.Itoa(input)
	return input
}
`

const fixtureBaseCallers = `package app

func A() int { return Compute(1) }
func B() int { return Compute(2) }
func C() int { return Compute(3) }

func Unrelated() int {
	total := 0
	total++
	return total
}
`

const fixtureHeadCallers = `package app

func A() int { return Compute(1) }
func B() int { return Compute(2) }
func C() int { return Compute(3) }

func Unrelated() int {
	total := 0
	return total
}
`

// gitFixture is a temporary repository with a base and a head commit
type gitFixture struct {
	dir     string
	baseSHA string
	headSHA string
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-c", "user.email=test@example.com", "-c", "user.name=Test User"}, args...)
	command := exec.Command("git", fullArgs...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, output)
	return string(output)
}

func writeFixtureFile(t *testing.T, dir, relative, content string) {
	t.Helper()
	path := filepath.Join(dir, relative)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func newGitFixture(t *testing.T) gitFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	writeFixtureFile(t, dir, "pkg/app/core.go", fixtureBaseCore)
	writeFixtureFile(t, dir, "pkg/app/callers.go", fixtureBaseCallers)
	writeFixtureFile(t, dir, "README.md", "# Fixture\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "base")
	baseSHA := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))

	writeFixtureFile(t, dir, "pkg/app/core.go", fixtureHeadCore)
	writeFixtureFile(t, dir, "pkg/app/callers.go", fixtureHeadCallers)
	writeFixtureFile(t, dir, "README.md", "# Fixture\n\nNow with docs.\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "head")
	headSHA := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))

	return gitFixture{dir: dir, baseSHA: baseSHA, headSHA: headSHA}
}

func (fixture gitFixture) diff(t *testing.T, unified string) string {
	return runGit(t, fixture.dir, "diff", fixture.baseSHA, fixture.headSHA, "--unified="+unified, "-M")
}

func findFunction(t *testing.T, functions []FunctionRisk, id string) FunctionRisk {
	t.Helper()
	for _, function := range functions {
		if function.ID == id {
			return function
		}
	}
	require.Failf(t, "function not found", "id %s", id)
	return FunctionRisk{}
}

func buildFixtureContext(t *testing.T) (*ReviewContext, gitFixture) {
	fixture := newGitFixture(t)
	pr := PullRequest{Number: 7, BaseSHA: fixture.baseSHA, HeadSHA: fixture.headSHA, MergeBase: fixture.baseSHA}
	reviewContext, err := BuildContext(context.Background(), pr, fixture.dir,
		fixture.diff(t, "3"), fixture.diff(t, "0"), BuildOptions{SkipChurn: true})
	require.NoError(t, err)
	return reviewContext, fixture
}

func TestBuildContextChangedFunctions(t *testing.T) {
	reviewContext, fixture := buildFixtureContext(t)

	resolved, err := filepath.EvalSymlinks(fixture.dir)
	require.NoError(t, err)
	assert.Equal(t, resolved, reviewContext.WorktreePath)
	assert.Equal(t, 3, reviewContext.PR.FileCount)
	assert.False(t, reviewContext.GeneratedAt.IsZero())

	ids := []string{}
	for _, function := range reviewContext.Functions {
		ids = append(ids, function.ID)
	}
	assert.ElementsMatch(t, []string{"app.Compute", "app.normalize", "app.Unrelated"}, ids)

	compute := findFunction(t, reviewContext.Functions, "app.Compute")
	assert.Equal(t, "pkg/app/core.go", compute.File)
	assert.Equal(t, "Go", compute.Language)
	assert.Equal(t, 5, compute.StartLine)
	assert.Equal(t, 17, compute.EndLine)
	assert.Equal(t, 10, compute.LinesAdded)
	assert.False(t, compute.IsNew)
	require.NotNil(t, compute.Metrics)
	assert.GreaterOrEqual(t, compute.Metrics.CyclomaticComplexity, 6)

	normalize := findFunction(t, reviewContext.Functions, "app.normalize")
	assert.True(t, normalize.IsNew)
	assert.Equal(t, 19, normalize.StartLine)
	assert.Equal(t, 25, normalize.EndLine)
	assert.Equal(t, 7, normalize.LinesAdded)

	// A function that only lost a line is still found via deletion anchors
	unrelated := findFunction(t, reviewContext.Functions, "app.Unrelated")
	assert.Equal(t, 0, unrelated.LinesAdded)
	assert.Equal(t, 1, unrelated.LinesRemoved)
}

func TestBuildContextCallersAndCallees(t *testing.T) {
	reviewContext, _ := buildFixtureContext(t)
	compute := findFunction(t, reviewContext.Functions, "app.Compute")

	assert.Equal(t, 3, compute.FanIn)
	assert.Equal(t, 3, compute.CallerCount)
	assert.False(t, compute.FanInApproximate)
	callerIDs := []string{}
	for _, caller := range compute.Callers {
		callerIDs = append(callerIDs, caller.ID)
		assert.Equal(t, "pkg/app/callers.go", caller.File)
	}
	assert.Equal(t, []string{"app.A", "app.B", "app.C"}, callerIDs)
	assert.Equal(t, 3, compute.Callers[0].Line)

	require.Len(t, compute.Callees, 1)
	assert.Equal(t, "app.normalize", compute.Callees[0].ID)
	assert.Equal(t, 19, compute.Callees[0].Line)

	normalize := findFunction(t, reviewContext.Functions, "app.normalize")
	require.Len(t, normalize.Callees, 1)
	assert.Equal(t, "strconv.Itoa", normalize.Callees[0].ID)
	assert.True(t, normalize.Callees[0].External)
}

func TestBuildContextRiskOrderAndFiles(t *testing.T) {
	reviewContext, _ := buildFixtureContext(t)

	assert.Equal(t, "app.Compute", reviewContext.Functions[0].ID)
	for index := 1; index < len(reviewContext.Functions); index++ {
		assert.GreaterOrEqual(t, reviewContext.Functions[index-1].RiskScore, reviewContext.Functions[index].RiskScore)
	}
	assert.Greater(t, reviewContext.Functions[0].RiskScore, 0.0)

	require.Len(t, reviewContext.Files, 3)
	assert.Equal(t, "pkg/app/core.go", reviewContext.Files[0].Path)
	assert.Equal(t, reviewContext.Functions[0].RiskScore, reviewContext.Files[0].RiskScore)
	assert.Equal(t, 25, reviewContext.Files[0].LineCount)

	// Zero-risk files tie and are ordered by path
	readme := reviewContext.Files[1]
	assert.Equal(t, "README.md", readme.Path)
	assert.Equal(t, "pkg/app/callers.go", reviewContext.Files[2].Path)
	assert.Equal(t, "", readme.Language)
	assert.Equal(t, 0.0, readme.RiskScore)
	assert.Equal(t, 3, readme.LineCount)
	assert.NotNil(t, reviewContext.Concerns)
}

func TestBuildContextDisplayLineNumbers(t *testing.T) {
	reviewContext, _ := buildFixtureContext(t)
	core := reviewContext.Files[0]
	require.Equal(t, "pkg/app/core.go", core.Path)

	found := false
	for _, hunk := range core.Hunks {
		for _, line := range hunk.Lines {
			if line.Text == "func normalize(input int) int {" {
				found = true
				assert.Equal(t, LineAdded, line.Kind)
				assert.Equal(t, 19, line.NewLine)
			}
			if line.Text == "\treturn value * 2" {
				assert.Equal(t, LineContext, line.Kind)
				assert.Equal(t, 4, line.OldLine)
				assert.Equal(t, 16, line.NewLine)
			}
		}
	}
	assert.True(t, found)
}

func TestBuildContextGraph(t *testing.T) {
	reviewContext, _ := buildFixtureContext(t)

	kinds := make(map[string]GraphNodeKind)
	for _, node := range reviewContext.Graph.Nodes {
		_, duplicate := kinds[node.ID]
		assert.False(t, duplicate, "duplicate node %s", node.ID)
		kinds[node.ID] = node.Kind
	}
	assert.Equal(t, NodeChanged, kinds["app.Compute"])
	assert.Equal(t, NodeChanged, kinds["app.normalize"], "changed wins over callee")
	assert.Equal(t, NodeCaller, kinds["app.A"])
	assert.Equal(t, NodeExternal, kinds["strconv.Itoa"])
	assert.Len(t, kinds, 7) // Compute, normalize, Unrelated, A, B, C, strconv.Itoa

	edgeSet := make(map[string]bool)
	for _, edge := range reviewContext.Graph.Edges {
		assert.Contains(t, kinds, edge.From)
		assert.Contains(t, kinds, edge.To)
		edgeSet[edge.From+"->"+edge.To] = true
	}
	assert.True(t, edgeSet["app.A->app.Compute"])
	assert.True(t, edgeSet["app.Compute->app.normalize"])
	assert.True(t, edgeSet["app.normalize->strconv.Itoa"])
}

func TestBuildContextCancelled(t *testing.T) {
	fixture := newGitFixture(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := BuildContext(cancelled, PullRequest{}, fixture.dir, fixture.diff(t, "3"), fixture.diff(t, "0"), BuildOptions{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBuildContextMissingWorktree(t *testing.T) {
	_, err := BuildContext(context.Background(), PullRequest{}, filepath.Join(t.TempDir(), "missing"), "", "", BuildOptions{})
	assert.Error(t, err)
}

func TestRemovalAnchorsAndCounts(t *testing.T) {
	hunk := Hunk{NewStart: 10, Lines: []DiffLine{
		{Kind: LineContext, OldLine: 10, NewLine: 10},
		{Kind: LineRemoved, OldLine: 11},
		{Kind: LineRemoved, OldLine: 12},
		{Kind: LineContext, OldLine: 13, NewLine: 11},
		{Kind: LineAdded, NewLine: 12},
		{Kind: LineRemoved, OldLine: 14},
	}}
	assert.Equal(t, []int{11, 13}, removalAnchors(hunk))

	file := &FileDiff{Hunks: []Hunk{hunk}}
	added, removed, isNew := countChangesInRange(file, 10, 13)
	assert.Equal(t, 1, added)
	assert.Equal(t, 3, removed)
	assert.False(t, isNew)

	added, _, isNew = countChangesInRange(file, 12, 12)
	assert.Equal(t, 1, added)
	assert.True(t, isNew)
}

func TestCountLines(t *testing.T) {
	assert.Equal(t, 0, countLines(nil))
	assert.Equal(t, 1, countLines([]byte("one")))
	assert.Equal(t, 2, countLines([]byte("one\ntwo\n")))
	assert.Equal(t, 3, countLines([]byte("one\ntwo\nthree")))
}

// A PR that adds a symlink to /dev/zero, a link outside the checkout or a FIFO must not
// make the analyzers read it (it would hang or exhaust memory); the files are listed
// with a risk reason instead.
func TestBuildContextQuarantinesSymlinksAndSpecialFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	writeFixtureFile(t, repo, "main.go", "package main\n\nfunc main() {}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	require.NoError(t, os.Symlink("/dev/zero", filepath.Join(repo, "zero.go")))
	require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(repo, "hosts.txt")))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "links")
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	display := runGit(t, repo, "diff", base, head, "--unified=3", "-M")
	mapping := runGit(t, repo, "diff", base, head, "--unified=0", "-M")

	done := make(chan struct{})
	var reviewContext *ReviewContext
	var buildErr error
	go func() {
		defer close(done)
		reviewContext, buildErr = BuildContext(context.Background(), PullRequest{Number: 1, HeadSHA: head}, repo, display, mapping, BuildOptions{SkipChurn: true})
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("BuildContext did not finish: a symlink to /dev/zero was followed")
	}
	require.NoError(t, buildErr)

	reasons := map[string]string{}
	for _, file := range reviewContext.Files {
		if len(file.RiskReasons) > 0 {
			reasons[file.Path] = file.RiskReasons[0]
		}
		assert.Zero(t, file.LineCount, "%s must not be read", file.Path)
	}
	assert.Contains(t, reasons["zero.go"], "symlink")
	assert.Contains(t, reasons["hosts.txt"], "/etc/hosts")
	assert.Empty(t, reviewContext.Functions)
}

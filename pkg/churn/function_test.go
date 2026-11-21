package churn

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const calcVersion1 = `package calc

func add(a, b int) int {
	return a + b
}

func addAll(values []int) int {
	return 0
}

type Counter struct{ n int }

func (counter *Counter) Increment() {
	counter.n++
}
`

// version 2 changes add and the Increment method
const calcVersion2 = `package calc

func add(a, b int) int {
	// sum
	return a + b
}

func addAll(values []int) int {
	return 0
}

type Counter struct{ n int }

func (counter *Counter) Increment() {
	counter.n += 1
}
`

// version 3 changes add again
const calcVersion3 = `package calc

func add(a, b int) int {
	// sum
	return b + a
}

func addAll(values []int) int {
	return 0
}

type Counter struct{ n int }

func (counter *Counter) Increment() {
	counter.n += 1
}
`

// version 4 changes only addAll, whose name add is a prefix of
const calcVersion4 = `package calc

func add(a, b int) int {
	// sum
	return b + a
}

func addAll(values []int) int {
	total := 0
	return total
}

type Counter struct{ n int }

func (counter *Counter) Increment() {
	counter.n += 1
}
`

// newCalcRepo commits the four calc.go versions and returns the repo and file path
func newCalcRepo(t *testing.T) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	runTestGit(t, repo, "init", "-q")
	filePath := filepath.Join(repo, "calc.go")
	for index, content := range []string{calcVersion1, calcVersion2, calcVersion3, calcVersion4} {
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))
		runTestGit(t, repo, "add", "calc.go")
		runTestGit(t, repo, "commit", "-q", "-m", "version "+string(rune('1'+index)))
	}
	return repo, filePath
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"-c", "user.email=test@example.com", "-c", "user.name=Test User"}, args...)
	command := exec.Command("git", fullArgs...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, output)
}

func TestGetFunctionChurnInRangeWithRealGit(t *testing.T) {
	repo, filePath := newCalcRepo(t)
	analyzer := NewGitChurnAnalyzer(repo)
	since := time.Now().AddDate(0, 0, -1)

	// add is lines 3-6 at HEAD: created, then changed twice
	metric, err := analyzer.GetFunctionChurnInRange(filePath, 3, 6, since)
	require.NoError(t, err)
	assert.Equal(t, 3, metric.TotalCommits)
	assert.Equal(t, 1, metric.AuthorCount)
	assert.Equal(t, []string{"Test User"}, metric.Contributors)
	assert.Equal(t, 5, metric.LinesAdded, "3 created + 1 comment + 1 rewritten return")
	assert.Equal(t, 1, metric.LinesDeleted)
	assert.False(t, metric.LastModified.IsZero())
}

func TestGetFunctionChurnByNameWithRealGit(t *testing.T) {
	repo, filePath := newCalcRepo(t)
	analyzer := NewGitChurnAnalyzer(repo)
	since := time.Now().AddDate(0, 0, -1)

	cases := map[string]int{
		"add":       3, // not confused with addAll, which it prefixes
		"addAll":    2,
		"Increment": 2, // methods are found too
	}
	for name, commits := range cases {
		metric, err := analyzer.GetFunctionChurn(filePath, name, since)
		require.NoError(t, err, name)
		assert.Equal(t, commits, metric.TotalCommits, name)
	}
}

// Failures must surface as errors (churn unknown), never as a zero metric
func TestGetFunctionChurnErrorsAreNotZero(t *testing.T) {
	repo, filePath := newCalcRepo(t)
	analyzer := NewGitChurnAnalyzer(repo)
	since := time.Now().AddDate(0, 0, -1)

	metric, err := analyzer.GetFunctionChurnInRange(filePath, 100, 120, since)
	assert.Error(t, err, "a range past the end of the file")
	assert.Nil(t, metric)

	metric, err = analyzer.GetFunctionChurnInRange(filePath, 5, 2, since)
	assert.Error(t, err)
	assert.Nil(t, metric)

	metric, err = analyzer.GetFunctionChurn(filePath, "missing", since)
	assert.Error(t, err)
	assert.Nil(t, metric)

	metric, err = analyzer.GetFunctionChurn(filepath.Join(repo, "script.py"), "main", since)
	assert.Error(t, err)
	assert.Nil(t, metric)
}

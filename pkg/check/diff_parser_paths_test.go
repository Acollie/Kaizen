package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiffOutputSpecialPaths(t *testing.T) {
	cases := []struct {
		name     string
		diff     string
		expected []DiffHunk
	}{
		{
			name: "spaces",
			diff: "diff --git a/dir/x y.go b/dir/x y.go\nindex 1..2 100644\n--- a/dir/x y.go\t\n+++ b/dir/x y.go\t\n@@ -1 +1,2 @@\n+a\n",
			expected: []DiffHunk{{FilePath: "dir/x y.go", NewStart: 1, NewCount: 2}},
		},
		{
			name: "quoted",
			diff: "diff --git \"a/q\\\"x.go\" \"b/q\\\"x.go\"\n--- \"a/q\\\"x.go\"\n+++ \"b/q\\\"x.go\"\n@@ -0,0 +1 @@\n+a\n",
			expected: []DiffHunk{{FilePath: `q"x.go`, NewStart: 1, NewCount: 1}},
		},
		{
			name: "unicode octal escapes",
			diff: "diff --git \"a/caf\\303\\251.go\" \"b/caf\\303\\251.go\"\n--- \"a/caf\\303\\251.go\"\n+++ \"b/caf\\303\\251.go\"\n@@ -3 +3 @@\n+a\n",
			expected: []DiffHunk{{FilePath: "café.go", NewStart: 3, NewCount: 1}},
		},
		{
			name: "renamed with spaces",
			diff: "diff --git a/old name.go b/new name.go\nsimilarity index 90%\nrename from old name.go\nrename to new name.go\n--- a/old name.go\t\n+++ b/new name.go\t\n@@ -4 +4 @@\n+a\n",
			expected: []DiffHunk{{FilePath: "new name.go", NewStart: 4, NewCount: 1}},
		},
		{
			name: "deleted file does not leak into the next",
			diff: "diff --git a/gone.go b/gone.go\ndeleted file mode 100644\n--- a/gone.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n" +
				"diff --git \"a/odd\\tname.go\" \"b/odd\\tname.go\"\n--- \"a/odd\\tname.go\"\n+++ \"b/odd\\tname.go\"\n@@ -7 +7,2 @@\n+b\n",
			expected: []DiffHunk{{FilePath: "odd\tname.go", NewStart: 7, NewCount: 2}},
		},
		{
			name:     "unparseable header resets the file",
			diff:     "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n+x\ndiff --git garbage\n@@ -1 +1 @@\n+y\n",
			expected: []DiffHunk{{FilePath: "a.go", NewStart: 1, NewCount: 1}},
		},
		{
			name:     "added line that looks like a +++ header",
			diff:     "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1,2 @@\n+++ b/evil.go\n@@ -9 +10 @@\n+z\n",
			expected: []DiffHunk{{FilePath: "a.go", NewStart: 1, NewCount: 2}, {FilePath: "a.go", NewStart: 10, NewCount: 1}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			hunks, err := ParseDiffOutput(testCase.diff)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, hunks)
		})
	}
}

// TestParseDiffOutputRealGitPaths runs git itself so the parser is checked against
// the quoting git really emits
func TestParseDiffOutputRealGitPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	names := []string{"plain.go", "with space.go", "quote\"d.go", "café.go", "old name.go", "doomed.go"}
	for _, name := range names {
		writeTestFile(t, repo, name, "one\n")
	}
	renameBase := "a\nb\nc\nd\ne\nf\ng\nh\n"
	writeTestFile(t, repo, "old name.go", renameBase)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "base")

	for _, name := range []string{"plain.go", "with space.go", "quote\"d.go", "café.go"} {
		writeTestFile(t, repo, name, "one\ntwo\n")
	}
	runGit(t, repo, "mv", "old name.go", "new name.go")
	writeTestFile(t, repo, "new name.go", "a\nnew\nb\nc\nd\ne\nf\ng\nh\n")
	runGit(t, repo, "rm", "-q", "doomed.go")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "head")

	diff := runGit(t, repo, "diff", "-M", "--unified=0", "--no-color", "HEAD~1", "HEAD")
	hunks, err := ParseDiffOutput(diff)
	require.NoError(t, err)

	paths := make(map[string]int)
	for _, hunk := range hunks {
		paths[hunk.FilePath] = hunk.NewStart
	}
	assert.Equal(t, map[string]int{
		"plain.go": 2, "with space.go": 2, "quote\"d.go": 2, "café.go": 2, "new name.go": 2,
	}, paths)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-c", "user.email=test@example.com", "-c", "user.name=Test", "-c", "core.quotePath=true"}, args...)
	command := exec.Command("git", fullArgs...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, output)
	return string(output)
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

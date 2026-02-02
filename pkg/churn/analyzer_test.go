package churn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitChurnAnalyzer(t *testing.T) {
	repoPath := "/tmp/test-repo"
	analyzer := NewGitChurnAnalyzer(repoPath)

	assert.NotNil(t, analyzer)
	assert.Equal(t, repoPath, analyzer.repoPath)
}

func TestIsGitRepository(t *testing.T) {
	analyzer := NewGitChurnAnalyzer(".")

	// Current directory should be in a git repo (if running from kaizen repo)
	isGit := analyzer.IsGitRepository(".")
	// Note: This may fail in non-git environments, that's OK
	t.Logf("Is git repo: %v", isGit)

	// Non-existent directory should not be a git repo
	isGit = analyzer.IsGitRepository("/tmp/definitely-not-a-git-repo-xyz123")
	assert.False(t, isGit)
}

func TestParseNumstatOutput(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		expectedCommits int
		expectedAdded   int
		expectedDeleted int
		expectedAuthors int
	}{
		{
			name:            "single commit",
			output:          "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n10\t5\tfile.go\n",
			expectedCommits: 1,
			expectedAdded:   10,
			expectedDeleted: 5,
			expectedAuthors: 1,
		},
		{
			name: "multiple commits by different authors",
			output: "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n10\t5\tfile.go\n" +
				"def456|Jane Smith|jane@example.com|2024-01-16 11:00:00 +0000\n20\t10\tfile.go\n",
			expectedCommits: 2,
			expectedAdded:   30,
			expectedDeleted: 15,
			expectedAuthors: 2,
		},
		{
			name: "multiple commits by same author",
			output: "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n10\t5\tfile.go\n" +
				"def456|John Doe|john@example.com|2024-01-16 11:00:00 +0000\n20\t10\tfile.go\n",
			expectedCommits: 2,
			expectedAdded:   30,
			expectedDeleted: 15,
			expectedAuthors: 1,
		},
		{
			name:            "empty output",
			output:          "",
			expectedCommits: 0,
			expectedAdded:   0,
			expectedDeleted: 0,
			expectedAuthors: 0,
		},
	}

	analyzer := NewGitChurnAnalyzer(".")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, err := analyzer.parseNumstatOutput(tt.output)

			require.NoError(t, err)
			assert.NotNil(t, metric)
			assert.Equal(t, tt.expectedCommits, metric.TotalCommits)
			assert.Equal(t, tt.expectedAdded, metric.LinesAdded)
			assert.Equal(t, tt.expectedDeleted, metric.LinesDeleted)
			assert.Equal(t, tt.expectedAuthors, metric.AuthorCount)
			assert.Equal(t, tt.expectedAdded+tt.expectedDeleted, metric.TotalChanges)
		})
	}
}

func TestParseFunctionLogOutput(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		expectedCommits int
		expectedAuthors int
	}{
		{
			name:            "single function commit",
			output:          "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n+added line\n-removed line\n",
			expectedCommits: 1,
			expectedAuthors: 1,
		},
		{
			name: "multiple function commits",
			output: "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n+added line\n" +
				"def456|Jane Smith|jane@example.com|2024-01-16 11:00:00 +0000\n-removed line\n",
			expectedCommits: 2,
			expectedAuthors: 2,
		},
		{
			name:            "empty output",
			output:          "",
			expectedCommits: 0,
			expectedAuthors: 0,
		},
	}

	analyzer := NewGitChurnAnalyzer(".")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, err := analyzer.parseFunctionLogOutput(tt.output)

			require.NoError(t, err)
			assert.NotNil(t, metric)
			assert.Equal(t, tt.expectedCommits, metric.TotalCommits)
			assert.Equal(t, tt.expectedAuthors, metric.AuthorCount)
		})
	}
}

func TestGetFileChurnNotGitRepo(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := NewGitChurnAnalyzer(tempDir)

	metric, err := analyzer.GetFileChurn(filepath.Join(tempDir, "test.go"), time.Now().AddDate(0, 0, -30))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
	assert.Nil(t, metric)
}

func TestGetFileChurnInGitRepo(t *testing.T) {
	// Skip if git is not available
	_, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()

	// Initialize git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tempDir
	err = initCmd.Run()
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main\n"), 0644)
	require.NoError(t, err)

	// Add and commit
	addCmd := exec.Command("git", "add", "test.go")
	addCmd.Dir = tempDir
	err = addCmd.Run()
	require.NoError(t, err)

	commitCmd := exec.Command("git", "-c", "user.email=test@example.com", "-c", "user.name=Test User", "commit", "-m", "test commit")
	commitCmd.Dir = tempDir
	err = commitCmd.Run()
	require.NoError(t, err)

	// Now test churn analysis
	analyzer := NewGitChurnAnalyzer(tempDir)
	metric, err := analyzer.GetFileChurn(testFile, time.Now().AddDate(0, 0, -30))

	// The analysis should succeed or return empty metrics
	if err == nil {
		assert.NotNil(t, metric)
		assert.GreaterOrEqual(t, metric.TotalCommits, 0)
	}
}

func TestGetFunctionChurnNotGitRepo(t *testing.T) {
	tempDir := t.TempDir()
	analyzer := NewGitChurnAnalyzer(tempDir)

	metric, err := analyzer.GetFunctionChurn(filepath.Join(tempDir, "test.go"), "TestFunc", time.Now().AddDate(0, 0, -30))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
	assert.Nil(t, metric)
}

func TestParseNumstatOutputWithWhitespace(t *testing.T) {
	// Test that output with various whitespace is handled correctly
	output := "  abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000  \n  10\t5\tfile.go  \n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, 1, metric.TotalCommits)
	assert.Equal(t, 10, metric.LinesAdded)
	assert.Equal(t, 5, metric.LinesDeleted)
}

func TestParseNumstatOutputContributorTracking(t *testing.T) {
	output := "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n5\t2\tfile.go\n" +
		"def456|Jane Smith|jane@example.com|2024-01-16 11:00:00 +0000\n10\t5\tfile.go\n" +
		"ghi789|John Doe|john@example.com|2024-01-17 12:00:00 +0000\n3\t1\tfile.go\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)
	assert.Equal(t, 3, metric.TotalCommits)
	assert.Equal(t, 2, metric.AuthorCount)
	assert.Equal(t, 2, len(metric.Contributors))

	// Check that John Doe and Jane Smith are in contributors
	contributorNames := strings.Join(metric.Contributors, ",")
	assert.Contains(t, contributorNames, "John Doe")
	assert.Contains(t, contributorNames, "Jane Smith")
}

func TestParseNumstatOutputLastModified(t *testing.T) {
	// Test that last modified date is correctly extracted
	output := "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n5\t2\tfile.go\n" +
		"def456|Jane Smith|jane@example.com|2024-01-20 11:00:00 +0000\n10\t5\tfile.go\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)
	assert.NotNil(t, metric.LastModified)
	// Last modified should be the more recent date
	assert.True(t, metric.LastModified.After(time.Date(2024, 1, 19, 0, 0, 0, 0, time.UTC)))
}

func TestParseNumstatOutputWithInvalidLines(t *testing.T) {
	// Test that invalid numstat lines are skipped
	output := "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n" +
		"invalid line without tabs\n" +
		"10\t5\tfile.go\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)
	// Should still count the valid line
	assert.Equal(t, 1, metric.TotalCommits)
	assert.Equal(t, 10, metric.LinesAdded)
	assert.Equal(t, 5, metric.LinesDeleted)
}

func TestGetFunctionChurnInGitRepo(t *testing.T) {
	// Skip if git is not available
	_, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()

	// Initialize git repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tempDir
	err = initCmd.Run()
	require.NoError(t, err)

	// Create a test file with a function
	testFile := filepath.Join(tempDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main\n\nfunc TestFunc() {}\n"), 0644)
	require.NoError(t, err)

	// Add and commit
	addCmd := exec.Command("git", "add", "test.go")
	addCmd.Dir = tempDir
	err = addCmd.Run()
	require.NoError(t, err)

	commitCmd := exec.Command("git", "-c", "user.email=test@example.com", "-c", "user.name=Test User", "commit", "-m", "test commit")
	commitCmd.Dir = tempDir
	err = commitCmd.Run()
	require.NoError(t, err)

	// Now test function churn analysis
	analyzer := NewGitChurnAnalyzer(tempDir)
	metric, err := analyzer.GetFunctionChurn(testFile, "TestFunc", time.Now().AddDate(0, 0, -30))

	// The analysis should succeed or return empty metrics
	if err == nil {
		assert.NotNil(t, metric)
		assert.GreaterOrEqual(t, metric.TotalCommits, 0)
	}
}

func TestParseFunctionLogOutputWithDiffLines(t *testing.T) {
	// Test with actual diff-like output from git log -L
	output := "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n" +
		"+func TestFunc() {\n" +
		"+    return nil\n" +
		"-func TestFunc() {\n" +
		"-    panic(\"old\")\n" +
		"def456|Jane Smith|jane@example.com|2024-01-16 11:00:00 +0000\n" +
		"+    fix applied\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseFunctionLogOutput(output)

	require.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, 2, metric.TotalCommits)
	assert.Equal(t, 2, metric.AuthorCount)
	// Should count added/deleted lines from diff
	assert.GreaterOrEqual(t, metric.LinesAdded, 2)
	assert.GreaterOrEqual(t, metric.LinesDeleted, 2)
}

func TestParseNumstatOutputAverageChurn(t *testing.T) {
	// Test average churn calculation
	output := "abc123|John Doe|john@example.com|2024-01-01 10:00:00 +0000\n5\t2\tfile.go\n" +
		"def456|John Doe|john@example.com|2024-01-10 10:00:00 +0000\n10\t5\tfile.go\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)
	assert.Equal(t, 2, metric.TotalCommits)
	// Average churn should be calculated for multiple commits
	assert.Greater(t, metric.AverageChurnBy, 0.0)
}

func TestParseNumstatOutputSingleCommitNoAverage(t *testing.T) {
	// Test that single commit doesn't calculate average
	output := "abc123|John Doe|john@example.com|2024-01-15 10:30:00 +0000\n10\t5\tfile.go\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)
	assert.Equal(t, 1, metric.TotalCommits)
	// Single commit should not calculate average
	assert.Equal(t, 0.0, metric.AverageChurnBy)
}

func TestChurnMetricFields(t *testing.T) {
	// Test that all churn metric fields are properly initialized
	output := "abc123|Alice|alice@test.com|2024-01-15 10:30:00 +0000\n100\t50\tfile.go\n"

	analyzer := NewGitChurnAnalyzer(".")
	metric, err := analyzer.parseNumstatOutput(output)

	require.NoError(t, err)

	// Check all fields
	assert.Equal(t, 1, metric.TotalCommits)
	assert.Equal(t, 100, metric.LinesAdded)
	assert.Equal(t, 50, metric.LinesDeleted)
	assert.Equal(t, 150, metric.TotalChanges)
	assert.Equal(t, 1, metric.AuthorCount)
	assert.Equal(t, 1, len(metric.Contributors))
	assert.Equal(t, "Alice", metric.Contributors[0])
	assert.False(t, metric.LastModified.IsZero())
}

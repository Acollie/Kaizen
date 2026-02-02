package ownership

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodeOwnersBasic(t *testing.T) {
	content := `* @maintainers
pkg/storage/ @storage-team
pkg/api/ @api-team @shared-team`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)

	require.NoError(t, err)
	assert.NotNil(t, codeowners)
	assert.Equal(t, codeownersPath, codeowners.Path)
	assert.Len(t, codeowners.Rules, 3)
}

func TestParseCodeOwnersWithComments(t *testing.T) {
	content := `# This is a comment
* @maintainers

# Team assignments
pkg/storage/ @storage-team
pkg/api/ @api-team`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)

	require.NoError(t, err)
	assert.NotNil(t, codeowners)
	// Comments and blank lines should be skipped
	assert.Len(t, codeowners.Rules, 3)
}

func TestParseCodeOwnersWithEmptyLines(t *testing.T) {
	content := `* @maintainers

pkg/storage/ @storage-team

pkg/api/ @api-team`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)

	require.NoError(t, err)
	assert.Len(t, codeowners.Rules, 3)
}

func TestParseCodeOwnersFileNotFound(t *testing.T) {
	codeowners, err := ParseCodeOwners("/nonexistent/path/CODEOWNERS")

	assert.Error(t, err)
	assert.Nil(t, codeowners)
	assert.Contains(t, err.Error(), "could not open CODEOWNERS file")
}

func TestParseCodeOwnersNegationPatterns(t *testing.T) {
	content := `* @maintainers
!*.md @docs-team`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)

	require.NoError(t, err)
	assert.NotNil(t, codeowners)
	assert.Len(t, codeowners.Rules, 2)
	assert.Equal(t, "!*.md", codeowners.Rules[1].Pattern)
}

func TestParseRuleBasic(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		shouldError bool
		expectedLen int
	}{
		{
			name:        "single owner",
			line:        "* @maintainers",
			shouldError: false,
			expectedLen: 1,
		},
		{
			name:        "multiple owners",
			line:        "pkg/api/ @api-team @shared-team",
			shouldError: false,
			expectedLen: 2,
		},
		{
			name:        "pattern with multiple spaces",
			line:        "src/admin   @admin-team   @backup-team",
			shouldError: false,
			expectedLen: 2,
		},
		{
			name:        "email owner",
			line:        "*.md john@example.com",
			shouldError: false,
			expectedLen: 1,
		},
		{
			name:        "invalid - no owners",
			line:        "* ",
			shouldError: true,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := parseRule(tt.line, 1)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, rule.Owners, tt.expectedLen)
			}
		})
	}
}

func TestParseRuleLineNumber(t *testing.T) {
	rule, err := parseRule("* @team", 42)

	require.NoError(t, err)
	assert.Equal(t, 42, rule.LineNumber)
}

func TestParseRuleOwnerNormalization(t *testing.T) {
	// Test that owners with @ are preserved
	rule1, err := parseRule("* @team-name", 1)
	require.NoError(t, err)
	assert.Equal(t, "@team-name", rule1.Owners[0])

	// Test that email addresses are preserved
	rule2, err := parseRule("* user@example.com", 1)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", rule2.Owners[0])
}

func TestParseRulePattern(t *testing.T) {
	tests := []struct {
		line           string
		expectedPattern string
	}{
		{"* @team", "*"},
		{"pkg/api/ @team", "pkg/api/"},
		{"src/**/*.js @team", "src/**/*.js"},
		{"/absolute/path @team", "/absolute/path"},
		{"!negative.pattern @team", "!negative.pattern"},
	}

	for _, tt := range tests {
		rule, err := parseRule(tt.line, 1)
		require.NoError(t, err)
		assert.Equal(t, tt.expectedPattern, rule.Pattern)
	}
}

// TestCodeOwnersGetOwners tests the GetOwners method
func TestCodeOwnersGetOwners(t *testing.T) {
	content := `* @maintainers
pkg/storage/ @storage-team
pkg/storage/sqlite.go @db-expert`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)
	require.NoError(t, err)

	tests := []struct {
		path           string
		expectedOwners []string
	}{
		{"main.go", []string{"@maintainers"}},
		{"pkg/storage/sqlite.go", []string{"@db-expert"}},
		{"pkg/storage/migrations.go", []string{"@storage-team"}},
		{"pkg/api/api.go", []string{"@maintainers"}},
	}

	for _, tt := range tests {
		owners := codeowners.GetOwners(tt.path)
		assert.Equal(t, tt.expectedOwners, owners, "path=%q", tt.path)
	}
}

func TestCodeOwnersMultipleRules(t *testing.T) {
	content := `# Global default
* @maintainers

# Team assignments
pkg/storage/ @storage-team
pkg/api/ @api-team
pkg/ui/ @frontend-team

# Override for specific storage file
pkg/storage/sqlite.go @db-expert`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)

	require.NoError(t, err)
	assert.Len(t, codeowners.Rules, 5)

	// Verify first rule
	assert.Equal(t, "*", codeowners.Rules[0].Pattern)
	assert.Equal(t, []string{"@maintainers"}, codeowners.Rules[0].Owners)

	// Verify sqlite override rule
	assert.Equal(t, "pkg/storage/sqlite.go", codeowners.Rules[4].Pattern)
	assert.Equal(t, []string{"@db-expert"}, codeowners.Rules[4].Owners)
}

// TestCodeOwnersLastMatchWins tests that CODEOWNERS uses last-match-wins semantics
func TestCodeOwnersLastMatchWins(t *testing.T) {
	content := `* @maintainers
pkg/storage/ @storage-team
pkg/storage/sqlite.go @db-expert
*.md @docs-team`

	tempDir := t.TempDir()
	codeownersPath := filepath.Join(tempDir, "CODEOWNERS")
	err := os.WriteFile(codeownersPath, []byte(content), 0644)
	require.NoError(t, err)

	codeowners, err := ParseCodeOwners(codeownersPath)
	require.NoError(t, err)

	// pkg/storage/sqlite.go matches both "pkg/storage/" and "pkg/storage/sqlite.go"
	// The last match should win: @db-expert
	owners := codeowners.GetOwners("pkg/storage/sqlite.go")

	// Since we're testing the parsing, we should have the most specific rule
	assert.NotEmpty(t, owners)
	assert.Contains(t, owners, "@db-expert")
}

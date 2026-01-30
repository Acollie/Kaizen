package ownership

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseCodeOwners parses a CODEOWNERS file
func ParseCodeOwners(path string) (*CodeOwners, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open CODEOWNERS file: %w", err)
	}
	defer file.Close()

	codeowners := &CodeOwners{
		Path:  path,
		Rules: []OwnershipRule{},
	}

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse rule
		rule, err := parseRule(line, lineNumber)
		if err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to parse CODEOWNERS line %d: %v\n", lineNumber, err)
			continue
		}

		codeowners.Rules = append(codeowners.Rules, rule)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading CODEOWNERS file: %w", err)
	}

	return codeowners, nil
}

// parseRule parses a single CODEOWNERS rule line
func parseRule(line string, lineNumber int) (OwnershipRule, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return OwnershipRule{}, fmt.Errorf("invalid rule format (expected pattern and at least one owner)")
	}

	pattern := parts[0]
	owners := parts[1:]

	// Validate owners (should start with @ for GitHub/GitLab)
	for i, owner := range owners {
		// Allow email addresses too
		if !strings.HasPrefix(owner, "@") && !strings.Contains(owner, "@") {
			return OwnershipRule{}, fmt.Errorf("invalid owner format: %s (should start with @ or be email)", owner)
		}
		// Normalize - add @ if missing and it's a username
		if !strings.HasPrefix(owner, "@") && !strings.Contains(owner, "@") {
			owners[i] = "@" + owner
		}
	}

	return OwnershipRule{
		Pattern:    pattern,
		Owners:     owners,
		LineNumber: lineNumber,
	}, nil
}

// GetOwners returns the owners for a given file path
// Last matching rule wins (GitHub semantics)
func (co *CodeOwners) GetOwners(filePath string) []string {
	var lastMatch []string

	for _, rule := range co.Rules {
		if matchesPattern(filePath, rule.Pattern) {
			lastMatch = rule.Owners
		}
	}

	return lastMatch
}

// GetOwnersWithPattern returns owners and the matching pattern
func (co *CodeOwners) GetOwnersWithPattern(filePath string) ([]string, string) {
	var lastMatch []string
	var lastPattern string

	for _, rule := range co.Rules {
		if matchesPattern(filePath, rule.Pattern) {
			lastMatch = rule.Owners
			lastPattern = rule.Pattern
		}
	}

	return lastMatch, lastPattern
}

// matchesPattern checks if a file path matches a CODEOWNERS pattern
func matchesPattern(filePath, pattern string) bool {
	// Normalize paths
	filePath = strings.TrimPrefix(filePath, "./")
	pattern = strings.TrimPrefix(pattern, "./")

	// Exact match
	if filePath == pattern {
		return true
	}

	// Directory match (pattern ends with /)
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(filePath, dir+"/")
	}

	// Wildcard patterns
	if strings.Contains(pattern, "*") {
		return globMatch(filePath, pattern)
	}

	// Suffix match (pattern doesn't start with /)
	if !strings.HasPrefix(pattern, "/") && strings.HasSuffix(filePath, pattern) {
		return true
	}

	// Prefix match (pattern is directory-like)
	if strings.HasPrefix(filePath, pattern+"/") {
		return true
	}

	return false
}

// globMatch implements basic glob pattern matching
func globMatch(path, pattern string) bool {
	// Handle ** (matches any number of directories)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) != 2 {
			return false
		}

		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		if suffix != "" && !strings.HasSuffix(path, suffix) {
			return false
		}

		return true
	}

	// Handle single * (matches within a directory)
	if strings.Contains(pattern, "*") {
		// Simple * matching
		parts := strings.Split(pattern, "*")
		if len(parts) != 2 {
			return false
		}

		prefix := parts[0]
		suffix := parts[1]

		if !strings.HasPrefix(path, prefix) {
			return false
		}

		if !strings.HasSuffix(path, suffix) {
			return false
		}

		// Make sure * doesn't match across directory boundaries
		if suffix != "" && strings.Contains(suffix, "/") {
			// The part after * must come after one directory level
			remainder := strings.TrimPrefix(path, prefix)
			if strings.Count(remainder[:len(remainder)-len(suffix)], "/") > 0 {
				return false
			}
		}

		return true
	}

	return false
}

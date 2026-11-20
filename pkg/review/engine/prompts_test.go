package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexcollie/kaizen/pkg/review"
)

func TestLeadPromptSpreadsCoverageWithPackageReviewers(t *testing.T) {
	prompt := leadPrompt(review.PullRequest{Number: 4, Title: "Add thing"})
	assert.Contains(t, prompt, "mcp__kaizen__get_package_overview")
	assert.Contains(t, prompt, fmt.Sprintf("One %s per changed package with meaningful non-test code", agentPackage))
	assert.Contains(t, prompt, fmt.Sprintf("at most %d, chosen by max risk", maxPackageReviewers))
	assert.Contains(t, prompt, "which changed packages got no dedicated review")
	assert.NotContains(t, prompt, "%!", "every format verb is filled")
}

func TestPackageReviewerIsToldToBeEfficient(t *testing.T) {
	definition := subagentDefinitions("haiku")[agentPackage]
	assert.Equal(t, "haiku", definition.Model)
	assert.Equal(t, subagentTools, definition.Tools)
	for _, phrase := range []string{"Be efficient", "Skim test files", "report only issues you verified", "Contract mismatches between files", "mcp__kaizen__get_file_diff"} {
		assert.Contains(t, definition.Prompt, phrase)
	}
}

func TestAPIPromptHasPackagePass(t *testing.T) {
	assert.Contains(t, apiSystemPrompt, "then get_package_overview")
	assert.Contains(t, apiSystemPrompt, "Package pass")
	assert.Contains(t, apiSystemPrompt, "which changed packages you did not examine")
}

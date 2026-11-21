package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review/engine"
)

func TestSelectReviewEnginePassesEffortToAPIEngine(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	selected, err := selectReviewEngine(reviewFlags{engineName: "api", model: "claude-haiku-4-5", effort: "low"})
	require.NoError(t, err)
	apiEngine, ok := selected.(*engine.APIEngine)
	require.True(t, ok, "expected *engine.APIEngine, got %T", selected)
	assert.Equal(t, "low", apiEngine.Effort)
	assert.Equal(t, "claude-haiku-4-5", apiEngine.Model)
}

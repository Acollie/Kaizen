package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexcollie/kaizen/pkg/review"
)

func TestTerminalSafeStripsControlCharacters(t *testing.T) {
	assert.Equal(t, "fix [2Jclear", terminalSafe("fix \x1b[2Jclear"))
	assert.Equal(t, "a title", terminalSafe("a\r title\a"))
	assert.Equal(t, "naïve ✓", terminalSafe("naïve ✓"))
}

func TestReviewCacheKeyChangesWithSettings(t *testing.T) {
	cli := cacheKeyEngine{name: "claude-cli"}
	base := reviewFlags{effort: "high", focus: 5}
	assert.Equal(t, "claude-cli model= effort=high focus=5", reviewCacheKey(cli, base))

	withModel := base
	withModel.model = "sonnet"
	assert.NotEqual(t, reviewCacheKey(cli, base), reviewCacheKey(cli, withModel))

	withFocus := base
	withFocus.focus = 8
	assert.NotEqual(t, reviewCacheKey(cli, base), reviewCacheKey(cli, withFocus))
	assert.Empty(t, reviewCacheKey(nil, base))
}

type cacheKeyEngine struct{ name string }

func (engine cacheKeyEngine) Name() string { return engine.name }
func (engine cacheKeyEngine) Run(context.Context, *review.ReviewContext, review.EmitFunc) (*review.ReviewResult, error) {
	return nil, nil
}

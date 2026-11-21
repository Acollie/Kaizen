package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/reviewui"
)

// namedEngine is an engine that only has a name; replay never runs it
type namedEngine struct{ name string }

func (engine namedEngine) Name() string { return engine.name }

func (engine namedEngine) Run(context.Context, *review.ReviewContext, review.EmitFunc) (*review.ReviewResult, error) {
	return &review.ReviewResult{}, nil
}

func TestReplayCachedReviewOnlyForSameEngine(t *testing.T) {
	pr := review.PullRequest{Number: 5, HeadSHA: "abcdef1234567890"}
	cachePath := review.ReviewCachePath(t.TempDir(), pr, reviewCacheKey(namedEngine{name: "claude-cli"}, reviewFlags{effort: "high", focus: 5}))
	settings := reviewFlags{effort: "high", focus: 5}
	require.NoError(t, review.SaveReviewCache(cachePath, pr, reviewCacheKey(namedEngine{name: "claude-cli"}, settings), []review.Event{
		review.NewEvent(review.EventResult, review.ReviewResult{Verdict: review.VerdictComment}),
	}))

	replay := func(engineName string) bool {
		engine := namedEngine{name: engineName}
		return replayCachedReview(reviewRun{
			engine:    engine,
			context:   &review.ReviewContext{PR: pr},
			events:    reviewui.NewEventLog(),
			cachePath: cachePath,
			cacheKey:  reviewCacheKey(engine, settings),
		})
	}
	assert.False(t, replay("anthropic-api"), "--engine=api must not replay a claude-cli review")
	assert.True(t, replay("claude-cli"))
}

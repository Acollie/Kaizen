package engine

import (
	"context"
	"encoding/json"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
)

func TestAPIEffortMapping(t *testing.T) {
	cases := map[string]anthropic.BetaOutputConfigEffort{
		"":       anthropic.BetaOutputConfigEffortHigh,
		"low":    anthropic.BetaOutputConfigEffortLow,
		"medium": anthropic.BetaOutputConfigEffortMedium,
		"High":   anthropic.BetaOutputConfigEffortHigh,
		"xhigh":  anthropic.BetaOutputConfigEffortXhigh,
		" max ":  anthropic.BetaOutputConfigEffortMax,
	}
	for name, want := range cases {
		got, err := apiEffort(name)
		require.NoError(t, err, name)
		assert.Equal(t, want, got, name)
	}
	_, err := apiEffort("extreme")
	assert.ErrorContains(t, err, `unknown effort "extreme"`)
}

func TestRunnerParamsUseConfiguredEffort(t *testing.T) {
	engine := (&APIEngine{Effort: "low"}).resolved()
	params := engine.runnerParams(review.PullRequest{Number: 3}, engine.MaxIterations)
	encoded, err := json.Marshal(params.BetaMessageNewParams)
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))
	assert.Equal(t, map[string]any{"effort": "low"}, wire["output_config"])
}

func TestAPIEngineRejectsUnknownEffort(t *testing.T) {
	_, err := (&APIEngine{Effort: "turbo"}).Run(context.Background(), liveContext(t), nil)
	assert.ErrorContains(t, err, `unknown effort "turbo"`)
}

package engine

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Live tests spend real tokens. Run them by hand:
//
//	KAIZEN_LIVE_TEST=1 go test -v -run TestLive ./pkg/review/engine
//	KAIZEN_LIVE_TEST=1 ANTHROPIC_API_KEY=... go test -v -run TestLiveAPI ./pkg/review/engine

const liveBuggySource = `package calc

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a - b
}

// Total sums a slice with Add.
func Total(values []int) int {
	sum := 0
	for _, value := range values {
		sum = Add(sum, value)
	}
	return sum
}
`

func liveContext(t *testing.T) *review.ReviewContext {
	t.Helper()
	worktree := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(worktree, "calc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "calc", "calc.go"), []byte(liveBuggySource), 0o644))
	return &review.ReviewContext{
		PR: review.PullRequest{Number: 1, Title: "Simplify Add", Author: "dev", BaseRef: "main", HeadRef: "simplify",
			Additions: 1, Deletions: 1, FileCount: 1},
		Files: []review.FileDiff{{
			Path: "calc/calc.go", OldPath: "calc/calc.go", Status: review.FileModified, Language: "Go",
			Additions: 1, Deletions: 1, LineCount: 15, RiskScore: 55,
			Hunks: []review.Hunk{{OldStart: 4, OldCount: 3, NewStart: 4, NewCount: 3, Header: "func Add(a, b int) int {",
				Lines: []review.DiffLine{
					{Kind: review.LineContext, OldLine: 4, NewLine: 4, Text: "func Add(a, b int) int {"},
					{Kind: review.LineRemoved, OldLine: 5, Text: "\treturn a + b"},
					{Kind: review.LineAdded, NewLine: 5, Text: "\treturn a - b"},
					{Kind: review.LineContext, OldLine: 6, NewLine: 6, Text: "}"},
				}}},
		}},
		Functions: []review.FunctionRisk{{
			ID: "example.com/calc.Add", Name: "Add", File: "calc/calc.go", Language: "Go", StartLine: 4, EndLine: 6,
			LinesAdded: 1, LinesRemoved: 1, FanIn: 1,
			Callers:   []review.FunctionRef{{ID: "example.com/calc.Total", Name: "Total", File: "calc/calc.go", Line: 12}},
			RiskScore: 55, RiskReasons: []string{"fan-in 1", "return expression changed"}, Focus: true,
		}},
		WorktreePath: worktree,
		GeneratedAt:  time.Now(),
	}
}

// eventRecorder collects events; it is safe for concurrent use
type eventRecorder struct {
	mu     sync.Mutex
	events []review.Event
}

func (recorder *eventRecorder) emit(event review.Event) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
}

func (recorder *eventRecorder) snapshot() []review.Event {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]review.Event(nil), recorder.events...)
}

func (recorder *eventRecorder) ofType(eventType review.EventType) []review.Event {
	var matching []review.Event
	for _, event := range recorder.snapshot() {
		if event.Type == eventType {
			matching = append(matching, event)
		}
	}
	return matching
}

func buildKaizen(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "kaizen")
	build := exec.Command("go", "build", "-o", binary, "github.com/alexcollie/kaizen/cmd/kaizen")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	return binary
}

func TestLiveCLIEngine(t *testing.T) {
	if os.Getenv("KAIZEN_LIVE_TEST") != "1" {
		t.Skip("set KAIZEN_LIVE_TEST=1 to run against the real claude CLI")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not on PATH")
	}
	engine := &CLIEngine{KaizenPath: buildKaizen(t), Model: "haiku", Effort: "low"}
	recorder := &eventRecorder{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := engine.Run(ctx, liveContext(t), recorder.emit)
	for _, event := range recorder.snapshot() {
		encoded, _ := json.Marshal(event.Data)
		t.Logf("%-9s %s", event.Type, encoded)
	}
	require.NoError(t, err)
	t.Logf("verdict=%s cost=$%.4f findings=%d\n%s", result.Verdict, result.CostUSD, len(result.Findings), result.Summary)
	assert.NotEmpty(t, recorder.ofType(review.EventToolCall))
	assert.NotEmpty(t, result.Findings, "expected the subtraction bug to be found")
	assert.NotEqual(t, review.VerdictApprove, result.Verdict)
}

func TestLiveAPIEngine(t *testing.T) {
	if os.Getenv("KAIZEN_LIVE_TEST") != "1" || os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("set KAIZEN_LIVE_TEST=1 and ANTHROPIC_API_KEY to run against the real API")
	}
	engine := &APIEngine{Model: "claude-haiku-5-5", MaxIterations: 15}
	recorder := &eventRecorder{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := engine.Run(ctx, liveContext(t), recorder.emit)
	require.NoError(t, err)
	t.Logf("verdict=%s findings=%d\n%s", result.Verdict, len(result.Findings), result.Summary)
	assert.NotEmpty(t, result.Findings)
	assert.NotEmpty(t, recorder.ofType(review.EventFinding))
}

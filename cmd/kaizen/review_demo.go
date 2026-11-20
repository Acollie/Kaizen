package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/reviewui"
	"github.com/alexcollie/kaizen/pkg/visualization"
	"github.com/spf13/cobra"
)

var (
	reviewDemoPort  int
	reviewDemoOpen  bool
	reviewDemoSpeed float64
)

var reviewDemoCmd = &cobra.Command{
	Use:    "review-demo",
	Short:  "Serve the PR review UI with a fixture PR and a scripted AI event stream",
	Hidden: true,
	Long: `Serves the review web UI from a built-in fixture context and replays a scripted
stream of engine events (status, thinking, tool calls, findings and a final result),
so the UI can be developed and demoed without GitHub or an AI engine. Posting is
simulated: the request is logged and a fake URL is returned.`,
	RunE: runReviewDemo,
}

func init() {
	reviewDemoCmd.Flags().IntVar(&reviewDemoPort, "port", 0, "Port to listen on (0 picks a free port)")
	reviewDemoCmd.Flags().BoolVar(&reviewDemoOpen, "open", true, "Open the UI in the default browser")
	reviewDemoCmd.Flags().Float64Var(&reviewDemoSpeed, "speed", 1, "Replay speed multiplier (2 = twice as fast)")
	rootCmd.AddCommand(reviewDemoCmd)
}

// runReviewDemo serves the fixture until interrupted
func runReviewDemo(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reviewContext, cleanup, err := reviewui.LoadDemoContext()
	if err != nil {
		return err
	}
	defer cleanup()

	// A throwaway state file: the demo never touches a real review's saved work
	stateDir, err := os.MkdirTemp("", "kaizen-review-demo-state-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stateDir) }()

	output := cmd.OutOrStdout()
	eventLog := reviewui.NewEventLog()
	server, err := reviewui.New(reviewui.Options{
		Context:   reviewContext,
		Events:    eventLog,
		Poster:    &demoPoster{output: output},
		Addr:      fmt.Sprintf("127.0.0.1:%d", reviewDemoPort),
		StatePath: filepath.Join(stateDir, "state.json"),
	})
	if err != nil {
		return err
	}
	url, err := server.Start()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(output, "Review demo for PR #%d running at:\n  %s\nPress Ctrl-C to stop.\n",
		reviewContext.PR.Number, url)
	if reviewDemoOpen {
		if err := visualization.OpenInBrowser(url); err != nil {
			_, _ = fmt.Fprintf(output, "Could not open a browser: %v\n", err)
		}
	}

	go replayDemoScript(ctx, eventLog, demoScript(), reviewDemoSpeed)

	<-ctx.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return server.Shutdown(shutdownContext)
}

// demoStep is one scripted event and the pause before it
type demoStep struct {
	delay time.Duration
	event review.Event
}

// replayDemoScript emits each step after its delay, then closes the log
func replayDemoScript(ctx context.Context, eventLog *reviewui.EventLog, steps []demoStep, speed float64) {
	defer eventLog.Close()
	if speed <= 0 {
		speed = 1
	}
	for _, step := range steps {
		timer := time.NewTimer(time.Duration(float64(step.delay) / speed))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		eventLog.Emit(review.NewEvent(step.event.Type, step.event.Data))
	}
}

// demoPoster pretends to post a review: it logs the request and returns a fake URL
type demoPoster struct {
	output io.Writer
}

// PostReview logs the request and classifies findings the way the GitHub poster would
func (poster *demoPoster) PostReview(ctx context.Context, pr review.PullRequest, files []review.FileDiff, request review.PostRequest) (*review.PostResult, error) {
	inline, body := 0, 0
	for _, finding := range request.Findings {
		if findingOnDiffLine(finding, files) {
			inline++
		} else {
			body++
		}
	}
	_, _ = fmt.Fprintf(poster.output, "[demo] would post review to %s/%s#%d: %d inline comments, %d in body, summary %d chars\n",
		pr.Owner, pr.Repo, pr.Number, inline, body, len(request.Summary))
	for _, finding := range request.Findings {
		_, _ = fmt.Fprintf(poster.output, "[demo]   %-10s %s:%d %s\n", finding.Severity, finding.File, finding.Line, finding.Title)
	}
	return &review.PostResult{
		URL:            fmt.Sprintf("https://github.com/%s/%s/pull/%d#pullrequestreview-demo", pr.Owner, pr.Repo, pr.Number),
		InlineComments: inline,
		BodyFindings:   body,
	}, nil
}

// findingOnDiffLine reports whether a finding's line is a head-side line shown in the diff
func findingOnDiffLine(finding review.Finding, files []review.FileDiff) bool {
	if finding.Line <= 0 {
		return false
	}
	for _, file := range files {
		if file.Path != finding.File {
			continue
		}
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind != review.LineRemoved && line.NewLine == finding.Line {
					return true
				}
			}
		}
	}
	return false
}

// demoScript is the scripted engine run replayed by review-demo
func demoScript() []demoStep {
	findings := demoFindings()
	steps := []demoStep{
		{300 * time.Millisecond, statusEvent("lead", "Loading review context for PR #142 (8 changed functions, 3 focus)")},
		{900 * time.Millisecond, thinkingEvent("lead", "The PR touches two hotspots: Pipeline.Analyze gains a worker pool and GetFileChurn gains a cache and retry path. "+
			"The new churnCache is reached from every worker goroutine, so its locking is the first thing to check. "+
			"I'll dispatch the hotspot reviewer on the churn package, a concurrency pass on Analyze, and a quick look at the Python report script.")},
		{700 * time.Millisecond, toolEvent("lead", "get_focus_functions", "get_focus_functions()")},
		{800 * time.Millisecond, statusEvent("hotspot-reviewer", "Dispatching hotspot-reviewer for GetFileChurn, churnCache.Get")},
		{900 * time.Millisecond, toolEvent("hotspot-reviewer", "Read", "Read(pkg/churn/cache.go)")},
		{1000 * time.Millisecond, thinkingEvent("hotspot-reviewer", "Get takes RLock and then calls order.MoveToFront, which rewires the list's next/prev pointers. "+
			"Two concurrent Gets on different keys both mutate the list under a shared lock. That's a data race and can corrupt the LRU.")},
		{900 * time.Millisecond, findingEvent(findings[0])},
		{800 * time.Millisecond, toolEvent("hotspot-reviewer", "get_callers", "get_callers(GetFileChurn)")},
		{900 * time.Millisecond, findingEvent(findings[1])},
		{1000 * time.Millisecond, findingEvent(findings[2])},
		{700 * time.Millisecond, statusEvent("concurrency-reviewer", "Dispatching concurrency-reviewer for Pipeline.Analyze")},
		{900 * time.Millisecond, toolEvent("concurrency-reviewer", "get_function", "get_function(Pipeline.Analyze)")},
		{800 * time.Millisecond, thinkingEvent("concurrency-reviewer", "Workers write results[index] for distinct indices, which is safe without a lock. "+
			"The cancellation path closes jobs and waits, which is correct. AnalyzeFile errors are dropped silently, though, both before and after this change.")},
		{900 * time.Millisecond, findingEvent(findings[3])},
		{800 * time.Millisecond, findingEvent(findings[4])},
		{700 * time.Millisecond, toolEvent("hotspot-reviewer", "Grep", "Grep(\"--format=%H|%an|%at\")")},
		{900 * time.Millisecond, findingEvent(findings[5])},
		{800 * time.Millisecond, statusEvent("python-reviewer", "Dispatching python-reviewer for summarize_churn")},
		{900 * time.Millisecond, toolEvent("python-reviewer", "Read", "Read(scripts/churn_report.py)")},
		{900 * time.Millisecond, findingEvent(findings[6])},
		{800 * time.Millisecond, findingEvent(findings[7])},
		{700 * time.Millisecond, findingEvent(findings[8])},
		{900 * time.Millisecond, statusEvent("lead", "Merging findings from 3 reviewers")},
		{1000 * time.Millisecond, thinkingEvent("lead", "One critical data race in the new cache, two behaviour changes in GetFileChurn that callers won't expect, "+
			"and a small crash in the report script. The parallel pipeline itself looks sound. Requesting changes.")},
		{700 * time.Millisecond, findingEvent(findings[9])},
		{1200 * time.Millisecond, resultEvent(findings)},
		{200 * time.Millisecond, review.NewEvent(review.EventDone, nil)},
	}
	return steps
}

// demoFindings are the findings the scripted reviewers report
func demoFindings() []review.Finding {
	return []review.Finding{
		{ID: "f1", Severity: review.SeverityCritical, Category: review.CategoryCorrectness, File: "pkg/churn/cache.go", Line: 41,
			FunctionID: "github.com/alexcollie/kaizen/pkg/churn.(*churnCache).Get",
			Title:      "Data race: Get mutates the LRU list under a read lock",
			Body: "`MoveToFront` rewrites the list's `prev`/`next` pointers, but `Get` only holds `RLock`. " +
				"`Pipeline.Analyze` now calls `GetFileChurn` from one goroutine per CPU, so two concurrent `Get`s can corrupt the list " +
				"and `go test -race` will flag it.\n\nTake the write lock in `Get` (the critical section is tiny), or drop recency updates on read.",
			Suggestion: "\tcache.mutex.Lock()\n\tdefer cache.mutex.Unlock()",
			Confidence: 0.95, Source: "hotspot-reviewer"},
		{ID: "f2", Severity: review.SeverityWarning, Category: review.CategoryCorrectness, File: "pkg/churn/analyzer.go", Line: 61,
			FunctionID: "github.com/alexcollie/kaizen/pkg/churn.(*Analyzer).GetFileChurn",
			Title:      "Git failures now silently report zero churn",
			Body: "Previously a failing `git log` returned an error. Now any non-cancellation failure returns an empty `ChurnMetric`, " +
				"so a missing git binary or a shallow clone shows up as \"0 commits\" everywhere, and `runDiff` will skip every file.\n\n" +
				"Return the error (wrapped) and let callers decide; `analyzeChurn` already ignores errors.",
			Confidence: 0.85, Source: "hotspot-reviewer"},
		{ID: "f3", Severity: review.SeverityWarning, Category: review.CategoryCorrectness, File: "pkg/churn/analyzer.go", Line: 95, EndLine: 96,
			FunctionID: "github.com/alexcollie/kaizen/pkg/churn.(*Analyzer).runGitWithRetry",
			Title:      "Backoff sleep ignores context cancellation",
			Body: "`time.Sleep` blocks for up to 200ms + 400ms + 800ms + 1.6s even after the user hits Ctrl-C, and it also sleeps after the final attempt. " +
				"Wait on a timer and `ctx.Done()` together instead.",
			Suggestion: "\t\tselect {\n\t\tcase <-time.After(delay):\n\t\tcase <-ctx.Done():\n\t\t\treturn nil, ctx.Err()\n\t\t}\n\t\tdelay *= 2",
			Confidence: 0.9, Source: "hotspot-reviewer"},
		{ID: "f4", Severity: review.SeveritySuggestion, Category: review.CategoryDesign, File: "pkg/analyzer/pipeline.go", Line: 64,
			FunctionID: "github.com/alexcollie/kaizen/pkg/analyzer.(*Pipeline).Analyze",
			Title:      "Count files that failed to analyze",
			Body:       "Errors from `AnalyzeFile` are dropped. With parallel workers this is harder to debug; consider collecting a count, or the first few errors, into `AnalysisResult` so `kaizen analyze` can print \"3 files skipped\".",
			Confidence: 0.6, Source: "concurrency-reviewer"},
		{ID: "f5", Severity: review.SeverityPraise, Category: review.CategoryDesign, File: "pkg/analyzer/pipeline.go", Line: 71,
			FunctionID: "github.com/alexcollie/kaizen/pkg/analyzer.(*Pipeline).Analyze",
			Title:      "Index-addressed results keep output order deterministic",
			Body:       "Writing into `results[index]` instead of appending from workers keeps file order stable across runs and avoids a mutex. Nice.",
			Confidence: 0.8, Source: "concurrency-reviewer"},
		{ID: "f6", Severity: review.SeveritySuggestion, Category: review.CategoryCorrectness, File: "pkg/churn/analyzer.go", Line: 125,
			FunctionID: "github.com/alexcollie/kaizen/pkg/churn.parseNumstat",
			Title:      "Author names containing `|` break header parsing",
			Body:       "`--format=%H|%an|%at` is split on `|`, so an author like `Ops | Bot` yields 4 fields and the commit is counted as a numstat line. Use a separator git never emits in names, such as `%x1f`.",
			Confidence: 0.7, Source: "hotspot-reviewer"},
		{ID: "f7", Severity: review.SeverityWarning, Category: review.CategoryCorrectness, File: "scripts/churn_report.py", Line: 43,
			FunctionID: "scripts/churn_report.py:summarize_churn",
			Title:      "ZeroDivisionError when a file has churn but zero commits",
			Body:       "`churned` keeps any file with a truthy `churn` dict, but `total_commits` can be `0` when the analysis window excludes every commit. Guard the division or filter on `total_commits > 0`.",
			Suggestion: "            \"changes_per_commit\": churn[\"total_changes\"] / max(churn[\"total_commits\"], 1),",
			Confidence: 0.8, Source: "python-reviewer"},
		{ID: "f8", Severity: review.SeveritySuggestion, Category: review.CategoryDesign, File: "scripts/churn_report.py", Line: 35,
			FunctionID: "scripts/churn_report.py:summarize_churn",
			Title:      "Avoid single-letter lambda names",
			Body:       "The project style guide asks for descriptive names. `lambda entry: entry[\"churn\"][\"total_commits\"]` reads better.",
			Confidence: 0.5, Source: "python-reviewer"},
		{ID: "f9", Severity: review.SeveritySuggestion, Category: review.CategoryTests, File: "pkg/churn/cache.go",
			Title:      "Add tests for churnCache",
			Body:       "The new LRU has eviction and recency logic but no `cache_test.go`. A table test for eviction order plus a `-race` test with parallel `Get`/`Put` would have caught the locking issue.",
			Confidence: 0.75, Source: "lead"},
		{ID: "f10", Severity: review.SeverityWarning, Category: review.CategoryDesign, File: "pkg/analyzer/pipeline.go", Line: 103,
			Title:      "analyzeChurn also swallows context cancellation",
			Body:       "Outside the diff, but related: `analyzeChurn` drops every error, including `context.Canceled` that `GetFileChurn` now returns deliberately. A cancelled run will finish with partial churn instead of stopping.",
			Confidence: 0.65, Source: "lead"},
	}
}

// resultEvent is the final authoritative result
func resultEvent(findings []review.Finding) review.Event {
	summary := strings.Join([]string{
		"**Request changes.** The parallel pipeline is a solid speed-up, but the new churn cache has a data race on the exact path the worker pool now exercises concurrently.",
		"",
		"- **Critical:** `churnCache.Get` mutates the LRU under `RLock`, a race under `Pipeline.Analyze`'s workers.",
		"- `GetFileChurn` now hides git failures as zero churn, and its retry backoff ignores cancellation.",
		"- `churn_report.py` can divide by zero.",
		"",
		"The index-addressed results in `Analyze` are a nice touch. Please add a `-race` test for the cache.",
	}, "\n")
	return review.NewEvent(review.EventResult, review.ReviewResult{
		Summary:  summary,
		Verdict:  review.VerdictRequestChanges,
		Findings: findings,
		Engine:   "demo",
		Model:    "scripted",
		CostUSD:  0.42,
	})
}

func statusEvent(agent, message string) review.Event {
	return review.NewEvent(review.EventStatus, review.StatusData{Message: message, Agent: agent})
}

func thinkingEvent(agent, text string) review.Event {
	return review.NewEvent(review.EventThinking, review.TextData{Text: text, Agent: agent})
}

func toolEvent(agent, tool, summary string) review.Event {
	return review.NewEvent(review.EventToolCall, review.ToolCallData{Tool: tool, Summary: summary, Agent: agent})
}

func findingEvent(finding review.Finding) review.Event {
	return review.NewEvent(review.EventFinding, finding)
}

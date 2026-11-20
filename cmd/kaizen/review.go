package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/alexcollie/kaizen/pkg/github"
	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/engine"
	"github.com/alexcollie/kaizen/pkg/reviewui"
	"github.com/alexcollie/kaizen/pkg/visualization"
)

var reviewCmd = &cobra.Command{
	Use:   "review <pr-number|pr-url>",
	Short: "AI review of a GitHub pull request with a risk-focused interactive UI",
	Long: `Fetch a GitHub pull request, rank its changed functions by risk using Kaizen's
metrics (complexity, fan-in, churn, size, security-sensitive code), and open a local
review UI: a sidebar of changes, the diff with expandable context, and a call graph
of the changed code. Claude reviews the PR in the background, starting with the
riskiest functions, and findings stream into the UI as they are found. Selected
findings can be posted back to the PR as a GitHub review.

Engines:
  auto  use the Claude Code CLI if 'claude' is installed, else the Anthropic API
  cli   Claude Code CLI with reviewer subagents (uses your Claude login)
  api   Anthropic API via ANTHROPIC_API_KEY

Requires git and an authenticated GitHub CLI (gh auth login), and must be run inside
a clone of the PR's repository.`,
	Example: `  kaizen review 42
  kaizen review https://github.com/owner/repo/pull/42 --engine=api
  kaizen review 42 --no-ai`,
	Args: cobra.ExactArgs(1),
	RunE: runReview,
}

type reviewFlags struct {
	engineName   string
	model        string
	effort       string
	port         int
	open         bool
	noAI         bool
	focus        int
	keepWorktree bool
	skipChurn    bool
	repoPath     string
}

var reviewOptions reviewFlags

func init() {
	flags := reviewCmd.Flags()
	flags.StringVar(&reviewOptions.engineName, "engine", "auto", "AI engine: auto, cli or api")
	flags.StringVar(&reviewOptions.model, "model", "", "model override (CLI alias like 'opus' or 'sonnet', or a full API model ID)")
	flags.StringVar(&reviewOptions.effort, "effort", "high", "reasoning effort: low, medium, high, xhigh or max")
	flags.IntVar(&reviewOptions.port, "port", 0, "port for the review UI (0 picks a free port)")
	flags.BoolVar(&reviewOptions.open, "open", true, "open the review UI in the default browser")
	flags.BoolVar(&reviewOptions.noAI, "no-ai", false, "show the Kaizen risk analysis only, without an AI review")
	flags.IntVar(&reviewOptions.focus, "focus", review.DefaultFocusCount, "number of highest-risk functions the review concentrates on")
	flags.BoolVar(&reviewOptions.keepWorktree, "keep-worktree", false, "keep the PR checkout after exiting (its path is printed)")
	flags.BoolVar(&reviewOptions.skipChurn, "skip-churn", false, "skip per-function git churn lookups (faster on large histories)")
	flags.StringVar(&reviewOptions.repoPath, "repo", ".", "path to a local clone of the PR's repository")
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	number, err := parsePRReference(args[0])
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Fail fast on a missing engine, before fetching and checking out the PR
	var selectedEngine review.Engine
	if !reviewOptions.noAI {
		selectedEngine, err = selectReviewEngine(reviewOptions)
		if err != nil {
			return err
		}
	}

	client := github.NewClient(reviewOptions.repoPath)
	reviewContext, cleanup, err := prepareReviewContext(ctx, client, number, reviewOptions)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	events := reviewui.NewEventLog()
	server, err := reviewui.New(reviewui.Options{
		Context: reviewContext,
		Events:  events,
		Poster:  client,
		Addr:    fmt.Sprintf("127.0.0.1:%d", reviewOptions.port),
	})
	if err != nil {
		return err
	}
	url, err := server.Start()
	if err != nil {
		return fmt.Errorf("starting review UI: %w", err)
	}
	defer shutdownServer(server)

	fmt.Printf("\n→ Review UI: %s\n  Press Ctrl+C to stop.\n\n", url)
	if reviewOptions.open {
		if openErr := visualization.OpenInBrowser(url); openErr != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser (%v); open the URL above manually.\n", openErr)
		}
	}

	go runReviewEngine(ctx, selectedEngine, reviewContext, events)

	<-ctx.Done()
	fmt.Println("\nShutting down…")
	return nil
}

// prPattern matches ".../pull/123" in a PR URL
var prPattern = regexp.MustCompile(`/pull/(\d+)`)

// parsePRReference accepts "123", "#123" or a GitHub PR URL
func parsePRReference(reference string) (int, error) {
	if match := prPattern.FindStringSubmatch(reference); match != nil {
		reference = match[1]
	}
	if len(reference) > 0 && reference[0] == '#' {
		reference = reference[1:]
	}
	number, err := strconv.Atoi(reference)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("invalid pull request %q: expected a number like 42 or a PR URL", reference)
	}
	return number, nil
}

// selectReviewEngine resolves --engine/--model/--effort into an engine
func selectReviewEngine(options reviewFlags) (review.Engine, error) {
	selectOptions := engine.SelectOptions{
		CLI: engine.CLIEngine{Model: options.model, Effort: options.effort},
		API: engine.APIEngine{Model: options.model},
	}
	return engine.Select(options.engineName, selectOptions)
}

// prepareReviewContext fetches and checks out the PR and runs Kaizen's analysis on it.
// The returned cleanup removes the worktree (or reports where it was kept) and is
// non-nil whenever a worktree was created, even if a later step failed.
func prepareReviewContext(ctx context.Context, client *github.Client, number int, options reviewFlags) (*review.ReviewContext, func(), error) {
	started := time.Now()
	fmt.Printf("Fetching PR #%d…\n", number)
	pr, _, err := client.FetchPR(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("✓ #%d %s (+%d/-%d)\n", pr.Number, pr.Title, pr.Additions, pr.Deletions)

	worktree, removeWorktree, err := client.Checkout(ctx, pr)
	if err != nil {
		return nil, nil, fmt.Errorf("checking out PR #%d: %w", number, err)
	}
	cleanup := worktreeCleanup(worktree, removeWorktree, options.keepWorktree)
	fmt.Printf("✓ checked out %s at %s\n", shortSHA(pr.HeadSHA), worktree)

	displayDiff, err := client.Diff(ctx, worktree, pr.MergeBase, pr.HeadSHA, 3)
	if err != nil {
		return nil, cleanup, err
	}
	mappingDiff, err := client.Diff(ctx, worktree, pr.MergeBase, pr.HeadSHA, 0)
	if err != nil {
		return nil, cleanup, err
	}

	reviewContext, err := review.BuildContext(ctx, *pr, worktree, displayDiff, mappingDiff, review.BuildOptions{
		FocusCount: options.focus,
		SkipChurn:  options.skipChurn,
	})
	if err != nil {
		return nil, cleanup, fmt.Errorf("analyzing PR #%d: %w", number, err)
	}
	printRiskSummary(reviewContext, time.Since(started))
	return reviewContext, cleanup, nil
}

// worktreeCleanup wraps the worktree removal so it runs once and honours --keep-worktree
func worktreeCleanup(worktree string, remove func() error, keep bool) func() {
	return func() {
		if keep {
			fmt.Printf("PR checkout kept at %s\n", worktree)
			return
		}
		if err := remove(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove worktree %s: %v\n", worktree, err)
		}
	}
}

// printRiskSummary prints the focus functions so the terminal is useful on its own
func printRiskSummary(reviewContext *review.ReviewContext, elapsed time.Duration) {
	focusCount := 0
	for _, function := range reviewContext.Functions {
		if function.Focus {
			focusCount++
		}
	}
	fmt.Printf("✓ risk analysis: %d files, %d changed functions, %d to focus on (%.1fs)\n",
		len(reviewContext.Files), len(reviewContext.Functions), focusCount, elapsed.Seconds())
	for _, function := range reviewContext.Functions {
		if !function.Focus {
			continue
		}
		reason := ""
		if len(function.RiskReasons) > 0 {
			reason = " · " + function.RiskReasons[0]
		}
		fmt.Printf("  ★ %5.1f  %s (%s:%d)%s\n", function.RiskScore, function.Name, function.File, function.StartLine, reason)
	}
}

// runReviewEngine runs the AI review and records its outcome in the event log.
// With no engine (--no-ai) it just marks the stream finished.
func runReviewEngine(ctx context.Context, selectedEngine review.Engine, reviewContext *review.ReviewContext, events *reviewui.EventLog) {
	defer events.Close()
	defer events.Emit(review.NewEvent(review.EventDone, nil))

	if selectedEngine == nil {
		events.Emit(review.NewEvent(review.EventStatus, review.StatusData{Message: "AI review disabled (--no-ai): showing Kaizen risk analysis only"}))
		return
	}

	events.Emit(review.NewEvent(review.EventStatus, review.StatusData{Message: "Starting review with " + selectedEngine.Name(), Agent: "lead"}))
	fmt.Printf("Reviewing with %s… (findings stream into the UI)\n", selectedEngine.Name())

	result, err := selectedEngine.Run(ctx, reviewContext, events.Emit)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		fmt.Fprintf(os.Stderr, "Review failed: %v\n", err)
		events.Emit(review.NewEvent(review.EventError, review.TextData{Text: err.Error()}))
		return
	}
	fmt.Printf("✓ review complete: %s, %d findings\n", result.Verdict, len(result.Findings))
}

func shutdownServer(server *reviewui.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// Package engine runs the AI review over a prepared review.ReviewContext.
// CLIEngine drives the Claude Code CLI with Kaizen's MCP tool server and three
// review subagents; APIEngine calls the Messages API directly with the same
// tools. Select picks one.
package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

const (
	// CLIEngineName is the Engine name of CLIEngine and the ReviewResult.Engine value it sets
	CLIEngineName = "claude-cli"

	defaultCLIModel  = "opus"
	defaultCLIEffort = "high"
	// DefaultMaxBudgetUSD caps what one CLI review may spend when MaxBudgetUSD is unset
	DefaultMaxBudgetUSD = 10.0
	stderrTailBytes     = 4096
	killWaitDelay       = 5 * time.Second
)

// leadAllowedTools are the tools the CLI may run without asking. With
// --permission-mode dontAsk anything else is denied.
var leadAllowedTools = []string{"Read", "Grep", "Glob", toolAgent, "mcp__kaizen__*"}

// leadBuiltinTools restricts the built-in tool set; everything else (Bash,
// Edit, Write, WebFetch...) is removed outright rather than merely denied
var leadBuiltinTools = []string{"Read", "Grep", "Glob", toolAgent}

// CLIEngine runs the review with the Claude Code CLI (`claude -p`). Kaizen's
// own binary serves the review tools to it over MCP (`kaizen review-mcp`).
type CLIEngine struct {
	ClaudePath string   // path to the claude binary; default: exec.LookPath("claude")
	KaizenPath string   // path to the kaizen binary for the MCP server; default: os.Executable()
	Model      string   // --model; default "opus"
	Effort     string   // --effort; default "high"
	ExtraArgs  []string // appended to the claude arguments
	// MaxBudgetUSD is passed as --max-budget-usd so a runaway (or prompt-injected)
	// review stops. 0 (the zero value) means DefaultMaxBudgetUSD; negative is an error.
	MaxBudgetUSD float64
	// WorkerModel is the model of the breadth subagents (package, sweep and
	// evasion reviewers). Empty (the default) runs them on Model like the rest.
	WorkerModel string
}

// cliStrippedEnv are removed from the claude process environment so the CLI
// always uses the user's Claude Code login, never an API key or proxy that
// happens to be set in the parent shell
var cliStrippedEnv = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL"}

// claudeEnvironment returns environ without the variables in cliStrippedEnv.
// Everything else (HOME, PATH, XDG dirs...) is kept so the CLI finds its login.
func claudeEnvironment(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !isStrippedEnv(name) {
			kept = append(kept, entry)
		}
	}
	return kept
}

func isStrippedEnv(name string) bool {
	for _, stripped := range cliStrippedEnv {
		if strings.EqualFold(name, stripped) {
			return true
		}
	}
	return false
}

// Name implements review.Engine
func (engine *CLIEngine) Name() string { return CLIEngineName }

// resolved returns a copy with defaults filled in
func (engine *CLIEngine) resolved() (CLIEngine, error) {
	config := *engine
	if config.Model == "" {
		config.Model = defaultCLIModel
	}
	config.Effort = strings.ToLower(strings.TrimSpace(config.Effort))
	if config.Effort == "" {
		config.Effort = defaultCLIEffort
	}
	if config.MaxBudgetUSD < 0 {
		return config, fmt.Errorf("max budget must not be negative (got %g); 0 means the default of $%g", config.MaxBudgetUSD, DefaultMaxBudgetUSD)
	}
	if config.MaxBudgetUSD == 0 {
		config.MaxBudgetUSD = DefaultMaxBudgetUSD
	}
	if config.ClaudePath == "" {
		path, err := exec.LookPath("claude")
		if err != nil {
			return config, fmt.Errorf("claude CLI not found on PATH: %w", err)
		}
		config.ClaudePath = path
	}
	if config.KaizenPath == "" {
		path, err := os.Executable()
		if err != nil {
			return config, fmt.Errorf("locate kaizen binary for the MCP server: %w", err)
		}
		config.KaizenPath = path
	}
	return config, nil
}

// Run implements review.Engine
func (engine *CLIEngine) Run(ctx context.Context, reviewContext *review.ReviewContext, emit review.EmitFunc) (*review.ReviewResult, error) {
	if reviewContext == nil || reviewContext.WorktreePath == "" {
		return nil, errors.New("review context has no worktree path")
	}
	config, err := engine.resolved()
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "kaizen-review-")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	contextFile, err := writeContextFile(tempDir, reviewContext)
	if err != nil {
		return nil, err
	}
	args, err := config.buildArgs(contextFile, reviewContext.PR)
	if err != nil {
		return nil, err
	}
	parser := newStreamParser(emit, reviewContext.WorktreePath)
	parser.toolbox = tools.New(reviewContext)
	emitStatus(emit, fmt.Sprintf("Starting Claude Code review (model %s, effort %s)", config.Model, config.Effort), sourceLead)
	if err := config.runClaude(ctx, args, reviewContext.WorktreePath, parser); err != nil {
		var exited *claudeExit
		if !errors.As(err, &exited) || !parser.hasStructuredOutput() {
			return nil, err
		}
		// The CLI exited non-zero (a budget stop, say) after the lead had already
		// produced its structured review: keep the review and surface the exit
		emitStatus(emit, "Warning: claude exited with an error after producing the review, which is kept: "+err.Error(), sourceLead)
	}
	return config.finish(parser, reviewContext, emit)
}

// writeContextFile saves the context JSON for the MCP server, readable only by the user
func writeContextFile(dir string, reviewContext *review.ReviewContext) (string, error) {
	encoded, err := json.Marshal(reviewContext)
	if err != nil {
		return "", fmt.Errorf("encode review context: %w", err)
	}
	path := filepath.Join(dir, "context.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return "", fmt.Errorf("write review context: %w", err)
	}
	return path, nil
}

// buildArgs assembles the claude command line. The prompt comes first because
// several flags are variadic and would otherwise swallow it.
func (engine *CLIEngine) buildArgs(contextFile string, pr review.PullRequest) ([]string, error) {
	mcpConfig, err := mcpConfigJSON(engine.KaizenPath, contextFile)
	if err != nil {
		return nil, err
	}
	agents, err := json.Marshal(subagentDefinitions(engine.Model, engine.WorkerModel))
	if err != nil {
		return nil, fmt.Errorf("encode agents: %w", err)
	}
	// The sandbox flags below are pinned by TestCLIEngineSandboxFlags. The CLI runs
	// inside the PR's own checkout, so:
	//   --restricted ignores user, project and local settings files (claude --help),
	//     which is what stops a PR's .claude/settings.json hooks and permissions from
	//     loading; it also drops Bash and WebFetch and confines file tools to the worktree
	//   --strict-mcp-config ignores the PR's .mcp.json; only Kaizen's server runs
	//   --permission-mode dontAsk denies every tool not in --allowedTools
	//   --tools removes Bash, Edit, Write, WebFetch etc. outright
	//   --no-session-persistence leaves no transcript of PR content on disk
	args := []string{
		"-p", leadPrompt(pr),
		"--output-format", "stream-json",
		"--verbose",
		"--model", engine.Model,
		"--effort", engine.Effort,
		"--restricted",
		"--permission-mode", "dontAsk",
		"--tools", strings.Join(leadBuiltinTools, ","),
		"--allowedTools", strings.Join(leadAllowedTools, " "),
		"--disable-slash-commands",
		"--strict-mcp-config",
		"--mcp-config", mcpConfig,
		"--agents", string(agents),
		"--json-schema", string(ReviewOutputSchema()),
		"--no-session-persistence",
		"--max-budget-usd", strconv.FormatFloat(engine.MaxBudgetUSD, 'f', -1, 64),
	}
	return append(args, engine.ExtraArgs...), nil
}

// mcpConfigJSON points Claude Code at `kaizen review-mcp --context <file>`
func mcpConfigJSON(kaizenPath, contextFile string) (string, error) {
	config := map[string]any{
		"mcpServers": map[string]any{
			"kaizen": map[string]any{
				"type":    "stdio",
				"command": kaizenPath,
				"args":    []string{"review-mcp", "--context", contextFile},
			},
		},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode MCP config: %w", err)
	}
	return string(encoded), nil
}

// runClaude starts claude, feeds stdout to the parser until the process
// exits, and maps failures to errors that carry the stderr tail
func (engine *CLIEngine) runClaude(ctx context.Context, args []string, workDir string, parser *streamParser) error {
	command := exec.CommandContext(ctx, engine.ClaudePath, args...)
	command.Dir = workDir
	command.Env = claudeEnvironment(os.Environ())
	command.Stdin = nil
	command.WaitDelay = killWaitDelay
	configureProcessGroup(command)
	stderr := &tailBuffer{limit: stderrTailBytes}
	command.Stderr = stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("claude stdout: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}
	readErr := readLines(stdout, parser.handleLine)
	waitErr := command.Wait()
	if ctx.Err() != nil {
		return fmt.Errorf("review cancelled: %w", ctx.Err())
	}
	if waitErr != nil {
		return claudeExitError(waitErr, stderr.String(), parser.lastResultSnapshot())
	}
	if readErr != nil {
		return fmt.Errorf("read claude output: %w", readErr)
	}
	return nil
}

// readLines calls handle for every newline-terminated line, with no line length limit
func readLines(reader io.Reader, handle func([]byte)) error {
	buffered := bufio.NewReaderSize(reader, 1<<20)
	for {
		line, err := buffered.ReadBytes('\n')
		if len(line) > 0 {
			handle(line)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// finish turns the parsed stream into the final result
func (engine *CLIEngine) finish(parser *streamParser, reviewContext *review.ReviewContext, emit review.EmitFunc) (*review.ReviewResult, error) {
	result, structured := parser.outcome()
	if result == nil && structured == nil {
		return nil, errors.New("claude produced no result")
	}
	if structured == nil {
		return nil, missingOutputError(result)
	}
	output, err := parseReviewOutput(structured)
	if err != nil {
		return nil, err
	}
	reviewResult := assembleResult(output, parser.live, parser.toolbox, emit)
	reviewResult.Engine = CLIEngineName
	reviewResult.Model = engine.Model
	if result != nil {
		reviewResult.CostUSD = result.CostUSD
	}
	emitEvent(emit, review.EventResult, *reviewResult)
	return reviewResult, nil
}

// loginHelp is the fix for an authentication failure: the CLI engine only
// ever uses the Claude Code login
const loginHelp = `Claude Code is not logged in (or its login expired). Run "claude" in a terminal and log in (/login), then retry. Kaizen's CLI engine always uses your Claude Code login and ignores ANTHROPIC_API_KEY.`

// claudeExit is the error for a non-zero claude exit, so Run can tell it
// apart from a failure to start or a cancellation
type claudeExit struct{ err error }

func (exit *claudeExit) Error() string { return exit.err.Error() }
func (exit *claudeExit) Unwrap() error { return exit.err }

// claudeExitError explains a non-zero exit, with the login fix for auth
// failures. It reads only CLI signals: stderr, and the last result's subtype,
// terminal reason and (for an error result, which the CLI writes) its text,
// never the model's own prose in a successful result.
func claudeExitError(waitErr error, stderrTail string, last *streamResult) error {
	if isAuthFailure(stderrTail) || (last != nil && last.IsError && isAuthFailure(last.Text)) {
		return &claudeExit{fmt.Errorf("%s\nclaude exited with an error: %w%s", loginHelp, waitErr, formatTail(stderrTail))}
	}
	if isBudgetStop(last) || isBudgetStderr(stderrTail) {
		return &claudeExit{fmt.Errorf("the review stopped at its spending cap (--max-budget-usd); raise the budget and rerun: %w%s", waitErr, formatTail(stderrTail))}
	}
	return &claudeExit{fmt.Errorf("claude exited with an error: %w%s", waitErr, formatTail(stderrTail))}
}

// authFailureMarkers are lower-case phrases the CLI and the API print on an
// authentication failure. A bare "401" is deliberately absent: it matches line
// numbers and counts in ordinary output.
var authFailureMarkers = []string{
	"not logged in",
	"please run /login",
	"invalid api key",
	"invalid x-api-key",
	"authentication_error",
	"authentication failed",
	"oauth token has expired",
	"401 unauthorized",
	"status 401",
	"status code 401",
	"status: 401",
	"http 401",
	"error 401",
	"(401)",
}

// isAuthFailure spots the CLI's authentication errors in its output
func isAuthFailure(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range authFailureMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// budgetSubtype is the result subtype the CLI sets when --max-budget-usd stops it
const budgetSubtype = "error_max_budget_usd"

// isBudgetStop reports a result the CLI ended because of --max-budget-usd,
// from its subtype or terminal reason only
func isBudgetStop(result *streamResult) bool {
	if result == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(result.Subtype), budgetSubtype) {
		return true
	}
	reason := strings.ToLower(strings.TrimSpace(result.TerminalReason))
	return reason == "max_budget_usd" || reason == "max_budget" || reason == "budget_exceeded"
}

// budgetStderrMarkers are lower-case phrases the CLI prints to stderr when it
// stops at --max-budget-usd
var budgetStderrMarkers = []string{"--max-budget-usd", "max_budget_usd", "maximum budget", "budget exceeded", "exceeded budget"}

func isBudgetStderr(stderrTail string) bool {
	lower := strings.ToLower(stderrTail)
	for _, marker := range budgetStderrMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func missingOutputError(result *streamResult) error {
	if result == nil {
		return errors.New("claude finished without a review result")
	}
	detail := strings.TrimSpace(result.Text)
	if len(detail) > 500 {
		detail = detail[:500] + "…"
	}
	if isBudgetStop(result) {
		return fmt.Errorf("the review stopped at its spending cap (--max-budget-usd, spent $%.2f) before producing a result; raise the budget and rerun", result.CostUSD)
	}
	if result.IsError && isAuthFailure(detail) {
		return fmt.Errorf("%s (%s)", loginHelp, detail)
	}
	if result.IsError {
		return fmt.Errorf("claude reported an error (%s, %s): %s", result.Subtype, result.TerminalReason, detail)
	}
	return fmt.Errorf("claude finished without structured review output (%s after %d turns): %s", result.Subtype, result.NumTurns, detail)
}

func formatTail(tail string) string {
	tail = strings.TrimSpace(tail)
	if tail == "" {
		return ""
	}
	return "\nclaude stderr:\n" + tail
}

// tailBuffer keeps the last limit bytes written to it
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (buffer *tailBuffer) Write(chunk []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.data = append(buffer.data, chunk...)
	if overflow := len(buffer.data) - buffer.limit; overflow > 0 {
		buffer.data = append([]byte(nil), buffer.data[overflow:]...)
	}
	return len(chunk), nil
}

func (buffer *tailBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return strings.ToValidUTF8(string(buffer.data), "")
}

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
	stderrTailBytes  = 4096
	killWaitDelay    = 5 * time.Second
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
}

// Name implements review.Engine
func (engine *CLIEngine) Name() string { return CLIEngineName }

// resolved returns a copy with defaults filled in
func (engine *CLIEngine) resolved() (CLIEngine, error) {
	config := *engine
	if config.Model == "" {
		config.Model = defaultCLIModel
	}
	if config.Effort == "" {
		config.Effort = defaultCLIEffort
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
	emitStatus(emit, fmt.Sprintf("Starting Claude Code review (model %s, effort %s)", config.Model, config.Effort), sourceLead)
	if err := config.runClaude(ctx, args, reviewContext.WorktreePath, parser); err != nil {
		return nil, err
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
	agents, err := json.Marshal(subagentDefinitions(engine.Model))
	if err != nil {
		return nil, fmt.Errorf("encode agents: %w", err)
	}
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
		return fmt.Errorf("claude exited with an error: %w%s", waitErr, formatTail(stderr.String()))
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
	reviewResult := assembleResult(output, parser.live, tools.New(reviewContext), emit)
	reviewResult.Engine = CLIEngineName
	reviewResult.Model = engine.Model
	if result != nil {
		reviewResult.CostUSD = result.CostUSD
	}
	emitEvent(emit, review.EventResult, *reviewResult)
	return reviewResult, nil
}

func missingOutputError(result *streamResult) error {
	if result == nil {
		return errors.New("claude finished without a review result")
	}
	detail := strings.TrimSpace(result.Text)
	if len(detail) > 500 {
		detail = detail[:500] + "…"
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

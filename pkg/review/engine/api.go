package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	anthropic "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/anthropics/anthropic-sdk-go/v2/toolrunner"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

const (
	// APIEngineName is the Engine name of APIEngine and the ReviewResult.Engine value it sets
	APIEngineName = "anthropic-api"

	defaultAPIModel         = "claude-opus-5-5"
	defaultAPIMaxIterations = 30
	defaultAPIMaxTokens     = 32000
	nudgeMaxIterations      = 3
	thinkingFlushBytes      = 400

	toolSubmitReview        = "submit_review"
	descriptionSubmitReview = `Delivers the finished review. Call it exactly once, at the very end, after every finding has been recorded with report_finding. Include the markdown summary, the verdict, and every finding that still stands, each with the id report_finding returned for it. The list is authoritative: a recorded finding you leave out is treated as withdrawn and removed from the review, so leave out exactly the ones you have concluded are wrong.`
)

// APIEngine runs the review against the Anthropic Messages API with the beta
// tool runner, using the same tools the CLI engine serves over MCP
type APIEngine struct {
	Model         string                 // default "claude-opus-5-5"
	Effort        string                 // low, medium, high, xhigh or max; default "high"
	MaxIterations int                    // tool-runner turns; default 30
	MaxTokens     int64                  // per response; default 32000
	ClientOptions []option.RequestOption // extra client options, e.g. option.WithAPIKey
}

// apiEfforts maps the --effort values onto the API's output_config.effort
var apiEfforts = map[string]anthropic.BetaOutputConfigEffort{
	"low":    anthropic.BetaOutputConfigEffortLow,
	"medium": anthropic.BetaOutputConfigEffortMedium,
	"high":   anthropic.BetaOutputConfigEffortHigh,
	"xhigh":  anthropic.BetaOutputConfigEffortXhigh,
	"max":    anthropic.BetaOutputConfigEffortMax,
}

// apiEffort resolves an effort name, case-insensitively; empty means high
func apiEffort(name string) (anthropic.BetaOutputConfigEffort, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return anthropic.BetaOutputConfigEffortHigh, nil
	}
	effort, ok := apiEfforts[normalized]
	if !ok {
		return "", fmt.Errorf("unknown effort %q: use low, medium, high, xhigh or max", name)
	}
	return effort, nil
}

// Name implements review.Engine
func (engine *APIEngine) Name() string { return APIEngineName }

func (engine *APIEngine) resolved() APIEngine {
	config := *engine
	if config.Model == "" {
		config.Model = defaultAPIModel
	}
	if effort, err := apiEffort(config.Effort); err == nil {
		config.Effort = string(effort)
	}
	if config.MaxIterations <= 0 {
		config.MaxIterations = defaultAPIMaxIterations
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaultAPIMaxTokens
	}
	return config
}

// apiRun holds the state of one review run
type apiRun struct {
	toolbox   *tools.Toolbox
	emit      review.EmitFunc
	live      *findingSet
	mu        sync.Mutex
	submitted *ReviewOutput
}

// Run implements review.Engine
func (engine *APIEngine) Run(ctx context.Context, reviewContext *review.ReviewContext, emit review.EmitFunc) (*review.ReviewResult, error) {
	if reviewContext == nil || reviewContext.WorktreePath == "" {
		return nil, errors.New("review context has no worktree path")
	}
	if _, err := apiEffort(engine.Effort); err != nil {
		return nil, err
	}
	config := engine.resolved()
	run := newAPIRun(reviewContext, emit)
	betaTools, err := run.buildTools()
	if err != nil {
		return nil, err
	}
	client := anthropic.NewClient(config.ClientOptions...)
	params := config.runnerParams(reviewContext.PR, config.MaxIterations)
	emitStatus(emit, fmt.Sprintf("Starting API review (model %s, effort %s)", config.Model, config.Effort), sourceLead)

	messages, err := run.drive(ctx, client.Beta.Messages.NewToolRunnerStreaming(betaTools, params))
	if err != nil {
		return nil, err
	}
	if run.submission() == nil {
		emitStatus(emit, "The model ended without submitting; asking it to submit the review", sourceLead)
		params = config.runnerParams(reviewContext.PR, nudgeMaxIterations)
		params.Messages = append(messages, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(apiSubmitNudge)))
		if _, err := run.drive(ctx, client.Beta.Messages.NewToolRunnerStreaming(betaTools, params)); err != nil {
			return nil, err
		}
	}
	return run.finish(config.Model)
}

func newAPIRun(reviewContext *review.ReviewContext, emit review.EmitFunc) *apiRun {
	run := &apiRun{toolbox: tools.New(reviewContext), emit: emit, live: newFindingSet()}
	run.toolbox.OnFinding = func(finding review.Finding) {
		finding.Source = sourceLead
		if run.live.add(finding) {
			emitEvent(emit, review.EventFinding, finding)
		}
	}
	return run
}

// runnerParams builds the request: adaptive thinking with summarized display,
// the configured effort, cached system prompt, auto tool choice, strict and eagerly
// streamed tools, and the server-side refusal fallback
func (engine *APIEngine) runnerParams(pr review.PullRequest, maxIterations int) anthropic.BetaToolRunnerParams {
	adaptive := anthropic.BetaThinkingConfigAdaptiveParam{Display: anthropic.BetaThinkingConfigAdaptiveDisplaySummarized}
	return anthropic.BetaToolRunnerParams{
		BetaMessageNewParams: anthropic.BetaMessageNewParams{
			Model:     engine.Model,
			MaxTokens: engine.MaxTokens,
			System: []anthropic.BetaTextBlockParam{{
				Text:         apiSystemPrompt,
				CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
			}},
			Messages: []anthropic.BetaMessageParam{
				anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(apiInitialMessage(pr))),
			},
			Tools:        toolDefinitions(),
			ToolChoice:   anthropic.BetaToolChoiceUnionParam{OfAuto: &anthropic.BetaToolChoiceAutoParam{}},
			Thinking:     anthropic.BetaThinkingConfigParamUnion{OfAdaptive: &adaptive},
			OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffort(engine.Effort)},
			Fallbacks:    anthropic.BetaFallbacksParamOfDefault(),
			Betas:        []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		},
		MaxIterations: maxIterations,
		DisableThread: true,
	}
}

// apiToolSpec is one tool offered to the model
type apiToolSpec struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// apiToolSpecs lists the toolbox tools plus submit_review
func apiToolSpecs() []apiToolSpec {
	var specs []apiToolSpec
	for _, definition := range tools.Definitions() {
		specs = append(specs, apiToolSpec{definition.Name, definition.Description, definition.InputSchema})
	}
	return append(specs, apiToolSpec{toolSubmitReview, descriptionSubmitReview, ReviewOutputSchema()})
}

// toolDefinitions are the wire definitions. The runner sends a definition from
// Params.Tools in place of its own when the names match, which is how strict
// and eager_input_streaming reach the API.
func toolDefinitions() []anthropic.BetaToolUnionParam {
	var definitions []anthropic.BetaToolUnionParam
	for _, spec := range apiToolSpecs() {
		var schema anthropic.BetaToolInputSchemaParam
		if err := schema.UnmarshalJSON(spec.Schema); err != nil {
			panic(fmt.Sprintf("engine: tool schema %s: %v", spec.Name, err))
		}
		definitions = append(definitions, anthropic.BetaToolUnionParam{OfTool: &anthropic.BetaToolParam{
			Name:                spec.Name,
			Description:         anthropic.String(spec.Description),
			InputSchema:         schema,
			Strict:              anthropic.Bool(true),
			EagerInputStreaming: anthropic.Bool(true),
		}})
	}
	return definitions
}

// buildTools wraps every tool for the runner. Handlers take the raw JSON input
// so they can validate it against the schema before running: with eager input
// streaming the API does not validate or buffer it.
func (run *apiRun) buildTools() ([]anthropic.BetaTool, error) {
	var betaTools []anthropic.BetaTool
	for _, spec := range apiToolSpecs() {
		betaTool, err := toolrunner.NewBetaToolFromBytes(spec.Name, spec.Description, spec.Schema, run.handlerFor(spec))
		if err != nil {
			return nil, fmt.Errorf("wrap tool %s: %w", spec.Name, err)
		}
		betaTools = append(betaTools, betaTool)
	}
	return betaTools, nil
}

func (run *apiRun) handlerFor(spec apiToolSpec) func(context.Context, json.RawMessage) (anthropic.BetaToolResultBlockParamContentUnion, error) {
	return func(_ context.Context, input json.RawMessage) (anthropic.BetaToolResultBlockParamContentUnion, error) {
		target, targetKind := toolTarget(spec.Name, input, "", run.toolbox)
		emitEvent(run.emit, review.EventToolCall, review.ToolCallData{
			Tool:       spec.Name,
			Summary:    summarizeToolCall(spec.Name, input, ""),
			Agent:      sourceLead,
			Target:     target,
			TargetKind: targetKind,
		})
		output, err := run.callTool(spec, input)
		if err != nil {
			return anthropic.BetaToolResultBlockParamContentUnion{}, err
		}
		return anthropic.BetaToolResultBlockParamContentUnion{OfText: &anthropic.BetaTextBlockParam{Text: output}}, nil
	}
}

func (run *apiRun) callTool(spec apiToolSpec, input json.RawMessage) (string, error) {
	if spec.Name != toolSubmitReview {
		return run.toolbox.Call(spec.Name, input)
	}
	if err := tools.ValidateInput(spec.Schema, input); err != nil {
		return "", err
	}
	output, err := parseReviewOutput(input)
	if err != nil {
		return "", err
	}
	run.mu.Lock()
	run.submitted = output
	run.mu.Unlock()
	return "Review submitted. You are done; reply with a one-line acknowledgement.", nil
}

func (run *apiRun) submission() *ReviewOutput {
	run.mu.Lock()
	defer run.mu.Unlock()
	return run.submitted
}

// drive consumes the streaming runner, emitting thinking and text as it
// arrives, and returns the conversation so far
func (run *apiRun) drive(ctx context.Context, runner *anthropic.BetaToolRunnerStreaming) ([]anthropic.BetaMessageParam, error) {
	for events, err := range runner.AllStreaming(ctx) {
		if err != nil {
			return nil, friendlyAPIError(err)
		}
		if err := run.consumeTurn(events); err != nil {
			return nil, err
		}
	}
	if err := runner.Err(); err != nil {
		return nil, friendlyAPIError(err)
	}
	if err := run.checkStop(runner.LastMessage()); err != nil {
		return nil, err
	}
	return runner.Messages(), nil
}

// consumeTurn streams one model turn, buffering deltas into readable chunks
func (run *apiRun) consumeTurn(events func(func(anthropic.BetaRawMessageStreamEventUnion, error) bool)) error {
	stream := &deltaBuffer{emit: run.emit}
	for event, err := range events {
		if err != nil {
			return friendlyAPIError(err)
		}
		switch variant := event.AsAny().(type) {
		case anthropic.BetaRawContentBlockDeltaEvent:
			stream.addDelta(variant.Delta)
		case anthropic.BetaRawContentBlockStopEvent:
			stream.flush()
		}
	}
	stream.flush()
	return nil
}

// checkStop reports refusals and truncation, which end the runner early
func (run *apiRun) checkStop(message *anthropic.BetaMessage) error {
	if message == nil || run.submission() != nil {
		return nil
	}
	switch message.StopReason {
	case anthropic.BetaStopReasonRefusal:
		details := message.StopDetails
		return fmt.Errorf("the model declined to continue the review (category %q): %s", details.Category, details.Explanation)
	case anthropic.BetaStopReasonMaxTokens:
		return errors.New("the model ran out of output tokens mid-turn; raise MaxTokens and retry")
	case anthropic.BetaStopReasonModelContextWindowExceeded:
		return errors.New("the review exceeded the model's context window; try a smaller PR")
	}
	return nil
}

func (run *apiRun) finish(model string) (*review.ReviewResult, error) {
	output := run.submission()
	if output == nil {
		return nil, errors.New("the model finished without calling submit_review")
	}
	result := assembleResult(output, run.live, run.toolbox, run.emit)
	result.Engine = APIEngineName
	result.Model = model
	emitEvent(run.emit, review.EventResult, *result)
	return result, nil
}

// deltaBuffer joins text and thinking deltas into chunks before emitting them
type deltaBuffer struct {
	emit    review.EmitFunc
	pending strings.Builder
}

func (buffer *deltaBuffer) addDelta(delta anthropic.BetaRawContentBlockDeltaUnion) {
	switch variant := delta.AsAny().(type) {
	case anthropic.BetaTextDelta:
		buffer.pending.WriteString(variant.Text)
	case anthropic.BetaThinkingDelta:
		buffer.pending.WriteString(variant.Thinking)
	default:
		return
	}
	if buffer.pending.Len() >= thinkingFlushBytes {
		buffer.flush()
	}
}

func (buffer *deltaBuffer) flush() {
	text := strings.TrimSpace(buffer.pending.String())
	buffer.pending.Reset()
	if text != "" {
		emitEvent(buffer.emit, review.EventThinking, review.TextData{Text: text, Agent: sourceLead})
	}
}

// friendlyAPIError explains the common API failures
func friendlyAPIError(err error) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("anthropic API: %w", err)
	}
	switch {
	case apiErr.StatusCode == 401:
		return fmt.Errorf("anthropic API rejected the credentials (401): set ANTHROPIC_API_KEY to a valid key: %w", err)
	case apiErr.StatusCode == 403:
		return fmt.Errorf("anthropic API key lacks permission for this model or feature (403): %w", err)
	case apiErr.StatusCode == 429:
		return fmt.Errorf("anthropic API rate limit or quota exceeded (429); wait and retry: %w", err)
	case apiErr.StatusCode >= 500:
		return fmt.Errorf("anthropic API is unavailable (%d); retry later: %w", apiErr.StatusCode, err)
	default:
		return fmt.Errorf("anthropic API error (%d): %w", apiErr.StatusCode, err)
	}
}

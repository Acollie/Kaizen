package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

const (
	mcpToolPrefix        = "mcp__kaizen__"
	toolAgent            = "Agent"
	toolStructuredOutput = "StructuredOutput"
	maxSummaryLength     = 120
)

// streamLine is one line of `claude -p --output-format stream-json --verbose`.
// Only the fields the engine uses are decoded.
type streamLine struct {
	Type            string         `json:"type"`
	Subtype         string         `json:"subtype"`
	ParentToolUseID *string        `json:"parent_tool_use_id"`
	SubagentType    string         `json:"subagent_type"`
	Message         *streamMessage `json:"message"`

	// system events
	Model        string `json:"model"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	ToolUseID    string `json:"tool_use_id"`
	LastToolName string `json:"last_tool_name"`
	Summary      string `json:"summary"`

	// result events
	IsError           bool               `json:"is_error"`
	Result            string             `json:"result"`
	StructuredOutput  json.RawMessage    `json:"structured_output"`
	TotalCostUSD      float64            `json:"total_cost_usd"`
	NumTurns          int                `json:"num_turns"`
	TerminalReason    string             `json:"terminal_reason"`
	PermissionDenials []permissionDenial `json:"permission_denials"`

	// rate_limit_event
	RateLimitInfo *rateLimitInfo `json:"rate_limit_info"`
}

type streamMessage struct {
	Content json.RawMessage `json:"content"` // a string or an array of blocks
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

type permissionDenial struct {
	ToolName  string          `json:"tool_name"`
	ToolUseID string          `json:"tool_use_id"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type rateLimitInfo struct {
	Status        string `json:"status"`
	RateLimitType string `json:"rateLimitType"`
}

// streamResult is the outcome of one `result` event
type streamResult struct {
	Subtype          string
	IsError          bool
	Text             string
	StructuredOutput json.RawMessage
	CostUSD          float64
	NumTurns         int
	TerminalReason   string
}

type pendingFinding struct {
	finding review.Finding
	agent   string
}

// streamParser turns stream-json lines into review events. It tracks which
// agent each message belongs to, turns report_finding calls into live
// findings once the tool accepts them, and keeps the last result that carried
// structured output (the CLI can emit a result before background subagents
// finish, then continue and emit another).
type streamParser struct {
	emit         review.EmitFunc
	worktreePath string
	live         *findingSet

	mu                 sync.Mutex
	agentsByToolUse    map[string]string
	pendingFindings    map[string]pendingFinding
	reportedDenials    map[string]bool
	sessionStarted     bool
	lastResult         *streamResult
	structuredResult   *streamResult
	structuredToolCall json.RawMessage
}

func newStreamParser(emit review.EmitFunc, worktreePath string) *streamParser {
	return &streamParser{
		emit:            emit,
		worktreePath:    worktreePath,
		live:            newFindingSet(),
		agentsByToolUse: map[string]string{},
		pendingFindings: map[string]pendingFinding{},
		reportedDenials: map[string]bool{},
	}
}

// handleLine processes one line of output. Lines that are not JSON (the CLI
// should not print any, but a wrapper script might) are ignored.
func (parser *streamParser) handleLine(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return
	}
	var event streamLine
	if err := json.Unmarshal(line, &event); err != nil {
		return
	}
	parser.mu.Lock()
	defer parser.mu.Unlock()
	switch event.Type {
	case "system":
		parser.handleSystem(&event)
	case "assistant":
		parser.handleAssistant(&event)
	case "user":
		parser.handleUser(&event)
	case "result":
		parser.handleResult(&event)
	case "rate_limit_event":
		parser.handleRateLimit(&event)
	}
}

// agentFor names the agent that produced a message: "lead" or the subagent type
func (parser *streamParser) agentFor(event *streamLine) string {
	if event.ParentToolUseID == nil || *event.ParentToolUseID == "" {
		return sourceLead
	}
	if event.SubagentType != "" {
		return event.SubagentType
	}
	if agent, ok := parser.agentsByToolUse[*event.ParentToolUseID]; ok {
		return agent
	}
	return "subagent"
}

func (parser *streamParser) handleSystem(event *streamLine) {
	switch event.Subtype {
	case "init":
		if !parser.sessionStarted {
			parser.sessionStarted = true
			emitStatus(parser.emit, fmt.Sprintf("Claude Code session started (model %s)", event.Model), sourceLead)
		}
	case "task_started":
		agent := parser.subagentName(event)
		emitStatus(parser.emit, strings.TrimSpace(agent+" started: "+event.Description), agent)
	case "task_notification":
		agent := parser.subagentName(event)
		status := event.Status
		if status == "" {
			status = "finished"
		}
		emitStatus(parser.emit, fmt.Sprintf("%s %s", agent, status), agent)
	}
}

func (parser *streamParser) subagentName(event *streamLine) string {
	if event.SubagentType != "" {
		return event.SubagentType
	}
	if agent, ok := parser.agentsByToolUse[event.ToolUseID]; ok {
		return agent
	}
	return "subagent"
}

func (parser *streamParser) handleAssistant(event *streamLine) {
	agent := parser.agentFor(event)
	for _, block := range decodeBlocks(event.Message) {
		switch block.Type {
		case "text":
			parser.emitThinking(block.Text, agent)
		case "thinking":
			parser.emitThinking(block.Thinking, agent)
		case "tool_use":
			parser.handleToolUse(block, agent)
		}
	}
}

func (parser *streamParser) emitThinking(text, agent string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	emitEvent(parser.emit, review.EventThinking, review.TextData{Text: text, Agent: agent})
}

func (parser *streamParser) handleToolUse(block contentBlock, agent string) {
	switch block.Name {
	case toolStructuredOutput:
		parser.structuredToolCall = append(json.RawMessage(nil), block.Input...)
		return
	case toolAgent:
		parser.agentsByToolUse[block.ID] = subagentTypeOf(block.Input)
	case mcpToolPrefix + tools.ToolReportFinding:
		parser.trackFinding(block, agent)
	}
	emitEvent(parser.emit, review.EventToolCall, review.ToolCallData{
		Tool:    strings.TrimPrefix(block.Name, mcpToolPrefix),
		Summary: summarizeToolCall(block.Name, block.Input, parser.worktreePath),
		Agent:   agent,
	})
}

// trackFinding remembers a report_finding call until its tool_result shows whether the tool accepted it
func (parser *streamParser) trackFinding(block contentBlock, agent string) {
	var input tools.FindingInput
	if err := json.Unmarshal(block.Input, &input); err != nil {
		return
	}
	parser.pendingFindings[block.ID] = pendingFinding{finding: input.ToFinding(), agent: agent}
}

func (parser *streamParser) handleUser(event *streamLine) {
	for _, block := range decodeBlocks(event.Message) {
		if block.Type != "tool_result" {
			continue
		}
		pending, ok := parser.pendingFindings[block.ToolUseID]
		if !ok {
			continue
		}
		delete(parser.pendingFindings, block.ToolUseID)
		parser.acceptFinding(pending, block)
	}
}

// acceptFinding emits a live finding when report_finding returned "recorded <id>"
func (parser *streamParser) acceptFinding(pending pendingFinding, block contentBlock) {
	text := strings.TrimSpace(toolResultText(block.Content))
	if block.IsError || !strings.HasPrefix(text, "recorded ") {
		return
	}
	finding := pending.finding
	finding.ID = strings.TrimSpace(strings.TrimPrefix(text, "recorded "))
	finding.Source = pending.agent
	if finding.EndLine == finding.Line {
		finding.EndLine = 0
	}
	if parser.live.add(finding) {
		emitEvent(parser.emit, review.EventFinding, finding)
	}
}

func (parser *streamParser) handleResult(event *streamLine) {
	result := &streamResult{
		Subtype:        event.Subtype,
		IsError:        event.IsError,
		Text:           event.Result,
		CostUSD:        event.TotalCostUSD,
		NumTurns:       event.NumTurns,
		TerminalReason: event.TerminalReason,
	}
	if hasJSONValue(event.StructuredOutput) {
		result.StructuredOutput = append(json.RawMessage(nil), event.StructuredOutput...)
		parser.structuredResult = result
	}
	parser.lastResult = result
	parser.reportDenials(event.PermissionDenials)
}

func (parser *streamParser) reportDenials(denials []permissionDenial) {
	for _, denial := range denials {
		key := denial.ToolUseID
		if key == "" {
			key = denial.ToolName + string(denial.ToolInput)
		}
		if parser.reportedDenials[key] {
			continue
		}
		parser.reportedDenials[key] = true
		emitStatus(parser.emit, "Permission denied for tool call: "+summarizeToolCall(denial.ToolName, denial.ToolInput, parser.worktreePath), sourceLead)
	}
}

func (parser *streamParser) handleRateLimit(event *streamLine) {
	if event.RateLimitInfo == nil || event.RateLimitInfo.Status == "" || event.RateLimitInfo.Status == "allowed" {
		return
	}
	emitStatus(parser.emit, fmt.Sprintf("Claude rate limit: %s (%s)", event.RateLimitInfo.Status, event.RateLimitInfo.RateLimitType), sourceLead)
}

// outcome returns the result to use and the structured output, if any. A
// result carrying structured output wins over a later one without it.
func (parser *streamParser) outcome() (*streamResult, json.RawMessage) {
	parser.mu.Lock()
	defer parser.mu.Unlock()
	if parser.structuredResult != nil {
		return parser.structuredResult, parser.structuredResult.StructuredOutput
	}
	if hasJSONValue(parser.structuredToolCall) {
		return parser.lastResult, parser.structuredToolCall
	}
	return parser.lastResult, nil
}

func hasJSONValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && string(trimmed) != "null"
}

// decodeBlocks returns a message's content blocks; string content becomes one text block
func decodeBlocks(message *streamMessage) []contentBlock {
	if message == nil || len(message.Content) == 0 {
		return nil
	}
	var blocks []contentBlock
	if err := json.Unmarshal(message.Content, &blocks); err == nil {
		return blocks
	}
	var text string
	if err := json.Unmarshal(message.Content, &text); err == nil {
		return []contentBlock{{Type: "text", Text: text}}
	}
	return nil
}

// toolResultText flattens tool_result content (a string or a list of text blocks)
func toolResultText(content json.RawMessage) string {
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text
	}
	var blocks []contentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func subagentTypeOf(input json.RawMessage) string {
	var agentInput struct {
		SubagentType string `json:"subagent_type"`
	}
	if err := json.Unmarshal(input, &agentInput); err != nil || agentInput.SubagentType == "" {
		return "subagent"
	}
	return agentInput.SubagentType
}

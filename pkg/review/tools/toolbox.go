// Package tools implements the read-only review tools that both AI engines
// expose to the model: risk ranking, function source, callers and callees,
// file diffs, bounded file reads, and finding reporting. All PR content is
// untrusted, so every path is confined to the PR worktree and every output is
// size-capped.
package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/alexcollie/kaizen/pkg/review"
)

// Tool names, as the model sees them
const (
	ToolGetRiskRanking = "get_risk_ranking"
	ToolGetFunction    = "get_function"
	ToolGetCallers     = "get_callers"
	ToolGetCallees     = "get_callees"
	ToolGetFileDiff    = "get_file_diff"
	ToolReadLines      = "read_lines"
	ToolReportFinding  = "report_finding"
)

// maxOutputBytes caps every tool result sent to the model
const maxOutputBytes = 48 * 1024

// Toolbox serves the review tools over one review context. It is safe for
// concurrent use; the model may call several tools in parallel.
type Toolbox struct {
	Context *review.ReviewContext
	// OnFinding, if set, is called for every finding that passes validation
	OnFinding func(review.Finding)

	rootOnce sync.Once
	root     string
	rootErr  error

	mu       sync.Mutex
	findings []review.Finding
}

// New returns a toolbox over the given context
func New(reviewContext *review.ReviewContext) *Toolbox {
	return &Toolbox{Context: reviewContext}
}

// RiskRankingInput is the (empty) input of get_risk_ranking
type RiskRankingInput struct{}

// FunctionInput names a changed function
type FunctionInput struct {
	ID string `json:"id" jsonschema:"required" jsonschema_description:"Function id exactly as listed by get_risk_ranking (for example 'github.com/acme/app/pkg.(*Server).Handle' or 'src/app.py:handle'). A bare function name also works when it is unambiguous."`
}

// FileDiffInput names a changed file
type FileDiffInput struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"Repo-relative path of a file changed in the PR, as listed by get_risk_ranking."`
}

// ReadLinesInput selects a line range of any file in the PR head checkout
type ReadLinesInput struct {
	Path  string `json:"path" jsonschema:"required" jsonschema_description:"Repo-relative path of any file in the PR head checkout."`
	Start int    `json:"start" jsonschema:"required" jsonschema_description:"First line to return, 1-based."`
	End   int    `json:"end" jsonschema:"required" jsonschema_description:"Last line to return, inclusive. At most 400 lines are returned per call."`
}

// Definition describes one tool for registration with an MCP server or the Messages API
type Definition struct {
	Name        string
	Description string
	InputSchema json.RawMessage // JSON Schema object with additionalProperties false
}

type handlerFunc func(toolbox *Toolbox, raw json.RawMessage) (string, error)

type toolSpec struct {
	name        string
	description string
	input       any
	handler     handlerFunc
}

var specs = []toolSpec{
	{ToolGetRiskRanking, descriptionRiskRanking, RiskRankingInput{}, bind((*Toolbox).GetRiskRanking)},
	{ToolGetFunction, descriptionGetFunction, FunctionInput{}, bind((*Toolbox).GetFunction)},
	{ToolGetCallers, descriptionGetCallers, FunctionInput{}, bind((*Toolbox).GetCallers)},
	{ToolGetCallees, descriptionGetCallees, FunctionInput{}, bind((*Toolbox).GetCallees)},
	{ToolGetFileDiff, descriptionGetFileDiff, FileDiffInput{}, bind((*Toolbox).GetFileDiff)},
	{ToolReadLines, descriptionReadLines, ReadLinesInput{}, bind((*Toolbox).ReadLines)},
	{ToolReportFinding, descriptionReportFinding, FindingInput{}, bind((*Toolbox).ReportFinding)},
}

// bind adapts a typed tool method to a raw-JSON handler that decodes strictly
func bind[In any](method func(*Toolbox, In) (string, error)) handlerFunc {
	return func(toolbox *Toolbox, raw json.RawMessage) (string, error) {
		var input In
		if err := decodeStrict(raw, &input); err != nil {
			return "", err
		}
		return method(toolbox, input)
	}
}

// decodeStrict unmarshals tool input, rejecting unknown fields and trailing data
func decodeStrict(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		raw = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("invalid tool input: trailing data after JSON object")
	}
	return nil
}

// Call runs the named tool with raw JSON input. The output is capped at maxOutputBytes.
func (t *Toolbox) Call(name string, raw json.RawMessage) (string, error) {
	for _, spec := range specs {
		if spec.name == name {
			if err := ValidateInput(schemaOf(spec), raw); err != nil {
				return "", err
			}
			output, err := spec.handler(t, raw)
			if err != nil {
				return "", err
			}
			return capOutput(output), nil
		}
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

var schemaCache sync.Map // tool name -> json.RawMessage

// schemaOf returns the cached input schema of a tool
func schemaOf(spec toolSpec) json.RawMessage {
	if cached, ok := schemaCache.Load(spec.name); ok {
		return cached.(json.RawMessage)
	}
	schema := SchemaFor(spec.input)
	schemaCache.Store(spec.name, schema)
	return schema
}

// Names lists every tool name in registration order
func Names() []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.name)
	}
	return names
}

// Definitions returns name, description and input schema for every tool, so
// the MCP server and the API engine register exactly the same set
func Definitions() []Definition {
	definitions := make([]Definition, 0, len(specs))
	for _, spec := range specs {
		definitions = append(definitions, Definition{
			Name:        spec.name,
			Description: spec.description,
			InputSchema: append(json.RawMessage(nil), schemaOf(spec)...),
		})
	}
	return definitions
}

// Findings returns a copy of every finding recorded so far, in report order
func (t *Toolbox) Findings() []review.Finding {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]review.Finding(nil), t.findings...)
}

// capOutput truncates tool output that would flood the model's context
func capOutput(output string) string {
	if len(output) <= maxOutputBytes {
		return output
	}
	return strings.ToValidUTF8(output[:maxOutputBytes], "") + "\n…[output truncated; narrow the request, e.g. read_lines with a smaller range]"
}

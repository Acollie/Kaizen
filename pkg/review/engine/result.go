package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
)

// sourceLead tags findings and events produced by the lead agent
const sourceLead = "lead"

// ReviewOutput is the final structured output the model produces: the CLI
// engine passes its schema to --json-schema and the API engine uses it as the
// submit_review tool input
type ReviewOutput struct {
	Summary  string               `json:"summary" jsonschema:"required" jsonschema_description:"Markdown review summary for the PR author: what the PR does, the main risks you checked, and why you chose the verdict. 2-6 short paragraphs or bullets. Do not restate every finding in full."`
	Verdict  string               `json:"verdict" jsonschema:"required,enum=approve,enum=comment,enum=request_changes" jsonschema_description:"request_changes if any critical finding, or a warning that will break realistic use; comment if there are only warnings or suggestions worth discussing; approve if nothing material was found."`
	Findings []tools.FindingInput `json:"findings" jsonschema:"required" jsonschema_description:"Every finding that still stands after your review, including the ones subagents recorded, each with the id returned by report_finding. This list is authoritative: a recorded finding you leave out is treated as withdrawn and removed from the review, so leave out exactly the ones you have concluded are wrong. Empty if there are none."`
}

// ReviewOutputSchema returns the JSON Schema of ReviewOutput
func ReviewOutputSchema() json.RawMessage {
	return tools.SchemaFor(ReviewOutput{})
}

// parseReviewOutput decodes structured output leniently (unknown fields are ignored)
func parseReviewOutput(raw json.RawMessage) (*ReviewOutput, error) {
	var output ReviewOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("decode structured review output: %w", err)
	}
	return &output, nil
}

// normalizeVerdict maps model output onto the Verdict enum, defaulting to comment
func normalizeVerdict(verdict string) review.Verdict {
	switch review.Verdict(strings.ToLower(strings.TrimSpace(verdict))) {
	case review.VerdictApprove:
		return review.VerdictApprove
	case review.VerdictRequestChanges:
		return review.VerdictRequestChanges
	default:
		return review.VerdictComment
	}
}

// findingSet collects findings, dropping duplicates by id and by file, line and title
type findingSet struct {
	mu    sync.Mutex
	items []review.Finding
	byID  map[string]int
	byKey map[string]int
}

func newFindingSet() *findingSet {
	return &findingSet{byID: map[string]int{}, byKey: map[string]int{}}
}

func dedupKey(finding review.Finding) string {
	return fmt.Sprintf("%s\x00%d\x00%s", finding.File, finding.Line, tools.NormalizeTitle(finding.Title))
}

// add records a finding and reports whether it was new
func (set *findingSet) add(finding review.Finding) bool {
	set.mu.Lock()
	defer set.mu.Unlock()
	if finding.ID == "" {
		finding.ID = tools.FindingID(finding)
	}
	if _, exists := set.byID[finding.ID]; exists {
		return false
	}
	key := dedupKey(finding)
	if _, exists := set.byKey[key]; exists {
		return false
	}
	set.byID[finding.ID] = len(set.items)
	set.byKey[key] = len(set.items)
	set.items = append(set.items, finding)
	return true
}

// lookup finds a recorded finding with the same id or the same file, line and title
func (set *findingSet) lookup(finding review.Finding) (review.Finding, bool) {
	set.mu.Lock()
	defer set.mu.Unlock()
	if index, ok := set.byID[finding.ID]; ok && finding.ID != "" {
		return set.items[index], true
	}
	if index, ok := set.byKey[dedupKey(finding)]; ok {
		return set.items[index], true
	}
	return review.Finding{}, false
}

func (set *findingSet) list() []review.Finding {
	set.mu.Lock()
	defer set.mu.Unlock()
	return append([]review.Finding{}, set.items...)
}

// assembleResult merges the structured output with the live findings. When
// structured output is present it is authoritative: the structured version of
// a finding wins over the live one, and live findings it omits are treated as
// withdrawn by the lead and dropped (reported through emit). Without
// structured output every live finding is kept. Structured findings that fail
// validation fall back to their live counterpart if there is one, and are
// otherwise dropped and reported through emit.
func assembleResult(output *ReviewOutput, live *findingSet, toolbox *tools.Toolbox, emit review.EmitFunc) *review.ReviewResult {
	merged := newFindingSet()
	result := &review.ReviewResult{Verdict: review.VerdictComment, Findings: []review.Finding{}}
	if output == nil {
		for _, finding := range live.list() {
			merged.add(finding)
		}
		result.Findings = merged.list()
		return result
	}
	result.Summary = strings.TrimSpace(output.Summary)
	result.Verdict = normalizeVerdict(output.Verdict)
	for _, input := range output.Findings {
		addStructuredFinding(input.ToFinding(), merged, live, toolbox, emit)
	}
	reportWithdrawn(withdrawnFindings(live, merged), emit)
	result.Findings = merged.list()
	return result
}

// withdrawnFindings lists the live findings the structured output left out
func withdrawnFindings(live, merged *findingSet) []review.Finding {
	var withdrawn []review.Finding
	for _, finding := range live.list() {
		if _, kept := merged.lookup(finding); !kept {
			withdrawn = append(withdrawn, finding)
		}
	}
	return withdrawn
}

// reportWithdrawn emits one status event naming the withdrawn findings
func reportWithdrawn(withdrawn []review.Finding, emit review.EmitFunc) {
	if len(withdrawn) == 0 {
		return
	}
	titles := make([]string, len(withdrawn))
	for index, finding := range withdrawn {
		titles[index] = strconv.Quote(finding.Title)
	}
	noun := "findings"
	if len(withdrawn) == 1 {
		noun = "finding"
	}
	emitStatus(emit, fmt.Sprintf("Lead reviewer withdrew %d %s: %s", len(withdrawn), noun, strings.Join(titles, ", ")), sourceLead)
}

func addStructuredFinding(finding review.Finding, merged, live *findingSet, toolbox *tools.Toolbox, emit review.EmitFunc) {
	if err := toolbox.ValidateFinding(&finding); err != nil {
		if previous, ok := live.lookup(finding); ok {
			merged.add(previous)
			return
		}
		emitStatus(emit, fmt.Sprintf("Dropped finding %q from the final output: %v", finding.Title, err), sourceLead)
		return
	}
	if previous, ok := live.lookup(finding); ok {
		finding.ID = previous.ID
		if finding.Source == "" {
			finding.Source = previous.Source
		}
	}
	if finding.ID == "" {
		finding.ID = tools.FindingID(finding)
	}
	if finding.Source == "" {
		finding.Source = sourceLead
	}
	merged.add(finding)
}

// emitStatus sends a status event if emit is set
func emitStatus(emit review.EmitFunc, message, agent string) {
	if emit != nil {
		emit(review.NewEvent(review.EventStatus, review.StatusData{Message: message, Agent: agent}))
	}
}

// emitEvent sends an event if emit is set
func emitEvent(emit review.EmitFunc, eventType review.EventType, data any) {
	if emit != nil {
		emit(review.NewEvent(eventType, data))
	}
}

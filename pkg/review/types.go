// Package review builds a risk-focused review context for a pull request and
// runs an AI review engine over it. The types in this file are the shared
// contract between the data layer (context building and risk scoring), the AI
// engines, the MCP tool server, and the web UI. Everything here serialises to
// JSON because the UI and the MCP server consume it in that form.
package review

import (
	"context"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
)

// PullRequest holds the GitHub metadata for the PR under review
type PullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Author    string `json:"author"`
	URL       string `json:"url"`
	Owner     string `json:"owner"` // repository owner, e.g. "alexcollie"
	Repo      string `json:"repo"`  // repository name, e.g. "kaizen"
	BaseRef   string `json:"base_ref"`
	HeadRef   string `json:"head_ref"`
	BaseSHA   string `json:"base_sha"`
	HeadSHA   string `json:"head_sha"`
	MergeBase string `json:"merge_base"` // the commit the diff is computed against
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	FileCount int    `json:"file_count"`
}

// FileStatus describes how a file changed in the PR
type FileStatus string

const (
	FileAdded    FileStatus = "added"
	FileModified FileStatus = "modified"
	FileDeleted  FileStatus = "deleted"
	FileRenamed  FileStatus = "renamed"
)

// DiffLineKind marks a line in a hunk as context, added, or removed
type DiffLineKind string

const (
	LineContext DiffLineKind = "context"
	LineAdded   DiffLineKind = "added"
	LineRemoved DiffLineKind = "removed"
)

// DiffLine is one line of a display hunk. OldLine/NewLine are 1-based and 0
// when the line does not exist on that side (e.g. NewLine is 0 for removals).
type DiffLine struct {
	Kind    DiffLineKind `json:"kind"`
	OldLine int          `json:"old_line"`
	NewLine int          `json:"new_line"`
	Text    string       `json:"text"` // line content without the +/-/space prefix
}

// Hunk is one display hunk (from `git diff --unified=3`)
type Hunk struct {
	OldStart int        `json:"old_start"`
	OldCount int        `json:"old_count"`
	NewStart int        `json:"new_start"`
	NewCount int        `json:"new_count"`
	Header   string     `json:"header"` // trailing text after the @@ markers, often the enclosing function
	Lines    []DiffLine `json:"lines"`
}

// FileDiff is the full diff for one file plus its review-relevant summary
type FileDiff struct {
	Path      string     `json:"path"`     // repo-relative path on the head side
	OldPath   string     `json:"old_path"` // repo-relative path on the base side (differs for renames)
	Status    FileStatus `json:"status"`
	Language  string     `json:"language"` // "Go", "Python", "Kotlin", "Swift", or "" if unsupported
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Binary    bool       `json:"binary"`
	Hunks     []Hunk     `json:"hunks"`
	LineCount int        `json:"line_count"` // total lines in the head version, for context expansion bounds
	RiskScore float64    `json:"risk_score"` // max of the file's own risk and its riskiest changed function

	// RiskReasons explains file-level risk that is not captured by a function, e.g.
	// "CI workflow: runs with repository secrets". Empty for ordinary source files.
	RiskReasons []string `json:"risk_reasons,omitempty"`
}

// FunctionRef names a function that is related to a changed function (a caller or callee)
type FunctionRef struct {
	ID       string `json:"id"` // call-graph full name where known, else "path:name"
	Name     string `json:"name"`
	File     string `json:"file"` // repo-relative, empty for external functions
	Line     int    `json:"line"` // call site line for callers, definition line for callees
	External bool   `json:"external"`
}

// FunctionRisk is a changed function with its metrics, relationships and risk ranking
type FunctionRisk struct {
	ID           string `json:"id"` // stable key: call-graph full name for Go, else "path:name"
	Name         string `json:"name"`
	ReceiverType string `json:"receiver_type,omitempty"`
	File         string `json:"file"` // repo-relative
	Language     string `json:"language"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	IsNew        bool   `json:"is_new"` // the function does not exist on the base side

	Metrics *models.FunctionAnalysis `json:"metrics,omitempty"` // nil if the language analyzer failed

	FanIn            int           `json:"fan_in"`
	FanInApproximate bool          `json:"fan_in_approximate"` // true for grep-based (non-Go) counts
	Callers          []FunctionRef `json:"callers"`
	Callees          []FunctionRef `json:"callees"`

	RiskScore   float64  `json:"risk_score"`   // 0-100, higher is riskier
	RiskReasons []string `json:"risk_reasons"` // human-readable, e.g. "cyclomatic complexity 21 (high)"
	Focus       bool     `json:"focus"`        // in the top-N the review should concentrate on
}

// GraphNodeKind distinguishes changed functions from their context in the review graph
type GraphNodeKind string

const (
	NodeChanged  GraphNodeKind = "changed"
	NodeCaller   GraphNodeKind = "caller"
	NodeCallee   GraphNodeKind = "callee"
	NodeExternal GraphNodeKind = "external"
)

// GraphNode is a node in the review graph
type GraphNode struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	File       string        `json:"file"`
	Line       int           `json:"line"`
	Kind       GraphNodeKind `json:"kind"`
	RiskScore  float64       `json:"risk_score"` // 0 for non-changed nodes
	Focus      bool          `json:"focus"`
	FanIn      int           `json:"fan_in"`
	Complexity int           `json:"complexity"`
}

// GraphEdge is a call from one node to another
type GraphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
}

// ReviewGraph is the subgraph shown in the UI: changed functions plus one hop of callers and callees
type ReviewGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// ReviewContext is everything Kaizen knows about the PR before any AI runs.
// It is written to disk as JSON for the MCP server and served to the UI.
type ReviewContext struct {
	PR           PullRequest      `json:"pr"`
	Files        []FileDiff       `json:"files"`     // sorted by RiskScore, descending
	Functions    []FunctionRisk   `json:"functions"` // sorted by RiskScore, descending
	Graph        ReviewGraph      `json:"graph"`
	Concerns     []models.Concern `json:"concerns"`      // blast radius etc. from pkg/check
	WorktreePath string           `json:"worktree_path"` // absolute path of the PR head checkout
	GeneratedAt  time.Time        `json:"generated_at"`
}

// Severity ranks how serious a finding is
type Severity string

const (
	SeverityCritical   Severity = "critical"
	SeverityWarning    Severity = "warning"
	SeveritySuggestion Severity = "suggestion"
	SeverityPraise     Severity = "praise"
)

// Category groups findings by the kind of problem
type Category string

const (
	CategoryCorrectness Category = "correctness"
	CategorySecurity    Category = "security"
	CategoryPerformance Category = "performance"
	CategoryDesign      Category = "design"
	CategoryTests       Category = "tests"
)

// Finding is one review comment produced by the AI
type Finding struct {
	ID         string   `json:"id"` // assigned by the engine if the model omits it
	Severity   Severity `json:"severity"`
	Category   Category `json:"category"`
	File       string   `json:"file"`               // repo-relative
	Line       int      `json:"line,omitempty"`     // head-side line; 0 means file- or PR-level
	EndLine    int      `json:"end_line,omitempty"` // optional range end
	FunctionID string   `json:"function_id,omitempty"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`                 // markdown
	Suggestion string   `json:"suggestion,omitempty"` // optional replacement code for the line range
	Verify     string   `json:"verify,omitempty"`     // one concrete step a developer can take to confirm it themselves
	Confidence float64  `json:"confidence"`           // 0-1
	Source     string   `json:"source,omitempty"`     // which agent produced it, e.g. "hotspot-reviewer"
}

// Verdict is the overall recommendation
type Verdict string

const (
	VerdictApprove        Verdict = "approve"
	VerdictComment        Verdict = "comment"
	VerdictRequestChanges Verdict = "request_changes"
)

// ReviewResult is the final output of an engine run
type ReviewResult struct {
	Summary  string    `json:"summary"` // markdown
	Verdict  Verdict   `json:"verdict"`
	Findings []Finding `json:"findings"`
	Engine   string    `json:"engine"`
	Model    string    `json:"model,omitempty"`
	CostUSD  float64   `json:"cost_usd,omitempty"` // reported by the CLI engine when available
}

// EventType identifies what an Event carries
type EventType string

const (
	EventStatus   EventType = "status"    // Data: StatusData
	EventThinking EventType = "thinking"  // Data: TextData (summarised reasoning or assistant prose)
	EventToolCall EventType = "tool_call" // Data: ToolCallData
	EventFinding  EventType = "finding"   // Data: Finding
	EventResult   EventType = "result"    // Data: ReviewResult (final, authoritative)
	EventError    EventType = "error"     // Data: TextData
	EventDone     EventType = "done"      // Data: nil; the stream is finished
)

// Event is one item in the live review stream sent to the UI over SSE
type Event struct {
	Seq  int       `json:"seq"` // assigned by the event log, monotonically increasing
	Type EventType `json:"type"`
	Time time.Time `json:"time"`
	Data any       `json:"data,omitempty"`
}

// StatusData reports progress, e.g. "Dispatching hotspot-reviewer for runAnalyze"
type StatusData struct {
	Message string `json:"message"`
	Agent   string `json:"agent,omitempty"` // "lead" or a subagent name
}

// TextData carries free text
type TextData struct {
	Text  string `json:"text"`
	Agent string `json:"agent,omitempty"`
}

// ToolCallData describes a tool the model invoked
type ToolCallData struct {
	Tool    string `json:"tool"`    // e.g. "get_callers", "Read"
	Summary string `json:"summary"` // short human description, e.g. "get_callers(runAnalyze)"
	Agent   string `json:"agent,omitempty"`
}

// EmitFunc receives events from an engine. Implementations must be safe for concurrent use.
type EmitFunc func(Event)

// Engine runs an AI review over a prepared context
type Engine interface {
	Name() string
	Run(ctx context.Context, reviewContext *ReviewContext, emit EmitFunc) (*ReviewResult, error)
}

// NewEvent builds an event with the current time; Seq is filled in by the event log
func NewEvent(eventType EventType, data any) Event {
	return Event{Type: eventType, Time: time.Now(), Data: data}
}

// PostRequest is what the UI sends when the user clicks "Post review"
type PostRequest struct {
	Summary  string    `json:"summary"`  // review body (markdown), editable in the UI
	Findings []Finding `json:"findings"` // only the findings the user selected
}

// PostResult reports where the review landed
type PostResult struct {
	URL            string `json:"url"`             // HTML URL of the created review
	InlineComments int    `json:"inline_comments"` // findings posted on diff lines
	BodyFindings   int    `json:"body_findings"`   // findings folded into the review body (no postable line)
}

// Poster publishes a review to the code host. pkg/github implements it; the UI depends only on this.
type Poster interface {
	PostReview(ctx context.Context, pr PullRequest, files []FileDiff, request PostRequest) (*PostResult, error)
}

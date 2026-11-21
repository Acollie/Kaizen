package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review"
)

const (
	maxTitleLength = 200
	maxBodyLength  = 8000
	// maxVerifyLength bounds the one-step verification; it is optional so older
	// cached findings without it still validate
	maxVerifyLength = 600
)

// FindingInput is the report_finding input; it mirrors review.Finding minus
// the engine-assigned Source
type FindingInput struct {
	ID         string  `json:"id,omitempty" jsonschema_description:"Optional. Leave empty for a new finding; the tool assigns one. Pass an existing id only to restate a finding you already recorded."`
	Severity   string  `json:"severity" jsonschema:"required,enum=critical,enum=warning,enum=suggestion,enum=praise" jsonschema_description:"critical: will cause a bug, outage, data loss or vulnerability in realistic use. warning: likely to cause a problem or a clear correctness risk. suggestion: a worthwhile improvement that is not a defect. praise: notably good work (use sparingly)."`
	Category   string  `json:"category" jsonschema:"required,enum=correctness,enum=security,enum=performance,enum=design,enum=tests" jsonschema_description:"The kind of problem."`
	File       string  `json:"file" jsonschema:"required" jsonschema_description:"Repo-relative path of a file changed in this PR."`
	Line       int     `json:"line" jsonschema:"required" jsonschema_description:"Head-side (new) line number the finding is about, as shown by get_function, get_file_diff or read_lines. Prefer a line that was added or changed in the PR. Use 0 only for a file-level finding."`
	EndLine    int     `json:"end_line,omitempty" jsonschema_description:"Optional last line of the range, inclusive. Omit or 0 for a single line."`
	FunctionID string  `json:"function_id,omitempty" jsonschema_description:"Optional id of the changed function this finding is about, from get_risk_ranking."`
	Title      string  `json:"title" jsonschema:"required" jsonschema_description:"One-line summary of the problem, under 100 characters, e.g. 'Nil map write when cfg.Labels is unset'."`
	Body       string  `json:"body" jsonschema:"required" jsonschema_description:"Markdown explanation: what goes wrong, the concrete input or call path that triggers it, and how you verified it. Cite line numbers."`
	Suggestion string  `json:"suggestion,omitempty" jsonschema_description:"Optional replacement code for the line range (line..end_line), exactly as it should appear, without diff markers."`
	Verify     string  `json:"verify" jsonschema:"required" jsonschema_description:"One concrete step the developer can take in a minute or two to confirm this finding themselves, naming the exact file:line, command, input or call. For example 'Run go test ./internal/config -run TestValidate with languages: [java]', 'Open action.yml:71 and check whether inputs.base-branch is quoted' or 'Call scoreDeltaIndicator(3) and look at the rendered comment'. Never a generic step such as 'Review the code'."`
	Confidence float64 `json:"confidence" jsonschema:"required" jsonschema_description:"Honest probability, from 0 to 1, that this is a real problem. Use the full range: 0.9 or above only when you read the whole code path and can name the triggering input; 0.6-0.8 when the path is read but a caller or runtime condition is unconfirmed; below 0.5 when you are inferring behaviour you did not read."`
}

var validSeverities = map[review.Severity]bool{
	review.SeverityCritical:   true,
	review.SeverityWarning:    true,
	review.SeveritySuggestion: true,
	review.SeverityPraise:     true,
}

var validCategories = map[review.Category]bool{
	review.CategoryCorrectness: true,
	review.CategorySecurity:    true,
	review.CategoryPerformance: true,
	review.CategoryDesign:      true,
	review.CategoryTests:       true,
}

// ToFinding converts the input to a review.Finding without validating it
func (input FindingInput) ToFinding() review.Finding {
	return review.Finding{
		ID:         strings.TrimSpace(input.ID),
		Severity:   review.Severity(strings.ToLower(strings.TrimSpace(input.Severity))),
		Category:   review.Category(strings.ToLower(strings.TrimSpace(input.Category))),
		File:       strings.TrimSpace(input.File),
		Line:       input.Line,
		EndLine:    input.EndLine,
		FunctionID: input.FunctionID,
		Title:      strings.TrimSpace(input.Title),
		Body:       strings.TrimSpace(input.Body),
		Suggestion: input.Suggestion,
		Verify:     strings.TrimSpace(input.Verify),
		Confidence: input.Confidence,
	}
}

// ReportFinding validates and records a finding. It returns "recorded <id>".
func (t *Toolbox) ReportFinding(input FindingInput) (string, error) {
	finding := input.ToFinding()
	if err := t.ValidateFinding(&finding); err != nil {
		return "", err
	}
	if finding.ID == "" {
		finding.ID = FindingID(finding)
	}
	t.record(finding)
	return "recorded " + finding.ID, nil
}

// record stores a finding, replacing an earlier one with the same id
func (t *Toolbox) record(finding review.Finding) {
	t.mu.Lock()
	replaced := false
	for index := range t.findings {
		if t.findings[index].ID == finding.ID {
			t.findings[index] = finding
			replaced = true
			break
		}
	}
	if !replaced {
		t.findings = append(t.findings, finding)
	}
	onFinding := t.OnFinding
	t.mu.Unlock()
	if onFinding != nil {
		onFinding(finding)
	}
}

// ValidateFinding checks a finding against the review context. It normalises
// the file path to the PR's spelling and clamps EndLine.
func (t *Toolbox) ValidateFinding(finding *review.Finding) error {
	if !validSeverities[finding.Severity] {
		return fmt.Errorf("invalid severity %q: use critical, warning, suggestion or praise", finding.Severity)
	}
	if !validCategories[finding.Category] {
		return fmt.Errorf("invalid category %q: use correctness, security, performance, design or tests", finding.Category)
	}
	if err := validateText(finding); err != nil {
		return err
	}
	if finding.Confidence < 0 || finding.Confidence > 1 {
		return fmt.Errorf("confidence %v must be between 0 and 1", finding.Confidence)
	}
	fileDiff := t.findFile(finding.File)
	if fileDiff == nil {
		return fmt.Errorf("file %q is not part of this PR; findings must be on changed files (see get_risk_ranking)", finding.File)
	}
	finding.File = fileDiff.Path
	return t.validateLines(finding, fileDiff)
}

func validateText(finding *review.Finding) error {
	if finding.Title == "" {
		return fmt.Errorf("title must not be empty")
	}
	if finding.Body == "" {
		return fmt.Errorf("body must not be empty")
	}
	if len(finding.Title) > maxTitleLength {
		return fmt.Errorf("title is %d characters; keep it under %d", len(finding.Title), maxTitleLength)
	}
	if len(finding.Body) > maxBodyLength {
		return fmt.Errorf("body is %d characters; keep it under %d", len(finding.Body), maxBodyLength)
	}
	if len(finding.Verify) > maxVerifyLength {
		return fmt.Errorf("verify is %d characters; keep it to one step under %d", len(finding.Verify), maxVerifyLength)
	}
	return nil
}

// validateLines checks Line and EndLine against the head-side length of the file
func (t *Toolbox) validateLines(finding *review.Finding, fileDiff *review.FileDiff) error {
	if finding.Line < 0 || finding.EndLine < 0 {
		return fmt.Errorf("line numbers must not be negative")
	}
	if finding.Line == 0 {
		finding.EndLine = 0
		return nil
	}
	lineCount := t.headLineCount(fileDiff)
	if lineCount == 0 {
		return fmt.Errorf("%s has no head-side lines (status %s); use line 0 for a file-level finding", fileDiff.Path, fileDiff.Status)
	}
	if finding.Line > lineCount {
		return fmt.Errorf("line %d is past the end of %s (%d lines)", finding.Line, fileDiff.Path, lineCount)
	}
	if finding.EndLine != 0 && finding.EndLine < finding.Line {
		return fmt.Errorf("end_line %d is before line %d", finding.EndLine, finding.Line)
	}
	if finding.EndLine > lineCount {
		return fmt.Errorf("end_line %d is past the end of %s (%d lines)", finding.EndLine, fileDiff.Path, lineCount)
	}
	if finding.EndLine == finding.Line {
		finding.EndLine = 0
	}
	return nil
}

// headLineCount returns the head-side line count of a changed file, reading
// the worktree when the context does not record it
func (t *Toolbox) headLineCount(fileDiff *review.FileDiff) int {
	if fileDiff.Status == review.FileDeleted {
		return 0
	}
	if fileDiff.LineCount > 0 {
		return fileDiff.LineCount
	}
	lines, err := t.readFileLines(fileDiff.Path)
	if err != nil {
		return 0
	}
	return len(lines)
}

// FindingID derives a stable id from where a finding is and what it says, so
// the same finding reported twice (or by two processes) gets the same id
func FindingID(finding review.Finding) string {
	key := fmt.Sprintf("%s\x00%d\x00%s", finding.File, finding.Line, NormalizeTitle(finding.Title))
	sum := sha256.Sum256([]byte(key))
	return "f-" + hex.EncodeToString(sum[:])[:10]
}

// NormalizeTitle lowercases a title and collapses whitespace, for dedup
func NormalizeTitle(title string) string {
	return strings.Join(strings.Fields(strings.ToLower(title)), " ")
}

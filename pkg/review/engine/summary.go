package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alexcollie/kaizen/pkg/review/tools"
)

// toolArguments holds the input fields the summaries use, across all tools
type toolArguments struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
	FilePath     string `json:"file_path"`
	Pattern      string `json:"pattern"`
	Glob         string `json:"glob"`
	SubagentType string `json:"subagent_type"`
	Description  string `json:"description"`
	Severity     string `json:"severity"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Title        string `json:"title"`
	Verdict      string `json:"verdict"`
}

// summarizeToolCall renders a short human description such as
// "get_callers(pkg.Func)" or "Read(pkg/a.go)"
func summarizeToolCall(name string, input json.RawMessage, worktreePath string) string {
	var arguments toolArguments
	_ = json.Unmarshal(input, &arguments)
	short := strings.TrimPrefix(name, mcpToolPrefix)
	summary := summarizeKnownTool(short, arguments, worktreePath)
	if summary == "" {
		summary = fmt.Sprintf("%s(%s)", short, compactJSON(input))
	}
	return truncate(summary, maxSummaryLength)
}

func summarizeKnownTool(name string, arguments toolArguments, worktreePath string) string {
	switch name {
	case tools.ToolGetRiskRanking:
		return "get_risk_ranking()"
	case tools.ToolGetFunction, tools.ToolGetCallers, tools.ToolGetCallees:
		return fmt.Sprintf("%s(%s)", name, arguments.ID)
	case tools.ToolGetFileDiff:
		return fmt.Sprintf("%s(%s)", name, arguments.Path)
	case tools.ToolReadLines:
		return fmt.Sprintf("read_lines(%s:%d-%d)", arguments.Path, arguments.Start, arguments.End)
	case tools.ToolReportFinding:
		return fmt.Sprintf("report_finding(%s %s:%d %s)", arguments.Severity, arguments.File, arguments.Line, arguments.Title)
	case toolSubmitReview:
		return fmt.Sprintf("submit_review(%s)", arguments.Verdict)
	case "Read":
		return fmt.Sprintf("Read(%s)", relativeTo(worktreePath, arguments.FilePath))
	case "Grep":
		return summarizeSearch("Grep", arguments, worktreePath)
	case "Glob":
		return summarizeSearch("Glob", arguments, worktreePath)
	case toolAgent:
		return fmt.Sprintf("Agent(%s: %s)", arguments.SubagentType, arguments.Description)
	default:
		return ""
	}
}

func summarizeSearch(name string, arguments toolArguments, worktreePath string) string {
	summary := fmt.Sprintf("%s(%q", name, arguments.Pattern)
	if arguments.Path != "" {
		summary += " in " + relativeTo(worktreePath, arguments.Path)
	}
	if arguments.Glob != "" {
		summary += " " + arguments.Glob
	}
	return summary + ")"
}

// relativeTo shows paths inside the worktree relative to it
func relativeTo(root, path string) string {
	if root == "" || path == "" || !filepath.IsAbs(path) {
		return path
	}
	roots := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		roots = append(roots, resolved)
	}
	for _, candidate := range roots {
		relative, err := filepath.Rel(candidate, path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(relative)
		}
	}
	return path
}

func compactJSON(raw json.RawMessage) string {
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		return ""
	}
	return compacted.String()
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.ToValidUTF8(text[:limit], "") + "…"
}

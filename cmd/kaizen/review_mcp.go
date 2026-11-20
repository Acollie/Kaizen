package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// reviewMCPServerName is the MCP server name; Claude Code exposes its tools as mcp__kaizen__<tool>
const reviewMCPServerName = "kaizen"

var reviewMCPCmd = &cobra.Command{
	Use:    "review-mcp",
	Short:  "Serve the PR review tools over MCP on stdio (used by `kaizen review`)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE:   runReviewMCP,
	// stdout carries MCP protocol messages only, so nothing else may print there
	SilenceUsage: true,
}

func init() {
	reviewMCPCmd.Flags().String("context", "", "Path to the review context JSON file")
	_ = reviewMCPCmd.MarkFlagRequired("context")
	reviewMCPCmd.SetOut(os.Stderr)
	rootCmd.AddCommand(reviewMCPCmd)
}

func runReviewMCP(cmd *cobra.Command, _ []string) error {
	contextPath, _ := cmd.Flags().GetString("context")
	reviewContext, err := loadReviewContext(contextPath)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	server := newReviewMCPServer(tools.New(reviewContext), logger)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		return fmt.Errorf("review MCP server: %w", err)
	}
	return nil
}

// loadReviewContext reads a ReviewContext written by `kaizen review`
func loadReviewContext(path string) (*review.ReviewContext, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open review context: %w", err)
	}
	defer func() { _ = file.Close() }()
	var reviewContext review.ReviewContext
	if err := json.NewDecoder(io.LimitReader(file, 256<<20)).Decode(&reviewContext); err != nil {
		return nil, fmt.Errorf("parse review context %s: %w", path, err)
	}
	if reviewContext.WorktreePath == "" {
		return nil, fmt.Errorf("review context %s has no worktree_path", path)
	}
	return &reviewContext, nil
}

// newReviewMCPServer registers every Toolbox tool on a new MCP server. It uses
// the shared schemas from tools.Definitions and lets the Toolbox decode and
// validate arguments, so the MCP and API engines behave identically.
func newReviewMCPServer(toolbox *tools.Toolbox, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: reviewMCPServerName, Version: "1.0.0"},
		&mcp.ServerOptions{
			Logger:       logger,
			Instructions: "Read-only tools over a pull request under review. Start with get_risk_ranking. Report findings only through report_finding.",
		},
	)
	for _, definition := range tools.Definitions() {
		server.AddTool(&mcp.Tool{
			Name:        definition.Name,
			Description: definition.Description,
			InputSchema: definition.InputSchema,
			Annotations: toolAnnotations(definition.Name),
		}, toolHandler(toolbox, definition.Name))
	}
	return server
}

func toolAnnotations(name string) *mcp.ToolAnnotations {
	readOnly := name != tools.ToolReportFinding
	return &mcp.ToolAnnotations{ReadOnlyHint: readOnly, IdempotentHint: true}
}

func toolHandler(toolbox *tools.Toolbox, name string) mcp.ToolHandler {
	return func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		output, err := toolbox.Call(name, request.Params.Arguments)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + err.Error()}},
				IsError: true,
			}, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: output}}}, nil
	}
}

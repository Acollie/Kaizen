package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/review/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewMCPFixture(t *testing.T) *review.ReviewContext {
	t.Helper()
	worktree := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "main.go"), []byte("package main\n\nfunc run() int {\n\treturn 1\n}\n"), 0o644))
	return &review.ReviewContext{
		PR: review.PullRequest{Number: 42, Title: "Add run", Author: "dev"},
		Files: []review.FileDiff{{
			Path: "main.go", Status: review.FileAdded, Language: "Go", LineCount: 5,
		}},
		Functions: []review.FunctionRisk{{
			ID: "main.run", Name: "run", File: "main.go", StartLine: 3, EndLine: 5, RiskScore: 12, Focus: true,
		}},
		WorktreePath: worktree,
	}
}

func connectReviewMCP(t *testing.T, reviewContext *review.ReviewContext) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := newReviewMCPServer(tools.New(reviewContext), logger)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func callText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected text content")
	return text.Text
}

func TestReviewMCPListsAndCallsTools(t *testing.T) {
	session := connectReviewMCP(t, reviewMCPFixture(t))
	ctx := context.Background()

	listed, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		assert.NotEmpty(t, tool.Description)
	}
	assert.ElementsMatch(t, tools.Names(), names)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tools.ToolGetRiskRanking, Arguments: map[string]any{}})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, callText(t, result), "PR #42 by dev: Add run")
	assert.Contains(t, callText(t, result), "id=main.run")
}

func TestReviewMCPToolErrorsAreResults(t *testing.T) {
	session := connectReviewMCP(t, reviewMCPFixture(t))
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      tools.ToolReadLines,
		Arguments: map[string]any{"path": "../../etc/passwd", "start": 1, "end": 5},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.True(t, strings.HasPrefix(callText(t, result), "Error: "))
}

func TestReviewMCPReportFinding(t *testing.T) {
	session := connectReviewMCP(t, reviewMCPFixture(t))
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: tools.ToolReportFinding,
		Arguments: map[string]any{
			"severity": "warning", "category": "tests", "file": "main.go", "line": 3,
			"title": "run is untested", "body": "No test covers run.", "confidence": 0.8,
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError, callText(t, result))
	assert.True(t, strings.HasPrefix(callText(t, result), "recorded f-"))
}

func TestLoadReviewContext(t *testing.T) {
	reviewContext := reviewMCPFixture(t)
	path := filepath.Join(t.TempDir(), "context.json")
	encoded, err := json.Marshal(reviewContext)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encoded, 0o600))

	loaded, err := loadReviewContext(path)
	require.NoError(t, err)
	assert.Equal(t, reviewContext.WorktreePath, loaded.WorktreePath)

	require.NoError(t, os.WriteFile(path, []byte(`{"pr":{}}`), 0o600))
	_, err = loadReviewContext(path)
	assert.ErrorContains(t, err, "no worktree_path")

	_, err = loadReviewContext(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)
}

package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexcollie/kaizen/pkg/review"
)

func TestGetPackageOverview(t *testing.T) {
	toolbox := New(&review.ReviewContext{
		Files: []review.FileDiff{
			{Path: "pkg/server/handler.go", Additions: 40, Deletions: 2, RiskScore: 10},
			{Path: "pkg/server/handler_test.go", Additions: 30},
			{Path: "pkg/store/db.go", Additions: 5, Deletions: 5, RiskScore: 3},
			{Path: "docs/guide.md", Additions: 12},
			{Path: "main.go", Additions: 1},
			{Path: "internal/fixtures/testdata/a.json", Additions: 4},
		},
		Functions: []review.FunctionRisk{
			{ID: "server.Handle", File: "pkg/server/handler.go", RiskScore: 31, Focus: true},
			{ID: "server.helper", File: "pkg/server/handler.go", RiskScore: 4},
			{ID: "store.Save", File: "pkg/store/db.go", RiskScore: 12},
		},
	})
	output, err := toolbox.Call(ToolGetPackageOverview, json.RawMessage(`{}`))
	require.NoError(t, err)

	assert.Contains(t, output, "Changed packages (5 groups from 6 files")
	assert.Contains(t, output, "pkg/server: 2 files, 2 changed functions, 72 lines changed (42 non-test), max risk 31, 1 FOCUS (riskiest: server.Handle)")
	assert.Contains(t, output, "pkg/store: 1 files, 1 changed functions, 10 lines changed (10 non-test), max risk 12, 0 FOCUS")
	assert.Contains(t, output, "(repository root): 1 files")
	assert.Contains(t, output, "internal/fixtures/testdata: 1 files, 0 changed functions, 4 lines changed (0 non-test), max risk 0, 0 FOCUS [tests only]")
	assert.Less(t, strings.Index(output, "pkg/server:"), strings.Index(output, "pkg/store:"), "riskiest first")
	assert.Contains(t, output, "  pkg/server/handler_test.go\n")
}

func TestGetPackageOverviewCollapsesDeepDirectories(t *testing.T) {
	var files []review.FileDiff
	for service := range 6 {
		for module := range 6 {
			files = append(files, review.FileDiff{Path: fmt.Sprintf("services/svc%d/mod%d/file.go", service, module), Additions: 1})
		}
		files = append(files, review.FileDiff{Path: fmt.Sprintf("services/svc%d/main.go", service), Additions: 1})
	}
	groups := buildPackageGroups(files, nil)
	assert.Len(t, groups, 6, "36 leaf directories collapse to their 6 parents")
	for _, group := range groups {
		assert.True(t, strings.HasSuffix(group.directory, "/..."), group.directory)
		assert.Len(t, group.files, 7, "the parent's own file joins its merged subtree")
	}
}

func TestCollapseDirectoriesStopsAtTopLevel(t *testing.T) {
	directories := map[string]string{}
	for index := range 30 {
		directories[fmt.Sprintf("dir%d/a.go", index)] = fmt.Sprintf("dir%d", index)
	}
	collapseDirectories(directories, maxOverviewGroups)
	assert.Equal(t, 30, distinctCount(directories), "top-level directories are never merged together")
}

func TestPackageOverviewIsRegistered(t *testing.T) {
	assert.Contains(t, Names(), ToolGetPackageOverview)
}

func TestFunctionIDDescriptionUsesRealFormats(t *testing.T) {
	schema := string(SchemaFor(FunctionInput{}))
	assert.Contains(t, schema, "copied exactly from get_risk_ranking")
	assert.Contains(t, schema, "review.Server.Handle")
	assert.Contains(t, schema, "src/app.py:handle")
	assert.NotContains(t, schema, "(*Server)")
	assert.NotContains(t, schema, "github.com/acme")
}

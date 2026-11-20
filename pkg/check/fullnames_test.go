package check

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexcollie/kaizen/pkg/models"
)

func buildFullNamesTestGraph(repoPath string) *models.CallGraph {
	graph := models.NewCallGraph()
	graph.AddNode(&models.CallNode{Name: "Run", FullName: "app.Run", File: filepath.Join(repoPath, "app/run.go"), Line: 5})
	graph.AddNode(&models.CallNode{Name: "String", FullName: "app.User.String", File: filepath.Join(repoPath, "app/types.go"), Line: 10})
	graph.AddNode(&models.CallNode{Name: "String", FullName: "app.Group.String", File: filepath.Join(repoPath, "app/types.go"), Line: 20})
	graph.AddNode(&models.CallNode{Name: "Run", FullName: "other.Run", File: filepath.Join(repoPath, "other/run.go"), Line: 3})
	graph.AddNode(&models.CallNode{Name: "Println", FullName: "fmt.Println", IsExternal: true})
	return graph
}

func TestMatchCallGraphNames(t *testing.T) {
	repoPath := "/repo"
	graph := buildFullNamesTestGraph(repoPath)

	testCases := []struct {
		name     string
		changed  []ChangedFunction
		expected []string
	}{
		{
			name:     "absolute path from MapHunksToFunctions",
			changed:  []ChangedFunction{{FilePath: "/repo/app/run.go", Name: "Run", StartLine: 5}},
			expected: []string{"app.Run"},
		},
		{
			name:     "repo-relative path",
			changed:  []ChangedFunction{{FilePath: "other/run.go", Name: "Run", StartLine: 3}},
			expected: []string{"other.Run"},
		},
		{
			name: "method disambiguated by receiver",
			changed: []ChangedFunction{
				{FilePath: "/repo/app/types.go", Name: "String", HasReceiver: true, ReceiverType: "Group", StartLine: 20},
			},
			expected: []string{"app.Group.String"},
		},
		{
			name: "duplicates removed and order kept",
			changed: []ChangedFunction{
				{FilePath: "/repo/other/run.go", Name: "Run"},
				{FilePath: "/repo/app/run.go", Name: "Run"},
				{FilePath: "/repo/other/run.go", Name: "Run"},
			},
			expected: []string{"other.Run", "app.Run"},
		},
		{
			name:     "no match",
			changed:  []ChangedFunction{{FilePath: "/repo/app/run.go", Name: "Missing"}},
			expected: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, MatchCallGraphNames(testCase.changed, graph, repoPath))
		})
	}
}

func TestMatchCallGraphNamesNilGraph(t *testing.T) {
	assert.Nil(t, MatchCallGraphNames([]ChangedFunction{{Name: "Run"}}, nil, "/repo"))
}

func TestRepoRelativePath(t *testing.T) {
	assert.Equal(t, "app/run.go", RepoRelativePath("/repo/app/run.go", "/repo"))
	assert.Equal(t, "app/run.go", RepoRelativePath("app/run.go", "/repo"))
	assert.Equal(t, "app/run.go", RepoRelativePath("./app/run.go", "."))
	assert.Equal(t, "/elsewhere/x.go", RepoRelativePath("/elsewhere/x.go", "/repo"))
	assert.Equal(t, "", RepoRelativePath("", "/repo"))
}

func TestMatchCallGraphNameIgnoresFixtureCopies(t *testing.T) {
	repoPath := "/repo"
	graph := models.NewCallGraph()
	graph.AddNode(&models.CallNode{Name: "Analyze", FullName: "analyzer.Pipeline.Analyze", File: "/repo/pkg/analyzer/pipeline.go", Line: 40})

	fixture := ChangedFunction{
		FilePath: "pkg/reviewui/testdata/worktree/pkg/analyzer/pipeline.go", Name: "Analyze",
		HasReceiver: true, ReceiverType: "Pipeline", StartLine: 40,
	}
	assert.Empty(t, MatchCallGraphName(fixture, graph, repoPath))

	original := fixture
	original.FilePath = "/repo/pkg/analyzer/pipeline.go"
	assert.Equal(t, "analyzer.Pipeline.Analyze", MatchCallGraphName(original, graph, repoPath))
}

func TestSameSourceFile(t *testing.T) {
	testCases := []struct {
		name        string
		nodePath    string
		changedPath string
		expected    bool
	}{
		{"identical relative", "pkg/a/b.go", "pkg/a/b.go", true},
		{"identical after cleaning", "./pkg/a/../a/b.go", "pkg/a/b.go", true},
		{"fixture copy is a different file", "pkg/x/testdata/pkg/a/b.go", "pkg/a/b.go", false},
		{"fixture copy, reversed", "pkg/a/b.go", "pkg/x/testdata/pkg/a/b.go", false},
		{"absolute node, relative change", "/elsewhere/repo/pkg/a/b.go", "pkg/a/b.go", true},
		{"relative node, absolute change", "pkg/a/b.go", "/elsewhere/repo/pkg/a/b.go", true},
		{"suffix must end on a segment boundary", "/elsewhere/xpkg/a/b.go", "pkg/a/b.go", false},
		{"two different absolute paths", "/one/pkg/a/b.go", "/two/pkg/a/b.go", false},
		{"empty", "", "pkg/a/b.go", false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, sameSourceFile(testCase.nodePath, testCase.changedPath))
		})
	}
}

// Directory-qualified keys (two packages named main) still match by file
func TestMatchCallGraphNameWithDirectoryQualifiedKeys(t *testing.T) {
	repoPath := "/repo"
	graph := models.NewCallGraph()
	graph.AddNode(&models.CallNode{Name: "main", FullName: "cmd/app:main.main", File: "/repo/cmd/app/main.go", Line: 5})
	graph.AddNode(&models.CallNode{Name: "main", FullName: "demo/sample:main.main", File: "/repo/demo/sample/main.go", Line: 5})
	graph.AddNode(&models.CallNode{Name: "Run", FullName: "demo/sample:main.Server.Run", File: "/repo/demo/sample/main.go", Line: 9})

	match := func(changed ChangedFunction) string { return MatchCallGraphName(changed, graph, repoPath) }
	assert.Equal(t, "cmd/app:main.main", match(ChangedFunction{FilePath: "cmd/app/main.go", Name: "main", StartLine: 5}))
	assert.Equal(t, "demo/sample:main.main", match(ChangedFunction{FilePath: "demo/sample/main.go", Name: "main", StartLine: 5}))
	assert.Equal(t, "demo/sample:main.Server.Run",
		match(ChangedFunction{FilePath: "demo/sample/main.go", Name: "Run", HasReceiver: true, ReceiverType: "Server", StartLine: 9}))
}

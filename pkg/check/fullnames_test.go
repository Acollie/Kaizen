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

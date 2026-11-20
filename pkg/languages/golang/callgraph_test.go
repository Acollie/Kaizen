package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeGoFiles writes each relative path -> source pair under a temp directory
func writeGoFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for relativePath, source := range files {
		fullPath := filepath.Join(root, relativePath)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(source), 0o644))
	}
	return root
}

func TestCallGraphCallCountIndependentOfFileOrder(t *testing.T) {
	// a_callers.go is walked before z_callee.go, so callers are seen before the callee is declared
	root := writeGoFiles(t, map[string]string{
		"a_callers.go": "package shop\n\nfunc One() { Total() }\nfunc Two() { Total(); Total() }\n",
		"z_callee.go":  "package shop\n\nfunc Total() int { return 1 }\n",
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	total := graph.Nodes["shop.Total"]
	require.NotNil(t, total)
	assert.False(t, total.IsExternal, "declared function must not be left as an external placeholder")
	assert.Equal(t, 3, total.CallCount)
	assert.Len(t, graph.Callers("shop.Total"), 2)
	assert.Equal(t, 2, graph.Nodes["shop.Two"].CallsOut)
}

func TestCallGraphResolvesReceiverMethodCalls(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"cart.go": `package shop

type Cart struct{}

func (cart *Cart) Checkout() { cart.validate() }
func (cart *Cart) validate()  {}

type Order struct{}

func (order *Order) validate() {}
`,
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	callers := graph.Callers("shop.Cart.validate")
	require.Len(t, callers, 1)
	assert.Equal(t, "shop.Cart.Checkout", callers[0].From)
	assert.Empty(t, graph.Callers("shop.Order.validate"))
}

func TestCallGraphResolvesUniqueMethodNamesOnValues(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"store/store.go": "package store\n\ntype Store struct{}\n\nfunc (store *Store) Persist() {}\n",
		"app/app.go": `package app

import "example.com/demo/store"

func Run(target *store.Store) {
	target.Persist()
	store.Open()
}
`,
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	persistCallers := graph.Callers("store.Store.Persist")
	require.Len(t, persistCallers, 1)
	assert.Equal(t, "app.Run", persistCallers[0].From)

	// Package-qualified call resolved through the import, even though Open is not declared
	assert.Len(t, graph.Callers("store.Open"), 1)
}

func TestCallGraphSkipsBuiltinsAndConversions(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"main.go": "package demo\n\ntype ID int\n\nfunc Build(items []int) int {\n\tlist := make([]int, 0, len(items))\n\treturn int(ID(len(append(list, 1))))\n}\n",
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	assert.Empty(t, graph.Edges)
	assert.Len(t, graph.Nodes, 1)
}

func TestCallGraphSkipsHiddenVendorAndTestdataDirectories(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"main.go":                          "package demo\n\nfunc Real() {}\n",
		".claude/worktrees/copy/main.go":   "package demo\n\nfunc Copy() {}\n",
		"vendor/lib/lib.go":                "package lib\n\nfunc Vendored() {}\n",
		"testdata/fixture/fixture.go":      "package fixture\n\nfunc Fixture() {}\n",
		"internal/feature/feature.go":      "package feature\n\nfunc Feature() {}\n",
		"internal/feature/feature_test.go": "package feature\n\nfunc TestHelper() {}\n",
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	assert.Contains(t, graph.Nodes, "demo.Real")
	assert.Contains(t, graph.Nodes, "feature.Feature")
	assert.Len(t, graph.Nodes, 2)
}

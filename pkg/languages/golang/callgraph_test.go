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

func TestCallGraphMarksUniqueMethodNameEdgesInferred(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"store/store.go": "package store\n\ntype Store struct{}\n\nfunc (store *Store) Persist() {}\n\nfunc (store *Store) Flushes() { store.Persist() }\n",
		"app/app.go":     "package app\n\nfunc Run(target *Thing) { target.Persist() }\n",
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	inferredByCaller := map[string]bool{}
	for _, edge := range graph.Callers("store.Store.Persist") {
		inferredByCaller[edge.From] = edge.Inferred
	}
	assert.Equal(t, map[string]bool{"app.Run": true, "store.Store.Flushes": false}, inferredByCaller)
}

func TestCallGraphDoesNotResolveCommonMethodNamesByName(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"buffer/tail.go": "package buffer\n\ntype tailBuffer struct{}\n\nfunc (tail *tailBuffer) String() string { return \"\" }\n\nfunc (tail *tailBuffer) Dump() string { return tail.String() }\n",
		"report/report.go": `package report

import "strings"

func Render() string {
	var buf strings.Builder
	buf.WriteString("x")
	return buf.String()
}
`,
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	callers := graph.Callers("buffer.tailBuffer.String")
	require.Len(t, callers, 1, "only the receiver call inside tailBuffer resolves")
	assert.Equal(t, "buffer.tailBuffer.Dump", callers[0].From)
	assert.False(t, callers[0].Inferred)
}

func TestCallGraphDoesNotInferMethodsOnImportedValues(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"sink/sink.go": "package sink\n\ntype Sink struct{}\n\nfunc (sink *Sink) Emit() {}\n",
		"app/app.go":   "package app\n\nimport \"example.com/vendor/logs\"\n\nfunc Run() { logs.Default.Emit() }\n",
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	assert.Empty(t, graph.Callers("sink.Sink.Emit"))
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

// Two directories declaring the same package name must not overwrite each other's
// nodes: both are qualified with their directory, and callers stay per-package
func TestCallGraphQualifiesPackageNamesDeclaredInSeveralDirectories(t *testing.T) {
	root := writeGoFiles(t, map[string]string{
		"go.mod":                "module example.com/app\n",
		"cmd/app/main.go":       "package main\n\nimport \"example.com/app/internal/util\"\n\nfunc main() { run(); util.Help() }\nfunc run() {}\n",
		"demo/sample/main.go":   "package main\n\nimport \"example.com/app/demo/util\"\n\nfunc main() { start(); start(); util.Help() }\nfunc start() {}\n",
		"internal/util/util.go": "package util\n\nfunc Help() {}\n",
		"demo/util/util.go":     "package util\n\nfunc Help() {}\n",
		"internal/shop/shop.go": "package shop\n\nfunc Total() int { return add() }\nfunc add() int { return 1 }\n",
		"internal/shop/cart.go": "package shop\n\ntype Cart struct{}\n\nfunc (cart *Cart) Sum() int { return Total() }\n",
	})

	graph, err := NewCallGraphAnalyzer().AnalyzeDirectory(root)
	require.NoError(t, err)

	appMain := graph.Nodes["cmd/app:main.main"]
	demoMain := graph.Nodes["demo/sample:main.main"]
	require.NotNil(t, appMain)
	require.NotNil(t, demoMain)
	assert.Equal(t, filepath.Join(root, "cmd/app/main.go"), appMain.File)
	assert.Equal(t, filepath.Join(root, "demo/sample/main.go"), demoMain.File)
	assert.Equal(t, "main", appMain.Package)
	assert.NotContains(t, graph.Nodes, "main.main", "ambiguous names are never left unqualified")

	runCallers := graph.Callers("cmd/app:main.run")
	require.Len(t, runCallers, 1)
	assert.Equal(t, "cmd/app:main.main", runCallers[0].From)
	startCallers := graph.Callers("demo/sample:main.start")
	require.Len(t, startCallers, 1)
	assert.Equal(t, "demo/sample:main.main", startCallers[0].From)
	assert.Equal(t, 2, startCallers[0].Weight)

	// Imports of an ambiguous package name resolve to the directory the path ends with
	internalHelp := graph.Callers("internal/util:util.Help")
	require.Len(t, internalHelp, 1)
	assert.Equal(t, "cmd/app:main.main", internalHelp[0].From)
	demoHelp := graph.Callers("demo/util:util.Help")
	require.Len(t, demoHelp, 1)
	assert.Equal(t, "demo/sample:main.main", demoHelp[0].From)

	// Unambiguous packages keep the plain format
	assert.Len(t, graph.Callers("shop.Total"), 1)
	assert.Equal(t, "shop.Cart.Sum", graph.Callers("shop.Total")[0].From)
	assert.Len(t, graph.Callers("shop.add"), 1)
}

package models

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestCallGraph() *CallGraph {
	graph := NewCallGraph()
	for _, name := range []string{"pkg.A", "pkg.B", "pkg.C", "pkg.D"} {
		graph.AddNode(&CallNode{Name: name[4:], FullName: name})
	}
	graph.AddEdge(CallEdge{From: "pkg.A", To: "pkg.C", File: "a.go", Line: 10})
	graph.AddEdge(CallEdge{From: "pkg.B", To: "pkg.C", File: "b.go", Line: 20})
	graph.AddEdge(CallEdge{From: "pkg.C", To: "pkg.D", File: "c.go", Line: 30})
	return graph
}

func TestCallersAndCallees(t *testing.T) {
	graph := buildTestCallGraph()

	callers := graph.Callers("pkg.C")
	require.Len(t, callers, 2)
	assert.Equal(t, "pkg.A", callers[0].From)
	assert.Equal(t, 10, callers[0].Line)
	assert.Equal(t, "pkg.B", callers[1].From)

	callees := graph.Callees("pkg.C")
	require.Len(t, callees, 1)
	assert.Equal(t, "pkg.D", callees[0].To)

	assert.Empty(t, graph.Callers("pkg.A"))
	assert.Empty(t, graph.Callees("pkg.D"))
	assert.Empty(t, graph.Callers("pkg.Missing"))
}

func TestCallersIndexRefreshesAfterAddEdge(t *testing.T) {
	graph := buildTestCallGraph()
	require.Len(t, graph.Callers("pkg.D"), 1)

	graph.AddEdge(CallEdge{From: "pkg.A", To: "pkg.D"})
	assert.Len(t, graph.Callers("pkg.D"), 2)
	assert.Len(t, graph.Callees("pkg.A"), 2)
}

func TestCallersReflectsWeightIncrements(t *testing.T) {
	graph := buildTestCallGraph()
	require.Equal(t, 1, graph.Callers("pkg.C")[0].Weight)

	// A repeated call increments the weight in place without adding an edge
	graph.AddEdge(CallEdge{From: "pkg.A", To: "pkg.C"})
	assert.Equal(t, 2, graph.Callers("pkg.C")[0].Weight)
}

func TestCallersReturnsCopy(t *testing.T) {
	graph := buildTestCallGraph()
	callers := graph.Callers("pkg.C")
	callers[0].From = "mutated"
	assert.Equal(t, "pkg.A", graph.Callers("pkg.C")[0].From)
}

func TestCallersConcurrentAccess(t *testing.T) {
	graph := buildTestCallGraph()
	var waitGroup sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			assert.Len(t, graph.Callers("pkg.C"), 2)
			assert.Len(t, graph.Callees("pkg.C"), 1)
		}()
	}
	waitGroup.Wait()
}

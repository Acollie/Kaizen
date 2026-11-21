package models

import "sync"

// CallGraph represents the function call relationships in a codebase
type CallGraph struct {
	Nodes map[string]*CallNode `json:"nodes"` // Key is function full name
	Edges []CallEdge           `json:"edges"`
	Stats CallGraphStats       `json:"stats"`

	// edgeIndex is a lazily built lookup from node full name to edge positions.
	// It is rebuilt whenever the number of edges changes.
	indexMutex sync.Mutex
	edgeIndex  *callEdgeIndex

	// edgePositions maps From/To to an edge's position in Edges, for O(1) deduplication in AddEdge
	edgePositions map[string]int
}

// callEdgeIndex maps a function full name to the positions of its edges in CallGraph.Edges
type callEdgeIndex struct {
	edgeCount    int
	incomingByTo map[string][]int
	outgoingFrom map[string][]int
}

// CallNode represents a function in the call graph
type CallNode struct {
	Name       string `json:"name"`        // Function name
	FullName   string `json:"full_name"`   // Package.Function
	Package    string `json:"package"`     // Package path
	File       string `json:"file"`        // File path
	Line       int    `json:"line"`        // Starting line
	Complexity int    `json:"complexity"`  // Cyclomatic complexity
	Length     int    `json:"length"`      // Function length
	CallCount  int    `json:"call_count"`  // How many times it's called (fan-in)
	CallsOut   int    `json:"calls_out"`   // How many functions it calls (fan-out)
	IsExternal bool   `json:"is_external"` // Is from external package
	IsExported bool   `json:"is_exported"` // Is exported (starts with capital)
}

// CallEdge represents a function call relationship
type CallEdge struct {
	From   string `json:"from"`   // Caller full name
	To     string `json:"to"`     // Callee full name
	Weight int    `json:"weight"` // Number of call sites
	File   string `json:"file"`   // Where the call happens
	Line   int    `json:"line"`   // Line number of call

	// Inferred is true when every call site behind this edge was resolved by method
	// name alone (no import or receiver evidence), so the callee may be wrong
	Inferred bool `json:"inferred,omitempty"`
}

// CallGraphStats provides summary statistics
type CallGraphStats struct {
	TotalFunctions   int     `json:"total_functions"`
	TotalCalls       int     `json:"total_calls"`
	TotalEdges       int     `json:"total_edges"`
	AvgCallsPerFunc  float64 `json:"avg_calls_per_func"`
	MaxFanIn         int     `json:"max_fan_in"`
	MaxFanOut        int     `json:"max_fan_out"`
	MostCalledFunc   string  `json:"most_called_func"`
	UnreachableFuncs int     `json:"unreachable_funcs"` // Never called
}

// NewCallGraph creates a new call graph
func NewCallGraph() *CallGraph {
	return &CallGraph{
		Nodes: make(map[string]*CallNode),
		Edges: []CallEdge{},
		Stats: CallGraphStats{},
	}
}

// AddNode adds a function node to the graph
func (graph *CallGraph) AddNode(node *CallNode) {
	graph.Nodes[node.FullName] = node
}

// AddEdge adds a call edge to the graph, merging repeated calls into one weighted edge
func (graph *CallGraph) AddEdge(edge CallEdge) {
	// Increment call counts
	if callee, exists := graph.Nodes[edge.To]; exists {
		callee.CallCount++
	}

	if caller, exists := graph.Nodes[edge.From]; exists {
		caller.CallsOut++
	}

	positions := graph.edgePositionIndex()
	key := edgeKey(edge.From, edge.To)
	if position, exists := positions[key]; exists {
		graph.Edges[position].Weight++
		if !edge.Inferred {
			graph.Edges[position].Inferred = false
		}
		return
	}

	edge.Weight = 1
	positions[key] = len(graph.Edges)
	graph.Edges = append(graph.Edges, edge)
}

// edgePositionIndex returns the From/To -> position map used to deduplicate edges,
// rebuilding it when Edges was modified directly (e.g. by a filter building a new graph)
func (graph *CallGraph) edgePositionIndex() map[string]int {
	if graph.edgePositions != nil && len(graph.edgePositions) == len(graph.Edges) {
		return graph.edgePositions
	}
	graph.edgePositions = make(map[string]int, len(graph.Edges))
	for position, edge := range graph.Edges {
		graph.edgePositions[edgeKey(edge.From, edge.To)] = position
	}
	return graph.edgePositions
}

func edgeKey(from, to string) string {
	return from + "\x00" + to
}

// RecomputeCallCounts derives every node's CallCount (call sites targeting it) and
// CallsOut (call sites inside it) from the edge weights. Builders call this once all
// nodes exist, so counts are correct regardless of the order nodes were added in.
func (graph *CallGraph) RecomputeCallCounts() {
	for _, node := range graph.Nodes {
		node.CallCount = 0
		node.CallsOut = 0
	}
	for _, edge := range graph.Edges {
		if callee, exists := graph.Nodes[edge.To]; exists {
			callee.CallCount += edge.Weight
		}
		if caller, exists := graph.Nodes[edge.From]; exists {
			caller.CallsOut += edge.Weight
		}
	}
}

// FilterByFunctionNames returns a new CallGraph containing only the named functions
// and their immediate callers and callees (one hop in each direction).
func (graph *CallGraph) FilterByFunctionNames(names []string) *CallGraph {
	if len(names) == 0 {
		return graph
	}

	// Build set of seed node names
	seeds := make(map[string]bool, len(names))
	for _, name := range names {
		seeds[name] = true
	}

	// Collect seed nodes + immediate neighbors (callers and callees)
	included := make(map[string]bool)
	for _, name := range names {
		if _, exists := graph.Nodes[name]; exists {
			included[name] = true
		}
	}

	// Add neighbors via edges
	for _, edge := range graph.Edges {
		_, fromIsSeed := seeds[edge.From]
		_, toIsSeed := seeds[edge.To]
		if fromIsSeed {
			included[edge.To] = true
			included[edge.From] = true
		}
		if toIsSeed {
			included[edge.From] = true
			included[edge.To] = true
		}
	}

	// Build filtered graph
	filtered := NewCallGraph()
	for fullName := range included {
		if node, exists := graph.Nodes[fullName]; exists {
			nodeCopy := *node
			filtered.Nodes[fullName] = &nodeCopy
		}
	}

	for _, edge := range graph.Edges {
		_, fromExists := filtered.Nodes[edge.From]
		_, toExists := filtered.Nodes[edge.To]
		if fromExists && toExists {
			filtered.Edges = append(filtered.Edges, edge)
		}
	}

	filtered.CalculateStats()
	return filtered
}

// FilterByMinCalls returns a new CallGraph with only nodes that have at least minCalls
func (graph *CallGraph) FilterByMinCalls(minCalls int) *CallGraph {
	if minCalls <= 0 {
		return graph
	}

	filtered := NewCallGraph()

	// Copy nodes that meet the threshold
	for fullName, node := range graph.Nodes {
		if node.CallCount >= minCalls || node.CallsOut >= minCalls {
			nodeCopy := *node
			filtered.Nodes[fullName] = &nodeCopy
		}
	}

	// Copy edges where both endpoints are in the filtered set
	for _, edge := range graph.Edges {
		_, fromExists := filtered.Nodes[edge.From]
		_, toExists := filtered.Nodes[edge.To]
		if fromExists && toExists {
			filtered.Edges = append(filtered.Edges, edge)
		}
	}

	filtered.CalculateStats()
	return filtered
}

// CalculateStats computes summary statistics
func (graph *CallGraph) CalculateStats() {
	graph.Stats.TotalFunctions = len(graph.Nodes)
	graph.Stats.TotalEdges = len(graph.Edges)

	totalCalls := 0
	maxFanIn := 0
	maxFanOut := 0
	mostCalledFunc := ""
	unreachable := 0

	for _, node := range graph.Nodes {
		totalCalls += node.CallCount

		if node.CallCount > maxFanIn {
			maxFanIn = node.CallCount
			mostCalledFunc = node.FullName
		}

		if node.CallsOut > maxFanOut {
			maxFanOut = node.CallsOut
		}

		if node.CallCount == 0 && !node.IsExternal {
			unreachable++
		}
	}

	graph.Stats.TotalCalls = totalCalls
	graph.Stats.MaxFanIn = maxFanIn
	graph.Stats.MaxFanOut = maxFanOut
	graph.Stats.MostCalledFunc = mostCalledFunc
	graph.Stats.UnreachableFuncs = unreachable

	if len(graph.Nodes) > 0 {
		graph.Stats.AvgCallsPerFunc = float64(totalCalls) / float64(len(graph.Nodes))
	}
}

// Callers returns the edges whose callee is fullName (who calls this function).
// The result is a copy and safe to modify; the order follows CallGraph.Edges.
func (graph *CallGraph) Callers(fullName string) []CallEdge {
	index := graph.currentEdgeIndex()
	return graph.edgesAt(index.incomingByTo[fullName])
}

// Callees returns the edges whose caller is fullName (what this function calls).
// The result is a copy and safe to modify; the order follows CallGraph.Edges.
func (graph *CallGraph) Callees(fullName string) []CallEdge {
	index := graph.currentEdgeIndex()
	return graph.edgesAt(index.outgoingFrom[fullName])
}

// currentEdgeIndex returns the edge index, rebuilding it if edges were added since it was built
func (graph *CallGraph) currentEdgeIndex() *callEdgeIndex {
	graph.indexMutex.Lock()
	defer graph.indexMutex.Unlock()

	if graph.edgeIndex != nil && graph.edgeIndex.edgeCount == len(graph.Edges) {
		return graph.edgeIndex
	}

	index := &callEdgeIndex{
		edgeCount:    len(graph.Edges),
		incomingByTo: make(map[string][]int),
		outgoingFrom: make(map[string][]int),
	}
	for position, edge := range graph.Edges {
		index.incomingByTo[edge.To] = append(index.incomingByTo[edge.To], position)
		index.outgoingFrom[edge.From] = append(index.outgoingFrom[edge.From], position)
	}

	graph.edgeIndex = index
	return index
}

// edgesAt copies the edges at the given positions
func (graph *CallGraph) edgesAt(positions []int) []CallEdge {
	if len(positions) == 0 {
		return nil
	}

	edges := make([]CallEdge, 0, len(positions))
	for _, position := range positions {
		edges = append(edges, graph.Edges[position])
	}
	return edges
}

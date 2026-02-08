package models

// CallGraph represents the function call relationships in a codebase
type CallGraph struct {
	Nodes map[string]*CallNode `json:"nodes"` // Key is function full name
	Edges []CallEdge           `json:"edges"`
	Stats CallGraphStats       `json:"stats"`
}

// CallNode represents a function in the call graph
type CallNode struct {
	Name       string  `json:"name"`        // Function name
	FullName   string  `json:"full_name"`   // Package.Function
	Package    string  `json:"package"`     // Package path
	File       string  `json:"file"`        // File path
	Line       int     `json:"line"`        // Starting line
	Complexity int     `json:"complexity"`  // Cyclomatic complexity
	Length     int     `json:"length"`      // Function length
	CallCount  int     `json:"call_count"`  // How many times it's called (fan-in)
	CallsOut   int     `json:"calls_out"`   // How many functions it calls (fan-out)
	IsExternal bool    `json:"is_external"` // Is from external package
	IsExported bool    `json:"is_exported"` // Is exported (starts with capital)
}

// CallEdge represents a function call relationship
type CallEdge struct {
	From   string `json:"from"`   // Caller full name
	To     string `json:"to"`     // Callee full name
	Weight int    `json:"weight"` // Number of call sites
	File   string `json:"file"`   // Where the call happens
	Line   int    `json:"line"`   // Line number of call
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

// AddEdge adds a call edge to the graph
func (graph *CallGraph) AddEdge(edge CallEdge) {
	// Increment call counts
	if callee, exists := graph.Nodes[edge.To]; exists {
		callee.CallCount++
	}

	if caller, exists := graph.Nodes[edge.From]; exists {
		caller.CallsOut++
	}

	// Check if edge already exists and increment weight
	for index := range graph.Edges {
		if graph.Edges[index].From == edge.From && graph.Edges[index].To == edge.To {
			graph.Edges[index].Weight++
			return
		}
	}

	// Add new edge
	edge.Weight = 1
	graph.Edges = append(graph.Edges, edge)
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

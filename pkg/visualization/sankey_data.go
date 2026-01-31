package visualization

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/alexcollie/kaizen/pkg/ownership"
)

// SankeyData represents the complete data for a Sankey diagram
type SankeyData struct {
	Nodes []SankeyNode `json:"nodes"`
	Links []SankeyLink `json:"links"`
	Stats SankeyStats  `json:"stats"`
}

// SankeyNode represents an owner or function
type SankeyNode struct {
	ID      int                    `json:"id"`
	Name    string                 `json:"name"`
	Type    string                 `json:"type"` // "owner" or "function"
	Value   int                    `json:"value"` // Total calls or files
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// SankeyLink represents calls from owner to function
type SankeyLink struct {
	Source int    `json:"source"` // Node ID
	Target int    `json:"target"` // Node ID
	Value  int    `json:"value"`  // Number of calls
	Color  string `json:"color,omitempty"`
}

// SankeyStats provides summary statistics
type SankeyStats struct {
	TotalOwners          int     `json:"total_owners"`
	TotalCommonFunctions int     `json:"total_common_functions"`
	TotalLinks           int     `json:"total_links"`
	AvgCallsPerFunction  float64 `json:"avg_calls_per_function"`
	MaxCallsToFunction   int     `json:"max_calls_to_function"`
	MostSharedFunction   string  `json:"most_shared_function"`
}

// BuildSankeyData aggregates owner-to-function call data
func BuildSankeyData(
	result *models.AnalysisResult,
	ownerReport *ownership.OwnerReport,
	callGraph *models.CallGraph,
	minOwners int,
	minCalls int,
) (*SankeyData, error) {
	if result == nil || ownerReport == nil || callGraph == nil {
		return nil, fmt.Errorf("invalid input: nil parameters")
	}

	// Step 1: Use the FileOwnershipMap directly (it's already file → owners)
	fileToOwners := ownerReport.FileOwnershipMap

	// Step 2: Build file → functions mapping
	fileToFunctions := make(map[string][]string)
	for _, fileAnalysis := range result.Files {
		for _, fn := range fileAnalysis.Functions {
			funcFullName := fmt.Sprintf("%s::%s", fileAnalysis.Path, fn.Name)
			fileToFunctions[fileAnalysis.Path] = append(fileToFunctions[fileAnalysis.Path], funcFullName)
		}
	}

	// Normalize file paths for matching (call graph uses absolute, ownership uses relative)
	fileToOwnersNormalized := fileToOwners

	// Step 3: Aggregate owner → function call counts
	// ownerFunctionCalls[owner][function] = call_count
	ownerFunctionCalls := make(map[string]map[string]int)
	functionOwners := make(map[string]map[string]bool) // track which owners call each function

	for _, edge := range callGraph.Edges {
		// Get the caller and callee nodes
		callerNode, callerExists := callGraph.Nodes[edge.From]
		calleeNode, calleeExists := callGraph.Nodes[edge.To]

		if !callerExists || !calleeExists {
			continue
		}

		// Get owners of the caller's file
		// Try exact match first, then try to match by suffix (relative path)
		callerOwners, ok := fileToOwnersNormalized[callerNode.File]
		if !ok || len(callerOwners) == 0 {
			// Try matching by suffix (e.g., "/full/path/pkg/file.go" matches "pkg/file.go")
			for ownerFile, owners := range fileToOwnersNormalized {
				if strings.HasSuffix(callerNode.File, ownerFile) {
					callerOwners = owners
					ok = true
					break
				}
			}
			if !ok || len(callerOwners) == 0 {
				continue // Skip if no owner assigned
			}
		}

		// Build callee function full name
		calleeFunctionName := calleeNode.FullName

		// Increment call count for each owner
		for _, owner := range callerOwners {
			if ownerFunctionCalls[owner] == nil {
				ownerFunctionCalls[owner] = make(map[string]int)
			}
			ownerFunctionCalls[owner][calleeFunctionName]++

			// Track that this owner calls this function
			if functionOwners[calleeFunctionName] == nil {
				functionOwners[calleeFunctionName] = make(map[string]bool)
			}
			functionOwners[calleeFunctionName][owner] = true
		}
	}

	// Step 4: Filter common functions (called by >= minOwners AND >= minCalls)
	commonFunctions := make(map[string]bool)
	for funcName, owners := range functionOwners {
		// Check owner count threshold
		if len(owners) < minOwners {
			continue
		}

		// Calculate total calls to this function
		totalCalls := 0
		for _, ownerCalls := range ownerFunctionCalls {
			if count, ok := ownerCalls[funcName]; ok {
				totalCalls += count
			}
		}

		// Check call count threshold
		if totalCalls >= minCalls {
			commonFunctions[funcName] = true
		}
	}

	if len(commonFunctions) == 0 {
		return nil, fmt.Errorf("no common functions found with min-owners=%d and min-calls=%d (try lowering thresholds)", minOwners, minCalls)
	}

	// Step 5: Build Sankey nodes
	nodes := []SankeyNode{}
	nodeIDMap := make(map[string]int)
	nextNodeID := 0

	// Add owner nodes (left side)
	ownerList := make([]string, 0, len(ownerFunctionCalls))
	for owner := range ownerFunctionCalls {
		ownerList = append(ownerList, owner)
	}
	sort.Strings(ownerList)

	for _, owner := range ownerList {
		// Calculate total calls from this owner
		totalCalls := 0
		for funcName, count := range ownerFunctionCalls[owner] {
			if commonFunctions[funcName] {
				totalCalls += count
			}
		}

		if totalCalls == 0 {
			continue // Skip owners with no calls to common functions
		}

		// Find owner metrics
		var ownerMetrics *ownership.OwnerMetrics
		for idx := range ownerReport.OwnerMetrics {
			if ownerReport.OwnerMetrics[idx].Owner == owner {
				ownerMetrics = &ownerReport.OwnerMetrics[idx]
				break
			}
		}

		metrics := map[string]interface{}{}
		if ownerMetrics != nil {
			metrics["files"] = ownerMetrics.FileCount
			metrics["functions"] = ownerMetrics.FunctionCount
			metrics["health_score"] = ownerMetrics.OverallHealthScore
			metrics["complexity_avg"] = ownerMetrics.AvgCyclomaticComplexity
		}

		node := SankeyNode{
			ID:      nextNodeID,
			Name:    owner,
			Type:    "owner",
			Value:   totalCalls,
			Metrics: metrics,
		}
		nodes = append(nodes, node)
		nodeIDMap[owner] = nextNodeID
		nextNodeID++
	}

	// Add function nodes (right side)
	funcList := make([]string, 0, len(commonFunctions))
	for funcName := range commonFunctions {
		funcList = append(funcList, funcName)
	}
	sort.Strings(funcList)

	// functionMetricsCache[functionFullName] = metrics
	functionMetricsCache := make(map[string]map[string]interface{})

	for _, funcName := range funcList {
		// Calculate total calls to this function
		totalCalls := 0
		for _, ownerCalls := range ownerFunctionCalls {
			if count, ok := ownerCalls[funcName]; ok {
				totalCalls += count
			}
		}

		// Extract function metrics from analysis result
		metrics := extractFunctionMetrics(result, funcName)
		functionMetricsCache[funcName] = metrics

		node := SankeyNode{
			ID:      nextNodeID,
			Name:    funcName,
			Type:    "function",
			Value:   totalCalls,
			Metrics: metrics,
		}
		nodes = append(nodes, node)
		nodeIDMap[funcName] = nextNodeID
		nextNodeID++
	}

	// Step 6: Build Sankey links
	links := []SankeyLink{}

	for owner, functionCalls := range ownerFunctionCalls {
		ownerNodeID, ok := nodeIDMap[owner]
		if !ok {
			continue
		}

		for funcName, callCount := range functionCalls {
			if !commonFunctions[funcName] {
				continue // Skip non-common functions
			}

			funcNodeID, ok := nodeIDMap[funcName]
			if !ok {
				continue
			}

			link := SankeyLink{
				Source: ownerNodeID,
				Target: funcNodeID,
				Value:  callCount,
			}
			links = append(links, link)
		}
	}

	// Step 7: Calculate statistics
	stats := calculateSankeyStats(nodes, links, functionOwners, commonFunctions)

	return &SankeyData{
		Nodes: nodes,
		Links: links,
		Stats: stats,
	}, nil
}

// extractFunctionMetrics retrieves metrics for a specific function
func extractFunctionMetrics(result *models.AnalysisResult, functionFullName string) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Parse function full name and find in Files for metrics
	for _, fileAnalysis := range result.Files {
		for _, fn := range fileAnalysis.Functions {
			// Match by function name within the file
			if functionFullName == fmt.Sprintf("%s::%s", fileAnalysis.Path, fn.Name) ||
				functionFullName == fmt.Sprintf("%s.%s", fileAnalysis.Path, fn.Name) {
				metrics["complexity"] = fn.CyclomaticComplexity
				metrics["cognitive_complexity"] = fn.CognitiveComplexity
				metrics["lines"] = fn.Length
				metrics["maintainability"] = fn.MaintainabilityIndex
				return metrics
			}
		}
	}

	return metrics
}

// calculateSankeyStats computes summary statistics
func calculateSankeyStats(
	nodes []SankeyNode,
	links []SankeyLink,
	functionOwners map[string]map[string]bool,
	commonFunctions map[string]bool,
) SankeyStats {
	ownerCount := 0
	functionCount := 0

	for _, node := range nodes {
		if node.Type == "owner" {
			ownerCount++
		} else if node.Type == "function" {
			functionCount++
		}
	}

	totalCalls := 0
	for _, link := range links {
		totalCalls += link.Value
	}

	avgCallsPerFunction := 0.0
	if functionCount > 0 {
		avgCallsPerFunction = float64(totalCalls) / float64(functionCount)
	}

	// Find most shared function (called by most owners)
	maxOwners := 0
	mostSharedFunction := ""
	maxCalls := 0

	for funcName := range commonFunctions {
		ownerCount := len(functionOwners[funcName])
		if ownerCount > maxOwners {
			maxOwners = ownerCount
			mostSharedFunction = funcName
		}

		// Also track max calls
		funcCalls := 0
		for _, link := range links {
			if nodes[link.Target].Name == funcName {
				funcCalls += link.Value
			}
		}
		if funcCalls > maxCalls {
			maxCalls = funcCalls
		}
	}

	return SankeyStats{
		TotalOwners:          ownerCount,
		TotalCommonFunctions: functionCount,
		TotalLinks:           len(links),
		AvgCallsPerFunction:  avgCallsPerFunction,
		MaxCallsToFunction:   maxCalls,
		MostSharedFunction:   mostSharedFunction,
	}
}

package visualization

import (
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
)

// HTMLVisualizer generates interactive HTML heat maps
type HTMLVisualizer struct{}

// NewHTMLVisualizer creates a new HTML visualizer
func NewHTMLVisualizer() *HTMLVisualizer {
	return &HTMLVisualizer{}
}

// TreeNode represents a node in the treemap hierarchy
type TreeNode struct {
	Name     string      `json:"name"`
	Value    int         `json:"value,omitempty"`
	Children []TreeNode  `json:"children,omitempty"`
	Metrics  TreeMetrics `json:"metrics,omitempty"`
}

// TreeMetrics contains all metric scores for a folder/file
type TreeMetrics struct {
	ComplexityScore      float64 `json:"complexity_score"`
	ChurnScore           float64 `json:"churn_score"`
	HotspotScore         float64 `json:"hotspot_score"`
	LengthScore          float64 `json:"length_score"`
	MaintainabilityScore float64 `json:"maintainability_score"`
	CognitiveScore       float64 `json:"cognitive_score"`
	TotalFunctions       int     `json:"total_functions"`
	HotspotCount         int     `json:"hotspot_count"`
}

// GenerateHTML creates an interactive HTML heat map with Nordic warm color scheme
func (visualizer *HTMLVisualizer) GenerateHTML(result *models.AnalysisResult) (string, error) {
	// Build tree data structure
	treeData := visualizer.buildTreeData(result)

	// Convert to JSON
	jsonData, err := json.Marshal(treeData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tree data: %w", err)
	}

	// Convert score report to JSON if available
	var scoreReportJSON []byte
	var scoreReportMap map[string]interface{}
	if result.ScoreReport != nil {
		scoreReportJSON, err = json.Marshal(result.ScoreReport)
		if err != nil {
			return "", fmt.Errorf("failed to marshal score report: %w", err)
		}

		// Also create a map for easier template access
		_ = json.Unmarshal(scoreReportJSON, &scoreReportMap)
	}

	// Render HTML template using Nordic theme
	tmpl := template.Must(template.New("heatmap").Parse(htmlNordicTemplate))

	templateData := map[string]interface{}{
		"TreeData":        template.JS(jsonData),
		"Summary":         result.Summary,
		"HasScoreReport":  result.ScoreReport != nil,
		"ScoreReportJSON": template.JS(scoreReportJSON),
		"Repository":      result.Repository,
	}

	// Add score report fields for template access
	if result.ScoreReport != nil {
		templateData["OverallGrade"] = result.ScoreReport.OverallGrade
		templateData["OverallScore"] = result.ScoreReport.OverallScore
		templateData["ScoreReportMap"] = scoreReportMap
	}

	var builder strings.Builder
	err = tmpl.Execute(&builder, templateData)

	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return builder.String(), nil
}

// buildTreeData converts analysis results to a proper hierarchical tree structure
func (visualizer *HTMLVisualizer) buildTreeData(result *models.AnalysisResult) TreeNode {
	// Find leaf folders (folders that don't have children in the stats)
	leafFolders := findLeafFolders(result.FolderStats)

	// Build tree from leaf folders
	root := TreeNode{
		Name:     getShortName(result.Repository),
		Children: []TreeNode{},
	}

	// Create a map for quick node lookup during tree building
	nodeMap := make(map[string]*TreeNode)
	nodeMap[""] = &root

	// Sort paths to ensure parents are processed before children
	paths := make([]string, 0, len(leafFolders))
	for path := range leafFolders {
		paths = append(paths, path)
	}
	sortPaths(paths)

	// Build the tree structure
	for _, path := range paths {
		folder := leafFolders[path]
		parts := strings.Split(path, "/")

		// Ensure all parent nodes exist
		currentPath := ""
		for idx, part := range parts {
			parentPath := currentPath
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "/" + part
			}

			if _, exists := nodeMap[currentPath]; !exists {
				// Create new node
				newNode := &TreeNode{
					Name:     part,
					Children: []TreeNode{},
				}

				// If this is the leaf node, add metrics
				if idx == len(parts)-1 {
					newNode.Value = folder.TotalCodeLines
					newNode.Metrics = TreeMetrics{
						ComplexityScore:      folder.ComplexityScore,
						ChurnScore:           folder.ChurnScore,
						HotspotScore:         folder.HotspotScore,
						LengthScore:          folder.LengthScore,
						MaintainabilityScore: folder.MaintainabilityScore,
						CognitiveScore:       folder.ComplexityScore,
						TotalFunctions:       folder.TotalFunctions,
						HotspotCount:         folder.HotspotCount,
					}
				}

				nodeMap[currentPath] = newNode

				// Add to parent
				if parent, ok := nodeMap[parentPath]; ok {
					parent.Children = append(parent.Children, *newNode)
					// Update the map reference to point to the child in the slice
					nodeMap[currentPath] = &parent.Children[len(parent.Children)-1]
				}
			}
		}
	}

	// Collapse single-child intermediate nodes for cleaner visualization
	root = collapseSingleChildren(root)

	return root
}

// findLeafFolders returns only the most specific folders (those without children)
func findLeafFolders(folderStats map[string]models.FolderMetrics) map[string]models.FolderMetrics {
	leafFolders := make(map[string]models.FolderMetrics)

	for path, folder := range folderStats {
		isLeaf := true
		for otherPath := range folderStats {
			if otherPath != path && strings.HasPrefix(otherPath, path+"/") {
				isLeaf = false
				break
			}
		}
		if isLeaf {
			leafFolders[path] = folder
		}
	}

	return leafFolders
}

// sortPaths sorts paths so shorter paths come first (parents before children)
func sortPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		// Sort by depth first, then alphabetically
		depthI := strings.Count(paths[i], "/")
		depthJ := strings.Count(paths[j], "/")
		if depthI != depthJ {
			return depthI < depthJ
		}
		return paths[i] < paths[j]
	})
}

// collapseSingleChildren merges nodes that have only one child
func collapseSingleChildren(node TreeNode) TreeNode {
	// Recursively collapse children first
	for idx := range node.Children {
		node.Children[idx] = collapseSingleChildren(node.Children[idx])
	}

	// If this node has exactly one child and no value of its own, merge with child
	if len(node.Children) == 1 && node.Value == 0 {
		child := node.Children[0]
		return TreeNode{
			Name:     node.Name + "/" + child.Name,
			Value:    child.Value,
			Children: child.Children,
			Metrics:  child.Metrics,
		}
	}

	return node
}

// getShortName extracts the last component of a path
func getShortName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

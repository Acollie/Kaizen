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

// GenerateHTML creates an interactive HTML heat map
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
	if result.ScoreReport != nil {
		scoreReportJSON, err = json.Marshal(result.ScoreReport)
		if err != nil {
			return "", fmt.Errorf("failed to marshal score report: %w", err)
		}
	}

	// Render HTML template
	tmpl := template.Must(template.New("heatmap").Parse(htmlTemplate))

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

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Kaizen Code Heat Map</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            padding: 20px;
        }

        .header {
            text-align: center;
            margin-bottom: 30px;
        }

        h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .grade-and-stats {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 60px;
            margin-bottom: 20px;
            flex-wrap: wrap;
        }

        .grade-circle {
            width: 120px;
            height: 120px;
            border-radius: 50%;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
        }

        .grade-circle.grade-A { background: linear-gradient(135deg, #22c55e 0%, #16a34a 100%); }
        .grade-circle.grade-B { background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%); }
        .grade-circle.grade-C { background: linear-gradient(135deg, #eab308 0%, #ca8a04 100%); }
        .grade-circle.grade-D { background: linear-gradient(135deg, #f97316 0%, #ea580c 100%); }
        .grade-circle.grade-F { background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%); }

        .grade-letter {
            font-size: 3em;
            font-weight: bold;
            color: white;
            line-height: 1;
        }

        .grade-score {
            font-size: 1em;
            color: rgba(255, 255, 255, 0.9);
            margin-top: 4px;
        }

        .summary {
            display: flex;
            justify-content: center;
            gap: 40px;
            flex-wrap: wrap;
        }

        .stat {
            text-align: center;
        }

        .stat-value {
            font-size: 2em;
            font-weight: bold;
            color: #667eea;
        }

        .stat-label {
            font-size: 0.9em;
            color: #999;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .component-scores {
            display: flex;
            justify-content: center;
            gap: 20px;
            margin: 20px 0;
            flex-wrap: wrap;
        }

        .component-score {
            background: #2a2a2a;
            padding: 12px 20px;
            border-radius: 8px;
            text-align: center;
            min-width: 140px;
        }

        .component-name {
            font-size: 0.85em;
            color: #999;
            margin-bottom: 8px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .component-bar {
            height: 8px;
            background: #3a3a3a;
            border-radius: 4px;
            overflow: hidden;
            margin-bottom: 4px;
        }

        .component-bar-fill {
            height: 100%;
            border-radius: 4px;
            transition: width 0.5s ease;
        }

        .component-bar-fill.excellent { background: linear-gradient(90deg, #22c55e, #16a34a); }
        .component-bar-fill.good { background: linear-gradient(90deg, #3b82f6, #2563eb); }
        .component-bar-fill.moderate { background: linear-gradient(90deg, #eab308, #ca8a04); }
        .component-bar-fill.poor { background: linear-gradient(90deg, #f97316, #ea580c); }
        .component-bar-fill.critical { background: linear-gradient(90deg, #ef4444, #dc2626); }

        .component-value {
            font-size: 0.9em;
            color: #e0e0e0;
        }

        .controls {
            text-align: center;
            margin-bottom: 30px;
        }

        .metric-selector {
            display: inline-flex;
            gap: 10px;
            background: #2a2a2a;
            padding: 10px;
            border-radius: 12px;
            flex-wrap: wrap;
            justify-content: center;
        }

        .metric-btn {
            padding: 12px 24px;
            border: none;
            background: #3a3a3a;
            color: #e0e0e0;
            border-radius: 8px;
            cursor: pointer;
            font-size: 0.95em;
            font-weight: 500;
            transition: all 0.3s ease;
        }

        .metric-btn:hover {
            background: #4a4a4a;
            transform: translateY(-2px);
        }

        .metric-btn.active {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }

        #breadcrumb {
            margin-bottom: 10px;
            padding: 10px 15px;
            background: #2a2a2a;
            border-radius: 8px;
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 14px;
            min-height: 44px;
        }

        .breadcrumb-item {
            color: #667eea;
            cursor: pointer;
            padding: 4px 8px;
            border-radius: 4px;
            transition: background 0.2s;
        }

        .breadcrumb-item:hover {
            background: #3a3a3a;
            text-decoration: underline;
        }

        .breadcrumb-separator {
            color: #666;
        }

        .breadcrumb-current {
            color: #e0e0e0;
            font-weight: 500;
        }

        .breadcrumb-hint {
            color: #666;
            font-size: 12px;
            margin-left: auto;
        }

        #treemap {
            width: 100%;
            height: 700px;
            background: #1a1a1a;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
        }

        .group-bg {
            pointer-events: none;
        }

        .group-label {
            pointer-events: none;
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }

        .cell-group {
            cursor: pointer;
        }

        .cell {
            transition: all 0.2s ease;
        }

        .cell-group:hover .cell {
            filter: brightness(1.1);
        }

        .cell-label {
            font-size: 13px;
            font-weight: 600;
            pointer-events: none;
            fill: white;
            text-shadow: 1px 1px 3px rgba(0, 0, 0, 0.9);
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }

        .cell-score {
            pointer-events: none;
        }

        .tooltip {
            position: absolute;
            padding: 12px 16px;
            background: rgba(0, 0, 0, 0.95);
            color: white;
            border-radius: 8px;
            pointer-events: none;
            font-size: 14px;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
            z-index: 1000;
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .tooltip-title {
            font-weight: bold;
            margin-bottom: 8px;
            font-size: 15px;
            color: #667eea;
        }

        .tooltip-row {
            display: flex;
            justify-content: space-between;
            gap: 20px;
            margin: 4px 0;
        }

        .tooltip-label {
            color: #999;
        }

        .tooltip-value {
            font-weight: 500;
        }

        .concerns-panel {
            margin-top: 30px;
            background: #2a2a2a;
            border-radius: 12px;
            overflow: hidden;
        }

        .concerns-header {
            padding: 15px 20px;
            background: #3a3a3a;
            cursor: pointer;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .concerns-header:hover {
            background: #4a4a4a;
        }

        .concerns-title {
            font-weight: 600;
            font-size: 1.1em;
        }

        .concerns-toggle {
            font-size: 1.2em;
            transition: transform 0.3s ease;
        }

        .concerns-toggle.collapsed {
            transform: rotate(-90deg);
        }

        .concerns-content {
            padding: 20px;
            display: none;
        }

        .concerns-content.expanded {
            display: block;
        }

        .concern-item {
            padding: 15px;
            margin-bottom: 15px;
            border-radius: 8px;
            background: #1a1a1a;
            border-left: 4px solid;
        }

        .concern-item.critical {
            border-left-color: #ef4444;
            background: linear-gradient(90deg, rgba(239, 68, 68, 0.1) 0%, #1a1a1a 100%);
        }
        .concern-item.warning {
            border-left-color: #eab308;
            background: linear-gradient(90deg, rgba(234, 179, 8, 0.1) 0%, #1a1a1a 100%);
        }
        .concern-item.info {
            border-left-color: #3b82f6;
            background: linear-gradient(90deg, rgba(59, 130, 246, 0.1) 0%, #1a1a1a 100%);
        }

        .concern-severity {
            font-size: 0.75em;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 5px;
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
        }

        .concern-severity.critical { color: #fff; background: #ef4444; }
        .concern-severity.warning { color: #000; background: #eab308; }
        .concern-severity.info { color: #fff; background: #3b82f6; }

        .concern-title-text {
            font-weight: 600;
            margin-bottom: 5px;
            font-size: 1.1em;
        }

        .concern-description {
            color: #999;
            font-size: 0.9em;
            margin-bottom: 12px;
        }

        .concern-items {
            font-size: 0.85em;
        }

        .concern-file {
            padding: 8px 12px;
            margin: 4px 0;
            background: #252525;
            border-radius: 6px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            gap: 8px;
        }

        .concern-file-link {
            color: #667eea;
            text-decoration: none;
            font-family: 'SF Mono', 'Consolas', monospace;
            font-size: 0.9em;
        }

        .concern-file-link:hover {
            color: #8b9ef8;
            text-decoration: underline;
        }

        .concern-file-func {
            color: #22c55e;
            font-weight: 500;
        }

        .concern-file-metrics {
            display: flex;
            gap: 12px;
            color: #999;
            font-size: 0.85em;
        }

        .concern-metric {
            display: flex;
            align-items: center;
            gap: 4px;
        }

        .concern-metric-value {
            color: #e0e0e0;
            font-weight: 500;
        }

        .concern-metric-value.high {
            color: #ef4444;
        }

        .concern-metric-value.medium {
            color: #eab308;
        }

        .no-concerns {
            text-align: center;
            padding: 30px;
            color: #22c55e;
            font-size: 1.1em;
        }

        .concerns-summary {
            display: flex;
            gap: 15px;
            margin-bottom: 15px;
            flex-wrap: wrap;
        }

        .concerns-summary-item {
            display: flex;
            align-items: center;
            gap: 6px;
            padding: 6px 12px;
            border-radius: 6px;
            background: #252525;
            font-size: 0.9em;
        }

        .concerns-summary-item.critical { border: 1px solid #ef4444; }
        .concerns-summary-item.warning { border: 1px solid #eab308; }
        .concerns-summary-item.info { border: 1px solid #3b82f6; }

        .legend {
            text-align: center;
            margin-top: 20px;
            padding: 15px;
            background: #2a2a2a;
            border-radius: 8px;
        }

        .legend-title {
            font-weight: 500;
            margin-bottom: 10px;
            color: #999;
        }

        .legend-gradient {
            display: inline-block;
            width: 300px;
            height: 20px;
            background: linear-gradient(to right, #22c55e, #eab308, #ef4444);
            border-radius: 4px;
            margin: 0 10px;
        }

        .legend-labels {
            display: flex;
            justify-content: space-between;
            width: 300px;
            margin: 5px auto;
            font-size: 0.85em;
            color: #999;
        }

        .footer {
            text-align: center;
            margin-top: 30px;
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🗺️ Kaizen Code Heat Map</h1>

        <div class="grade-and-stats">
            {{if .HasScoreReport}}
            <div class="grade-circle grade-{{.OverallGrade}}">
                <div class="grade-letter">{{.OverallGrade}}</div>
                <div class="grade-score">{{printf "%.0f" .OverallScore}}/100</div>
            </div>
            {{end}}

            <div class="summary">
                <div class="stat">
                    <div class="stat-value">{{.Summary.TotalFiles}}</div>
                    <div class="stat-label">Files</div>
                </div>
                <div class="stat">
                    <div class="stat-value">{{.Summary.TotalFunctions}}</div>
                    <div class="stat-label">Functions</div>
                </div>
                <div class="stat">
                    <div class="stat-value">{{printf "%.1f" .Summary.AverageCyclomaticComplexity}}</div>
                    <div class="stat-label">Avg Complexity</div>
                </div>
                <div class="stat">
                    <div class="stat-value">{{printf "%.1f" .Summary.AverageMaintainabilityIndex}}</div>
                    <div class="stat-label">Maintainability</div>
                </div>
                <div class="stat">
                    <div class="stat-value">{{.Summary.HotspotCount}}</div>
                    <div class="stat-label">Hotspots 🔥</div>
                </div>
            </div>
        </div>

        {{if .HasScoreReport}}
        <div class="component-scores" id="component-scores"></div>
        {{end}}
    </div>

    <div class="controls">
        <div class="metric-selector">
            <button class="metric-btn active" data-metric="hotspot">🔥 Hotspot</button>
            <button class="metric-btn" data-metric="complexity">🔀 Complexity</button>
            <button class="metric-btn" data-metric="churn">📈 Churn</button>
            <button class="metric-btn" data-metric="length">📏 Length</button>
            <button class="metric-btn" data-metric="maintainability">🔧 Maintainability</button>
        </div>
    </div>

    <div id="breadcrumb"></div>
    <div id="treemap"></div>

    <div class="legend">
        <div class="legend-title">Color Scale</div>
        <div class="legend-gradient"></div>
        <div class="legend-labels">
            <span>Low (Good)</span>
            <span>Medium</span>
            <span>High (Needs Attention)</span>
        </div>
    </div>

    {{if .HasScoreReport}}
    <div class="concerns-panel" id="concerns-panel"></div>
    {{end}}

    <div class="footer">
        Generated by Kaizen Code Analysis Tool
    </div>

    <script>
        // Data
        const data = {{.TreeData}};
        const scoreReport = {{.ScoreReportJSON}};
        const hasScoreReport = {{.HasScoreReport}};
        const repositoryPath = "{{.Repository}}";

        // Current metric
        let currentMetric = 'hotspot';

        // Dimensions
        const width = document.getElementById('treemap').clientWidth;
        const height = 700;

        // Color scale
        const colorScale = d3.scaleSequential()
            .domain([0, 100])
            .interpolator(t => {
                if (t < 0.33) return d3.interpolateRgb('#22c55e', '#eab308')(t * 3);
                if (t < 0.67) return d3.interpolateRgb('#eab308', '#ef4444')((t - 0.33) * 3);
                return d3.interpolateRgb('#ef4444', '#dc2626')((t - 0.67) * 3);
            });

        // Create SVG
        const svg = d3.select('#treemap')
            .append('svg')
            .attr('width', width)
            .attr('height', height);

        // Tooltip
        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'tooltip')
            .style('opacity', 0);

        // Treemap layout with hierarchical padding
        const treemap = d3.treemap()
            .size([width, height])
            .paddingOuter(3)
            .paddingTop(22)  // Room for group labels
            .paddingInner(2)
            .round(true);

        // Current zoom state
        let currentRoot = null;
        let fullRoot = null;

        // Function to get metric value
        function getMetricValue(node, metric) {
            if (!node.data.metrics) return 0;

            switch(metric) {
                case 'complexity': return node.data.metrics.complexity_score;
                case 'churn': return node.data.metrics.churn_score;
                case 'hotspot': return node.data.metrics.hotspot_score;
                case 'length': return node.data.metrics.length_score;
                case 'maintainability': return 100 - node.data.metrics.maintainability_score;
                default: return node.data.metrics.hotspot_score;
            }
        }

        // Get ancestors of a node (for breadcrumb)
        function getAncestors(node) {
            const ancestors = [];
            let current = node;
            while (current) {
                ancestors.unshift(current);
                current = current.parent;
            }
            return ancestors;
        }

        // Get full path for a node
        function getNodePath(node) {
            const path = [];
            let current = node;
            while (current.parent) {
                path.unshift(current.data.name);
                current = current.parent;
            }
            return path.join('/');
        }

        // Update breadcrumb navigation
        function updateBreadcrumb(node) {
            const breadcrumb = document.getElementById('breadcrumb');
            const ancestors = getAncestors(node);

            let html = '';

            ancestors.forEach((ancestor, index) => {
                if (index > 0) {
                    html += '<span class="breadcrumb-separator">›</span>';
                }

                if (index === ancestors.length - 1) {
                    html += '<span class="breadcrumb-current">' + ancestor.data.name + '</span>';
                } else {
                    html += '<span class="breadcrumb-item" data-depth="' + index + '">' + ancestor.data.name + '</span>';
                }
            });

            // Add hint if we can zoom
            if (node.children) {
                html += '<span class="breadcrumb-hint">Click a section to zoom in</span>';
            } else if (node.parent) {
                html += '<span class="breadcrumb-hint">Click path above to zoom out</span>';
            }

            breadcrumb.innerHTML = html;

            // Add click handlers for breadcrumb items
            breadcrumb.querySelectorAll('.breadcrumb-item').forEach(item => {
                item.addEventListener('click', function() {
                    const depth = parseInt(this.getAttribute('data-depth'));
                    const targetNode = ancestors[depth];
                    zoomTo(targetNode);
                });
            });
        }

        // Zoom to a specific node
        function zoomTo(node) {
            currentRoot = node;
            renderTreemap(node, currentMetric);
            updateBreadcrumb(node);
        }

        // Function to render treemap for a given root
        function renderTreemap(rootNode, metric) {
            // Clear previous content
            svg.selectAll('*').remove();

            // Clone and recalculate layout for this subtree
            const displayRoot = rootNode.copy();

            // Recalculate the treemap for this subtree
            treemap(displayRoot
                .sum(d => d.value || 0)
                .sort((a, b) => b.value - a.value)
            );

            // Draw group backgrounds and labels for intermediate nodes
            const groups = svg.selectAll('g.group')
                .data(displayRoot.descendants().filter(d => d.children && d.depth < displayRoot.height))
                .enter()
                .append('g')
                .attr('class', 'group');

            // Group backgrounds (clickable for zooming)
            groups.append('rect')
                .attr('class', 'group-bg')
                .attr('x', d => d.x0)
                .attr('y', d => d.y0)
                .attr('width', d => d.x1 - d.x0)
                .attr('height', d => d.y1 - d.y0)
                .attr('fill', '#252525')
                .attr('stroke', '#3a3a3a')
                .attr('stroke-width', 1)
                .style('cursor', d => d.children ? 'pointer' : 'default')
                .on('click', function(event, d) {
                    if (d.children && d.depth > 0) {
                        event.stopPropagation();
                        // Find the corresponding node in the full tree
                        const targetNode = findNodeByPath(fullRoot, getNodePath(d));
                        if (targetNode) {
                            zoomTo(targetNode);
                        }
                    }
                });

            // Group labels (also clickable)
            groups.append('text')
                .attr('class', 'group-label')
                .attr('x', d => d.x0 + 6)
                .attr('y', d => d.y0 + 16)
                .text(d => {
                    const width = d.x1 - d.x0;
                    if (width < 50) return '';
                    return d.data.name;
                })
                .attr('fill', '#888')
                .attr('font-size', '12px')
                .attr('font-weight', '500')
                .style('cursor', d => d.children ? 'pointer' : 'default')
                .style('pointer-events', 'all')
                .on('click', function(event, d) {
                    if (d.children) {
                        event.stopPropagation();
                        const targetNode = findNodeByPath(fullRoot, getNodePath(d));
                        if (targetNode) {
                            zoomTo(targetNode);
                        }
                    }
                });

            // Draw leaf cells (or nodes without children at current zoom level)
            const leaves = displayRoot.leaves();
            const cells = svg.selectAll('g.cell')
                .data(leaves)
                .enter()
                .append('g')
                .attr('class', 'cell-group')
                .attr('transform', d => 'translate(' + d.x0 + ',' + d.y0 + ')');

            cells.append('rect')
                .attr('class', 'cell')
                .attr('width', d => Math.max(0, d.x1 - d.x0))
                .attr('height', d => Math.max(0, d.y1 - d.y0))
                .attr('fill', d => colorScale(getMetricValue(d, metric)))
                .attr('rx', 2)
                .attr('ry', 2);

            // Cell labels
            cells.append('text')
                .attr('class', 'cell-label')
                .attr('x', 5)
                .attr('y', 18)
                .text(d => {
                    const cellWidth = d.x1 - d.x0;
                    const cellHeight = d.y1 - d.y0;
                    if (cellWidth < 50 || cellHeight < 25) return '';
                    return d.data.name;
                });

            // Score badge
            cells.append('text')
                .attr('class', 'cell-score')
                .attr('x', d => (d.x1 - d.x0) - 5)
                .attr('y', 18)
                .attr('text-anchor', 'end')
                .attr('fill', 'rgba(255,255,255,0.7)')
                .attr('font-size', '11px')
                .text(d => {
                    const cellWidth = d.x1 - d.x0;
                    if (cellWidth < 80) return '';
                    return Math.round(getMetricValue(d, metric));
                });

            // Hover events
            cells
                .on('mouseover', function(event, d) {
                    d3.select(this).select('rect')
                        .attr('stroke', '#fff')
                        .attr('stroke-width', 2);

                    tooltip.transition()
                        .duration(200)
                        .style('opacity', 1);

                    const metrics = d.data.metrics || {};
                    const metricValue = getMetricValue(d, metric).toFixed(0);
                    const fullPath = getNodePath(d);

                    tooltip.html(
                        '<div class="tooltip-title">' + fullPath + '</div>' +
                        '<div class="tooltip-row">' +
                            '<span class="tooltip-label">Lines of Code:</span>' +
                            '<span class="tooltip-value">' + (d.value || 0).toLocaleString() + '</span>' +
                        '</div>' +
                        '<div class="tooltip-row">' +
                            '<span class="tooltip-label">Functions:</span>' +
                            '<span class="tooltip-value">' + (metrics.total_functions || 0) + '</span>' +
                        '</div>' +
                        '<div class="tooltip-row">' +
                            '<span class="tooltip-label">' + metric.charAt(0).toUpperCase() + metric.slice(1) + ' Score:</span>' +
                            '<span class="tooltip-value">' + metricValue + '/100</span>' +
                        '</div>' +
                        (metrics.hotspot_count > 0 ?
                            '<div class="tooltip-row">' +
                                '<span class="tooltip-label">🔥 Hotspots:</span>' +
                                '<span class="tooltip-value">' + metrics.hotspot_count + '</span>' +
                            '</div>' : '')
                    )
                    .style('left', (event.pageX + 10) + 'px')
                    .style('top', (event.pageY - 10) + 'px');
                })
                .on('mouseout', function() {
                    d3.select(this).select('rect')
                        .attr('stroke', null)
                        .attr('stroke-width', null);

                    tooltip.transition()
                        .duration(500)
                        .style('opacity', 0);
                });
        }

        // Find node by path in the full hierarchy
        function findNodeByPath(root, path) {
            if (!path || path === root.data.name) return root;

            const parts = path.split('/');
            let current = root;

            for (const part of parts) {
                if (!current.children) return null;
                const child = current.children.find(c => c.data.name === part);
                if (!child) return null;
                current = child;
            }

            return current;
        }

        // Function to update treemap (called when metric changes)
        function updateTreemap(metric) {
            currentMetric = metric;
            renderTreemap(currentRoot, metric);
        }

        // Initialize the full hierarchy
        function initializeTreemap() {
            fullRoot = d3.hierarchy(data)
                .sum(d => d.value || 0)
                .sort((a, b) => b.value - a.value);

            currentRoot = fullRoot;
            renderTreemap(currentRoot, currentMetric);
            updateBreadcrumb(currentRoot);
        }

        // Metric button handlers
        d3.selectAll('.metric-btn').on('click', function() {
            d3.selectAll('.metric-btn').classed('active', false);
            d3.select(this).classed('active', true);

            const metric = this.getAttribute('data-metric');
            updateTreemap(metric);
        });

        // Initialize treemap
        initializeTreemap();

        // Render component scores if available
        if (hasScoreReport && scoreReport) {
            renderComponentScores(scoreReport.component_scores);
            renderConcerns(scoreReport.concerns);
        }

        function renderComponentScores(scores) {
            const container = document.getElementById('component-scores');
            if (!container) return;

            const components = [
                { name: 'Complexity', data: scores.complexity },
                { name: 'Maintainability', data: scores.maintainability },
                { name: 'Churn', data: scores.churn },
                { name: 'Function Size', data: scores.function_size },
                { name: 'Code Structure', data: scores.code_structure }
            ];

            container.innerHTML = components.map(comp => {
                const score = comp.data.score;
                const category = comp.data.category;
                const isChurnNA = comp.name === 'Churn' && !scoreReport.has_churn_data;

                return '<div class="component-score">' +
                    '<div class="component-name">' + comp.name + '</div>' +
                    '<div class="component-bar">' +
                        '<div class="component-bar-fill ' + category + '" style="width: ' + (isChurnNA ? 0 : score) + '%"></div>' +
                    '</div>' +
                    '<div class="component-value">' + (isChurnNA ? 'N/A' : Math.round(score) + '/100') + '</div>' +
                '</div>';
            }).join('');
        }

        function renderConcerns(concerns) {
            const container = document.getElementById('concerns-panel');
            if (!container) return;

            const concernCount = concerns ? concerns.length : 0;
            const headerText = concernCount > 0
                ? '⚠️ Areas of Concern (' + concernCount + ')'
                : '✅ No Concerns Detected';

            // Count by severity
            const severityCounts = { critical: 0, warning: 0, info: 0 };
            if (concerns) {
                concerns.forEach(c => severityCounts[c.severity]++);
            }

            // Start expanded if there are concerns
            const expandedClass = concernCount > 0 ? ' expanded' : '';
            const toggleClass = concernCount > 0 ? '' : ' collapsed';

            let html = '<div class="concerns-header" onclick="toggleConcerns()">' +
                '<span class="concerns-title">' + headerText + '</span>' +
                '<span class="concerns-toggle' + toggleClass + '" id="concerns-toggle">▼</span>' +
            '</div>' +
            '<div class="concerns-content' + expandedClass + '" id="concerns-content">';

            if (concernCount === 0) {
                html += '<div class="no-concerns">✨ Your codebase looks healthy! No issues detected.</div>';
            } else {
                // Add severity summary
                html += '<div class="concerns-summary">';
                if (severityCounts.critical > 0) {
                    html += '<div class="concerns-summary-item critical">🔴 ' + severityCounts.critical + ' Critical</div>';
                }
                if (severityCounts.warning > 0) {
                    html += '<div class="concerns-summary-item warning">🟡 ' + severityCounts.warning + ' Warning</div>';
                }
                if (severityCounts.info > 0) {
                    html += '<div class="concerns-summary-item info">🔵 ' + severityCounts.info + ' Info</div>';
                }
                html += '</div>';

                html += concerns.map(concern => {
                    const itemsHtml = concern.affected_items ? concern.affected_items.map(item => {
                        const filePath = item.file_path;
                        const line = item.line || 1;

                        // Create VS Code link using repository path
                        const fullPath = repositoryPath.startsWith('/')
                            ? repositoryPath + '/' + filePath
                            : filePath;
                        const vscodeUrl = 'vscode://file/' + fullPath + ':' + line;

                        // Build metrics display
                        let metricsHtml = '';
                        if (item.metrics) {
                            const metricParts = [];
                            if (item.metrics.complexity !== undefined) {
                                const level = item.metrics.complexity > 20 ? 'high' : (item.metrics.complexity > 10 ? 'medium' : '');
                                metricParts.push('<span class="concern-metric">CC: <span class="concern-metric-value ' + level + '">' + Math.round(item.metrics.complexity) + '</span></span>');
                            }
                            if (item.metrics.churn !== undefined) {
                                const level = item.metrics.churn > 20 ? 'high' : (item.metrics.churn > 10 ? 'medium' : '');
                                metricParts.push('<span class="concern-metric">Churn: <span class="concern-metric-value ' + level + '">' + Math.round(item.metrics.churn) + '</span></span>');
                            }
                            if (item.metrics.length !== undefined) {
                                const level = item.metrics.length > 100 ? 'high' : (item.metrics.length > 50 ? 'medium' : '');
                                metricParts.push('<span class="concern-metric">Lines: <span class="concern-metric-value ' + level + '">' + Math.round(item.metrics.length) + '</span></span>');
                            }
                            if (item.metrics.maintainability_index !== undefined) {
                                const level = item.metrics.maintainability_index < 20 ? 'high' : (item.metrics.maintainability_index < 40 ? 'medium' : '');
                                metricParts.push('<span class="concern-metric">MI: <span class="concern-metric-value ' + level + '">' + Math.round(item.metrics.maintainability_index) + '</span></span>');
                            }
                            if (item.metrics.nesting_depth !== undefined) {
                                const level = item.metrics.nesting_depth > 7 ? 'high' : (item.metrics.nesting_depth > 5 ? 'medium' : '');
                                metricParts.push('<span class="concern-metric">Nesting: <span class="concern-metric-value ' + level + '">' + Math.round(item.metrics.nesting_depth) + '</span></span>');
                            }
                            if (item.metrics.parameter_count !== undefined) {
                                const level = item.metrics.parameter_count > 10 ? 'high' : (item.metrics.parameter_count > 7 ? 'medium' : '');
                                metricParts.push('<span class="concern-metric">Params: <span class="concern-metric-value ' + level + '">' + Math.round(item.metrics.parameter_count) + '</span></span>');
                            }
                            if (metricParts.length > 0) {
                                metricsHtml = '<div class="concern-file-metrics">' + metricParts.join('') + '</div>';
                            }
                        }

                        const funcName = item.function_name ? '<span class="concern-file-func">' + item.function_name + '</span>' : '';

                        return '<div class="concern-file">' +
                            '<div>' +
                                '<a href="' + vscodeUrl + '" class="concern-file-link" title="Open in VS Code">' + filePath + ':' + line + '</a> ' +
                                funcName +
                            '</div>' +
                            metricsHtml +
                        '</div>';
                    }).join('') : '';

                    return '<div class="concern-item ' + concern.severity + '">' +
                        '<div class="concern-severity ' + concern.severity + '">' + concern.severity.toUpperCase() + '</div>' +
                        '<div class="concern-title-text">' + concern.title + '</div>' +
                        '<div class="concern-description">' + concern.description + '</div>' +
                        '<div class="concern-items">' + itemsHtml + '</div>' +
                    '</div>';
                }).join('');
            }

            html += '</div>';
            container.innerHTML = html;
        }

        function toggleConcerns() {
            const content = document.getElementById('concerns-content');
            const toggle = document.getElementById('concerns-toggle');

            if (content.classList.contains('expanded')) {
                content.classList.remove('expanded');
                toggle.classList.add('collapsed');
            } else {
                content.classList.add('expanded');
                toggle.classList.remove('collapsed');
            }
        }
    </script>
</body>
</html>`

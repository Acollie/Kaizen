package visualization

import (
	"encoding/json"
	"fmt"
	"html/template"
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

	// Render HTML template
	tmpl := template.Must(template.New("heatmap").Parse(htmlTemplate))

	var builder strings.Builder
	err = tmpl.Execute(&builder, map[string]interface{}{
		"TreeData": template.JS(jsonData),
		"Summary":  result.Summary,
	})

	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return builder.String(), nil
}

// buildTreeData converts analysis results to tree structure
func (visualizer *HTMLVisualizer) buildTreeData(result *models.AnalysisResult) TreeNode {
	root := TreeNode{
		Name:     result.Repository,
		Children: []TreeNode{},
	}

	// Group by top-level folders
	folderMap := make(map[string][]models.FolderMetrics)

	for path, folder := range result.FolderStats {
		// Get top-level folder (first part of path)
		parts := strings.Split(path, "/")
		topLevel := parts[0]

		folderMap[topLevel] = append(folderMap[topLevel], folder)
	}

	// Create tree nodes for each top-level folder
	for topLevel, folders := range folderMap {
		node := TreeNode{
			Name:     topLevel,
			Children: []TreeNode{},
		}

		// Add child folders
		for _, folder := range folders {
			childNode := TreeNode{
				Name:  folder.Path,
				Value: folder.TotalCodeLines,
				Metrics: TreeMetrics{
					ComplexityScore:      folder.ComplexityScore,
					ChurnScore:           folder.ChurnScore,
					HotspotScore:         folder.HotspotScore,
					LengthScore:          folder.LengthScore,
					MaintainabilityScore: folder.MaintainabilityScore,
					CognitiveScore:       folder.ComplexityScore, // Use complexity for now
					TotalFunctions:       folder.TotalFunctions,
					HotspotCount:         folder.HotspotCount,
				},
			}
			node.Children = append(node.Children, childNode)
		}

		// If only one child, use its value
		if len(node.Children) == 1 {
			node.Value = node.Children[0].Value
			node.Metrics = node.Children[0].Metrics
		}

		root.Children = append(root.Children, node)
	}

	return root
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

        .summary {
            display: flex;
            justify-content: center;
            gap: 40px;
            margin-bottom: 20px;
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

        #treemap {
            width: 100%;
            height: 700px;
            background: #2a2a2a;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
        }

        .cell {
            stroke: #1a1a1a;
            stroke-width: 2px;
            cursor: pointer;
            transition: opacity 0.2s;
        }

        .cell:hover {
            opacity: 0.8;
            stroke-width: 3px;
        }

        .cell-label {
            font-size: 12px;
            font-weight: 500;
            pointer-events: none;
            fill: white;
            text-shadow: 1px 1px 2px rgba(0, 0, 0, 0.8);
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

    <div class="controls">
        <div class="metric-selector">
            <button class="metric-btn active" data-metric="hotspot">🔥 Hotspot</button>
            <button class="metric-btn" data-metric="complexity">🔀 Complexity</button>
            <button class="metric-btn" data-metric="churn">📈 Churn</button>
            <button class="metric-btn" data-metric="length">📏 Length</button>
            <button class="metric-btn" data-metric="maintainability">🔧 Maintainability</button>
        </div>
    </div>

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

    <div class="footer">
        Generated by Kaizen Code Analysis Tool
    </div>

    <script>
        // Data
        const data = {{.TreeData}};

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

        // Treemap layout
        const treemap = d3.treemap()
            .size([width, height])
            .padding(2)
            .round(true);

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

        // Function to update treemap
        function updateTreemap(metric) {
            currentMetric = metric;

            // Prepare hierarchy
            const root = d3.hierarchy(data)
                .sum(d => d.value || 0)
                .sort((a, b) => b.value - a.value);

            treemap(root);

            // Bind data
            const cells = svg.selectAll('g')
                .data(root.leaves());

            // Enter
            const cellsEnter = cells.enter()
                .append('g')
                .attr('transform', d => 'translate(' + d.x0 + ',' + d.y0 + ')');

            cellsEnter.append('rect')
                .attr('class', 'cell')
                .attr('width', d => d.x1 - d.x0)
                .attr('height', d => d.y1 - d.y0);

            cellsEnter.append('text')
                .attr('class', 'cell-label')
                .attr('x', 5)
                .attr('y', 20);

            // Update
            const cellsUpdate = cells.merge(cellsEnter);

            cellsUpdate.select('rect')
                .transition()
                .duration(750)
                .attr('width', d => d.x1 - d.x0)
                .attr('height', d => d.y1 - d.y0)
                .attr('fill', d => colorScale(getMetricValue(d, metric)));

            cellsUpdate.select('text')
                .text(d => {
                    const width = d.x1 - d.x0;
                    const name = d.data.name.split('/').pop();
                    if (width > 60) return name;
                    return '';
                })
                .attr('x', d => Math.min(5, (d.x1 - d.x0) / 2))
                .attr('y', 20);

            // Hover events
            cellsUpdate
                .on('mouseover', function(event, d) {
                    tooltip.transition()
                        .duration(200)
                        .style('opacity', 1);

                    const metrics = d.data.metrics || {};
                    const metricValue = getMetricValue(d, metric).toFixed(1);

                    tooltip.html(
                        '<div class="tooltip-title">' + d.data.name + '</div>' +
                        '<div class="tooltip-row">' +
                            '<span class="tooltip-label">Lines:</span>' +
                            '<span class="tooltip-value">' + (d.value || 0) + '</span>' +
                        '</div>' +
                        '<div class="tooltip-row">' +
                            '<span class="tooltip-label">Functions:</span>' +
                            '<span class="tooltip-value">' + (metrics.total_functions || 0) + '</span>' +
                        '</div>' +
                        '<div class="tooltip-row">' +
                            '<span class="tooltip-label">Current Metric:</span>' +
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
                    tooltip.transition()
                        .duration(500)
                        .style('opacity', 0);
                });

            // Exit
            cells.exit().remove();
        }

        // Metric button handlers
        d3.selectAll('.metric-btn').on('click', function() {
            d3.selectAll('.metric-btn').classed('active', false);
            d3.select(this).classed('active', true);

            const metric = this.getAttribute('data-metric');
            updateTreemap(metric);
        });

        // Initial render
        updateTreemap('hotspot');
    </script>
</body>
</html>`

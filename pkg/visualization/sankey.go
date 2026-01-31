package visualization

import (
	"encoding/json"
	"html/template"
	"strings"
)

// SankeyVisualizer generates Sankey diagram HTML
type SankeyVisualizer struct{}

// NewSankeyVisualizer creates a new Sankey visualizer
func NewSankeyVisualizer() *SankeyVisualizer {
	return &SankeyVisualizer{}
}

// GenerateHTML creates interactive Sankey diagram
func (visualizer *SankeyVisualizer) GenerateHTML(
	sankeyData *SankeyData,
	title string,
) (string, error) {
	// Convert to JSON
	jsonData, err := json.Marshal(sankeyData)
	if err != nil {
		return "", err
	}

	// Render template with D3.js Sankey
	tmpl := template.Must(template.New("sankey").Parse(sankeyHTMLTemplate))

	templateData := map[string]interface{}{
		"SankeyData": template.JS(jsonData),
		"Title":      title,
	}

	var builder strings.Builder
	err = tmpl.Execute(&builder, templateData)
	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

const sankeyHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Kaizen: Code Ownership Flow - {{.Title}}</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/d3-sankey@0.12.3/dist/d3-sankey.min.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --bg-primary: #F5F1E8;
            --bg-secondary: #FDFBF7;
            --accent-terracotta: #C97064;
            --accent-amber: #D4A574;
            --accent-sage: #A8B5A3;
            --accent-rust: #B85C50;
            --accent-wheat: #E6A86F;
            --text-primary: #3E3833;
            --text-secondary: #6B6358;
            --border-subtle: #E0D7C6;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            padding: 2rem;
        }

        .container {
            max-width: 1600px;
            margin: 0 auto;
        }

        header {
            margin-bottom: 2rem;
        }

        h1 {
            font-size: 2rem;
            font-weight: 700;
            color: var(--text-primary);
            margin-bottom: 0.5rem;
        }

        .subtitle {
            font-size: 1rem;
            color: var(--text-secondary);
            margin-bottom: 1.5rem;
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }

        .stat-card {
            background: var(--bg-secondary);
            padding: 1.5rem;
            border-radius: 8px;
            border: 1px solid var(--border-subtle);
        }

        .stat-label {
            font-size: 0.875rem;
            color: var(--text-secondary);
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            font-weight: 600;
        }

        .stat-value {
            font-size: 1.75rem;
            font-weight: 700;
            color: var(--text-primary);
        }

        .stat-value.highlight {
            color: var(--accent-terracotta);
        }

        #sankey-container {
            background: var(--bg-secondary);
            border-radius: 12px;
            padding: 2rem;
            border: 1px solid var(--border-subtle);
            margin-bottom: 2rem;
        }

        #sankey {
            overflow-x: auto;
        }

        .node rect {
            cursor: pointer;
            fill-opacity: 0.9;
            stroke: white;
            stroke-width: 2;
            transition: fill-opacity 0.2s;
        }

        .node rect:hover {
            fill-opacity: 1;
            stroke-width: 3;
        }

        .node text {
            pointer-events: none;
            text-shadow: 0 1px 0 #fff;
            font-size: 12px;
            font-weight: 600;
            fill: var(--text-primary);
        }

        .link {
            fill: none;
            stroke-opacity: 0.3;
            transition: stroke-opacity 0.2s;
        }

        .link:hover {
            stroke-opacity: 0.6;
        }

        .tooltip {
            position: absolute;
            background: var(--bg-secondary);
            border: 2px solid var(--border-subtle);
            border-radius: 8px;
            padding: 1rem;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.2s;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
            max-width: 300px;
            z-index: 1000;
        }

        .tooltip.visible {
            opacity: 1;
        }

        .tooltip-title {
            font-weight: 700;
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
            color: var(--text-primary);
        }

        .tooltip-item {
            font-size: 0.75rem;
            color: var(--text-secondary);
            margin: 0.25rem 0;
        }

        .tooltip-metric {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .tooltip-metric-label {
            font-weight: 600;
        }

        .tooltip-metric-value {
            color: var(--accent-terracotta);
            font-weight: 700;
        }

        .legend {
            background: var(--bg-secondary);
            border-radius: 8px;
            padding: 1.5rem;
            border: 1px solid var(--border-subtle);
        }

        .legend-title {
            font-size: 1rem;
            font-weight: 700;
            margin-bottom: 1rem;
            color: var(--text-primary);
        }

        .legend-items {
            display: flex;
            flex-wrap: wrap;
            gap: 1rem;
        }

        .legend-item {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .legend-color {
            width: 20px;
            height: 20px;
            border-radius: 4px;
        }

        .legend-label {
            font-size: 0.875rem;
            color: var(--text-secondary);
        }

        footer {
            margin-top: 2rem;
            text-align: center;
            color: var(--text-secondary);
            font-size: 0.875rem;
        }

        footer a {
            color: var(--accent-terracotta);
            text-decoration: none;
        }

        footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>🔄 Code Ownership Flow</h1>
            <div class="subtitle">{{.Title}}</div>
        </header>

        <div id="stats" class="stats-grid"></div>

        <div id="sankey-container">
            <div id="sankey"></div>
        </div>

        <div class="legend">
            <div class="legend-title">How to Read This Diagram</div>
            <div class="legend-items">
                <div class="legend-item">
                    <div class="legend-color" style="background: var(--accent-terracotta);"></div>
                    <div class="legend-label">Code Owners (left)</div>
                </div>
                <div class="legend-item">
                    <div class="legend-color" style="background: var(--accent-amber);"></div>
                    <div class="legend-label">Common Functions (right)</div>
                </div>
                <div class="legend-item">
                    <div class="legend-label">Flow width = number of calls</div>
                </div>
                <div class="legend-item">
                    <div class="legend-label">Hover for details</div>
                </div>
            </div>
        </div>

        <footer>
            Generated by <a href="https://github.com/alexcollie/kaizen" target="_blank">Kaizen</a>
        </footer>
    </div>

    <div class="tooltip" id="tooltip"></div>

    <script>
        const data = {{.SankeyData}};

        // Render statistics
        renderStats(data.stats);

        // Set up dimensions
        const margin = {top: 20, right: 200, bottom: 20, left: 200};
        const width = 1400 - margin.left - margin.right;
        const height = 800 - margin.top - margin.bottom;

        // Create SVG
        const svg = d3.select("#sankey")
            .append("svg")
            .attr("width", width + margin.left + margin.right)
            .attr("height", height + margin.top + margin.bottom)
            .append("g")
            .attr("transform", "translate(" + margin.left + "," + margin.top + ")");

        // Create Sankey layout
        const sankey = d3.sankey()
            .nodeId(d => d.id)
            .nodeWidth(15)
            .nodePadding(10)
            .extent([[1, 1], [width - 1, height - 6]]);

        // Generate layout
        const {nodes, links} = sankey({
            nodes: data.nodes.map(d => Object.assign({}, d)),
            links: data.links.map(d => Object.assign({}, d))
        });

        // Color scale for owners
        const ownerColors = ["#C97064", "#D4A574", "#A8B5A3", "#E6A86F", "#B85C50"];
        const ownerNodes = nodes.filter(d => d.type === "owner");
        const ownerColor = d3.scaleOrdinal()
            .domain(ownerNodes.map(d => d.name))
            .range(ownerColors);

        // Draw links
        const link = svg.append("g")
            .selectAll("path")
            .data(links)
            .enter()
            .append("path")
            .attr("class", "link")
            .attr("d", d3.sankeyLinkHorizontal())
            .attr("stroke", d => {
                const sourceNode = nodes.find(n => n.id === d.source.id);
                return sourceNode.type === "owner" ? ownerColor(sourceNode.name) : "#D4A574";
            })
            .attr("stroke-width", d => Math.max(1, d.width))
            .on("mouseover", function(event, d) {
                d3.select(this).attr("stroke-opacity", 0.6);
                showLinkTooltip(event, d, nodes);
            })
            .on("mouseout", function() {
                d3.select(this).attr("stroke-opacity", 0.3);
                hideTooltip();
            });

        // Draw nodes
        const node = svg.append("g")
            .selectAll("g")
            .data(nodes)
            .enter()
            .append("g")
            .attr("class", "node");

        node.append("rect")
            .attr("x", d => d.x0)
            .attr("y", d => d.y0)
            .attr("height", d => d.y1 - d.y0)
            .attr("width", d => d.x1 - d.x0)
            .attr("fill", d => d.type === "owner" ? ownerColor(d.name) : "#D4A574")
            .on("mouseover", function(event, d) {
                showNodeTooltip(event, d);
            })
            .on("mouseout", hideTooltip);

        // Add labels for owner nodes (left side)
        node.filter(d => d.type === "owner")
            .append("text")
            .attr("x", d => d.x0 - 6)
            .attr("y", d => (d.y1 + d.y0) / 2)
            .attr("dy", "0.35em")
            .attr("text-anchor", "end")
            .text(d => truncateLabel(d.name, 25));

        // Add labels for function nodes (right side)
        node.filter(d => d.type === "function")
            .append("text")
            .attr("x", d => d.x1 + 6)
            .attr("y", d => (d.y1 + d.y0) / 2)
            .attr("dy", "0.35em")
            .attr("text-anchor", "start")
            .text(d => truncateLabel(extractFunctionName(d.name), 30));

        // Helper functions
        function renderStats(stats) {
            const statsContainer = d3.select("#stats");

            const statCards = [
                { label: "Code Owners", value: stats.total_owners },
                { label: "Common Functions", value: stats.total_common_functions, highlight: true },
                { label: "Dependencies", value: stats.total_links },
                { label: "Avg Calls/Function", value: stats.avg_calls_per_function.toFixed(1) }
            ];

            statCards.forEach(stat => {
                const card = statsContainer.append("div")
                    .attr("class", "stat-card");

                card.append("div")
                    .attr("class", "stat-label")
                    .text(stat.label);

                card.append("div")
                    .attr("class", stat.highlight ? "stat-value highlight" : "stat-value")
                    .text(stat.value);
            });
        }

        function showNodeTooltip(event, node) {
            const tooltip = d3.select("#tooltip");

            let html = '<div class="tooltip-title">' + node.name + '</div>';
            html += '<div class="tooltip-item">Type: ' + node.type + '</div>';
            html += '<div class="tooltip-item">Total calls: ' + node.value + '</div>';

            if (node.metrics) {
                html += '<div style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid var(--border-subtle);">';

                if (node.type === "owner") {
                    if (node.metrics.files !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Files:</span><span class="tooltip-metric-value">' + node.metrics.files + '</span></div>';
                    }
                    if (node.metrics.functions !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Functions:</span><span class="tooltip-metric-value">' + node.metrics.functions + '</span></div>';
                    }
                    if (node.metrics.health_score !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Health Score:</span><span class="tooltip-metric-value">' + node.metrics.health_score.toFixed(1) + '</span></div>';
                    }
                } else if (node.type === "function") {
                    if (node.metrics.complexity !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Complexity:</span><span class="tooltip-metric-value">' + node.metrics.complexity + '</span></div>';
                    }
                    if (node.metrics.cognitive_complexity !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Cognitive:</span><span class="tooltip-metric-value">' + node.metrics.cognitive_complexity + '</span></div>';
                    }
                    if (node.metrics.lines !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Lines:</span><span class="tooltip-metric-value">' + node.metrics.lines + '</span></div>';
                    }
                    if (node.metrics.maintainability !== undefined) {
                        html += '<div class="tooltip-metric"><span class="tooltip-metric-label">Maintainability:</span><span class="tooltip-metric-value">' + node.metrics.maintainability.toFixed(1) + '</span></div>';
                    }
                }

                html += '</div>';
            }

            tooltip.html(html)
                .style("left", (event.pageX + 15) + "px")
                .style("top", (event.pageY - 28) + "px")
                .classed("visible", true);
        }

        function showLinkTooltip(event, link, nodes) {
            const tooltip = d3.select("#tooltip");
            const sourceNode = nodes.find(n => n.id === link.source.id);
            const targetNode = nodes.find(n => n.id === link.target.id);

            let html = '<div class="tooltip-title">Dependency</div>';
            html += '<div class="tooltip-item"><strong>From:</strong> ' + truncateLabel(sourceNode.name, 30) + '</div>';
            html += '<div class="tooltip-item"><strong>To:</strong> ' + truncateLabel(extractFunctionName(targetNode.name), 30) + '</div>';
            html += '<div class="tooltip-metric" style="margin-top: 0.5rem;"><span class="tooltip-metric-label">Calls:</span><span class="tooltip-metric-value">' + link.value + '</span></div>';

            tooltip.html(html)
                .style("left", (event.pageX + 15) + "px")
                .style("top", (event.pageY - 28) + "px")
                .classed("visible", true);
        }

        function hideTooltip() {
            d3.select("#tooltip").classed("visible", false);
        }

        function extractFunctionName(fullName) {
            // Extract just the function name from "file/path.go::FunctionName"
            const parts = fullName.split("::");
            if (parts.length > 1) {
                return parts[parts.length - 1];
            }
            return fullName;
        }

        function truncateLabel(text, maxLength) {
            if (text.length <= maxLength) {
                return text;
            }
            return text.substring(0, maxLength - 3) + "...";
        }
    </script>
</body>
</html>
`

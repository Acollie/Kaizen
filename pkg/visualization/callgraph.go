package visualization

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"os"

	"github.com/alexcollie/kaizen/pkg/models"
)

const callGraphHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Kaizen Call Graph</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <style>
        body {
            margin: 0;
            padding: 0;
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a1a;
            color: white;
            overflow: hidden;
        }

        #header {
            padding: 20px;
            text-align: center;
            background: #2a2a2a;
            border-bottom: 2px solid #667eea;
        }

        h1 {
            margin: 0 0 10px 0;
            color: #667eea;
            font-size: 28px;
        }

        #stats {
            display: flex;
            justify-content: center;
            gap: 40px;
            margin-top: 15px;
            font-size: 14px;
            color: #999;
        }

        .stat {
            display: flex;
            flex-direction: column;
        }

        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #667eea;
        }

        .stat-label {
            font-size: 12px;
            color: #999;
            margin-top: 5px;
        }

        #controls {
            padding: 15px 20px;
            background: #2a2a2a;
            border-bottom: 1px solid #3a3a3a;
            display: flex;
            justify-content: center;
            gap: 15px;
            align-items: center;
        }

        .control-group {
            display: flex;
            align-items: center;
            gap: 10px;
        }

        label {
            font-size: 14px;
            color: #999;
        }

        select, input[type="range"] {
            padding: 5px 10px;
            background: #3a3a3a;
            border: 1px solid #4a4a4a;
            color: white;
            border-radius: 4px;
            font-size: 14px;
        }

        select:hover, input[type="range"]:hover {
            border-color: #667eea;
        }

        input[type="range"] {
            width: 150px;
        }

        button {
            padding: 8px 16px;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
        }

        button:hover {
            background: #5568d3;
        }

        #graph-container {
            position: relative;
            width: 100%;
            height: calc(100vh - 200px);
        }

        svg {
            width: 100%;
            height: 100%;
        }

        .link {
            stroke: #4a4a4a;
            stroke-opacity: 0.6;
        }

        .link.highlighted {
            stroke: #667eea;
            stroke-opacity: 1;
            stroke-width: 3px;
        }

        .node circle {
            cursor: pointer;
            transition: r 0.2s;
        }

        .node:hover circle {
            stroke: #667eea;
            stroke-width: 3px;
        }

        .node text {
            font-size: 11px;
            fill: white;
            pointer-events: none;
            text-anchor: middle;
            dominant-baseline: middle;
        }

        .tooltip {
            position: absolute;
            padding: 12px;
            background: rgba(42, 42, 42, 0.95);
            border: 1px solid #667eea;
            border-radius: 6px;
            pointer-events: none;
            font-size: 13px;
            line-height: 1.6;
            z-index: 1000;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
        }

        .tooltip-title {
            font-weight: bold;
            color: #667eea;
            margin-bottom: 8px;
            font-size: 14px;
        }

        .tooltip-item {
            display: flex;
            justify-content: space-between;
            gap: 20px;
        }

        .tooltip-label {
            color: #999;
        }

        .tooltip-value {
            color: white;
            font-weight: 500;
        }

        .legend {
            position: absolute;
            bottom: 20px;
            right: 20px;
            background: rgba(42, 42, 42, 0.95);
            border: 1px solid #4a4a4a;
            border-radius: 6px;
            padding: 15px;
            font-size: 12px;
        }

        .legend-title {
            font-weight: bold;
            margin-bottom: 10px;
            color: #667eea;
        }

        .legend-item {
            display: flex;
            align-items: center;
            gap: 10px;
            margin: 5px 0;
        }

        .legend-color {
            width: 20px;
            height: 20px;
            border-radius: 50%;
            border: 2px solid #1a1a1a;
        }
    </style>
</head>
<body>
    <div id="header">
        <h1>🔗 Kaizen Call Graph</h1>
        <div id="stats">
            <div class="stat">
                <div class="stat-value" id="total-functions">0</div>
                <div class="stat-label">Functions</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="total-calls">0</div>
                <div class="stat-label">Total Calls</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="max-fan-in">0</div>
                <div class="stat-label">Max Fan-In</div>
            </div>
            <div class="stat">
                <div class="stat-value" id="max-fan-out">0</div>
                <div class="stat-label">Max Fan-Out</div>
            </div>
        </div>
    </div>

    <div id="controls">
        <div class="control-group">
            <label for="color-metric">Color by:</label>
            <select id="color-metric">
                <option value="call_count">Call Count (Fan-In)</option>
                <option value="calls_out">Calls Out (Fan-Out)</option>
                <option value="complexity">Complexity</option>
                <option value="length">Function Length</option>
            </select>
        </div>
        <div class="control-group">
            <label for="show-labels">Show Labels:</label>
            <input type="checkbox" id="show-labels" checked>
        </div>
        <div class="control-group">
            <label for="show-external">Show External:</label>
            <input type="checkbox" id="show-external" checked>
        </div>
        <button onclick="resetZoom()">Reset View</button>
    </div>

    <div id="graph-container">
        <svg id="graph"></svg>
        <div class="legend">
            <div class="legend-title">Node Size</div>
            <div class="legend-item">
                <div class="legend-color" style="width: 10px; height: 10px; background: #667eea;"></div>
                <span>Low call count</span>
            </div>
            <div class="legend-item">
                <div class="legend-color" style="width: 20px; height: 20px; background: #667eea;"></div>
                <span>High call count</span>
            </div>
        </div>
    </div>

    <div class="tooltip" id="tooltip" style="display: none;"></div>

    <script>
        const data = {{.GraphDataJSON}};

        // Update stats
        document.getElementById('total-functions').textContent = data.stats.total_functions;
        document.getElementById('total-calls').textContent = data.stats.total_calls;
        document.getElementById('max-fan-in').textContent = data.stats.max_fan_in;
        document.getElementById('max-fan-out').textContent = data.stats.max_fan_out;

        // Prepare graph data
        const nodes = Object.values(data.nodes).map(n => ({
            id: n.full_name,
            name: n.name,
            package: n.package,
            file: n.file,
            line: n.line,
            complexity: n.complexity,
            length: n.length,
            call_count: n.call_count,
            calls_out: n.calls_out,
            is_external: n.is_external,
            is_exported: n.is_exported
        }));

        const links = data.edges.map(e => ({
            source: e.from,
            target: e.to,
            weight: e.weight,
            file: e.file,
            line: e.line
        }));

        // Setup SVG
        const container = d3.select('#graph-container');
        const svg = d3.select('#graph');
        const width = container.node().getBoundingClientRect().width;
        const height = container.node().getBoundingClientRect().height;

        const g = svg.append('g');

        // Zoom behavior
        const zoom = d3.zoom()
            .scaleExtent([0.1, 10])
            .on('zoom', (event) => {
                g.attr('transform', event.transform);
            });

        svg.call(zoom);

        function resetZoom() {
            svg.transition().duration(750).call(
                zoom.transform,
                d3.zoomIdentity.translate(width / 2, height / 2).scale(0.8)
            );
        }

        // Force simulation
        const simulation = d3.forceSimulation(nodes)
            .force('link', d3.forceLink(links).id(d => d.id).distance(100))
            .force('charge', d3.forceManyBody().strength(-300))
            .force('center', d3.forceCenter(width / 2, height / 2))
            .force('collision', d3.forceCollide().radius(d => getNodeRadius(d) + 5));

        // Create links
        const link = g.append('g')
            .selectAll('line')
            .data(links)
            .join('line')
            .attr('class', 'link')
            .attr('stroke-width', d => Math.sqrt(d.weight) * 2);

        // Create nodes
        const node = g.append('g')
            .selectAll('g')
            .data(nodes)
            .join('g')
            .attr('class', 'node')
            .call(d3.drag()
                .on('start', dragStarted)
                .on('drag', dragged)
                .on('end', dragEnded));

        const circles = node.append('circle')
            .attr('r', getNodeRadius)
            .attr('fill', d => getNodeColor(d, 'call_count'));

        const labels = node.append('text')
            .text(d => d.name)
            .attr('dy', d => getNodeRadius(d) + 15);

        // Tooltip
        const tooltip = d3.select('#tooltip');

        node.on('mouseover', (event, d) => {
            const fileName = d.file ? d.file.split('/').pop() : 'external';
            tooltip.style('display', 'block')
                .html('<div class="tooltip-title">' + d.name + '</div>' +
                    '<div class="tooltip-item">' +
                    '<span class="tooltip-label">Package:</span>' +
                    '<span class="tooltip-value">' + d.package + '</span>' +
                    '</div>' +
                    '<div class="tooltip-item">' +
                    '<span class="tooltip-label">File:</span>' +
                    '<span class="tooltip-value">' + fileName + '</span>' +
                    '</div>' +
                    '<div class="tooltip-item">' +
                    '<span class="tooltip-label">Called:</span>' +
                    '<span class="tooltip-value">' + d.call_count + ' times</span>' +
                    '</div>' +
                    '<div class="tooltip-item">' +
                    '<span class="tooltip-label">Calls out:</span>' +
                    '<span class="tooltip-value">' + d.calls_out + ' functions</span>' +
                    '</div>' +
                    '<div class="tooltip-item">' +
                    '<span class="tooltip-label">Complexity:</span>' +
                    '<span class="tooltip-value">' + d.complexity + '</span>' +
                    '</div>' +
                    '<div class="tooltip-item">' +
                    '<span class="tooltip-label">Length:</span>' +
                    '<span class="tooltip-value">' + d.length + ' lines</span>' +
                    '</div>');

            // Highlight connected nodes
            link.classed('highlighted', l => l.source.id === d.id || l.target.id === d.id);
        })
        .on('mousemove', (event) => {
            tooltip.style('left', (event.pageX + 15) + 'px')
                .style('top', (event.pageY + 15) + 'px');
        })
        .on('mouseout', () => {
            tooltip.style('display', 'none');
            link.classed('highlighted', false);
        });

        // Update simulation
        simulation.on('tick', () => {
            link
                .attr('x1', d => d.source.x)
                .attr('y1', d => d.source.y)
                .attr('x2', d => d.target.x)
                .attr('y2', d => d.target.y);

            node.attr('transform', d => 'translate(' + d.x + ',' + d.y + ')');
        });

        // Helper functions
        function getNodeRadius(d) {
            const minRadius = 5;
            const maxRadius = 30;
            const maxCallCount = data.stats.max_fan_in;

            if (maxCallCount === 0) return minRadius;

            return minRadius + (d.call_count / maxCallCount) * (maxRadius - minRadius);
        }

        function getNodeColor(d, metric) {
            if (d.is_external) return '#666';

            let value, max;
            switch(metric) {
                case 'call_count':
                    value = d.call_count;
                    max = data.stats.max_fan_in;
                    break;
                case 'calls_out':
                    value = d.calls_out;
                    max = data.stats.max_fan_out;
                    break;
                case 'complexity':
                    value = d.complexity;
                    max = Math.max(...nodes.filter(n => !n.is_external).map(n => n.complexity));
                    break;
                case 'length':
                    value = d.length;
                    max = Math.max(...nodes.filter(n => !n.is_external).map(n => n.length));
                    break;
            }

            const ratio = max > 0 ? value / max : 0;
            return interpolateColor(ratio);
        }

        function interpolateColor(ratio) {
            const colors = [
                { r: 34, g: 197, b: 94 },   // Green (low)
                { r: 234, g: 179, b: 8 },   // Yellow (medium)
                { r: 239, g: 68, b: 68 }    // Red (high)
            ];

            let colorIndex, localRatio;
            if (ratio < 0.5) {
                colorIndex = 0;
                localRatio = ratio * 2;
            } else {
                colorIndex = 1;
                localRatio = (ratio - 0.5) * 2;
            }

            const c1 = colors[colorIndex];
            const c2 = colors[colorIndex + 1];

            const r = Math.round(c1.r + (c2.r - c1.r) * localRatio);
            const g = Math.round(c1.g + (c2.g - c1.g) * localRatio);
            const b = Math.round(c1.b + (c2.b - c1.b) * localRatio);

            return 'rgb(' + r + ', ' + g + ', ' + b + ')';
        }

        // Drag functions
        function dragStarted(event, d) {
            if (!event.active) simulation.alphaTarget(0.3).restart();
            d.fx = d.x;
            d.fy = d.y;
        }

        function dragged(event, d) {
            d.fx = event.x;
            d.fy = event.y;
        }

        function dragEnded(event, d) {
            if (!event.active) simulation.alphaTarget(0);
            d.fx = null;
            d.fy = null;
        }

        // Controls
        d3.select('#color-metric').on('change', function() {
            const metric = this.value;
            circles.transition().duration(500)
                .attr('fill', d => getNodeColor(d, metric));
        });

        d3.select('#show-labels').on('change', function() {
            labels.style('display', this.checked ? 'block' : 'none');
        });

        d3.select('#show-external').on('change', function() {
            const showExternal = this.checked;
            node.style('display', d => (showExternal || !d.is_external) ? 'block' : 'none');
            link.style('display', l => {
                if (!showExternal) {
                    return (l.source.is_external || l.target.is_external) ? 'none' : 'block';
                }
                return 'block';
            });
        });

        // Initial zoom
        resetZoom();
    </script>
</body>
</html>
`

// GenerateCallGraphHTML generates an interactive HTML call graph visualization
func GenerateCallGraphHTML(graph *models.CallGraph, outputPath string) error {
	// Convert graph to JSON
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("failed to marshal graph data: %w", err)
	}

	// Create template
	tmpl, err := template.New("callgraph").Parse(callGraphHTMLTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Execute template
	data := struct {
		GraphDataJSON template.JS
	}{
		GraphDataJSON: template.JS(graphJSON),
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GenerateCallGraphSVG generates a static SVG call graph visualization
func GenerateCallGraphSVG(graph *models.CallGraph, outputPath string, width, height int) error {
	// Simple force-directed layout simulation
	nodes := make([]*svgNode, 0, len(graph.Nodes))
	nodeMap := make(map[string]*svgNode)

	// Create nodes with initial random positions
	for _, node := range graph.Nodes {
		svgN := &svgNode{
			node: node,
			x:    float64(width)/2 + (mathRand()*200 - 100),
			y:    float64(height)/2 + (mathRand()*200 - 100),
			vx:   0,
			vy:   0,
		}
		nodes = append(nodes, svgN)
		nodeMap[node.FullName] = svgN
	}

	// Run simplified force simulation (50 iterations)
	for iteration := 0; iteration < 50; iteration++ {
		// Repulsion between nodes
		for i, n1 := range nodes {
			for j, n2 := range nodes {
				if i >= j {
					continue
				}

				dx := n2.x - n1.x
				dy := n2.y - n1.y
				distance := math.Sqrt(dx*dx + dy*dy)
				if distance < 1 {
					distance = 1
				}

				force := 1000 / (distance * distance)
				n1.vx -= (dx / distance) * force
				n1.vy -= (dy / distance) * force
				n2.vx += (dx / distance) * force
				n2.vy += (dy / distance) * force
			}
		}

		// Attraction along edges
		for _, edge := range graph.Edges {
			source := nodeMap[edge.From]
			target := nodeMap[edge.To]
			if source == nil || target == nil {
				continue
			}

			dx := target.x - source.x
			dy := target.y - source.y
			distance := math.Sqrt(dx*dx + dy*dy)
			if distance < 1 {
				distance = 1
			}

			force := distance * 0.01
			source.vx += (dx / distance) * force
			source.vy += (dy / distance) * force
			target.vx -= (dx / distance) * force
			target.vy -= (dy / distance) * force
		}

		// Center force
		for _, n := range nodes {
			n.vx += (float64(width)/2 - n.x) * 0.01
			n.vy += (float64(height)/2 - n.y) * 0.01
		}

		// Update positions
		for _, n := range nodes {
			n.x += n.vx
			n.y += n.vy
			n.vx *= 0.8 // Damping
			n.vy *= 0.8
		}
	}

	// Generate SVG
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <title>Kaizen Call Graph</title>
  <rect width="%d" height="%d" fill="#1a1a1a"/>

  <!-- Edges -->
  <g id="edges">
`, width, height, width, height, width, height)

	for _, edge := range graph.Edges {
		source := nodeMap[edge.From]
		target := nodeMap[edge.To]
		if source == nil || target == nil {
			continue
		}

		strokeWidth := math.Sqrt(float64(edge.Weight)) * 2
		svg += fmt.Sprintf(`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#4a4a4a" stroke-width="%.2f" opacity="0.6"/>
`, source.x, source.y, target.x, target.y, strokeWidth)
	}

	svg += `  </g>

  <!-- Nodes -->
  <g id="nodes">
`

	maxCallCount := float64(graph.Stats.MaxFanIn)
	if maxCallCount == 0 {
		maxCallCount = 1
	}

	for _, svgN := range nodes {
		radius := 5 + (float64(svgN.node.CallCount)/maxCallCount)*25
		color := getColorForValue(float64(svgN.node.CallCount), maxCallCount)

		svg += fmt.Sprintf(`    <circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s" stroke="#1a1a1a" stroke-width="2">
      <title>%s
Calls: %d
Complexity: %d</title>
    </circle>
`, svgN.x, svgN.y, radius, color, svgN.node.Name, svgN.node.CallCount, svgN.node.Complexity)
	}

	svg += `  </g>
</svg>
`

	return os.WriteFile(outputPath, []byte(svg), 0644)
}

type svgNode struct {
	node *models.CallNode
	x, y float64
	vx, vy float64
}

func mathRand() float64 {
	// Simple pseudo-random for layout
	return float64(os.Getpid()%100) / 100.0
}

func getColorForValue(value, max float64) string {
	ratio := value / max

	if ratio < 0.5 {
		// Green to yellow
		r := int(34 + (234-34)*ratio*2)
		g := int(197 + (179-197)*ratio*2)
		b := int(94 + (8-94)*ratio*2)
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	} else {
		// Yellow to red
		ratio = (ratio - 0.5) * 2
		r := int(234 + (239-234)*ratio)
		g := int(179 + (68-179)*ratio)
		b := int(8 + (68-8)*ratio)
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
}

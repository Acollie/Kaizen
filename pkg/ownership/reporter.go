package ownership

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RenderOwnerReportASCII renders ownership report as ASCII table
func RenderOwnerReportASCII(report *OwnerReport) string {
	var output strings.Builder

	output.WriteString("👥 Code Ownership Report\n")
	output.WriteString("═════════════════════════════════════════════════════════════════════════════════\n\n")

	if report.TotalOwners == 0 {
		output.WriteString("No ownership data available\n")
		return output.String()
	}

	// Summary
	output.WriteString(fmt.Sprintf("Analyzed: %s | Total Owners: %d\n\n", report.AnalyzedAt, report.TotalOwners))

	// Header
	output.WriteString(fmt.Sprintf(
		"%-20s │ %-8s │ %-8s │ %-8s │ %-10s │ %-10s │ %-8s\n",
		"Owner", "Files", "Funcs", "Health", "Avg Cmplx", "Avg Maint", "Hotspots",
	))
	output.WriteString("─────────────────────┼──────────┼──────────┼──────────┼────────────┼────────────┼──────────\n")

	// Data rows
	for _, metrics := range report.OwnerMetrics {
		owner := metrics.Owner
		if len(owner) > 20 {
			owner = owner[:17] + "..."
		}

		output.WriteString(fmt.Sprintf(
			"%-20s │ %-8d │ %-8d │ %7.1f%% │ %10.1f │ %10.1f │ %-8d\n",
			owner,
			metrics.FileCount,
			metrics.FunctionCount,
			metrics.OverallHealthScore,
			metrics.AvgCyclomaticComplexity,
			metrics.AvgMaintainabilityIndex,
			metrics.HotspotCount,
		))
	}

	output.WriteString("\n")

	// Health indicator
	output.WriteString("Health Status:\n")
	for _, metrics := range report.OwnerMetrics {
		status := getHealthStatus(metrics.OverallHealthScore)
		output.WriteString(fmt.Sprintf("  %s %s (%d files, %d functions)\n",
			status,
			metrics.Owner,
			metrics.FileCount,
			metrics.FunctionCount,
		))
	}

	return output.String()
}

// RenderOwnerReportJSON renders report as JSON
func RenderOwnerReportJSON(report *OwnerReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RenderOwnerReportHTML generates interactive HTML report
func RenderOwnerReportHTML(report *OwnerReport) (string, error) {
	// Convert metrics to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		return "", err
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Kaizen: Code Ownership Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            padding: 40px 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
            padding: 40px;
        }
        h1 {
            color: #333;
            margin-bottom: 10px;
            font-size: 32px;
        }
        .subtitle {
            color: #666;
            margin-bottom: 30px;
            font-size: 14px;
        }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }
        .summary-card {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 20px;
            border-radius: 8px;
        }
        .summary-label {
            font-size: 12px;
            text-transform: uppercase;
            opacity: 0.9;
            margin-bottom: 8px;
        }
        .summary-value {
            font-size: 28px;
            font-weight: bold;
        }
        .table-container {
            margin-bottom: 40px;
            overflow-x: auto;
        }
        table {
            width: 100%%;
            border-collapse: collapse;
        }
        th {
            background: #f8f9fa;
            padding: 12px;
            text-align: left;
            font-weight: 600;
            color: #333;
            border-bottom: 2px solid #667eea;
        }
        td {
            padding: 12px;
            border-bottom: 1px solid #eee;
            color: #666;
        }
        tr:hover {
            background: #f8f9fa;
        }
        .health-excellent {
            color: #10b981;
            font-weight: 600;
        }
        .health-good {
            color: #3b82f6;
            font-weight: 600;
        }
        .health-fair {
            color: #f59e0b;
            font-weight: 600;
        }
        .health-poor {
            color: #ef4444;
            font-weight: 600;
        }
        .chart-row {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
            gap: 30px;
            margin-bottom: 40px;
        }
        .chart-container {
            position: relative;
            height: 300px;
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
        }
        .chart-title {
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 15px;
            color: #333;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            color: #666;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>👥 Code Ownership Report</h1>
        <div class="subtitle">Generated at %s</div>

        <div class="summary">
            <div class="summary-card">
                <div class="summary-label">Total Owners</div>
                <div class="summary-value" id="totalOwners">%d</div>
            </div>
            <div class="summary-card">
                <div class="summary-label">Avg Health Score</div>
                <div class="summary-value" id="avgHealth">-</div>
            </div>
            <div class="summary-card">
                <div class="summary-label">Total Files</div>
                <div class="summary-value" id="totalFiles">-</div>
            </div>
            <div class="summary-card">
                <div class="summary-label">Total Functions</div>
                <div class="summary-value" id="totalFunctions">-</div>
            </div>
        </div>

        <div class="table-container">
            <h2 style="margin-bottom: 20px; color: #333;">Owner Metrics</h2>
            <table id="ownerTable">
                <thead>
                    <tr>
                        <th>Owner</th>
                        <th>Files</th>
                        <th>Functions</th>
                        <th>Health Score</th>
                        <th>Avg Complexity</th>
                        <th>Avg Maintainability</th>
                        <th>Hotspots</th>
                    </tr>
                </thead>
                <tbody id="ownerBody"></tbody>
            </table>
        </div>

        <div class="chart-row">
            <div class="chart-container">
                <div class="chart-title">Health Scores by Owner</div>
                <canvas id="healthChart"></canvas>
            </div>
            <div class="chart-container">
                <div class="chart-title">Complexity by Owner</div>
                <canvas id="complexityChart"></canvas>
            </div>
        </div>

        <div class="footer">
            Generated by Kaizen · %s
        </div>
    </div>

    <script>
        const report = %s;

        // Populate summary
        let totalFiles = 0, totalFunctions = 0, totalHealth = 0;
        report.owner_metrics.forEach(m => {
            totalFiles += m.file_count;
            totalFunctions += m.function_count;
            totalHealth += m.overall_health_score;
        });

        document.getElementById('totalOwners').textContent = report.total_owners;
        document.getElementById('totalFiles').textContent = totalFiles;
        document.getElementById('totalFunctions').textContent = totalFunctions;
        document.getElementById('avgHealth').textContent = (totalHealth / report.total_owners).toFixed(1);

        // Populate table
        const tbody = document.getElementById('ownerBody');
        report.owner_metrics.forEach(m => {
            const healthClass = m.overall_health_score >= 80 ? 'health-excellent' :
                               m.overall_health_score >= 60 ? 'health-good' :
                               m.overall_health_score >= 40 ? 'health-fair' : 'health-poor';

            const row = tbody.insertRow();
            row.innerHTML = '<td>' + m.owner + '</td>' +
                '<td>' + m.file_count + '</td>' +
                '<td>' + m.function_count + '</td>' +
                '<td class="' + healthClass + '">' + m.overall_health_score.toFixed(1) + '</td>' +
                '<td>' + m.avg_cyclomatic_complexity.toFixed(1) + '</td>' +
                '<td>' + m.avg_maintainability_index.toFixed(1) + '</td>' +
                '<td>' + m.hotspot_count + '</td>';
        });

        // Health chart
        const owners = report.owner_metrics.map(m => m.owner);
        const healthScores = report.owner_metrics.map(m => m.overall_health_score);

        new Chart(document.getElementById('healthChart'), {
            type: 'bar',
            data: {
                labels: owners,
                datasets: [{
                    label: 'Health Score',
                    data: healthScores,
                    backgroundColor: healthScores.map(s =>
                        s >= 80 ? '#10b981' : s >= 60 ? '#3b82f6' : s >= 40 ? '#f59e0b' : '#ef4444'
                    ),
                }]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: { x: { max: 100 } }
            }
        });

        // Complexity chart
        const complexity = report.owner_metrics.map(m => m.avg_cyclomatic_complexity);

        new Chart(document.getElementById('complexityChart'), {
            type: 'bar',
            data: {
                labels: owners,
                datasets: [{
                    label: 'Avg Cyclomatic Complexity',
                    data: complexity,
                    backgroundColor: '#667eea',
                }]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
            }
        });
    </script>
</body>
</html>
`, report.AnalyzedAt, report.TotalOwners, time.Now().Format("2006-01-02 15:04:05"), string(jsonData))

	return html, nil
}

func getHealthStatus(score float64) string {
	if score >= 80 {
		return "✅"
	} else if score >= 60 {
		return "✓"
	} else if score >= 40 {
		return "⚠️"
	} else {
		return "❌"
	}
}

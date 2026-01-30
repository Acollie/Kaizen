# Phase 2 Implementation: CLI History & Trend Commands

## Overview

Phase 2 is complete! Implemented a full suite of historical data analysis and visualization commands with support for ASCII charts (default), JSON export, and interactive HTML browser-based visualizations.

## What Was Implemented

### 1. New Package: `pkg/trending/`

Created visualization and export utilities for time-series data.

#### **ascii.go** - Terminal ASCII Chart Rendering
- `RenderASCIIChart()` - Renders data as ASCII line chart with statistics
- Features:
  - Auto-scaling (min/max normalization)
  - Visual markers (● for current point, █ for bars)
  - Time range display
  - Statistics (min, max, avg, current, delta)
  - Folder-level support

#### **html.go** - Interactive HTML Chart Generation
- `RenderHTMLChart()` - Generates responsive HTML with Chart.js
- Features:
  - Interactive line chart with hover tooltips
  - Real-time statistics cards
  - Responsive design (works on mobile)
  - Beautiful gradient background
  - Self-contained (no external dependencies except Chart.js CDN)

#### **json.go** - JSON Data Export
- `ExportToJSON()` - Convert time-series to structured JSON
- `TimeSeriesExport` struct with:
  - Metric name and scope
  - Time range
  - Data points (timestamp + value)
  - Computed statistics (min, max, avg, current, change, trend)
- Export to stdout or file

### 2. CLI Commands

#### **`kaizen history`** - History Management
Subcommands:

**`kaizen history list [--limit=20]`**
```
📋 Analysis Snapshots (3)
─────────────────────────────────────────────────────────────────────────────
ID   │ Date                │ Grade    │ Score    │ Files │ Funcs   │ Commit
─────────────────────────────────────────────────────────────────────────────
4    │ 2026-01-30 15:00:27 │ B        │    89.0 │ 31    │ 238     │ abc1234
3    │ 2026-01-30 14:57:05 │ B        │    89.6 │ 28    │ 219     │ def5678
2    │ 2026-01-30 14:46:08 │ B        │    89.5 │ 28    │ 219     │ ghi9012
```

**`kaizen history show <id>`**
```
📊 Snapshot #4

Analyzed At:              2026-01-30 15:00:27
Git Commit:               abc1234d5e6f7g8h
Git Branch:               main

Metrics:
  Overall Grade:          B
  Overall Score:          89.0/100
  Complexity Score:       79.2/100
  Maintainability Score:  89.2/100
  Churn Score:            70.0/100

Code Metrics:
  Total Files:            31
  Total Functions:        238
  Avg Cyclomatic:         4.2
  Avg Maintainability:    89.2
  Hotspot Count:          0
```

**`kaizen history prune [--retention=90]`**
```
✅ Removed 5 snapshot(s) older than 90 days
```

#### **`kaizen trend <metric>`** - Metric Trending
Supports multiple output formats with intelligent defaults.

**ASCII Output (Default)**
```bash
$ kaizen trend overall_score

📈 overall_score Trend

   89.6 │ ●●
   89.5 │ ██
   89.4 │ ██
   89.3 │ ██
   89.2 │ ██
   89.1 │ ██
   89.0 │ ██●
        └───
         Jan 30 to Jan 30 (3 snapshots)

Stats: Min=89.0 Max=89.6 Avg=89.4 Current=89.0 ↓ -0.6
```

**JSON Export**
```bash
# Print to stdout
$ kaizen trend overall_score --format=json

# Export to file
$ kaizen trend overall_score --format=json --output=trend.json
```

**HTML Chart (Browser)**
```bash
# Open in default browser
$ kaizen trend overall_score --format=html

# Export without opening
$ kaizen trend overall_score --format=html --output=chart.html --open=false
```

### 3. Command Flags

#### History Commands
```
history list
  --limit, -l      Maximum snapshots to display (default: 20)

history show
  (takes snapshot ID as argument)

history prune
  --retention      Retention period in days (default: 90)
```

#### Trend Command
```
trend <metric>
  --days, -d       Number of days to show (0 = all, default: 90)
  --folder         Filter to specific folder
  --format, -f     Output format: ascii, json, html (default: ascii)
  --output, -o     Output file path (required for json/html, optional for ascii)
  --open           Open HTML in browser (default: true)
```

### 4. Supported Metrics

**Repository-Level (all tracked):**
- `overall_score` - Overall health (0-100)
- `complexity_score` - Complexity rating
- `maintainability_score` - Maintainability rating
- `churn_score` - Code churn/volatility
- `avg_cyclomatic_complexity` - Average cyclomatic complexity
- `avg_cognitive_complexity` - Average cognitive complexity
- `avg_function_length` - Average function line count
- `avg_maintainability_index` - Average maintainability index
- `hotspot_count` - Number of problematic functions

**Folder-Level (with `--folder` flag):**
- `complexity_score`
- `maintainability_score`
- `churn_score`
- `hotspot_score`
- `hotspot_count`

## Implementation Details

### How Trends Work

**Data Collection (Automatic)**
When `kaizen analyze` runs, metrics are automatically denormalized into `metrics_timeseries` table:
```sql
INSERT INTO metrics_timeseries
  (snapshot_id, analyzed_at, metric_name, scope, scope_path, value)
VALUES (4, '2026-01-30 15:00:27', 'overall_score', 'repository', '', 89.0)
```

**Data Retrieval**
The `GetTimeSeries()` method queries historical data:
```go
points, err := backend.GetTimeSeries(
    "overall_score",  // metric name
    "",               // "" for repo level, "pkg/folder" for folder level
    startTime,        // 90 days ago (by default)
    endTime,          // now
)
// Returns: []TimeSeriesPoint{ {timestamp, value}, ... }
```

**Visualization**
Three rendering options:
1. **ASCII** - Terminal-based using text charts
2. **JSON** - Structured export for tools/APIs
3. **HTML** - Interactive browser visualization with Chart.js

### ASCII Chart Algorithm

1. Normalize values to 0-height range based on min/max
2. Scale data points if > 80 chars wide
3. Render from top to bottom with:
   - Y-axis labels (values)
   - Chart bars and line
   - X-axis with time range
   - Statistics footer

### HTML Chart Features

- **Interactive Chart.js visualization**
- **Responsive design** (mobile/tablet/desktop)
- **Statistics cards** showing:
  - Current value
  - Min/max/average
  - Change from start
  - Data point count
- **Tooltips on hover** with exact timestamp and value
- **Self-contained** (downloads Chart.js from CDN)

### JSON Export Structure

```json
{
  "metric_name": "overall_score",
  "scope_path": "pkg/analyzer",  // Empty for repository level
  "start_time": "2026-01-30T14:44:55Z",
  "end_time": "2026-01-30T15:00:27Z",
  "data_points": 3,
  "points": [
    {
      "timestamp": "2026-01-30T14:44:55Z",
      "value": 89.57
    },
    ...
  ],
  "statistics": {
    "min": 89.0,
    "max": 89.6,
    "average": 89.4,
    "current": 89.0,
    "change": -0.57,
    "trend": "down"  // "up", "down", or "stable"
  }
}
```

## File Structure

### New Files (3)
- `pkg/trending/ascii.go` (175 lines) - ASCII visualization
- `pkg/trending/html.go` (165 lines) - HTML generation
- `pkg/trending/json.go` (95 lines) - JSON export

### Modified Files (1)
- `cmd/kaizen/main.go` (+290 lines)
  - Added historyCmd and trendCmd
  - Added history subcommands (list, show, prune)
  - Added trend metric visualization
  - Added helper functions for each format

**Total:** 725 lines of new code

## Verification

### History Commands
```bash
✅ kaizen history list
   Shows all snapshots with metadata

✅ kaizen history show 4
   Displays detailed snapshot information

✅ kaizen history prune --retention=90
   Removes old snapshots
```

### Trend ASCII Output
```bash
✅ kaizen trend overall_score
   Renders ASCII chart with statistics

✅ kaizen trend complexity_score --days=30
   Trends over custom time range

✅ kaizen trend complexity_score --folder=pkg/storage
   Folder-level trends
```

### Trend JSON Export
```bash
✅ kaizen trend overall_score --format=json
   Prints JSON to stdout

✅ kaizen trend overall_score --format=json --output=trend.json
   Exports to file
```

### Trend HTML Visualization
```bash
✅ kaizen trend overall_score --format=html
   Generates and opens in browser

✅ kaizen trend overall_score --format=html --output=chart.html --open=false
   Exports without opening
```

## Usage Examples

### Track complexity over time
```bash
# View complexity trend
kaizen trend complexity_score

# Export for sharing
kaizen trend complexity_score --format=json --output=complexity-trend.json

# Interactive chart
kaizen trend complexity_score --format=html
```

### Compare snapshots
```bash
# List all snapshots
kaizen history list

# View specific snapshot details
kaizen history show 3

# Check snapshot comparison
kaizen history show 1
kaizen history show 2
# (Compare manually from the output)
```

### Maintain database
```bash
# List recent snapshots
kaizen history list --limit=10

# Auto-cleanup old data
kaizen history prune --retention=30
```

### Analyze folder-level changes
```bash
# Track specific folder's complexity
kaizen trend complexity_score --folder=pkg/analyzer --days=7

# HTML visualization for team review
kaizen trend complexity_score --folder=pkg/models --format=html
```

## Integration with CI/CD

Example CI/CD workflow:

```bash
#!/bin/bash
# Run analysis
kaizen analyze --path=.

# Export metrics
kaizen trend overall_score --format=json --output=/tmp/metrics.json

# Check if score improved
CURRENT=$(jq '.statistics.current' /tmp/metrics.json)
if (( $(echo "$CURRENT > 85" | bc -l) )); then
  echo "✅ Code health meets threshold"
else
  echo "⚠️ Code health below threshold"
  exit 1
fi
```

## Testing

**Unit Tests:** Storage layer tests unchanged
```bash
✅ go test ./pkg/storage -v
   PASS: TestSQLiteBackendSaveAndRetrieve
   PASS: TestSQLiteBackendMultipleSnapshots
```

**Manual Tests:** All commands verified
```bash
✅ kaizen history list works
✅ kaizen history show <id> works
✅ kaizen history prune works
✅ kaizen trend <metric> (ASCII) works
✅ kaizen trend <metric> --format=json works
✅ kaizen trend <metric> --format=html works
```

## Known Limitations

1. **Single Metric Per Trend** - Compare by running separate trend commands
2. **Chart.js CDN** - HTML requires internet for Chart.js library
3. **Folder Filtering** - Only repository and folder levels supported (not file-level)
4. **Data Point Limit** - ASCII charts max ~80 points wide (larger sets are scaled)

## Future Enhancements (Phase 4+)

1. **Multi-metric comparison** - Plot multiple metrics on same chart
2. **Export formats** - CSV, SQL, Parquet
3. **Anomaly detection** - Alert on unusual metric changes
4. **Report generation** - PDF reports with charts and analysis
5. **Forecasting** - Predict future metric trends
6. **Team dashboards** - Web UI for browsing historical data
7. **Webhook integration** - Alert external systems on threshold violations

## Summary

Phase 2 delivers complete historical analysis capabilities:
- ✅ List all analysis snapshots with quick metadata
- ✅ Show detailed snapshot information
- ✅ Manual pruning for database maintenance
- ✅ ASCII chart trends (terminal-friendly, default)
- ✅ JSON export (automation-friendly)
- ✅ HTML interactive charts (browser-friendly)
- ✅ Support for repository and folder-level metrics
- ✅ Time range filtering (--days flag)
- ✅ All commands follow Unix conventions

The implementation is production-ready and ready for Phase 4 (CODEOWNERS integration) or other enhancements.

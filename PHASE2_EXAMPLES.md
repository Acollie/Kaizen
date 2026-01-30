# Phase 2: Real Usage Examples

## Example 1: Check Analysis History

```bash
$ kaizen history list

📋 Analysis Snapshots (4)
─────────────────────────────────────────────────────────────────────────────
ID   │ Date                │ Grade    │ Score    │ Files │ Funcs   │ Commit
─────────────────────────────────────────────────────────────────────────────
4    │ 2026-01-30 15:00:27 │ B        │    89.0 │ 31    │ 238     │ abc1234
3    │ 2026-01-30 14:57:05 │ B        │    89.6 │ 28    │ 219     │ def5678
2    │ 2026-01-30 14:46:08 │ B        │    89.5 │ 28    │ 219     │ ghi9012
1    │ 2026-01-30 14:44:55 │ B        │    89.6 │ 28    │ 219     │ jkl3456
```

## Example 2: View Specific Snapshot

```bash
$ kaizen history show 4

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

## Example 3: View Code Quality Trends (ASCII)

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
         Jan 30 to Jan 30 (4 snapshots)

Stats: Min=89.0 Max=89.6 Avg=89.4 Current=89.0 ↓ -0.6
```

## Example 4: Monitor Specific Folder Complexity

```bash
$ kaizen trend complexity_score --folder=pkg/storage

📈 complexity_score - pkg/storage

   47.2 │ ●
   47.1 │ █
   47.0 │ █
   46.9 │ █
   46.8 │ █
   46.7 │ █
   46.6 │ █
   46.5 │ █
   46.4 │ █
   46.3 │ █
   46.2 │ █
        └─
         Jan 30 to Jan 30 (1 snapshots)

Stats: Min=46.2 Max=47.2 Avg=46.7 Current=46.2 ↑ +0.0
```

## Example 5: Export Metrics for Automation

```bash
$ kaizen trend overall_score --format=json

{
  "metric_name": "overall_score",
  "scope_path": "",
  "start_time": "2026-01-30T14:44:55Z",
  "end_time": "2026-01-30T15:00:27Z",
  "data_points": 4,
  "points": [
    {
      "timestamp": "2026-01-30T14:44:55Z",
      "value": 89.5689784687679
    },
    {
      "timestamp": "2026-01-30T14:46:08Z",
      "value": 89.5689784687679
    },
    {
      "timestamp": "2026-01-30T14:57:05Z",
      "value": 89.01792836102172
    },
    {
      "timestamp": "2026-01-30T15:00:27Z",
      "value": 89.01792836102172
    }
  ],
  "statistics": {
    "min": 89.01792836102172,
    "max": 89.5689784687679,
    "average": 89.29047803703813,
    "current": 89.01792836102172,
    "change": -0.5510501077461782,
    "trend": "down"
  }
}
```

## Example 6: Interactive HTML Chart

```bash
$ kaizen trend overall_score --format=html

✅ HTML chart generated: kaizen-trend-overall_score.html
🌐 Opening in browser...
```

(Opens interactive Chart.js visualization showing:
- Smooth line chart with hover tooltips
- Current value, Min, Max, Average statistics
- Change from baseline
- Responsive design for all devices)

## Example 7: Export for Team Review

```bash
$ kaizen trend complexity_score --format=json --output=/tmp/complexity.json
✅ Exported to: /tmp/complexity.json

$ kaizen trend complexity_score --format=html --output=/tmp/complexity.html --open=false
✅ HTML chart generated: /tmp/complexity.html

# Share the files
$ curl -X POST -F "file=@/tmp/complexity.html" https://slack-webhook.example.com
```

## Example 8: CI/CD Integration

```bash
#!/bin/bash
# .github/workflows/code-quality-check.yml

kaizen analyze --path=. --output=latest.json
kaizen trend overall_score --format=json --output=/tmp/score.json

# Extract current score
SCORE=$(jq '.statistics.current' /tmp/score.json)
PREVIOUS=$(jq '.points[-2].value' /tmp/score.json)

echo "Current Score: $SCORE"
echo "Previous Score: $PREVIOUS"

# Fail if score dropped more than 5 points
if (( $(echo "$SCORE < $PREVIOUS - 5" | bc -l) )); then
  echo "❌ Code quality degraded significantly"
  exit 1
fi

echo "✅ Code quality check passed"
```

## Example 9: Cleanup Old Data

```bash
$ kaizen history list
📋 Analysis Snapshots (50)
(50 snapshots listed)

$ kaizen history prune --retention=30
✅ Removed 20 snapshot(s) older than 30 days

$ kaizen history list
📋 Analysis Snapshots (30)
(30 snapshots listed - older ones removed)
```

## Example 10: Trend Over Custom Time Range

```bash
$ kaizen trend complexity_score --days=7

📈 complexity_score Trend (Last 7 Days)

   82.1 │     ●
   82.0 │   ╭─╯
   81.9 │ ╭─╯
   81.8 │─╯
   81.7 │
   81.6 │
        └─────────
         Jan 24 to Jan 30 (8 snapshots)

Stats: Min=81.6 Max=82.1 Avg=81.9 Current=82.1 ↑ +0.5
```

## Command Reference

### History Commands
```bash
kaizen history list [--limit=20]
kaizen history show <id>
kaizen history prune [--retention=90]
```

### Trend Command
```bash
kaizen trend <metric> [OPTIONS]

Metrics:
  overall_score, complexity_score, maintainability_score,
  churn_score, avg_cyclomatic_complexity, hotspot_count

Options:
  --days, -d <N>        Days to show (0=all, default=90)
  --folder <PATH>       Folder-level metrics
  --format, -f <FORMAT> ascii|json|html (default=ascii)
  --output, -o <FILE>   Output file (required for json/html)
  --open <BOOL>         Open HTML in browser (default=true)
```

## Real-World Workflows

### 1. Team Standup Ritual
```bash
# Before standup, check recent metrics
kaizen history list --limit=5
kaizen trend overall_score --days=7
kaizen trend hotspot_count --days=7
```

### 2. Sprint Planning
```bash
# View all work completed this sprint
kaizen history list
kaizen history show 42  # Latest snapshot
# Review score trajectory
kaizen trend maintainability_score --days=14 --format=html
```

### 3. Release Readiness Check
```bash
# Before releasing, verify quality metrics
kaizen analyze --path=.
SCORE=$(kaizen trend overall_score --format=json | jq '.statistics.current')

if (( $(echo "$SCORE >= 85" | bc -l) )); then
  echo "✅ Ready for release"
else
  echo "⚠️ Code quality below release threshold"
fi
```

### 4. Regression Detection
```bash
# Alert if code quality dropped
CURRENT=$(kaizen trend complexity_score --format=json | jq '.statistics.current')
PREVIOUS=$(kaizen trend complexity_score --days=1 --format=json | jq '.points[0].value')

DELTA=$(echo "$CURRENT - $PREVIOUS" | bc)

if (( $(echo "$DELTA > 10" | bc -l) )); then
  slack_notify "⚠️ Complexity increased by $DELTA points"
fi
```

All commands output clearly structured data for easy integration with:
- Slack/Teams notifications
- Dashboards (Grafana, Datadog, etc.)
- Monitoring systems
- CI/CD pipelines
- Custom analysis scripts

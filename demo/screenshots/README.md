# Kaizen Screenshots

This directory contains high-quality screenshots demonstrating Kaizen's features.

## Screenshot Guide

To generate screenshots for this demo:

### 1. Terminal Output (`06-terminal-output.png`)

Run the demo script and capture the terminal output:

```bash
cd demo
./run-demo.sh
```

**What to capture:**
- Full terminal output with colors
- Progress indicators during analysis
- Code health grade display
- Areas of concern list
- Summary metrics

**Tools:**
- macOS: Screenshot with Cmd+Shift+4 or use iTerm2's "Capture Output" feature
- Linux: Use `gnome-screenshot` or `scrot`
- Enhance colors and contrast in preview

### 2. Interactive Heatmap (`02-interactive-heatmap.png`)

Open the generated heatmap and capture:

```bash
open demo/outputs/sample-heatmap.html
```

**What to capture:**
- Full treemap visualization
- Tooltip showing function metrics (hover over a box)
- Color gradient legend
- Drill-down capability (before/after shots)

**Tips:**
- Use browser's full screen mode
- Zoom to appropriate level
- Show hover state with tooltip
- High resolution (1920x1080 or higher)

### 3. Code Health Grade (`01-code-health-grade.png`)

Run Kaizen on a real project to show the grading:

```bash
./kaizen analyze --path=. --skip-churn
```

**What to capture:**
- Overall grade (A-F) with color
- Component scores breakdown
- Progress bars or indicators
- Summary statistics

### 4. Areas of Concern (`03-areas-of-concern.png`)

Capture the areas of concern output:

**What to capture:**
- List of functions with high complexity
- Severity levels (Critical, High, Medium, Low)
- File paths and line numbers
- Complexity scores for each function

### 5. Trend Analysis (`04-trend-analysis.png`)

Generate historical trends:

```bash
# Run analysis multiple times over time
./kaizen analyze --path=. --skip-churn

# Generate trend visualization
./kaizen trends --path=. --format=html --output=trends.html
```

**What to capture:**
- Time-series chart showing metrics over time
- Multiple metrics on same chart
- Trend lines (improving/degrading)
- Date labels on x-axis

### 6. Ownership Sankey (`05-ownership-sankey.png`)

Generate ownership report:

```bash
# Requires CODEOWNERS file
./kaizen owners --path=. --format=html --output=ownership.html
```

**What to capture:**
- Sankey diagram showing team flows
- Colored ribbons by team
- Node labels with team names
- Interactive highlighting (if applicable)

## Creating Professional Screenshots

### Best Practices

1. **Resolution**: Minimum 1920x1080, retina quality preferred
2. **Theme**: Use dark theme for terminal/editor (developer-friendly)
3. **Font**: Monospace fonts with ligatures (Fira Code, JetBrains Mono)
4. **Colors**: Ensure good contrast and readability
5. **Annotations**: Add subtle callouts for key features (optional)
6. **Format**: Save as PNG with appropriate compression

### Tools Recommended

- **macOS**: Built-in screenshot tools, CleanShot X, Skitch
- **Linux**: GNOME Screenshot, Flameshot, Shutter
- **Windows**: Snipping Tool, ShareX, Greenshot
- **Editing**: GIMP, Photoshop, Figma, or Preview (macOS)

### Example Workflow

```bash
# 1. Build and run demo
cd demo
./run-demo.sh

# 2. Capture terminal output
# (Use screenshot tool)

# 3. Open HTML visualizations
open outputs/sample-heatmap.html

# 4. Capture browser screenshots
# (Use screenshot tool, ensure full page capture)

# 5. Edit screenshots
# - Crop to relevant area
# - Adjust brightness/contrast
# - Add annotations if needed
# - Save as PNG

# 6. Optimize file sizes
optipng *.png  # If available
```

## Current Status

This directory is a placeholder for future screenshots. To contribute screenshots:

1. Follow the guidelines above
2. Generate high-quality captures
3. Name files according to the convention (01-06 with descriptive names)
4. Submit via pull request

## Example Screenshots to Create

Here are ideas for compelling screenshots:

### Terminal Output
- Show before/after comparison of code quality
- Highlight color-coded severity levels
- Display progress bars during analysis

### Heatmap
- Show drill-down interaction (2-3 levels deep)
- Capture tooltip with detailed metrics
- Include legend and controls

### Trends
- Show improvement over time (refactoring success story)
- Highlight specific metric changes
- Include multiple projects comparison

### Ownership
- Show complex team dependencies
- Highlight cross-team code ownership
- Display team-level quality metrics

## Need Help?

If you're creating screenshots for this project:
- Check existing open source projects for inspiration
- Use real data from analyzing Kaizen itself (dogfooding)
- Keep it simple and focused on one feature per screenshot
- Ensure text is readable at standard resolutions

## License

Screenshots in this directory are part of the Kaizen project and are licensed under the MIT License.

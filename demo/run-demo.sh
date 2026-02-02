#!/bin/bash

# Kaizen Demo Script
# This script runs a complete demonstration of Kaizen's capabilities

set -e  # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Kaizen Code Analysis Demo${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if we're in the demo directory
if [ ! -d "sample-project" ]; then
    echo -e "${YELLOW}Please run this script from the demo/ directory${NC}"
    exit 1
fi

# Step 1: Build Kaizen
echo -e "${GREEN}Step 1: Building Kaizen...${NC}"
cd ..
if [ ! -f "kaizen" ]; then
    go build -o kaizen ./cmd/kaizen
    echo -e "${GREEN}✓ Kaizen built successfully${NC}"
else
    echo -e "${GREEN}✓ Kaizen binary already exists${NC}"
fi
echo ""

# Step 2: Analyze Sample Project
echo -e "${GREEN}Step 2: Analyzing sample project...${NC}"
./kaizen analyze \
    --path=demo/sample-project \
    --skip-churn \
    --output=demo/outputs/sample-analysis.json

echo -e "${GREEN}✓ Analysis complete${NC}"
echo ""

# Step 3: Generate Visualizations
echo -e "${GREEN}Step 3: Generating visualizations...${NC}"

# HTML Heatmap
echo "  - Generating HTML heatmap..."
./kaizen visualize \
    --input=demo/outputs/sample-analysis.json \
    --format=html \
    --output=demo/outputs/sample-heatmap.html \
    --open=false

# ASCII Heatmap (display in terminal)
echo "  - Generating ASCII heatmap..."
./kaizen visualize \
    --input=demo/outputs/sample-analysis.json \
    --format=ascii

echo -e "${GREEN}✓ Visualizations generated${NC}"
echo ""

# Step 4: Display Results Summary
echo -e "${GREEN}Step 4: Analysis Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Parse JSON and display key metrics (using jq if available)
if command -v jq &> /dev/null; then
    echo "Code Health Metrics:"
    echo ""

    # Extract and display overall metrics
    cat demo/outputs/sample-analysis.json | jq -r '
        "  Total Files: \(.summary.total_files)",
        "  Total Functions: \(.summary.total_functions)",
        "  Average Complexity: \(.summary.average_complexity | tonumber | floor)",
        "  Code Health Grade: \(.code_health.grade)",
        "  Overall Score: \(.code_health.overall_score | tonumber | floor)/100",
        "",
        "Component Scores:",
        "  Complexity: \(.code_health.complexity_score | tonumber | floor)/100",
        "  Maintainability: \(.code_health.maintainability_score | tonumber | floor)/100"
    '

    echo ""
    echo "Areas of Concern:"
    cat demo/outputs/sample-analysis.json | jq -r '
        .areas_of_concern[] | "  - \(.function_name) (\(.file_path):\(.line)) - Severity: \(.severity)"
    '
else
    echo "Install jq for detailed metrics display:"
    echo "  brew install jq  (macOS)"
    echo "  apt-get install jq  (Ubuntu)"
fi

echo ""
echo -e "${BLUE}========================================${NC}"

# Step 5: Open HTML Visualizations
echo ""
echo -e "${GREEN}Step 5: Opening visualizations in browser...${NC}"

# Detect OS and open browser
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    open demo/outputs/sample-heatmap.html
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    xdg-open demo/outputs/sample-heatmap.html 2>/dev/null || echo "Please open demo/outputs/sample-heatmap.html manually"
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    # Windows
    start demo/outputs/sample-heatmap.html
fi

echo ""
echo -e "${GREEN}✓ Demo complete!${NC}"
echo ""
echo "Generated files:"
echo "  - demo/outputs/sample-analysis.json (raw data)"
echo "  - demo/outputs/sample-heatmap.html (interactive visualization)"
echo ""
echo "Next steps:"
echo "  1. Explore the interactive heatmap in your browser"
echo "  2. Review the areas of concern listed above"
echo "  3. Try running Kaizen on your own projects!"
echo ""
echo "Learn more:"
echo "  - Main README: ../README.md"
echo "  - Contributing: ../CONTRIBUTING.md"
echo "  - Demo guide: demo/README.md"
echo ""

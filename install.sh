#!/usr/bin/env bash
set -e

# Kaizen Installation Script
# Builds and installs kaizen with shell completion for zsh and fish

VERSION="latest"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
COMPLETION_DIR_ZSH="${XDG_DATA_HOME:-$HOME/.local/share}/zsh/site-functions"
COMPLETION_DIR_FISH="${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}📦 Kaizen Installation Script${NC}"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Error: Go is not installed${NC}"
    echo "Please install Go from https://go.dev/doc/install"
    exit 1
fi

echo -e "${GREEN}✓${NC} Go found: $(go version)"

# Build kaizen
echo ""
echo -e "${BLUE}🔨 Building kaizen...${NC}"
if go build -o kaizen ./cmd/kaizen; then
    echo -e "${GREEN}✓${NC} Build successful"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Create install directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Install binary
echo ""
echo -e "${BLUE}📥 Installing binary to $INSTALL_DIR...${NC}"
mv kaizen "$INSTALL_DIR/kaizen"
chmod +x "$INSTALL_DIR/kaizen"
echo -e "${GREEN}✓${NC} Binary installed"

# Check if install directory is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${YELLOW}⚠${NC}  Warning: $INSTALL_DIR is not in your PATH"
    echo "   Add this line to your ~/.zshrc or ~/.bashrc:"
    echo "   export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
fi

# Install zsh completion
if command -v zsh &> /dev/null; then
    echo ""
    echo -e "${BLUE}🐚 Installing zsh completion...${NC}"
    mkdir -p "$COMPLETION_DIR_ZSH"

    cat > "$COMPLETION_DIR_ZSH/_kaizen" << 'EOF'
#compdef kaizen

_kaizen() {
    local -a commands
    commands=(
        'analyze:Analyze a codebase and generate metrics'
        'visualize:Visualize analysis results'
        'callgraph:Generate function call graph'
        'sankey:Generate Sankey diagram of code ownership flow'
        'history:Manage historical analysis snapshots'
        'trend:Visualize metric trends over time'
        'report:Generate analysis reports'
    )

    local -a history_cmds
    history_cmds=(
        'list:List all analysis snapshots'
        'show:Display detailed snapshot information'
        'prune:Remove old snapshots'
    )

    local -a report_cmds
    report_cmds=(
        'owners:Generate code ownership report'
    )

    local -a metrics
    metrics=(
        'complexity' 'cognitive' 'churn' 'hotspot' 'length' 'maintainability'
    )

    local -a trend_metrics
    trend_metrics=(
        'overall_score' 'complexity_score' 'maintainability_score' 'churn_score'
        'avg_cyclomatic_complexity' 'avg_cognitive_complexity' 'avg_maintainability_index'
        'hotspot_count'
    )

    local -a formats
    formats=(
        'terminal' 'html' 'svg' 'ascii' 'json'
    )

    _arguments -C \
        '1: :->cmds' \
        '*::arg:->args'

    case "$state" in
        cmds)
            _describe 'command' commands
            ;;
        args)
            case "$words[1]" in
                analyze)
                    _arguments \
                        '(-p --path)'{-p,--path}'[Path to analyze]:path:_files -/' \
                        '(-s --since)'{-s,--since}'[Analyze churn since]:since:(30d 60d 90d)' \
                        '(-o --output)'{-o,--output}'[Output file path]:file:_files' \
                        '(-l --languages)'{-l,--languages}'[Languages to include]:language:(go kotlin python)' \
                        '(-e --exclude)'{-e,--exclude}'[Patterns to exclude]:pattern:' \
                        '--skip-churn[Skip git churn analysis]'
                    ;;
                visualize)
                    _arguments \
                        '(-i --input)'{-i,--input}'[Input JSON file]:file:_files -g "*.json"' \
                        '(-m --metric)'{-m,--metric}'[Metric to visualize]:metric:($metrics)' \
                        '(-f --format)'{-f,--format}'[Output format]:format:(terminal html svg)' \
                        '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                        '(-l --limit)'{-l,--limit}'[Number of top items]:limit:' \
                        '--open[Open in browser]' \
                        '--svg-width[SVG width]:width:' \
                        '--svg-height[SVG height]:height:'
                    ;;
                callgraph)
                    _arguments \
                        '(-p --path)'{-p,--path}'[Path to analyze]:path:_files -/' \
                        '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                        '(-f --format)'{-f,--format}'[Format]:format:(html svg json)' \
                        '--min-calls[Minimum call count]:calls:'
                    ;;
                sankey)
                    _arguments \
                        '(-i --input)'{-i,--input}'[Input analysis file]:file:_files -g "*.json"' \
                        '(-o --output)'{-o,--output}'[Output HTML file]:file:_files' \
                        '--min-owners[Minimum owners]:count:' \
                        '--min-calls[Minimum calls]:count:' \
                        '--open[Open in browser]'
                    ;;
                history)
                    _arguments '1: :->histcmds'
                    case "$state" in
                        histcmds)
                            _describe 'history command' history_cmds
                            ;;
                    esac
                    ;;
                trend)
                    _arguments \
                        '1:metric:($trend_metrics)' \
                        '(-d --days)'{-d,--days}'[Time range in days]:days:' \
                        '(-f --format)'{-f,--format}'[Format]:format:(ascii json html)' \
                        '(-o --output)'{-o,--output}'[Output file]:file:_files' \
                        '--folder[Show trends for folder]:folder:_files -/'
                    ;;
                report)
                    _arguments '1: :->repcmds'
                    case "$state" in
                        repcmds)
                            _describe 'report command' report_cmds
                            ;;
                    esac
                    ;;
            esac
            ;;
    esac
}

_kaizen "$@"
EOF

    echo -e "${GREEN}✓${NC} Zsh completion installed to $COMPLETION_DIR_ZSH/_kaizen"
    echo "   Run 'compinit' or restart your shell to enable completions"
fi

# Install fish completion
if command -v fish &> /dev/null; then
    echo ""
    echo -e "${BLUE}🐠 Installing fish completion...${NC}"
    mkdir -p "$COMPLETION_DIR_FISH"

    cat > "$COMPLETION_DIR_FISH/kaizen.fish" << 'EOF'
# Kaizen completions for fish

# Main commands
complete -c kaizen -f -n "__fish_use_subcommand" -a analyze -d "Analyze a codebase and generate metrics"
complete -c kaizen -f -n "__fish_use_subcommand" -a visualize -d "Visualize analysis results"
complete -c kaizen -f -n "__fish_use_subcommand" -a callgraph -d "Generate function call graph"
complete -c kaizen -f -n "__fish_use_subcommand" -a sankey -d "Generate Sankey diagram of code ownership flow"
complete -c kaizen -f -n "__fish_use_subcommand" -a history -d "Manage historical analysis snapshots"
complete -c kaizen -f -n "__fish_use_subcommand" -a trend -d "Visualize metric trends over time"
complete -c kaizen -f -n "__fish_use_subcommand" -a report -d "Generate analysis reports"

# analyze subcommand
complete -c kaizen -n "__fish_seen_subcommand_from analyze" -s p -l path -d "Path to analyze" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from analyze" -s s -l since -d "Analyze churn since" -x -a "30d 60d 90d"
complete -c kaizen -n "__fish_seen_subcommand_from analyze" -s o -l output -d "Output file path" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from analyze" -s l -l languages -d "Languages to include" -x -a "go kotlin python"
complete -c kaizen -n "__fish_seen_subcommand_from analyze" -s e -l exclude -d "Patterns to exclude" -x
complete -c kaizen -n "__fish_seen_subcommand_from analyze" -l skip-churn -d "Skip git churn analysis"

# visualize subcommand
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -s i -l input -d "Input JSON file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -s m -l metric -d "Metric to visualize" -x -a "complexity cognitive churn hotspot length maintainability"
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -s f -l format -d "Output format" -x -a "terminal html svg"
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -s o -l output -d "Output file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -s l -l limit -d "Number of top items" -x
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -l open -d "Open in browser"
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -l svg-width -d "SVG width" -x
complete -c kaizen -n "__fish_seen_subcommand_from visualize" -l svg-height -d "SVG height" -x

# callgraph subcommand
complete -c kaizen -n "__fish_seen_subcommand_from callgraph" -s p -l path -d "Path to analyze" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from callgraph" -s o -l output -d "Output file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from callgraph" -s f -l format -d "Format" -x -a "html svg json"
complete -c kaizen -n "__fish_seen_subcommand_from callgraph" -l min-calls -d "Minimum call count" -x

# sankey subcommand
complete -c kaizen -n "__fish_seen_subcommand_from sankey" -s i -l input -d "Input analysis file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from sankey" -s o -l output -d "Output HTML file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from sankey" -l min-owners -d "Minimum owners" -x
complete -c kaizen -n "__fish_seen_subcommand_from sankey" -l min-calls -d "Minimum calls" -x
complete -c kaizen -n "__fish_seen_subcommand_from sankey" -l open -d "Open in browser"

# history subcommands
complete -c kaizen -n "__fish_seen_subcommand_from history; and not __fish_seen_subcommand_from list show prune" -f -a list -d "List all analysis snapshots"
complete -c kaizen -n "__fish_seen_subcommand_from history; and not __fish_seen_subcommand_from list show prune" -f -a show -d "Display detailed snapshot information"
complete -c kaizen -n "__fish_seen_subcommand_from history; and not __fish_seen_subcommand_from list show prune" -f -a prune -d "Remove old snapshots"

complete -c kaizen -n "__fish_seen_subcommand_from history; and __fish_seen_subcommand_from list" -s l -l limit -d "Maximum snapshots to display" -x
complete -c kaizen -n "__fish_seen_subcommand_from history; and __fish_seen_subcommand_from prune" -l retention -d "Retention period in days" -x

# trend subcommand
complete -c kaizen -n "__fish_seen_subcommand_from trend" -f -a "overall_score complexity_score maintainability_score churn_score avg_cyclomatic_complexity avg_cognitive_complexity avg_maintainability_index hotspot_count"
complete -c kaizen -n "__fish_seen_subcommand_from trend" -s d -l days -d "Time range in days" -x
complete -c kaizen -n "__fish_seen_subcommand_from trend" -s f -l format -d "Format" -x -a "ascii json html"
complete -c kaizen -n "__fish_seen_subcommand_from trend" -s o -l output -d "Output file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from trend" -l folder -d "Show trends for folder" -r -F

# report subcommands
complete -c kaizen -n "__fish_seen_subcommand_from report; and not __fish_seen_subcommand_from owners" -f -a owners -d "Generate code ownership report"

complete -c kaizen -n "__fish_seen_subcommand_from report; and __fish_seen_subcommand_from owners" -s c -l codeowners -d "Path to CODEOWNERS file" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from report; and __fish_seen_subcommand_from owners" -s f -l format -d "Output format" -x -a "ascii json html"
complete -c kaizen -n "__fish_seen_subcommand_from report; and __fish_seen_subcommand_from owners" -s o -l output -d "Output file path" -r -F
complete -c kaizen -n "__fish_seen_subcommand_from report; and __fish_seen_subcommand_from owners" -l open -d "Open HTML in browser"
EOF

    echo -e "${GREEN}✓${NC} Fish completion installed to $COMPLETION_DIR_FISH/kaizen.fish"
    echo "   Completions will be available in new fish sessions"
fi

echo ""
echo -e "${GREEN}✨ Installation complete!${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "  1. Make sure $INSTALL_DIR is in your PATH"
echo "  2. Run 'kaizen --help' to get started"
echo "  3. Try 'kaizen analyze --path=.' to analyze your project"
echo ""
echo -e "${BLUE}Shell completion:${NC}"
if command -v zsh &> /dev/null; then
    echo "  • Zsh: Run 'exec zsh' or 'compinit' to reload completions"
fi
if command -v fish &> /dev/null; then
    echo "  • Fish: Completions available in new sessions"
fi
echo ""

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/analyzer"
	"github.com/alexcollie/kaizen/pkg/churn"
	"github.com/alexcollie/kaizen/pkg/languages"
	"github.com/alexcollie/kaizen/pkg/languages/golang"
	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/alexcollie/kaizen/pkg/ownership"
	"github.com/alexcollie/kaizen/pkg/storage"
	"github.com/alexcollie/kaizen/pkg/trending"
	"github.com/alexcollie/kaizen/pkg/visualization"
)

var (
	// Analyze flags
	rootPath         string
	sinceStr         string
	outputFile       string
	includeLanguages []string
	excludePatterns  []string
	skipChurn        bool

	// Visualize flags
	inputFile    string
	metric       string
	topLimit     int
	outputFormat string
	htmlOutput   string
	svgOutput    string
	svgWidth     int
	svgHeight    int
	openBrowser  bool

	// History flags
	historyLimit int

	// Trend flags
	trendDays    int
	trendFolder  string
	trendFormat  string
	trendOutput  string
	trendOpen    bool

	// Report flags
	reportSnapshotID int64
	reportFormat     string
	reportOutput     string
	reportOpen       bool
	reportCodeOwnersPath string

	// Callgraph flags
	callgraphPath   string
	callgraphOutput string
	callgraphFormat string
	saveJSON        bool
	minCalls        int
)

var rootCmd = &cobra.Command{
	Use:   "kaizen",
	Short: "Code analysis tool for measuring code quality and churn",
	Long: `Kaizen analyzes your codebase to identify:
  - Code complexity (cyclomatic, cognitive)
  - Function length and nesting depth
  - Code churn from git history
  - Hotspots (high churn + high complexity)

Generates heat maps to visualize code health by folder.`,
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Manage historical analysis snapshots",
	Long:  `View, list, and manage historical code analysis snapshots.`,
}

var trendCmd = &cobra.Command{
	Use:   "trend <metric>",
	Short: "Visualize metric trends over time",
	Long: `Visualize how code metrics have changed over time.

Supported metrics:
  - overall_score: Overall code health score
  - complexity_score: Code complexity score
  - maintainability_score: Code maintainability
  - churn_score: Code churn/volatility
  - avg_cyclomatic_complexity: Average cyclomatic complexity
  - avg_cognitive_complexity: Average cognitive complexity
  - avg_maintainability_index: Average maintainability index
  - hotspot_count: Number of hotspots

Examples:
  kaizen trend overall_score
  kaizen trend complexity_score --days=30
  kaizen trend complexity_score --format=json`,
	Args: cobra.ExactArgs(1),
	Run:  runTrend,
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate analysis reports",
	Long:  `Generate reports from analysis snapshots.`,
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze a codebase and generate metrics",
	Long: `Analyzes source code files and generates comprehensive metrics including:
  - Cyclomatic and cognitive complexity
  - Function length and parameter counts
  - Code churn from git history
  - Maintainability index
  - Identifies hotspots

Results are saved to a JSON file for visualization.`,
	Run: runAnalyze,
}

var visualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Visualize analysis results",
	Long: `Generates visualizations from analysis results:
  - Terminal heat map with color coding
  - Top hotspots list
  - Folder breakdown by metric

Supported metrics: complexity, cognitive, churn, hotspot, length, maintainability`,
	Run: runVisualize,
}

var callgraphCmd = &cobra.Command{
	Use:   "callgraph",
	Short: "Generate function call graph",
	Long: `Analyzes Go code to build a function call graph showing:
  - Function call relationships (who calls whom)
  - Call frequency (fan-in and fan-out)
  - Function complexity and size
  - Interactive D3.js force-directed graph visualization

Node size represents how often a function is called.
Node color represents complexity or other metrics.`,
	Run: runCallGraph,
}

func init() {
	// Add commands
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(visualizeCmd)
	rootCmd.AddCommand(callgraphCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(trendCmd)
	rootCmd.AddCommand(reportCmd)

	// Report subcommands
	reportOwnersCmd := &cobra.Command{
		Use:   "owners [snapshot-id]",
		Short: "Generate code ownership report",
		Run:   runReportOwners,
	}
	reportCmd.AddCommand(reportOwnersCmd)

	// Report flags
	reportOwnersCmd.Flags().StringVarP(&reportCodeOwnersPath, "codeowners", "c", "", "Path to CODEOWNERS file (auto-detected if not specified)")
	reportOwnersCmd.Flags().StringVarP(&reportFormat, "format", "f", "ascii", "Output format (ascii, json, html)")
	reportOwnersCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "Output file path")
	reportOwnersCmd.Flags().BoolVar(&reportOpen, "open", true, "Open HTML in browser (format=html only)")

	// History subcommands
	historyListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all analysis snapshots",
		Run:   runHistoryList,
	}
	historyShowCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display detailed snapshot information",
		Args:  cobra.ExactArgs(1),
		Run:   runHistoryShow,
	}
	historyPruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove old snapshots",
		Run:   runHistoryPrune,
	}
	historyCmd.AddCommand(historyListCmd)
	historyCmd.AddCommand(historyShowCmd)
	historyCmd.AddCommand(historyPruneCmd)

	// History flags
	historyListCmd.Flags().IntVarP(&historyLimit, "limit", "l", 20, "Maximum snapshots to display")
	historyPruneCmd.Flags().IntVar(&historyLimit, "retention", 90, "Retention period in days")

	// Analyze flags
	analyzeCmd.Flags().StringVarP(&rootPath, "path", "p", ".", "Path to analyze")
	analyzeCmd.Flags().StringVarP(&sinceStr, "since", "s", "90d", "Analyze churn since (e.g., 30d, 2024-01-01)")
	analyzeCmd.Flags().StringVarP(&outputFile, "output", "o", "kaizen-results.json", "Output file path")
	analyzeCmd.Flags().StringSliceVarP(&includeLanguages, "languages", "l", []string{}, "Languages to include (default: all)")
	analyzeCmd.Flags().StringSliceVarP(&excludePatterns, "exclude", "e", []string{"vendor", "node_modules", "*_test.go"}, "Patterns to exclude")
	analyzeCmd.Flags().BoolVar(&skipChurn, "skip-churn", false, "Skip git churn analysis")

	// Visualize flags
	visualizeCmd.Flags().StringVarP(&inputFile, "input", "i", "kaizen-results.json", "Input JSON file")
	visualizeCmd.Flags().StringVarP(&metric, "metric", "m", "hotspot", "Metric to visualize (complexity, cognitive, churn, hotspot, length, maintainability)")
	visualizeCmd.Flags().IntVarP(&topLimit, "limit", "l", 10, "Number of top hotspots to show")
	visualizeCmd.Flags().StringVarP(&outputFormat, "format", "f", "terminal", "Output format (terminal, html, svg)")
	visualizeCmd.Flags().StringVarP(&htmlOutput, "output", "o", "kaizen-heatmap.html", "HTML/SVG output file")
	visualizeCmd.Flags().IntVar(&svgWidth, "svg-width", 1200, "SVG width in pixels")
	visualizeCmd.Flags().IntVar(&svgHeight, "svg-height", 800, "SVG height in pixels")
	visualizeCmd.Flags().BoolVar(&openBrowser, "open", true, "Open HTML in browser automatically")

	// Trend flags
	trendCmd.Flags().IntVarP(&trendDays, "days", "d", 90, "Number of days to show (0 = all)")
	trendCmd.Flags().StringVar(&trendFolder, "folder", "", "Show metrics for specific folder")
	trendCmd.Flags().StringVarP(&trendFormat, "format", "f", "ascii", "Output format (ascii, json, html)")
	trendCmd.Flags().StringVarP(&trendOutput, "output", "o", "", "Output file path (required for json/html, optional for ascii)")
	trendCmd.Flags().BoolVar(&trendOpen, "open", true, "Open HTML in browser (format=html only)")

	// Callgraph flags
	callgraphCmd.Flags().StringVarP(&callgraphPath, "path", "p", ".", "Path to analyze")
	callgraphCmd.Flags().StringVarP(&callgraphOutput, "output", "o", "kaizen-callgraph.html", "Output file path")
	callgraphCmd.Flags().StringVarP(&callgraphFormat, "format", "f", "html", "Output format (html, svg, json)")
	callgraphCmd.Flags().IntVar(&svgWidth, "svg-width", 1600, "SVG width in pixels")
	callgraphCmd.Flags().IntVar(&svgHeight, "svg-height", 1000, "SVG height in pixels")
	callgraphCmd.Flags().BoolVar(&openBrowser, "open", true, "Open HTML in browser automatically")
	callgraphCmd.Flags().BoolVar(&saveJSON, "save-json", false, "Also save call graph data as JSON")
	callgraphCmd.Flags().IntVar(&minCalls, "min-calls", 0, "Minimum call count to include a function (filters noise)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAnalyze(cmd *cobra.Command, args []string) {
	fmt.Printf("🔍 Kaizen Code Analysis\n\n")
	fmt.Printf("Analyzing: %s\n", rootPath)

	// Load configuration
	cfg, err := config.LoadConfig(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Check if .kaizenignore exists
	kaizenIgnorePath := filepath.Join(rootPath, ".kaizenignore")
	if _, err := os.Stat(kaizenIgnorePath); err == nil {
		fmt.Printf("📋 Using .kaizenignore (%d patterns)\n", len(cfg.IgnorePatterns))
	}

	// Check if .kaizen.yaml exists
	kaizenYamlPath := filepath.Join(rootPath, ".kaizen.yaml")
	if _, err := os.Stat(kaizenYamlPath); err == nil {
		fmt.Printf("⚙️  Using .kaizen.yaml config\n")
	}

	// Parse since time (CLI overrides config)
	sinceValue := sinceStr
	if sinceValue == "90d" && cfg.Analysis.Since != "" {
		sinceValue = cfg.Analysis.Since
	}

	since, err := parseSinceTime(sinceValue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing --since: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Churn since: %s\n", since.Format("2006-01-02"))
	fmt.Printf("Output: %s\n\n", outputFile)

	// Merge CLI exclude patterns with config patterns
	allExcludePatterns := cfg.GetExcludePatterns()
	if len(excludePatterns) > 0 {
		allExcludePatterns = append(allExcludePatterns, excludePatterns...)
	}

	// Merge CLI languages with config languages
	allLanguages := cfg.Analysis.Languages
	if len(includeLanguages) > 0 {
		allLanguages = includeLanguages
	}

	// CLI skip-churn overrides config
	shouldSkipChurn := skipChurn || cfg.Analysis.SkipChurn

	// Create components
	registry := languages.NewRegistry()
	churnAnalyzer := churn.NewGitChurnAnalyzer(rootPath)
	aggregator := analyzer.NewAggregator()
	pipeline := analyzer.NewPipeline(registry, churnAnalyzer, aggregator)

	// Configure analysis options
	options := analyzer.AnalysisOptions{
		RootPath:         rootPath,
		Since:            since,
		IncludeLanguages: allLanguages,
		ExcludePatterns:  allExcludePatterns,
		IncludeChurn:     !shouldSkipChurn,
		MaxWorkers:       cfg.Analysis.MaxWorkers,
		ProgressCallback: func(file string, current int, total int) {
			fmt.Printf("\r[%d/%d] Analyzing: %s", current, total, truncate(file, 60))
		},
	}

	// Run analysis
	result, err := pipeline.Analyze(options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n\nError during analysis: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n\n✅ Analysis complete!\n\n")

	// Print summary
	printSummary(result)

	// Create storage backend with auto-detection
	dbPath, err := storage.DetectOrCreateDatabase(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not setup database: %v\n", err)
	} else {
		storageBackend, err := storage.NewBackend(storage.BackendConfig{
			Type: "sqlite",
			Path: dbPath,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create storage backend: %v\n", err)
		} else {
			defer storageBackend.Close()

			// Save to database
			metadata := storage.SnapshotMetadata{
				KaizenVersion: "1.0.0", // TODO: Use actual version
			}

			snapshotID, err := storageBackend.Save(result, metadata)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save to database: %v\n", err)
			} else {
				fmt.Printf("💾 Saved to database (ID: %d)\n", snapshotID)

				// Try to save ownership data if CODEOWNERS exists
				codeownersPath := findCodeOwnersFile(rootPath)
				if codeownersPath != "" {
					codeowners, err := ownership.ParseCodeOwners(codeownersPath)
					if err == nil {
						aggregator := ownership.NewAggregator(codeowners)
						ownerMetrics, fileOwnership := aggregator.AggregateByOwner(result)

						// Convert map to slice
						var metrics []storage.OwnerMetric
						for _, m := range ownerMetrics {
							metrics = append(metrics, storage.OwnerMetric{
								Owner:                       m.Owner,
								FileCount:                   m.FileCount,
								FunctionCount:               m.FunctionCount,
								AvgCyclomaticComplexity:     m.AvgCyclomaticComplexity,
								AvgMaintainabilityIndex:     m.AvgMaintainabilityIndex,
								HotspotCount:                m.HotspotCount,
								OverallHealthScore:          m.OverallHealthScore,
							})
						}

						err = storageBackend.SaveOwnershipData(snapshotID, fileOwnership, metrics, result.AnalyzedAt)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Warning: could not save ownership data: %v\n", err)
						} else {
							fmt.Printf("👥 Saved ownership data for %d owner(s)\n", len(ownerMetrics))
						}
					}
				}
			}
		}
	}

	// Save results to JSON file
	err = saveResults(result, outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving results: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("💾 Results saved to: %s\n", outputFile)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  kaizen visualize --input=%s --metric=hotspot\n", outputFile)
}

func parseSinceTime(sinceStr string) (time.Time, error) {
	// Try parsing as duration (e.g., "30d", "90d")
	if len(sinceStr) > 1 && sinceStr[len(sinceStr)-1] == 'd' {
		days := sinceStr[:len(sinceStr)-1]
		var daysInt int
		_, err := fmt.Sscanf(days, "%d", &daysInt)
		if err == nil {
			return time.Now().AddDate(0, 0, -daysInt), nil
		}
	}

	// Try parsing as date (e.g., "2024-01-01")
	parsedTime, err := time.Parse("2006-01-02", sinceStr)
	if err == nil {
		return parsedTime, nil
	}

	return time.Time{}, fmt.Errorf("invalid --since format (use '30d' or '2024-01-01')")
}

func printSummary(result *models.AnalysisResult) {
	summary := result.Summary

	fmt.Printf("📊 Summary:\n")
	fmt.Printf("  Files analyzed:     %d\n", summary.TotalFiles)
	fmt.Printf("  Total functions:    %d\n", summary.TotalFunctions)
	fmt.Printf("  Total lines:        %d\n", summary.TotalLines)
	fmt.Printf("  Code lines:         %d\n\n", summary.TotalCodeLines)

	fmt.Printf("📈 Averages:\n")
	fmt.Printf("  Cyclomatic complexity: %.1f\n", summary.AverageCyclomaticComplexity)
	fmt.Printf("  Cognitive complexity:  %.1f\n", summary.AverageCognitiveComplexity)
	fmt.Printf("  Function length:       %.1f lines\n", summary.AverageFunctionLength)
	fmt.Printf("  Maintainability index: %.1f\n\n", summary.AverageMaintainabilityIndex)

	fmt.Printf("⚠️  Issues:\n")
	fmt.Printf("  High complexity (>10):      %d\n", summary.HighComplexityCount)
	fmt.Printf("  Very high complexity (>20): %d\n", summary.VeryHighComplexityCount)
	fmt.Printf("  Long functions (>50):       %d\n", summary.LongFunctionCount)
	fmt.Printf("  Very long functions (>100): %d\n", summary.VeryLongFunctionCount)
	fmt.Printf("  🔥 Hotspots:                %d\n", summary.HotspotCount)

	// Print score report if available
	if result.ScoreReport != nil {
		printScoreReport(result.ScoreReport)
	}
}

func printScoreReport(report *models.ScoreReport) {
	fmt.Printf("\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Code Health Report\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Print grade with color coding
	gradeColor := getGradeColor(report.OverallGrade)
	fmt.Printf("Overall Grade: %s%s%s (%.0f/100)\n\n", gradeColor, report.OverallGrade, colorReset, report.OverallScore)

	// Print component scores
	fmt.Printf("Component Scores:\n")
	printComponentScore("Complexity", report.ComponentScores.Complexity)
	printComponentScore("Maintainability", report.ComponentScores.Maintainability)
	if report.HasChurnData {
		printComponentScore("Churn", report.ComponentScores.Churn)
	} else {
		fmt.Printf("  %-17s %s (no churn data)\n", "Churn:", "N/A")
	}
	printComponentScore("Function Size", report.ComponentScores.FunctionSize)
	printComponentScore("Code Structure", report.ComponentScores.CodeStructure)
	fmt.Printf("\n")

	// Print concerns
	printConcerns(report.Concerns)
}

func printComponentScore(name string, score models.CategoryScore) {
	barWidth := 10
	filled := int(score.Score / 10)
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	color := getScoreColor(score.Score)
	fmt.Printf("  %-17s %s%s%s %.0f/100 (%s)\n", name+":", color, bar, colorReset, score.Score, score.Category)
}

func printConcerns(concerns []models.Concern) {
	if len(concerns) == 0 {
		fmt.Printf("✨ No concerns detected\n")
		return
	}

	// Group concerns by severity
	criticalConcerns := filterConcernsBySeverity(concerns, "critical")
	warningConcerns := filterConcernsBySeverity(concerns, "warning")
	infoConcerns := filterConcernsBySeverity(concerns, "info")

	totalConcerns := len(criticalConcerns) + len(warningConcerns) + len(infoConcerns)
	fmt.Printf("Areas of Concern (%d):\n", totalConcerns)

	// Print critical concerns
	for _, concern := range criticalConcerns {
		printConcern(concern, colorRed, "CRITICAL")
	}

	// Print warning concerns
	for _, concern := range warningConcerns {
		printConcern(concern, colorYellow, "WARNING")
	}

	// Print info concerns
	for _, concern := range infoConcerns {
		printConcern(concern, colorCyan, "INFO")
	}
}

func printConcern(concern models.Concern, color string, label string) {
	fmt.Printf("\n  %s[%s]%s %s\n", color, label, colorReset, concern.Title)
	fmt.Printf("    %s\n", concern.Description)

	for _, item := range concern.AffectedItems {
		location := item.FilePath
		if item.Line > 0 {
			location = fmt.Sprintf("%s:%d", item.FilePath, item.Line)
		}
		if item.FunctionName != "" {
			fmt.Printf("    - %s (%s)\n", location, item.FunctionName)
		} else {
			fmt.Printf("    - %s\n", location)
		}
	}
}

func filterConcernsBySeverity(concerns []models.Concern, severity string) []models.Concern {
	var filtered []models.Concern
	for _, concern := range concerns {
		if concern.Severity == severity {
			filtered = append(filtered, concern)
		}
	}
	return filtered
}

func getGradeColor(grade string) string {
	switch grade {
	case "A":
		return colorGreen
	case "B":
		return colorBlue
	case "C":
		return colorYellow
	case "D":
		return colorOrange
	case "F":
		return colorRed
	default:
		return colorReset
	}
}

func getScoreColor(score float64) string {
	switch {
	case score >= 90:
		return colorGreen
	case score >= 75:
		return colorBlue
	case score >= 60:
		return colorYellow
	case score >= 40:
		return colorOrange
	default:
		return colorRed
	}
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorOrange = "\033[38;5;208m"
)

func saveResults(result *models.AnalysisResult, filename string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return "..." + str[len(str)-maxLen+3:]
}

func runVisualize(cmd *cobra.Command, args []string) {
	fmt.Printf("📊 Kaizen Visualization\n\n")

	// Load results
	data, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	var result models.AnalysisResult
	err = json.Unmarshal(data, &result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Handle different output formats
	switch outputFormat {
	case "html":
		generateHTMLOutput(&result)
	case "svg":
		generateSVGOutput(&result)
	case "terminal":
		generateTerminalOutput(&result)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s (use 'terminal', 'html', or 'svg')\n", outputFormat)
		os.Exit(1)
	}
}

func generateTerminalOutput(result *models.AnalysisResult) {
	// Create visualizer
	visualizer := visualization.NewTerminalVisualizer()

	// Render heat map
	heatMap := visualizer.RenderHeatMap(result, metric)
	fmt.Print(heatMap)

	// Render top hotspots
	if result.Summary.HotspotCount > 0 {
		hotspots := visualizer.RenderTopHotspots(result, topLimit)
		fmt.Print(hotspots)
	}
}

func generateHTMLOutput(result *models.AnalysisResult) {
	// Create HTML visualizer
	htmlVisualizer := visualization.NewHTMLVisualizer()

	// Generate HTML
	html, err := htmlVisualizer.GenerateHTML(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating HTML: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	err = os.WriteFile(htmlOutput, []byte(html), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing HTML file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ HTML heat map generated: %s\n", htmlOutput)

	// Open in browser
	if openBrowser {
		fmt.Printf("🌐 Opening in browser...\n")
		err = openInBrowser(htmlOutput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", err)
			fmt.Printf("Please open the file manually: %s\n", htmlOutput)
		}
	} else {
		fmt.Printf("\nTo view the heat map, open: %s\n", htmlOutput)
	}
}

func generateSVGOutput(result *models.AnalysisResult) {
	// Determine output filename
	outputFilename := htmlOutput
	if outputFilename == "kaizen-heatmap.html" {
		outputFilename = "kaizen-heatmap.svg"
	}
	// Allow user to override via --output flag
	if strings.HasSuffix(htmlOutput, ".html") {
		outputFilename = strings.TrimSuffix(htmlOutput, ".html") + ".svg"
	}

	// Create SVG visualizer
	svgVisualizer := visualization.NewSVGVisualizer(svgWidth, svgHeight)

	// Generate SVG
	svg, err := svgVisualizer.GenerateSVG(result, metric)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating SVG: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	err = os.WriteFile(outputFilename, []byte(svg), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing SVG file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ SVG heat map generated: %s\n", outputFilename)
	fmt.Printf("   Dimensions: %dx%d pixels\n", svgWidth, svgHeight)
	fmt.Printf("   Metric: %s\n", metric)
	fmt.Printf("\nOpen the file in a browser or image viewer to view the heat map.\n")
}

// openInBrowser opens a file in the default browser (cross-platform)
func openInBrowser(filename string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return err
	}

	// Platform-specific commands
	var command string
	var args []string

	switch runtime.GOOS {
	case "darwin": // macOS
		command = "open"
		args = []string{absPath}
	case "windows":
		command = "cmd"
		args = []string{"/c", "start", absPath}
	default: // linux, freebsd, etc.
		command = "xdg-open"
		args = []string{absPath}
	}

	cmd := exec.Command(command, args...)
	return cmd.Start()
}

func findCodeOwnersFile(rootPath string) string {
	// Check common locations
	locations := []string{
		filepath.Join(rootPath, ".github", "CODEOWNERS"),
		filepath.Join(rootPath, "CODEOWNERS"),
		filepath.Join(rootPath, ".gitlab", "CODEOWNERS"),
		filepath.Join(rootPath, ".gitea", "CODEOWNERS"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

func runReportOwners(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not get current directory: %v\n", err)
		os.Exit(1)
	}

	// Determine snapshot ID
	var snapshotID int64
	if len(args) > 0 {
		_, err := fmt.Sscanf(args[0], "%d", &snapshotID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid snapshot ID: %v\n", err)
			os.Exit(1)
		}
	}

	// Create storage backend
	dbPath, err := storage.DetectOrCreateDatabase(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not locate database: %v\n", err)
		os.Exit(1)
	}

	backend, err := storage.NewBackend(storage.BackendConfig{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open database: %v\n", err)
		os.Exit(1)
	}
	defer backend.Close()

	// Get snapshot
	var snapshot *models.AnalysisResult
	if snapshotID > 0 {
		snapshot, err = backend.GetByID(snapshotID)
	} else {
		snapshot, err = backend.GetLatest()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not retrieve snapshot: %v\n", err)
		os.Exit(1)
	}

	// Find CODEOWNERS file
	codeownersPath := reportCodeOwnersPath
	if codeownersPath == "" {
		codeownersPath = findCodeOwnersFile(cwd)
	}

	if codeownersPath == "" {
		fmt.Fprintf(os.Stderr, "Error: CODEOWNERS file not found (specify with --codeowners)\n")
		os.Exit(1)
	}

	// Parse CODEOWNERS
	codeowners, err := ownership.ParseCodeOwners(codeownersPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not parse CODEOWNERS: %v\n", err)
		os.Exit(1)
	}

	// Generate report
	aggregator := ownership.NewAggregator(codeowners)
	report := aggregator.GetOwnerReport(snapshot, snapshotID, snapshot.AnalyzedAt.Format("2006-01-02 15:04:05"))

	// Render output
	switch reportFormat {
	case "ascii":
		fmt.Print(ownership.RenderOwnerReportASCII(report))
	case "json":
		renderReportJSON(report, reportOutput)
	case "html":
		renderReportHTML(report, reportOutput, reportOpen)
	default:
		fmt.Fprintf(os.Stderr, "Error: unsupported format '%s'\n", reportFormat)
		os.Exit(1)
	}
}

func renderReportJSON(report *ownership.OwnerReport, outputPath string) {
	jsonStr, err := ownership.RenderOwnerReportJSON(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not format JSON: %v\n", err)
		os.Exit(1)
	}

	if outputPath == "" {
		fmt.Println(jsonStr)
	} else {
		err := os.WriteFile(outputPath, []byte(jsonStr), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not write file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Exported to: %s\n", outputPath)
	}
}

func renderReportHTML(report *ownership.OwnerReport, outputPath string, open bool) {
	html, err := ownership.RenderOwnerReportHTML(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not generate report: %v\n", err)
		os.Exit(1)
	}

	if outputPath == "" {
		outputPath = "kaizen-owners-report.html"
	}

	err = os.WriteFile(outputPath, []byte(html), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not write file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ HTML report generated: %s\n", outputPath)

	if open {
		fmt.Printf("🌐 Opening in browser...\n")
		err = openInBrowser(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", err)
			fmt.Printf("Please open the file manually: %s\n", outputPath)
		}
	}
}

func runHistoryList(cmd *cobra.Command, args []string) {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not get current directory: %v\n", err)
		os.Exit(1)
	}

	// Create storage backend
	dbPath, err := storage.DetectOrCreateDatabase(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not locate database: %v\n", err)
		os.Exit(1)
	}

	backend, err := storage.NewBackend(storage.BackendConfig{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open database: %v\n", err)
		os.Exit(1)
	}
	defer backend.Close()

	// Get snapshots
	snapshots, err := backend.ListSnapshots(historyLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not retrieve snapshots: %v\n", err)
		os.Exit(1)
	}

	if len(snapshots) == 0 {
		fmt.Println("No analysis snapshots found")
		return
	}

	// Print header
	fmt.Printf("\n📋 Analysis Snapshots (%d)\n", len(snapshots))
	fmt.Println("─────────────────────────────────────────────────────────────────────────────")
	fmt.Printf("%-4s │ %-19s │ %-8s │ %-8s │ %-5s │ %-7s │ %s\n",
		"ID", "Date", "Grade", "Score", "Files", "Funcs", "Commit")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────")

	// Print snapshots
	for _, snap := range snapshots {
		commit := snap.GitCommitHash
		if len(commit) > 7 {
			commit = commit[:7]
		}
		if commit == "" {
			commit = "-"
		}

		fmt.Printf("%-4d │ %s │ %-8s │ %7.1f │ %-5d │ %-7d │ %s\n",
			snap.ID,
			snap.AnalyzedAt.Format("2006-01-02 15:04:05"),
			snap.OverallGrade,
			snap.OverallScore,
			snap.TotalFiles,
			snap.TotalFunctions,
			commit,
		)
	}
	fmt.Println()
}

func runHistoryShow(cmd *cobra.Command, args []string) {
	// Parse ID
	var snapshotID int64
	_, err := fmt.Sscanf(args[0], "%d", &snapshotID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid snapshot ID: %v\n", err)
		os.Exit(1)
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not get current directory: %v\n", err)
		os.Exit(1)
	}

	// Create storage backend
	dbPath, err := storage.DetectOrCreateDatabase(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not locate database: %v\n", err)
		os.Exit(1)
	}

	backend, err := storage.NewBackend(storage.BackendConfig{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open database: %v\n", err)
		os.Exit(1)
	}
	defer backend.Close()

	// Get snapshot
	summary, err := backend.GetByIDSummary(snapshotID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not retrieve snapshot: %v\n", err)
		os.Exit(1)
	}

	// Print snapshot details
	fmt.Printf("\n📊 Snapshot #%d\n\n", summary.ID)
	fmt.Printf("Analyzed At:              %s\n", summary.AnalyzedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Git Commit:               %s\n", summary.GitCommitHash)
	fmt.Printf("Git Branch:               %s\n", summary.GitBranch)
	fmt.Printf("\nMetrics:\n")
	fmt.Printf("  Overall Grade:          %s\n", summary.OverallGrade)
	fmt.Printf("  Overall Score:          %.1f/100\n", summary.OverallScore)
	fmt.Printf("  Complexity Score:       %.1f/100\n", summary.ComplexityScore)
	fmt.Printf("  Maintainability Score:  %.1f/100\n", summary.MaintainabilityScore)
	fmt.Printf("  Churn Score:            %.1f/100\n", summary.ChurnScore)
	fmt.Printf("\nCode Metrics:\n")
	fmt.Printf("  Total Files:            %d\n", summary.TotalFiles)
	fmt.Printf("  Total Functions:        %d\n", summary.TotalFunctions)
	fmt.Printf("  Avg Cyclomatic:         %.1f\n", summary.AvgCyclomaticComplexity)
	fmt.Printf("  Avg Maintainability:    %.1f\n", summary.AvgMaintainabilityIndex)
	fmt.Printf("  Hotspot Count:          %d\n", summary.HotspotCount)
	fmt.Println()
}

func runHistoryPrune(cmd *cobra.Command, args []string) {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not get current directory: %v\n", err)
		os.Exit(1)
	}

	// Create storage backend
	dbPath, err := storage.DetectOrCreateDatabase(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not locate database: %v\n", err)
		os.Exit(1)
	}

	backend, err := storage.NewBackend(storage.BackendConfig{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open database: %v\n", err)
		os.Exit(1)
	}
	defer backend.Close()

	// Prune old snapshots
	deleted, err := backend.Prune(historyLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not prune snapshots: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Removed %d snapshot(s) older than %d days\n", deleted, historyLimit)
}

func runTrend(cmd *cobra.Command, args []string) {
	metricName := args[0]

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not get current directory: %v\n", err)
		os.Exit(1)
	}

	// Create storage backend
	dbPath, err := storage.DetectOrCreateDatabase(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not locate database: %v\n", err)
		os.Exit(1)
	}

	backend, err := storage.NewBackend(storage.BackendConfig{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open database: %v\n", err)
		os.Exit(1)
	}
	defer backend.Close()

	// Calculate time range
	endTime := time.Now()
	var startTime time.Time
	if trendDays > 0 {
		startTime = endTime.AddDate(0, 0, -trendDays)
	} else {
		startTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	// Get time-series data
	points, err := backend.GetTimeSeries(metricName, trendFolder, startTime, endTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not retrieve metric data: %v\n", err)
		os.Exit(1)
	}

	if len(points) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no data available for metric '%s'\n", metricName)
		os.Exit(1)
	}

	// Handle output based on format
	switch trendFormat {
	case "ascii":
		renderTrendASCII(metricName, trendFolder, points)
	case "json":
		renderTrendJSON(metricName, trendFolder, points, trendOutput)
	case "html":
		renderTrendHTML(metricName, trendFolder, points, trendOutput, trendOpen)
	default:
		fmt.Fprintf(os.Stderr, "Error: unsupported format '%s'\n", trendFormat)
		os.Exit(1)
	}
}

func renderTrendASCII(metricName, folder string, points []storage.TimeSeriesPoint) {
	output := trending.RenderASCIIChart(metricName, points, folder)
	fmt.Print(output)
}

func renderTrendJSON(metricName, folder string, points []storage.TimeSeriesPoint, outputPath string) {
	export, err := trending.ExportToJSON(metricName, folder, points)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not export data: %v\n", err)
		os.Exit(1)
	}

	// If no output file specified, print to stdout
	if outputPath == "" {
		jsonStr, err := trending.JSONToString(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not format JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonStr)
	} else {
		// Write to file
		err := trending.WriteJSONToFile(export, outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not write file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Exported to: %s\n", outputPath)
	}
}

func renderTrendHTML(metricName, folder string, points []storage.TimeSeriesPoint, outputPath string, open bool) {
	html, err := trending.RenderHTMLChart(metricName, points, folder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not generate chart: %v\n", err)
		os.Exit(1)
	}

	// Determine output file
	if outputPath == "" {
		outputPath = trending.FormatChartFilename(metricName)
	}

	err = trending.WriteHTMLToFile(html, outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not write file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ HTML chart generated: %s\n", outputPath)

	if open {
		fmt.Printf("🌐 Opening in browser...\n")
		err = openInBrowser(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", err)
			fmt.Printf("Please open the file manually: %s\n", outputPath)
		}
	}
}

func runCallGraph(cmd *cobra.Command, args []string) {
	fmt.Printf("🔗 Kaizen Call Graph Analysis\n\n")
	fmt.Printf("Analyzing: %s\n\n", callgraphPath)

	// Create call graph analyzer
	analyzer := golang.NewCallGraphAnalyzer()

	// Analyze directory
	graph, err := analyzer.AnalyzeDirectory(callgraphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing call graph: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Call graph analysis complete!\n\n")
	printCallGraphSummary(graph)

	// Apply min-calls filter if specified
	if minCalls > 0 {
		originalCount := len(graph.Nodes)
		graph = graph.FilterByMinCalls(minCalls)
		fmt.Printf("🔍 Filtered to functions with >= %d calls\n", minCalls)
		fmt.Printf("   %d → %d functions\n\n", originalCount, len(graph.Nodes))
	}

	// Save JSON if requested
	if saveJSON || callgraphFormat == "json" {
		jsonFilename := "kaizen-callgraph.json"
		if callgraphFormat == "json" {
			jsonFilename = callgraphOutput
		}

		jsonData, err := json.MarshalIndent(graph, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling call graph: %v\n", err)
			os.Exit(1)
		}

		err = os.WriteFile(jsonFilename, jsonData, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("💾 Call graph data saved to: %s\n", jsonFilename)
	}

	// Generate visualization based on format
	switch callgraphFormat {
	case "html":
		generateCallGraphHTML(graph)
	case "svg":
		generateCallGraphSVG(graph)
	case "json":
		// Already handled above
		fmt.Printf("\nTo visualize, use:\n")
		fmt.Printf("  kaizen callgraph --format=html\n")
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s (use 'html', 'svg', or 'json')\n", callgraphFormat)
		os.Exit(1)
	}
}

func printCallGraphSummary(graph *models.CallGraph) {
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("  Total functions:    %d\n", graph.Stats.TotalFunctions)
	fmt.Printf("  Total calls:        %d\n", graph.Stats.TotalCalls)
	fmt.Printf("  Total edges:        %d\n", graph.Stats.TotalEdges)
	fmt.Printf("  Average calls/func: %.1f\n\n", graph.Stats.AvgCallsPerFunc)

	fmt.Printf("📈 Statistics:\n")
	fmt.Printf("  Max fan-in:         %d (%s)\n", graph.Stats.MaxFanIn, graph.Stats.MostCalledFunc)
	fmt.Printf("  Max fan-out:        %d\n", graph.Stats.MaxFanOut)
	fmt.Printf("  Unreachable funcs:  %d\n\n", graph.Stats.UnreachableFuncs)
}

func generateCallGraphHTML(graph *models.CallGraph) {
	outputFilename := callgraphOutput
	if !strings.HasSuffix(outputFilename, ".html") {
		outputFilename += ".html"
	}

	err := visualization.GenerateCallGraphHTML(graph, outputFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating HTML: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Interactive call graph generated: %s\n", outputFilename)

	// Open in browser
	if openBrowser {
		fmt.Printf("🌐 Opening in browser...\n")
		err = openInBrowser(outputFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", err)
			fmt.Printf("Please open the file manually: %s\n", outputFilename)
		}
	} else {
		fmt.Printf("\nTo view the call graph, open: %s\n", outputFilename)
	}
}

func generateCallGraphSVG(graph *models.CallGraph) {
	outputFilename := callgraphOutput
	if !strings.HasSuffix(outputFilename, ".svg") {
		outputFilename = strings.TrimSuffix(outputFilename, ".html") + ".svg"
	}

	err := visualization.GenerateCallGraphSVG(graph, outputFilename, svgWidth, svgHeight)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating SVG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Static call graph generated: %s\n", outputFilename)
	fmt.Printf("   Dimensions: %dx%d pixels\n", svgWidth, svgHeight)
	fmt.Printf("\nOpen the file in a browser or image viewer to view the call graph.\n")
}

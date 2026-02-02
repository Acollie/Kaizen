package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alexcollie/kaizen/pkg/models"
	"github.com/alexcollie/kaizen/pkg/storage"
)

// DiffMetrics represents the difference between two analyses
type DiffMetrics struct {
	Timestamp          time.Time
	ScoreDelta         float64
	ComplexityDelta    float64
	MaintainabilityDelta float64
	ChurnDelta         float64
	HotspotCountDelta  int
	FunctionCountDelta int
	FileCountDelta     int
}

// TeamDiff represents differences for a specific team
type TeamDiff struct {
	Team             string
	FileCount        int
	FunctionCount    int
	AvgComplexity    float64
	AvgMaintainability float64
	HotspotCount     int
	HealthScore      float64
}

// AnalysisDiff contains comparison between two analysis runs
type AnalysisDiff struct {
	PreviousTimestamp time.Time
	CurrentTimestamp  time.Time
	GlobalMetrics     DiffMetrics
	TeamBreakdown     map[string]TeamDiff
	HotspotChanges    struct {
		New     []string // New hotspots
		Removed []string // Hotspots fixed
		Persistent []string // Still hotspots
	}
}

// CompareAnalyses compares two analysis results
func CompareAnalyses(previous, current *models.AnalysisResult) *AnalysisDiff {
	diff := &AnalysisDiff{
		PreviousTimestamp: previous.AnalyzedAt,
		CurrentTimestamp: current.AnalyzedAt,
		TeamBreakdown: make(map[string]TeamDiff),
	}

	// Calculate overall metrics differences
	if previous.ScoreReport != nil && current.ScoreReport != nil {
		diff.GlobalMetrics.ScoreDelta = current.ScoreReport.OverallScore - previous.ScoreReport.OverallScore
	}

	if previous.Summary.TotalFiles > 0 {
		prevAvgComplexity := previous.Summary.AverageCyclomaticComplexity
		currAvgComplexity := current.Summary.AverageCyclomaticComplexity
		diff.GlobalMetrics.ComplexityDelta = currAvgComplexity - prevAvgComplexity

		prevAvgMaint := previous.Summary.AverageMaintainabilityIndex
		currAvgMaint := current.Summary.AverageMaintainabilityIndex
		diff.GlobalMetrics.MaintainabilityDelta = currAvgMaint - prevAvgMaint
	}

	diff.GlobalMetrics.HotspotCountDelta = current.Summary.HotspotCount - previous.Summary.HotspotCount
	diff.GlobalMetrics.FunctionCountDelta = current.Summary.TotalFunctions - previous.Summary.TotalFunctions
	diff.GlobalMetrics.FileCountDelta = current.Summary.TotalFiles - previous.Summary.TotalFiles

	// Build maps for easier comparison
	prevHotspots := make(map[string]bool)
	currHotspots := make(map[string]bool)

	for _, file := range previous.Files {
		for _, fn := range file.Functions {
			if fn.IsHotspot {
				key := fmt.Sprintf("%s:%s", file.Path, fn.Name)
				prevHotspots[key] = true
			}
		}
	}

	for _, file := range current.Files {
		for _, fn := range file.Functions {
			if fn.IsHotspot {
				key := fmt.Sprintf("%s:%s", file.Path, fn.Name)
				currHotspots[key] = true
			}
		}
	}

	// Find new, removed, and persistent hotspots
	for hotspot := range currHotspots {
		if !prevHotspots[hotspot] {
			diff.HotspotChanges.New = append(diff.HotspotChanges.New, hotspot)
		} else {
			diff.HotspotChanges.Persistent = append(diff.HotspotChanges.Persistent, hotspot)
		}
	}

	for hotspot := range prevHotspots {
		if !currHotspots[hotspot] {
			diff.HotspotChanges.Removed = append(diff.HotspotChanges.Removed, hotspot)
		}
	}

	sort.Strings(diff.HotspotChanges.New)
	sort.Strings(diff.HotspotChanges.Removed)
	sort.Strings(diff.HotspotChanges.Persistent)

	return diff
}

// FormatDiffReport formats the diff as a readable report
func FormatDiffReport(diff *AnalysisDiff, showTeams bool) string {
	var sb strings.Builder

	sb.WriteString("════════════════════════════════════════════════════════════════════\n")
	sb.WriteString("📊 Code Analysis Diff Report\n")
	sb.WriteString("════════════════════════════════════════════════════════════════════\n\n")

	// Time comparison
	sb.WriteString("📅 Analysis Timeline\n")
	sb.WriteString(fmt.Sprintf("Previous: %s\n", diff.PreviousTimestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Current:  %s\n", diff.CurrentTimestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Delta:    %v\n\n", diff.CurrentTimestamp.Sub(diff.PreviousTimestamp)))

	// Overall metrics
	sb.WriteString("📈 Overall Metrics\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")

	scoreStr := fmt.Sprintf("%.1f", diff.GlobalMetrics.ScoreDelta)
	if diff.GlobalMetrics.ScoreDelta > 0 {
		scoreStr = "+" + scoreStr + " ⬆️"
	} else if diff.GlobalMetrics.ScoreDelta < 0 {
		scoreStr = scoreStr + " ⬇️"
	} else {
		scoreStr = scoreStr + " ➡️"
	}
	sb.WriteString(fmt.Sprintf("Overall Score:        %s\n", scoreStr))

	complexityStr := fmt.Sprintf("%.2f", diff.GlobalMetrics.ComplexityDelta)
	if diff.GlobalMetrics.ComplexityDelta < 0 {
		complexityStr = complexityStr + " ⬇️ (improved)"
	} else if diff.GlobalMetrics.ComplexityDelta > 0 {
		complexityStr = complexityStr + " ⬆️ (worsened)"
	} else {
		complexityStr = complexityStr + " ➡️"
	}
	sb.WriteString(fmt.Sprintf("Avg Complexity:       %s\n", complexityStr))

	maintStr := fmt.Sprintf("%.1f", diff.GlobalMetrics.MaintainabilityDelta)
	if diff.GlobalMetrics.MaintainabilityDelta > 0 {
		maintStr = "+" + maintStr + " ⬆️ (improved)"
	} else if diff.GlobalMetrics.MaintainabilityDelta < 0 {
		maintStr = maintStr + " ⬇️ (worsened)"
	} else {
		maintStr = maintStr + " ➡️"
	}
	sb.WriteString(fmt.Sprintf("Avg Maintainability: %s\n", maintStr))

	filesStr := fmt.Sprintf("%+d", diff.GlobalMetrics.FileCountDelta)
	sb.WriteString(fmt.Sprintf("Files:                %s\n", filesStr))

	functionsStr := fmt.Sprintf("%+d", diff.GlobalMetrics.FunctionCountDelta)
	sb.WriteString(fmt.Sprintf("Functions:            %s\n", functionsStr))

	hotspotsStr := fmt.Sprintf("%+d", diff.GlobalMetrics.HotspotCountDelta)
	if diff.GlobalMetrics.HotspotCountDelta < 0 {
		hotspotsStr = hotspotsStr + " ⬇️ (improved)"
	} else if diff.GlobalMetrics.HotspotCountDelta > 0 {
		hotspotsStr = hotspotsStr + " ⬆️ (worsened)"
	}
	sb.WriteString(fmt.Sprintf("Hotspots:             %s\n", hotspotsStr))

	// Hotspot changes
	sb.WriteString("\n🔥 Hotspot Changes\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────\n")

	if len(diff.HotspotChanges.New) > 0 {
		sb.WriteString(fmt.Sprintf("❌ New Hotspots (%d):\n", len(diff.HotspotChanges.New)))
		for _, spot := range diff.HotspotChanges.New {
			sb.WriteString(fmt.Sprintf("  - %s\n", spot))
		}
	}

	if len(diff.HotspotChanges.Removed) > 0 {
		sb.WriteString(fmt.Sprintf("✅ Fixed Hotspots (%d):\n", len(diff.HotspotChanges.Removed)))
		for _, spot := range diff.HotspotChanges.Removed {
			sb.WriteString(fmt.Sprintf("  - %s\n", spot))
		}
	}

	if len(diff.HotspotChanges.Persistent) > 0 {
		sb.WriteString(fmt.Sprintf("⚠️  Persistent Hotspots (%d):\n", len(diff.HotspotChanges.Persistent)))
		for i, spot := range diff.HotspotChanges.Persistent {
			if i < 10 { // Show first 10
				sb.WriteString(fmt.Sprintf("  - %s\n", spot))
			}
		}
		if len(diff.HotspotChanges.Persistent) > 10 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(diff.HotspotChanges.Persistent)-10))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// ConvertSnapshotToResult converts a SnapshotSummary to AnalysisResult for comparison
func ConvertSnapshotToResult(snapshot *storage.SnapshotSummary) *models.AnalysisResult {
	return &models.AnalysisResult{
		AnalyzedAt: snapshot.AnalyzedAt,
		Summary: models.SummaryMetrics{
			TotalFiles:                  snapshot.TotalFiles,
			TotalFunctions:              snapshot.TotalFunctions,
			AverageCyclomaticComplexity: snapshot.AvgCyclomaticComplexity,
			AverageMaintainabilityIndex: snapshot.AvgMaintainabilityIndex,
			HotspotCount:                snapshot.HotspotCount,
		},
		ScoreReport: &models.ScoreReport{
			OverallScore: snapshot.OverallScore,
			OverallGrade: snapshot.OverallGrade,
		},
	}
}

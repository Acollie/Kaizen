package check

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexcollie/kaizen/pkg/models"
)

// Blast-radius detection thresholds
const (
	ThresholdBlastRadiusWarning  = 5
	ThresholdBlastRadiusCritical = 15
	maxBlastRadiusItems          = 5
)

// DetectBlastRadius classifies fan-in results into concerns
func DetectBlastRadius(fanInResults []FanInResult) []models.Concern {
	var warningItems []models.AffectedItem
	var criticalItems []models.AffectedItem
	hasApproximate := false

	for _, result := range fanInResults {
		if result.FanIn < ThresholdBlastRadiusWarning {
			continue
		}

		item := models.AffectedItem{
			FilePath:     result.Function.FilePath,
			FunctionName: result.Function.Name,
			Line:         result.Function.StartLine,
			Metrics: map[string]float64{
				"fan_in":      float64(result.FanIn),
				"approximate": boolToFloat(result.Approximate),
			},
		}

		if result.Approximate {
			hasApproximate = true
		}

		if result.FanIn >= ThresholdBlastRadiusCritical {
			criticalItems = append(criticalItems, item)
		} else {
			warningItems = append(warningItems, item)
		}
	}

	var concerns []models.Concern

	// Add critical concerns first
	if len(criticalItems) > 0 {
		sortAffectedItemsByFanIn(criticalItems)
		concerns = append(concerns, models.Concern{
			Type:          "blast_radius",
			Severity:      "critical",
			Title:         "Critical Blast Radius",
			Description:   buildBlastRadiusDescription(limitItems(criticalItems, maxBlastRadiusItems), hasApproximate, "critical"),
			AffectedItems: limitItems(criticalItems, maxBlastRadiusItems),
		})
	}

	// Add warning concerns
	if len(warningItems) > 0 {
		sortAffectedItemsByFanIn(warningItems)
		concerns = append(concerns, models.Concern{
			Type:          "blast_radius",
			Severity:      "warning",
			Title:         "High Blast Radius",
			Description:   buildBlastRadiusDescription(limitItems(warningItems, maxBlastRadiusItems), hasApproximate, "warning"),
			AffectedItems: limitItems(warningItems, maxBlastRadiusItems),
		})
	}

	return concerns
}

// buildBlastRadiusDescription creates a human-readable description of blast-radius concerns
func buildBlastRadiusDescription(items []models.AffectedItem, hasApproximate bool, severity string) string {
	var buffer strings.Builder

	if severity == "critical" {
		buffer.WriteString("Changed functions are called by many other functions (high fan-in). ")
		buffer.WriteString("Changes here may propagate widely across the codebase.\n\n")
	} else {
		buffer.WriteString("Changed functions have moderate fan-in. ")
		buffer.WriteString("Changes may affect multiple call sites.\n\n")
	}

	buffer.WriteString("Affected functions:\n")
	for i, item := range items {
		fanIn := int(item.Metrics["fan_in"])
		buffer.WriteString(fmt.Sprintf("%d. %s (line %d) - called by %d function(s)\n",
			i+1, item.FunctionName, item.Line, fanIn))
	}

	if hasApproximate {
		buffer.WriteString("\nNote: fan-in for non-Go files is approximate (grep-based).")
	}

	return buffer.String()
}

// sortAffectedItemsByFanIn sorts items in descending order by fan-in
func sortAffectedItemsByFanIn(items []models.AffectedItem) {
	sort.Slice(items, func(i, j int) bool {
		fanInI := items[i].Metrics["fan_in"]
		fanInJ := items[j].Metrics["fan_in"]
		return fanInI > fanInJ
	})
}

// limitItems limits a slice to a maximum length
func limitItems(items []models.AffectedItem, maxItems int) []models.AffectedItem {
	if len(items) > maxItems {
		return items[:maxItems]
	}
	return items
}

// boolToFloat converts a boolean to 0.0 or 1.0
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

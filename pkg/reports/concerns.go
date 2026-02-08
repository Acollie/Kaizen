package reports

import (
	"fmt"
	"sort"

	"github.com/alexcollie/kaizen/internal/config"
	"github.com/alexcollie/kaizen/pkg/models"
)

const (
	MaxConcernItems = 5 // Max affected items to show per concern
)

// DetectConcerns analyzes the result and returns a list of concerns
func DetectConcerns(result *models.AnalysisResult, hasChurnData bool, thresholds config.ThresholdConfig) []models.Concern {
	var concerns []models.Concern

	// Collect all functions for analysis
	var allFunctions []functionWithFile
	for _, file := range result.Files {
		for _, function := range file.Functions {
			allFunctions = append(allFunctions, functionWithFile{
				filePath: file.Path,
				function: function,
			})
		}
	}

	// Detect different types of concerns
	if hasChurnData {
		concerns = append(concerns, detectChurnComplexityHotspots(allFunctions, thresholds)...)
		concerns = append(concerns, detectHighChurnLongFunctions(allFunctions, thresholds)...)
	}

	concerns = append(concerns, detectLowMaintainability(allFunctions, thresholds)...)
	concerns = append(concerns, detectDeepNesting(allFunctions, thresholds)...)
	concerns = append(concerns, detectTooManyParameters(allFunctions, thresholds)...)
	concerns = append(concerns, detectGodFunctions(allFunctions, thresholds)...)

	// Sort concerns by severity (critical first, then warning, then info)
	sortConcernsBySeverity(concerns)

	return concerns
}

type functionWithFile struct {
	filePath string
	function models.FunctionAnalysis
}

func detectChurnComplexityHotspots(functions []functionWithFile, thresholds config.ThresholdConfig) []models.Concern {
	var affectedItems []models.AffectedItem

	for _, funcFile := range functions {
		function := funcFile.function
		if function.Churn == nil {
			continue
		}

		churnCount := function.Churn.TotalCommits
		complexity := function.CyclomaticComplexity

		if complexity > thresholds.Hotspot.MinComplexity && churnCount > thresholds.Hotspot.MinChurn {
			affectedItems = append(affectedItems, models.AffectedItem{
				FilePath:     funcFile.filePath,
				FunctionName: function.Name,
				Line:         function.StartLine,
				Metrics: map[string]float64{
					"complexity": float64(complexity),
					"churn":      float64(churnCount),
				},
			})
		}
	}

	if len(affectedItems) == 0 {
		return nil
	}

	// Sort by combined score (complexity * churn)
	sortAffectedItemsByScore(affectedItems, func(item models.AffectedItem) float64 {
		return item.Metrics["complexity"] * item.Metrics["churn"]
	})

	return []models.Concern{{
		Type:          "churn_complexity_hotspot",
		Severity:      "critical",
		Title:         "Complexity Hotspots",
		Description:   buildHotspotDescription(affectedItems),
		AffectedItems: limitAffectedItems(affectedItems, MaxConcernItems),
	}}
}

func detectHighChurnLongFunctions(functions []functionWithFile, thresholds config.ThresholdConfig) []models.Concern {
	var warningItems []models.AffectedItem
	var criticalItems []models.AffectedItem

	for _, funcFile := range functions {
		function := funcFile.function
		if function.Churn == nil {
			continue
		}

		churnCount := function.Churn.TotalCommits
		length := function.Length

		if length > thresholds.FunctionLength.Warning && churnCount > thresholds.Churn.Warning {
			item := models.AffectedItem{
				FilePath:     funcFile.filePath,
				FunctionName: function.Name,
				Line:         function.StartLine,
				Metrics: map[string]float64{
					"length": float64(length),
					"churn":  float64(churnCount),
				},
			}

			if length > thresholds.FunctionLength.Critical && churnCount > thresholds.Churn.Critical {
				criticalItems = append(criticalItems, item)
			} else {
				warningItems = append(warningItems, item)
			}
		}
	}

	var concerns []models.Concern

	if len(criticalItems) > 0 {
		sortAffectedItemsByScore(criticalItems, func(item models.AffectedItem) float64 {
			return item.Metrics["length"] * item.Metrics["churn"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "high_churn_long_function",
			Severity:      "critical",
			Title:         "Large Functions with High Churn",
			Description:   buildChurnLengthDescription(criticalItems, "critical"),
			AffectedItems: limitAffectedItems(criticalItems, MaxConcernItems),
		})
	}

	if len(warningItems) > 0 {
		sortAffectedItemsByScore(warningItems, func(item models.AffectedItem) float64 {
			return item.Metrics["length"] * item.Metrics["churn"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "high_churn_long_function",
			Severity:      "warning",
			Title:         "Long Functions with Moderate Churn",
			Description:   buildChurnLengthDescription(warningItems, "warning"),
			AffectedItems: limitAffectedItems(warningItems, MaxConcernItems),
		})
	}

	return concerns
}

func detectLowMaintainability(functions []functionWithFile, thresholds config.ThresholdConfig) []models.Concern {
	var warningItems []models.AffectedItem
	var criticalItems []models.AffectedItem

	miThresholds := thresholds.MaintainabilityIndex

	for _, funcFile := range functions {
		function := funcFile.function
		maintainability := function.MaintainabilityIndex

		if maintainability < float64(miThresholds.Warning) {
			// Include all contributing factors so we can explain the score
			item := models.AffectedItem{
				FilePath:     funcFile.filePath,
				FunctionName: function.Name,
				Line:         function.StartLine,
				Metrics: map[string]float64{
					"maintainability_index": maintainability,
					"cyclomatic_complexity": float64(function.CyclomaticComplexity),
					"length":               float64(function.Length),
					"halstead_volume":       function.HalsteadVolume,
				},
			}

			if maintainability < float64(miThresholds.Critical) {
				criticalItems = append(criticalItems, item)
			} else {
				warningItems = append(warningItems, item)
			}
		}
	}

	var concerns []models.Concern

	if len(criticalItems) > 0 {
		sortAffectedItemsByScore(criticalItems, func(item models.AffectedItem) float64 {
			return 100 - item.Metrics["maintainability_index"] // Lower MI = higher priority
		})
		concerns = append(concerns, models.Concern{
			Type:          "low_maintainability",
			Severity:      "critical",
			Title:         "Critical Maintainability Issues",
			Description:   buildMaintainabilityDescription(criticalItems, miThresholds.Critical),
			AffectedItems: limitAffectedItems(criticalItems, MaxConcernItems),
		})
	}

	if len(warningItems) > 0 {
		sortAffectedItemsByScore(warningItems, func(item models.AffectedItem) float64 {
			return 100 - item.Metrics["maintainability_index"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "low_maintainability",
			Severity:      "warning",
			Title:         "Low Maintainability",
			Description:   buildMaintainabilityDescription(warningItems, miThresholds.Warning),
			AffectedItems: limitAffectedItems(warningItems, MaxConcernItems),
		})
	}

	return concerns
}

// buildMaintainabilityDescription analyzes the contributing factors and explains why scores are low
func buildMaintainabilityDescription(items []models.AffectedItem, threshold int) string {
	if len(items) == 0 {
		return fmt.Sprintf("Functions with maintainability index below %d", threshold)
	}

	// Analyze the dominant factors across all items
	var totalLength, totalComplexity, totalVolume float64
	var highLengthCount, highComplexityCount, highVolumeCount int

	for _, item := range items {
		length := item.Metrics["length"]
		complexity := item.Metrics["cyclomatic_complexity"]
		volume := item.Metrics["halstead_volume"]

		totalLength += length
		totalComplexity += complexity
		totalVolume += volume

		if length > 50 {
			highLengthCount++
		}
		if complexity > 10 {
			highComplexityCount++
		}
		if volume > 1000 {
			highVolumeCount++
		}
	}

	count := len(items)
	avgLength := totalLength / float64(count)
	avgComplexity := totalComplexity / float64(count)

	// Build explanation based on dominant factors
	var factors []string

	if highLengthCount > count/2 || avgLength > 40 {
		factors = append(factors, fmt.Sprintf("long functions (avg %.0f lines)", avgLength))
	}
	if highComplexityCount > count/2 || avgComplexity > 8 {
		factors = append(factors, fmt.Sprintf("high complexity (avg CC: %.1f)", avgComplexity))
	}
	if highVolumeCount > count/2 {
		factors = append(factors, "dense code with many operators/operands")
	}

	if len(factors) == 0 {
		return fmt.Sprintf("MI below %d indicates code that is harder to understand and modify. Consider simplifying logic or breaking into smaller functions.", threshold)
	}

	factorStr := factors[0]
	for i := 1; i < len(factors); i++ {
		if i == len(factors)-1 {
			factorStr += " and " + factors[i]
		} else {
			factorStr += ", " + factors[i]
		}
	}

	return fmt.Sprintf("Low scores driven by %s. Break into smaller, focused functions to improve readability.", factorStr)
}

func detectDeepNesting(functions []functionWithFile, thresholds config.ThresholdConfig) []models.Concern {
	var infoItems []models.AffectedItem
	var warningItems []models.AffectedItem

	nestingThresholds := thresholds.NestingDepth

	for _, funcFile := range functions {
		function := funcFile.function
		nesting := function.NestingDepth

		if nesting > nestingThresholds.Warning {
			item := models.AffectedItem{
				FilePath:     funcFile.filePath,
				FunctionName: function.Name,
				Line:         function.StartLine,
				Metrics: map[string]float64{
					"nesting_depth": float64(nesting),
				},
			}

			if nesting > nestingThresholds.Critical {
				warningItems = append(warningItems, item)
			} else {
				infoItems = append(infoItems, item)
			}
		}
	}

	var concerns []models.Concern

	if len(warningItems) > 0 {
		sortAffectedItemsByScore(warningItems, func(item models.AffectedItem) float64 {
			return item.Metrics["nesting_depth"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "deep_nesting",
			Severity:      "warning",
			Title:         "Very Deep Nesting",
			Description:   buildNestingDescription(warningItems, "warning"),
			AffectedItems: limitAffectedItems(warningItems, MaxConcernItems),
		})
	}

	if len(infoItems) > 0 {
		sortAffectedItemsByScore(infoItems, func(item models.AffectedItem) float64 {
			return item.Metrics["nesting_depth"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "deep_nesting",
			Severity:      "info",
			Title:         "Deep Nesting",
			Description:   buildNestingDescription(infoItems, "info"),
			AffectedItems: limitAffectedItems(infoItems, MaxConcernItems),
		})
	}

	return concerns
}

func detectTooManyParameters(functions []functionWithFile, thresholds config.ThresholdConfig) []models.Concern {
	var infoItems []models.AffectedItem
	var warningItems []models.AffectedItem

	paramThresholds := thresholds.ParameterCount

	for _, funcFile := range functions {
		function := funcFile.function
		params := function.ParameterCount

		if params > paramThresholds.Warning {
			item := models.AffectedItem{
				FilePath:     funcFile.filePath,
				FunctionName: function.Name,
				Line:         function.StartLine,
				Metrics: map[string]float64{
					"parameter_count": float64(params),
				},
			}

			if params > paramThresholds.Critical {
				warningItems = append(warningItems, item)
			} else {
				infoItems = append(infoItems, item)
			}
		}
	}

	var concerns []models.Concern

	if len(warningItems) > 0 {
		sortAffectedItemsByScore(warningItems, func(item models.AffectedItem) float64 {
			return item.Metrics["parameter_count"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "too_many_parameters",
			Severity:      "warning",
			Title:         "Too Many Parameters",
			Description:   buildParameterDescription(warningItems, "warning"),
			AffectedItems: limitAffectedItems(warningItems, MaxConcernItems),
		})
	}

	if len(infoItems) > 0 {
		sortAffectedItemsByScore(infoItems, func(item models.AffectedItem) float64 {
			return item.Metrics["parameter_count"]
		})
		concerns = append(concerns, models.Concern{
			Type:          "too_many_parameters",
			Severity:      "info",
			Title:         "Many Parameters",
			Description:   buildParameterDescription(infoItems, "info"),
			AffectedItems: limitAffectedItems(infoItems, MaxConcernItems),
		})
	}

	return concerns
}

func detectGodFunctions(functions []functionWithFile, thresholds config.ThresholdConfig) []models.Concern {
	var affectedItems []models.AffectedItem

	godThresholds := thresholds.GodFunction

	for _, funcFile := range functions {
		function := funcFile.function
		params := function.ParameterCount
		fanIn := function.FanIn

		if params > godThresholds.MinParameters && fanIn > godThresholds.MinFanIn {
			affectedItems = append(affectedItems, models.AffectedItem{
				FilePath:     funcFile.filePath,
				FunctionName: function.Name,
				Line:         function.StartLine,
				Metrics: map[string]float64{
					"parameter_count": float64(params),
					"fan_in":          float64(fanIn),
				},
			})
		}
	}

	if len(affectedItems) == 0 {
		return nil
	}

	sortAffectedItemsByScore(affectedItems, func(item models.AffectedItem) float64 {
		return item.Metrics["parameter_count"] * item.Metrics["fan_in"]
	})

	return []models.Concern{{
		Type:          "god_function",
		Severity:      "warning",
		Title:         "God Functions",
		Description:   buildGodFunctionDescription(affectedItems),
		AffectedItems: limitAffectedItems(affectedItems, MaxConcernItems),
	}}
}

func sortAffectedItemsByScore(items []models.AffectedItem, scoreFunc func(models.AffectedItem) float64) {
	sort.Slice(items, func(i, j int) bool {
		return scoreFunc(items[i]) > scoreFunc(items[j])
	})
}

func limitAffectedItems(items []models.AffectedItem, maxItems int) []models.AffectedItem {
	if len(items) <= maxItems {
		return items
	}
	return items[:maxItems]
}

func sortConcernsBySeverity(concerns []models.Concern) {
	severityOrder := map[string]int{
		"critical": 0,
		"warning":  1,
		"info":     2,
	}

	sort.Slice(concerns, func(i, j int) bool {
		return severityOrder[concerns[i].Severity] < severityOrder[concerns[j].Severity]
	})
}

// buildHotspotDescription explains why functions are complexity hotspots
func buildHotspotDescription(items []models.AffectedItem) string {
	if len(items) == 0 {
		return "High complexity functions that change frequently are risky to modify."
	}

	var totalComplexity, totalChurn float64
	for _, item := range items {
		totalComplexity += item.Metrics["complexity"]
		totalChurn += item.Metrics["churn"]
	}

	avgComplexity := totalComplexity / float64(len(items))
	avgChurn := totalChurn / float64(len(items))

	return fmt.Sprintf(
		"These functions average CC:%.0f with %.0f commits each. High complexity makes changes error-prone, and frequent changes multiply that risk. Consider refactoring to reduce complexity before the next change.",
		avgComplexity, avgChurn,
	)
}

// buildChurnLengthDescription explains why long functions with high churn are problematic
func buildChurnLengthDescription(items []models.AffectedItem, severity string) string {
	if len(items) == 0 {
		return "Long functions that change frequently are hard to maintain."
	}

	var totalLength, totalChurn float64
	for _, item := range items {
		totalLength += item.Metrics["length"]
		totalChurn += item.Metrics["churn"]
	}

	avgLength := totalLength / float64(len(items))
	avgChurn := totalChurn / float64(len(items))

	if severity == "critical" {
		return fmt.Sprintf(
			"Averaging %.0f lines and %.0f commits. Large functions are hard to understand and test. Each change risks unintended side effects. Split into smaller, single-purpose functions.",
			avgLength, avgChurn,
		)
	}

	return fmt.Sprintf(
		"These functions average %.0f lines with %.0f changes. Consider extracting logical sections into separate functions to improve readability and reduce change risk.",
		avgLength, avgChurn,
	)
}

// buildParameterDescription explains why too many parameters is a concern
func buildParameterDescription(items []models.AffectedItem, severity string) string {
	if len(items) == 0 {
		return "Functions with many parameters are harder to call correctly."
	}

	var totalParams float64
	for _, item := range items {
		totalParams += item.Metrics["parameter_count"]
	}
	avgParams := totalParams / float64(len(items))

	if severity == "warning" {
		return fmt.Sprintf(
			"These functions average %.0f parameters. Many parameters increase cognitive load, make testing harder, and often indicate the function is doing too much. Group related parameters into structs or split the function.",
			avgParams,
		)
	}

	return fmt.Sprintf(
		"Averaging %.0f parameters per function. Consider grouping related parameters into a config struct or options pattern to improve readability.",
		avgParams,
	)
}

// buildGodFunctionDescription explains why god functions are problematic
func buildGodFunctionDescription(items []models.AffectedItem) string {
	if len(items) == 0 {
		return "Functions with many parameters and high fan-in may be doing too much."
	}

	var totalParams, totalFanIn float64
	for _, item := range items {
		totalParams += item.Metrics["parameter_count"]
		totalFanIn += item.Metrics["fan_in"]
	}

	avgParams := totalParams / float64(len(items))
	avgFanIn := totalFanIn / float64(len(items))

	return fmt.Sprintf(
		"These functions average %.0f parameters and are called from %.0f places. High fan-in with many parameters suggests these are 'god functions' trying to do everything. Break them into focused, single-responsibility functions.",
		avgParams, avgFanIn,
	)
}

// buildNestingDescription explains why deep nesting is problematic
func buildNestingDescription(items []models.AffectedItem, severity string) string {
	if len(items) == 0 {
		return "Deeply nested code is hard to follow and test."
	}

	var totalNesting float64
	for _, item := range items {
		totalNesting += item.Metrics["nesting_depth"]
	}
	avgNesting := totalNesting / float64(len(items))

	if severity == "warning" {
		return fmt.Sprintf(
			"These functions have %.0f+ levels of nesting. Deep nesting forces readers to track multiple conditions mentally. Use guard clauses (early returns), extract nested blocks into helper functions, or consider the strategy pattern.",
			avgNesting,
		)
	}

	return fmt.Sprintf(
		"Averaging %.0f nesting levels. Use early returns to handle edge cases first, reducing the main logic's nesting depth.",
		avgNesting,
	)
}

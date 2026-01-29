package analyzer

import (
	"math"

	"github.com/alexcollie/kaizen/pkg/models"
)

// DefaultMetricCalculator provides utility functions for calculating various metrics
type DefaultMetricCalculator struct{}

// NewMetricCalculator creates a new metric calculator
func NewMetricCalculator() MetricCalculator {
	return &DefaultMetricCalculator{}
}

// CalculateMaintainabilityIndex computes the maintainability index
// Formula: MI = 171 - 5.2 * ln(HV) - 0.23 * CC - 16.2 * ln(LOC)
// Where HV = Halstead Volume, CC = Cyclomatic Complexity, LOC = Lines of Code
func (calculator *DefaultMetricCalculator) CalculateMaintainabilityIndex(
	halsteadVolume float64,
	cyclomaticComplexity int,
	linesOfCode int,
) float64 {
	if linesOfCode == 0 {
		return 100.0
	}

	hvTerm := 0.0
	if halsteadVolume > 0 {
		hvTerm = 5.2 * math.Log(halsteadVolume)
	}

	ccTerm := 0.23 * float64(cyclomaticComplexity)
	locTerm := 16.2 * math.Log(float64(linesOfCode))

	maintainabilityIndex := 171.0 - hvTerm - ccTerm - locTerm

	// Normalize to 0-100 range
	if maintainabilityIndex < 0 {
		maintainabilityIndex = 0
	}
	if maintainabilityIndex > 100 {
		maintainabilityIndex = 100
	}

	return maintainabilityIndex
}

// CalculateHalsteadMetrics computes Halstead complexity metrics
func (calculator *DefaultMetricCalculator) CalculateHalsteadMetrics(
	distinctOperators int,
	distinctOperands int,
	totalOperators int,
	totalOperands int,
) models.HalsteadMetrics {
	if distinctOperators == 0 || distinctOperands == 0 {
		return models.HalsteadMetrics{}
	}

	vocabulary := distinctOperators + distinctOperands
	length := totalOperators + totalOperands

	volume := 0.0
	difficulty := 0.0
	effort := 0.0
	timeToUnderstand := 0.0
	bugsDelivered := 0.0

	if vocabulary > 0 && length > 0 {
		// Volume = Length * log2(Vocabulary)
		volume = float64(length) * math.Log2(float64(vocabulary))

		// Difficulty = (n1/2) * (N2/n2)
		difficulty = (float64(distinctOperators) / 2.0) * (float64(totalOperands) / float64(distinctOperands))

		// Effort = Volume * Difficulty
		effort = volume * difficulty

		// Time to understand in seconds = Effort / 18
		timeToUnderstand = effort / 18.0

		// Bugs delivered = Volume / 3000
		bugsDelivered = volume / 3000.0
	}

	return models.HalsteadMetrics{
		DistinctOperators: distinctOperators,
		DistinctOperands:  distinctOperands,
		TotalOperators:    totalOperators,
		TotalOperands:     totalOperands,
		Vocabulary:        vocabulary,
		Length:            length,
		Volume:            volume,
		Difficulty:        difficulty,
		Effort:            effort,
		TimeToUnderstand:  timeToUnderstand,
		BugsDelivered:     bugsDelivered,
	}
}

// IsHotspot determines if a function is a hotspot (high churn + high complexity)
// Uses thresholds: churn score > 70 AND complexity score > 70
func (calculator *DefaultMetricCalculator) IsHotspot(churnScore float64, complexityScore float64) bool {
	const churnThreshold = 70.0
	const complexityThreshold = 70.0

	return churnScore > churnThreshold && complexityScore > complexityThreshold
}

// NormalizeTo100 normalizes a value to 0-100 scale using percentile ranking
func NormalizeTo100(value float64, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Count how many values are less than or equal to this value
	lessOrEqual := 0
	for _, otherValue := range values {
		if otherValue <= value {
			lessOrEqual++
		}
	}

	// Percentile = (lessOrEqual / total) * 100
	return (float64(lessOrEqual) / float64(len(values))) * 100.0
}

// CalculatePercentile calculates the percentile of a value in a dataset
func CalculatePercentile(value float64, values []float64, percentile float64) bool {
	if len(values) == 0 {
		return false
	}

	// Sort is not needed if we just count
	count := 0
	for _, otherValue := range values {
		if otherValue <= value {
			count++
		}
	}

	actualPercentile := (float64(count) / float64(len(values))) * 100.0
	return actualPercentile >= percentile
}

// GetComplexityCategory returns a category for cyclomatic complexity
func GetComplexityCategory(cyclomaticComplexity int) string {
	switch {
	case cyclomaticComplexity <= 5:
		return "low"
	case cyclomaticComplexity <= 10:
		return "moderate"
	case cyclomaticComplexity <= 20:
		return "high"
	default:
		return "very_high"
	}
}

// GetMaintainabilityCategory returns a category for maintainability index
func GetMaintainabilityCategory(maintainabilityIndex float64) string {
	switch {
	case maintainabilityIndex >= 80:
		return "excellent"
	case maintainabilityIndex >= 60:
		return "good"
	case maintainabilityIndex >= 40:
		return "moderate"
	case maintainabilityIndex >= 20:
		return "poor"
	default:
		return "critical"
	}
}

// GetFunctionLengthCategory returns a category for function length
func GetFunctionLengthCategory(length int) string {
	switch {
	case length <= 20:
		return "short"
	case length <= 50:
		return "moderate"
	case length <= 100:
		return "long"
	default:
		return "very_long"
	}
}

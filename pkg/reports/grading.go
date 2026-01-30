package reports

// Grade thresholds
const (
	GradeThresholdA = 90.0
	GradeThresholdB = 75.0
	GradeThresholdC = 60.0
	GradeThresholdD = 40.0
)

// Category score thresholds
const (
	CategoryExcellent = 90.0
	CategoryGood      = 75.0
	CategoryModerate  = 60.0
	CategoryPoor      = 40.0
)

// CalculateGrade converts a numeric score (0-100) to a letter grade
func CalculateGrade(score float64) string {
	switch {
	case score >= GradeThresholdA:
		return "A"
	case score >= GradeThresholdB:
		return "B"
	case score >= GradeThresholdC:
		return "C"
	case score >= GradeThresholdD:
		return "D"
	default:
		return "F"
	}
}

// GetCategoryLabel returns a descriptive label for a score
func GetCategoryLabel(score float64) string {
	switch {
	case score >= CategoryExcellent:
		return "excellent"
	case score >= CategoryGood:
		return "good"
	case score >= CategoryModerate:
		return "moderate"
	case score >= CategoryPoor:
		return "poor"
	default:
		return "critical"
	}
}

// clamp restricts a value to a given range
func clamp(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

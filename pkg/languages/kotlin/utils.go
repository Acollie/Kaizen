package kotlin

import "math"

// log2 calculates the base-2 logarithm
func log2(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Log2(value)
}

// log calculates the natural logarithm
func log(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Log(value)
}

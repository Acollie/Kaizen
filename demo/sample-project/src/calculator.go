package main

import (
	"errors"
	"math"
)

// Simple function - low complexity, good example
func Add(a, b int) int {
	return a + b
}

// Simple function - low complexity
func Subtract(a, b int) int {
	return a - b
}

// Moderate complexity - demonstrates basic branching
func Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

// High cyclomatic complexity example - many decision points
func CalculateGrade(score int) string {
	if score < 0 || score > 100 {
		return "Invalid"
	}

	if score >= 90 {
		return "A"
	} else if score >= 80 {
		return "B"
	} else if score >= 70 {
		return "C"
	} else if score >= 60 {
		return "D"
	} else {
		return "F"
	}
}

// High cognitive complexity - deeply nested logic
func ProcessData(data []int, threshold int, enableFilter bool, enableTransform bool) []int {
	result := []int{}

	for _, value := range data {
		if enableFilter {
			if value > threshold {
				if enableTransform {
					transformed := value * 2
					if transformed < 1000 {
						result = append(result, transformed)
					} else {
						// Nested even deeper
						if value%2 == 0 {
							result = append(result, value)
						}
					}
				} else {
					result = append(result, value)
				}
			}
		} else {
			if enableTransform {
				result = append(result, value*2)
			} else {
				result = append(result, value)
			}
		}
	}

	return result
}

// Long function with moderate complexity
func AnalyzeNumbers(numbers []int) map[string]interface{} {
	if len(numbers) == 0 {
		return map[string]interface{}{
			"count":   0,
			"sum":     0,
			"average": 0.0,
			"min":     0,
			"max":     0,
		}
	}

	sum := 0
	min := numbers[0]
	max := numbers[0]

	for _, num := range numbers {
		sum += num
		if num < min {
			min = num
		}
		if num > max {
			max = num
		}
	}

	average := float64(sum) / float64(len(numbers))

	// Calculate standard deviation
	variance := 0.0
	for _, num := range numbers {
		diff := float64(num) - average
		variance += diff * diff
	}
	variance /= float64(len(numbers))
	stdDev := math.Sqrt(variance)

	// Calculate median
	sorted := make([]int, len(numbers))
	copy(sorted, numbers)
	// Simple bubble sort for demonstration
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	median := 0.0
	if len(sorted)%2 == 0 {
		median = float64(sorted[len(sorted)/2-1]+sorted[len(sorted)/2]) / 2.0
	} else {
		median = float64(sorted[len(sorted)/2])
	}

	return map[string]interface{}{
		"count":   len(numbers),
		"sum":     sum,
		"average": average,
		"min":     min,
		"max":     max,
		"median":  median,
		"stddev":  stdDev,
	}
}

// Very complex function - combines high cyclomatic and cognitive complexity
func ValidateAndProcessOrder(order map[string]interface{}) (bool, []string) {
	errors := []string{}

	// Validate required fields
	if _, ok := order["customer_id"]; !ok {
		errors = append(errors, "customer_id is required")
	} else {
		if customerId, ok := order["customer_id"].(int); ok {
			if customerId <= 0 {
				errors = append(errors, "customer_id must be positive")
			}
		} else {
			errors = append(errors, "customer_id must be an integer")
		}
	}

	if _, ok := order["items"]; !ok {
		errors = append(errors, "items is required")
	} else {
		if items, ok := order["items"].([]interface{}); ok {
			if len(items) == 0 {
				errors = append(errors, "items cannot be empty")
			} else {
				for idx, item := range items {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if _, ok := itemMap["product_id"]; !ok {
							errors = append(errors, "item "+string(rune(idx))+" missing product_id")
						}
						if _, ok := itemMap["quantity"]; !ok {
							errors = append(errors, "item "+string(rune(idx))+" missing quantity")
						} else {
							if qty, ok := itemMap["quantity"].(int); ok {
								if qty <= 0 {
									errors = append(errors, "item "+string(rune(idx))+" quantity must be positive")
								} else if qty > 1000 {
									errors = append(errors, "item "+string(rune(idx))+" quantity exceeds maximum")
								}
							}
						}
					}
				}
			}
		} else {
			errors = append(errors, "items must be an array")
		}
	}

	// Validate payment method
	if _, ok := order["payment_method"]; !ok {
		errors = append(errors, "payment_method is required")
	} else {
		if paymentMethod, ok := order["payment_method"].(string); ok {
			validMethods := []string{"credit_card", "debit_card", "paypal", "bank_transfer"}
			valid := false
			for _, method := range validMethods {
				if paymentMethod == method {
					valid = true
					break
				}
			}
			if !valid {
				errors = append(errors, "invalid payment_method")
			}
		} else {
			errors = append(errors, "payment_method must be a string")
		}
	}

	// Validate shipping address
	if _, ok := order["shipping_address"]; !ok {
		errors = append(errors, "shipping_address is required")
	} else {
		if address, ok := order["shipping_address"].(map[string]interface{}); ok {
			requiredFields := []string{"street", "city", "state", "zip", "country"}
			for _, field := range requiredFields {
				if _, ok := address[field]; !ok {
					errors = append(errors, "shipping_address."+field+" is required")
				}
			}
		} else {
			errors = append(errors, "shipping_address must be an object")
		}
	}

	return len(errors) == 0, errors
}

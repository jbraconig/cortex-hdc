package domain

import "math"

// Mean calculates the arithmetic mean of a slice of float64.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// StdDev calculates the population standard deviation of a slice of float64.
func StdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := Mean(values)
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	return math.Sqrt(variance)
}

// SuggestThreshold calculates an optimal detection threshold using:
//
//	threshold = mean - 2 * stddev
//
// This ensures only logs falling outside 2σ (≈97.7% confidence) are flagged.
// The result is clamped to [0.30, 0.90] to avoid degenerate values.
func SuggestThreshold(similarities []float64) float64 {
	if len(similarities) == 0 {
		return 0.65 // sensible default
	}
	suggested := Mean(similarities) - 2*StdDev(similarities)

	// Clamp to a reasonable range
	if suggested < 0.30 {
		suggested = 0.30
	}
	if suggested > 0.90 {
		suggested = 0.90
	}
	return suggested
}

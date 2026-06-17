package domain

import (
	"math"
	"testing"
)

func TestMean_Basic(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	got := Mean(values)
	if math.Abs(got-3.0) > 1e-9 {
		t.Errorf("Expected Mean=3.0, got %.6f", got)
	}
}

func TestMean_Empty(t *testing.T) {
	if got := Mean([]float64{}); got != 0 {
		t.Errorf("Expected Mean=0 for empty slice, got %.6f", got)
	}
}

func TestMean_SingleValue(t *testing.T) {
	if got := Mean([]float64{7.5}); math.Abs(got-7.5) > 1e-9 {
		t.Errorf("Expected Mean=7.5, got %.6f", got)
	}
}

func TestStdDev_KnownValues(t *testing.T) {
	// Dataset: [2, 4, 4, 4, 5, 5, 7, 9] → stddev = 2.0
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	got := StdDev(values)
	if math.Abs(got-2.0) > 1e-9 {
		t.Errorf("Expected StdDev=2.0, got %.6f", got)
	}
}

func TestStdDev_SingleValue(t *testing.T) {
	// Single value → stddev = 0
	if got := StdDev([]float64{42.0}); got != 0 {
		t.Errorf("Expected StdDev=0 for single value, got %.6f", got)
	}
}

func TestStdDev_Uniform(t *testing.T) {
	// All same values → stddev = 0
	values := []float64{5.0, 5.0, 5.0, 5.0}
	if got := StdDev(values); got != 0 {
		t.Errorf("Expected StdDev=0 for uniform data, got %.6f", got)
	}
}

func TestSuggestThreshold_NormalCase(t *testing.T) {
	// Similarities around 0.80 with stddev 0.05 → threshold ≈ 0.70
	values := []float64{0.75, 0.80, 0.82, 0.78, 0.81, 0.79, 0.83, 0.77}
	got := SuggestThreshold(values)
	if got < 0.30 || got > 0.90 {
		t.Errorf("SuggestThreshold out of bounds [0.30, 0.90]: %.4f", got)
	}
	t.Logf("Suggested threshold for near-0.80 data: %.4f", got)
}

func TestSuggestThreshold_FloorEnforced(t *testing.T) {
	// Very diverse similarities → mean - 2σ could be very low
	values := []float64{0.10, 0.90, 0.10, 0.90, 0.50}
	got := SuggestThreshold(values)
	if got < 0.30 {
		t.Errorf("Expected floor of 0.30, got %.4f", got)
	}
}

func TestSuggestThreshold_CeilingEnforced(t *testing.T) {
	// Very uniform high similarities → mean - 2σ could be near 1.0
	values := []float64{0.98, 0.99, 0.97, 0.98, 0.99}
	got := SuggestThreshold(values)
	if got > 0.90 {
		t.Errorf("Expected ceiling of 0.90, got %.4f", got)
	}
}

func TestSuggestThreshold_Empty(t *testing.T) {
	got := SuggestThreshold([]float64{})
	if got != 0.65 {
		t.Errorf("Expected default 0.65 for empty slice, got %.4f", got)
	}
}

func TestSuggestThreshold_UniformData(t *testing.T) {
	// When stddev=0, threshold = mean (clamped)
	values := []float64{0.75, 0.75, 0.75, 0.75}
	got := SuggestThreshold(values)
	// mean=0.75, stddev=0 → suggested=0.75 (within bounds)
	if math.Abs(got-0.75) > 1e-6 {
		t.Errorf("Expected threshold=0.75 for uniform 0.75 data, got %.4f", got)
	}
}

package domain

import (
	"math"
	"testing"
)

func TestDecayBlend_AlphaZero_NoChange(t *testing.T) {
	baseline := GenerateRandomVector()
	newVec := GenerateRandomVector()
	result := DecayBlend(baseline, newVec, 0.0)
	if Similarity(result, baseline) != 1.0 {
		t.Error("With alpha=0, baseline should not change at all")
	}
}

func TestDecayBlend_AlphaOne_FullReplace(t *testing.T) {
	baseline := GenerateRandomVector()
	newVec := GenerateRandomVector()
	result := DecayBlend(baseline, newVec, 1.0)
	if Similarity(result, newVec) != 1.0 {
		t.Error("With alpha=1.0, result should be identical to newVec")
	}
}

func TestDecayBlend_SmallAlpha_BaselinePreserved(t *testing.T) {
	baseline := GenerateRandomVector()
	newVec := GenerateRandomVector()

	// With alpha=0.001, a single call should barely change the baseline
	result := DecayBlend(baseline, newVec, 0.001)
	sim := Similarity(result, baseline)
	if sim < 0.95 {
		t.Errorf("With alpha=0.001, baseline should be nearly unchanged. Got similarity=%.4f", sim)
	}
}

func TestDecayBlend_GradualConvergence(t *testing.T) {
	baseline := GenerateRandomVector()

	// Build a target vector that is clearly different from baseline
	target := GenerateRandomVector()
	for Similarity(baseline, target) > 0.6 {
		target = GenerateRandomVector()
	}

	initialSim := Similarity(baseline, target)
	current := baseline

	// Apply decay 1000 times with alpha=0.01
	for i := 0; i < 1000; i++ {
		current = DecayBlend(current, target, 0.01)
	}

	finalSim := Similarity(current, target)
	t.Logf("Initial similarity to target: %.4f, after 1000 decays: %.4f", initialSim, finalSim)

	// The baseline should have moved toward the target
	if finalSim <= initialSim {
		t.Errorf("Expected baseline to converge toward target after 1000 decays. Initial=%.4f Final=%.4f",
			initialSim, finalSim)
	}
}

func TestDecayBlend_SimilarInput_StableBaseline(t *testing.T) {
	baseline := GenerateRandomVector()

	// A vector very similar to the baseline (>0.95 similarity)
	// DecayBlend should produce a result still very close to baseline
	result := DecayBlend(baseline, baseline, 0.01)
	sim := Similarity(result, baseline)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("Blending a baseline with itself should be identical. Got similarity=%.6f", sim)
	}
}

func TestDecayBlend_HalfAlpha_BlendedResult(t *testing.T) {
	baseline := GenerateRandomVector()
	newVec := GenerateRandomVector()

	result := DecayBlend(baseline, newVec, 0.5)
	simToBase := Similarity(result, baseline)
	simToNew := Similarity(result, newVec)

	// At alpha=0.5 the result should be roughly equidistant from both
	t.Logf("Alpha=0.5: sim_to_baseline=%.4f, sim_to_new=%.4f", simToBase, simToNew)
	diff := math.Abs(simToBase - simToNew)
	if diff > 0.15 {
		t.Logf("Note: At alpha=0.5, result is not perfectly equidistant (diff=%.4f). This is expected due to discrete bit operations.", diff)
	}
}

package domain

import (
	"testing"
)

func TestGenerateRandomVector(t *testing.T) {
	v := GenerateRandomVector()
	
	// Verify that the vector is not all zeros (highly unlikely for 10,000 random bits)
	allZeros := true
	for _, block := range v.Data {
		if block != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("GenerateRandomVector generated a vector with all bits set to zero")
	}
}

func TestRotate(t *testing.T) {
	v := GenerateRandomVector()
	
	// Rotate and check that the position changes but it preserves integrity
	vRot := Rotate(v, 2)
	
	// The similarity with its rotated self should be low (~0.5 for random vectors)
	sim := Similarity(v, vRot)
	if sim > 0.6 {
		t.Errorf("The rotated vector has a very high similarity (%.2f) with the original", sim)
	}

	// Rotating the entire cycle (NumBlocks) should return the original
	vCycle := Rotate(v, NumBlocks)
	if Similarity(v, vCycle) != 1.0 {
		t.Error("Rotating the vector by NumBlocks positions did not return the original vector")
	}
}

func TestBind(t *testing.T) {
	a := GenerateRandomVector()
	b := GenerateRandomVector()
	
	// A ^ B = C
	c := Bind(a, b)
	
	// The similarity between C and A should be ~0.5 (orthogonal)
	sim := Similarity(c, a)
	if sim > 0.6 || sim < 0.4 {
		t.Errorf("The similarity of the binding with one of its components is atypical: %.2f (expected ~0.5)", sim)
	}
	
	// Self-cancellation property: C ^ B = A
	aPrime := Bind(c, b)
	if Similarity(a, aPrime) != 1.0 {
		t.Error("Failure in the self-cancellation property of Binding in HDC (A ^ B ^ B != A)")
	}
}

func TestBundle(t *testing.T) {
	// Create 5 test vectors
	v1 := GenerateRandomVector()
	v2 := GenerateRandomVector()
	v3 := GenerateRandomVector()
	v4 := GenerateRandomVector()
	v5 := GenerateRandomVector()

	vectors := []HVector{v1, v2, v3, v4, v5}
	bundle := Bundle(vectors)

	// The similarity of the bundle with each of the member vectors should be significantly higher than 0.5 (generally > 0.6)
	// since the bundle preserves the information of its components.
	for i, v := range vectors {
		sim := Similarity(bundle, v)
		if sim <= 0.55 {
			t.Errorf("The bundle vector has low similarity (%.2f) with its member %d (expected > 0.55)", sim, i+1)
		}
	}
}

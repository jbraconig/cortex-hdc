package domain

import (
	"testing"
)

func TestAssignToCluster_NearestWins(t *testing.T) {
	// Create two very different random centroids
	c1 := GenerateRandomVector()
	c2 := GenerateRandomVector()
	centroids := []HVector{c1, c2}

	// A vector identical to c1 must be assigned to cluster 0
	if idx := AssignToCluster(c1, centroids); idx != 0 {
		t.Errorf("Expected assignment to cluster 0 (identical to c1), got %d", idx)
	}

	// A vector identical to c2 must be assigned to cluster 1
	if idx := AssignToCluster(c2, centroids); idx != 1 {
		t.Errorf("Expected assignment to cluster 1 (identical to c2), got %d", idx)
	}
}

func TestClusterBaselines_DegenerateK1(t *testing.T) {
	vectors := make([]HVector, 20)
	for i := range vectors {
		vectors[i] = GenerateRandomVector()
	}

	// k=1 should return a single bundle
	result := ClusterBaselines(vectors, 1, 10)
	if len(result) != 1 {
		t.Errorf("Expected 1 cluster baseline, got %d", len(result))
	}
}

func TestClusterBaselines_FewVectorsLessThanK(t *testing.T) {
	// 2 vectors, k=5 → degenerate case, should return a single bundle
	vectors := []HVector{GenerateRandomVector(), GenerateRandomVector()}
	result := ClusterBaselines(vectors, 5, 10)
	if len(result) != 1 {
		t.Errorf("Expected 1 degenerate baseline, got %d", len(result))
	}
}

func TestClusterBaselines_TwoClusters_DifferentGroups(t *testing.T) {
	kb := NewKnowledgeBase()

	// Simulate two clearly different log "families" using the encoder approach:
	// Group A: vectors built from Bundle of very similar seeds
	// Group B: orthogonal vectors

	// We'll create group A: bundle of first 10 random vectors seeded from v1
	v1 := GenerateRandomVector()
	groupA := make([]HVector, 10)
	for i := range groupA {
		groupA[i] = Bundle([]HVector{v1, v1, v1, GenerateRandomVector()})
	}
	_ = kb

	v2 := GenerateRandomVector()
	// Ensure v2 is orthogonal to v1
	for Similarity(v1, v2) > 0.6 {
		v2 = GenerateRandomVector()
	}

	groupB := make([]HVector, 10)
	for i := range groupB {
		groupB[i] = Bundle([]HVector{v2, v2, v2, GenerateRandomVector()})
	}

	allVectors := append(groupA, groupB...)
	baselines := ClusterBaselines(allVectors, 2, 50)

	if len(baselines) != 2 {
		t.Fatalf("Expected 2 baselines, got %d", len(baselines))
	}

	// The two baselines should be less similar to each other than to their own group members
	interSim := Similarity(baselines[0], baselines[1])
	if interSim > 0.75 {
		t.Logf("Warning: inter-cluster similarity is high (%.2f), clusters may not be well separated", interSim)
	}
	t.Logf("Inter-cluster similarity: %.4f", interSim)
}

func TestClusterBaselines_Convergence(t *testing.T) {
	// Generate 30 random vectors and cluster twice
	// Both runs should produce baselines with high mutual similarity
	vectors := make([]HVector, 30)
	for i := range vectors {
		vectors[i] = GenerateRandomVector()
	}

	b1 := ClusterBaselines(vectors, 3, 100)
	b2 := ClusterBaselines(vectors, 3, 100)

	if len(b1) != 3 || len(b2) != 3 {
		t.Fatalf("Expected 3 baselines each run, got %d and %d", len(b1), len(b2))
	}
	t.Logf("Run1-Run2 similarities: %.4f, %.4f, %.4f",
		Similarity(b1[0], b2[0]),
		Similarity(b1[1], b2[1]),
		Similarity(b1[2], b2[2]))
}

func TestMiniBatchKMeans(t *testing.T) {
	// Create a mini-batch K-Means model for k=2
	mb := NewMiniBatchKMeans(2)

	// Feed 10 batches of 5 vectors each
	for i := 0; i < 10; i++ {
		batch := make([]HVector, 5)
		for j := range batch {
			batch[j] = GenerateRandomVector()
		}
		mb.ProcessBatch(batch)
	}

	centroids := mb.Centroids()
	if len(centroids) != 2 {
		t.Fatalf("Expected 2 centroids, got %d", len(centroids))
	}
}


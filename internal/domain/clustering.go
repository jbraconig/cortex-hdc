package domain

// ClusterBaselines groups a set of HVectors into k clusters using
// an HDC-adapted K-Means algorithm with Hamming distance.
// Returns a slice of k centroid HVectors (baselines).
func ClusterBaselines(vectors []HVector, k int, maxIter int) []HVector {
	if k <= 1 || len(vectors) <= k {
		// Degenerate case: return a single bundle
		return []HVector{Bundle(vectors)}
	}

	// --- Initialization: K-Means++ adapted to Hamming distance ---
	centroids := kMeansPlusPlusInit(vectors, k)

	// --- Iterative assignment and centroid update ---
	for iter := 0; iter < maxIter; iter++ {
		// Assign each vector to its nearest centroid
		clusters := make([][]HVector, k)
		for i := range clusters {
			clusters[i] = make([]HVector, 0)
		}

		for _, vec := range vectors {
			idx := AssignToCluster(vec, centroids)
			clusters[idx] = append(clusters[idx], vec)
		}

		// Recalculate centroids via Bundle (majority vote)
		converged := true
		for i := 0; i < k; i++ {
			if len(clusters[i]) == 0 {
				// Empty cluster: reinitialize with a random vector from the dataset
				clusters[i] = append(clusters[i], vectors[i%len(vectors)])
			}

			newCentroid := Bundle(clusters[i])
			if Similarity(newCentroid, centroids[i]) < 0.999 {
				converged = false
			}
			centroids[i] = newCentroid
		}

		if converged {
			break
		}
	}

	return centroids
}

// kMeansPlusPlusInit selects k diverse initial centroids using the K-Means++
// strategy adapted for Hamming distance: each new centroid is chosen
// from the vector with the lowest maximum similarity to existing centroids.
func kMeansPlusPlusInit(vectors []HVector, k int) []HVector {
	centroids := make([]HVector, 0, k)

	// Choose first centroid randomly (use index 0 as a deterministic seed)
	centroids = append(centroids, vectors[0])

	for len(centroids) < k {
		// Find the vector with the minimum best-similarity to existing centroids
		// (i.e., furthest away in Hamming space → most diverse)
		minMaxSim := 2.0
		bestIdx := 0

		for i, vec := range vectors {
			maxSim := 0.0
			for _, c := range centroids {
				sim := Similarity(vec, c)
				if sim > maxSim {
					maxSim = sim
				}
			}
			// We want the vector least similar to any existing centroid
			if maxSim < minMaxSim {
				minMaxSim = maxSim
				bestIdx = i
			}
		}

		centroids = append(centroids, vectors[bestIdx])
	}

	return centroids
}

// AssignToCluster returns the index of the nearest centroid using Similarity (Hamming).
func AssignToCluster(vec HVector, centroids []HVector) int {
	bestIdx := 0
	bestSim := -1.0

	for i, c := range centroids {
		sim := Similarity(vec, c)
		if sim > bestSim {
			bestSim = sim
			bestIdx = i
		}
	}

	return bestIdx
}

// MiniBatchKMeans holds the state of the streaming Mini-Batch K-Means algorithm.
type MiniBatchKMeans struct {
	k         int
	centroids []HVector
	vCounts   []int // Number of updates per centroid
}

// NewMiniBatchKMeans creates a new MiniBatchKMeans runner.
func NewMiniBatchKMeans(k int) *MiniBatchKMeans {
	return &MiniBatchKMeans{
		k:       k,
		vCounts: make([]int, k),
	}
}

// Centroids returns the current cluster centroids.
func (m *MiniBatchKMeans) Centroids() []HVector {
	return m.centroids
}

// ProcessBatch updates centroids using the mini-batch vectors.
func (m *MiniBatchKMeans) ProcessBatch(batch []HVector) {
	if len(batch) == 0 {
		return
	}

	// Initialize centroids if they are empty
	if len(m.centroids) == 0 {
		if len(batch) <= m.k {
			// Degenerate case: use all vectors in batch as centroids
			m.centroids = make([]HVector, len(batch))
			copy(m.centroids, batch)
			for i := range m.centroids {
				m.vCounts[i] = 1
			}
			return
		}
		// Initialize centroids using K-Means++ on the first batch
		m.centroids = kMeansPlusPlusInit(batch, m.k)
		for i := range m.centroids {
			m.vCounts[i] = 1
		}
	}

	// For each vector in the batch, assign to nearest centroid and update centroid
	for _, vec := range batch {
		idx := AssignToCluster(vec, m.centroids)
		m.vCounts[idx]++

		// Learning rate decay: eta = 1 / vCounts[idx]
		// Clamp at a minimum of 0.01 to allow perpetual learning/adaptation during streaming.
		eta := 1.0 / float64(m.vCounts[idx])
		if eta < 0.01 {
			eta = 0.01
		}

		m.centroids[idx] = DecayBlend(m.centroids[idx], vec, eta)
	}
}


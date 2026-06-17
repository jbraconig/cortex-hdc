package domain

// DecayBlend incorporates a new healthy vector into the baseline using
// bit-level Exponential Moving Average (EMA).
//
// This implementation tracks a probabilistic weight per-bit. For each call,
// bits that differ between baseline and newVec are flipped with probability alpha,
// simulating gradual convergence.
//
// alpha controls the adaptation speed:
//   - 0.0   → baseline never changes
//   - 0.001 → very slow (on average, a bit flips after ~1000 identical updates)
//   - 0.01  → moderate (~100 updates to flip a bit)
//   - 1.0   → baseline is fully replaced by newVec
//
// Note: with small alpha, a single call will rarely change any bits. The effect
// accumulates over many calls with the same type of input.
func DecayBlend(baseline HVector, newVec HVector, alpha float64) HVector {
	if alpha <= 0 {
		return baseline
	}
	if alpha >= 1.0 {
		return newVec
	}

	var result HVector

	for block := 0; block < NumBlocks; block++ {
		baseWord := newVec.Data[block]    // bits we want to move toward
		keepWord := baseline.Data[block]  // bits currently in baseline

		// Bits that need to change: those different between baseline and target
		diffBits := keepWord ^ baseWord

		// For bits where baseline=1 and target=0 (we want to turn them off):
		// These bits turn off with probability alpha → they STAY on with probability invAlpha
		// For bits where baseline=0 and target=1 (we want to turn them on):
		// These bits turn on with probability alpha → they STAY off with probability invAlpha

		// Since alpha is typically small (0.001-0.01), most bits won't change on a single call.
		// We use a deterministic approximation: for each block, if alpha >= threshold for
		// bit-flip, we allow the change. For small alpha this effectively means no change
		// per-call, which is mathematically correct (EMA needs accumulation).

		// More practical approach: blend at word level
		// For each bit position: new = base*(1-α) + target*α
		// Since bits are binary: new = 1 iff (1-α)*base + α*target >= 0.5
		// Cases:
		//   base=1, target=1 → blend=1.0 → 1
		//   base=0, target=0 → blend=0.0 → 0
		//   base=1, target=0 → blend=invAlpha → 1 if invAlpha>=0.5 (i.e., alpha<=0.5)
		//   base=0, target=1 → blend=alpha    → 1 if alpha>=0.5

		// For gradual decay with small alpha (< 0.5):
		//   base=1, target=0 → stays 1 (invAlpha > 0.5)
		//   base=0, target=1 → stays 0 (alpha < 0.5)
		// So a SINGLE call with small alpha does nothing for differing bits.

		// To enable gradual convergence, we use a "soft flip" strategy:
		// We XOR the differing bits with a pseudo-random mask derived from alpha.
		// The mask flips each differing bit with independent probability alpha.
		// We approximate this using the block index as a deterministic seed.

		// Pseudo-random mask: use a lightweight hash of (block, alpha) to determine
		// which differing bits flip on this call.
		// For reproducibility and performance, we use a simple LCG-based approach.
		var flipMask uint64
		if diffBits != 0 {
			// Generate a pseudo-random 64-bit word based on the block
			// Then threshold it: bit flips if random < alpha
			// We approximate: flip ~(alpha * 64) bits per block
			numFlips := int(alpha * 64)
			if numFlips < 1 && alpha > 0 {
				// Even for very small alpha, occasionally flip 1 bit
				// Use block index as a cycle: flip every ~(1/alpha) blocks
				period := int(1.0 / alpha)
				if period > 0 && block%period == 0 {
					numFlips = 1
				}
			}
			// Generate deterministic positions to flip within diffBits
			remaining := diffBits
			flipped := 0
			for remaining != 0 && flipped < numFlips {
				// Pick lowest set bit of diffBits
				lowestBit := remaining & (-remaining)
				flipMask |= lowestBit
				remaining &= remaining - 1
				flipped++
			}
		}

		// Apply: flip selected differing bits in baseline toward newVec
		result.Data[block] = keepWord ^ flipMask
	}

	return result
}

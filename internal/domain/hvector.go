package domain

import (
	"math/bits"
	"math/rand"
)

const (
	Dimensions = 10000
	BlockSize  = 64
	NumBlocks  = (Dimensions + BlockSize - 1) / BlockSize
)

// HVector represents our 10,000-bit memory chunk
type HVector struct {
	Data [NumBlocks]uint64
}

// GenerateRandomVector creates an HVector with random bits
func GenerateRandomVector() HVector {
	var v HVector
	for i := 0; i < NumBlocks; i++ {
		v.Data[i] = rand.Uint64()
	}
	return v
}

// Rotate shifts the vector, useful for preserving order (sequences)
func Rotate(v HVector, positions int) HVector {
	var result HVector
	for i := 0; i < NumBlocks; i++ {
		newPos := (i + positions) % NumBlocks
		result.Data[newPos] = v.Data[i]
	}
	return result
}

// Bind associates two vectors using XOR
func Bind(a, b HVector) HVector {
	var result HVector
	for i := 0; i < NumBlocks; i++ {
		result.Data[i] = a.Data[i] ^ b.Data[i]
	}
	return result
}

// Bundle superposes multiple vectors and applies majority rule to create a combined fingerprint
func Bundle(vectors []HVector) HVector {
	numVecs := len(vectors)
	if numVecs == 0 {
		return GenerateRandomVector()
	}

	var result HVector
	threshold := numVecs / 2

	// We process block by block.
	// This allows us to use a very small counters array (64 ints) that fits entirely in the L1 CPU cache.
	for block := 0; block < NumBlocks; block++ {
		var counts [64]int

		// We iterate by index to AVOID copying the HVector structure (~1.2KB) in each loop of the for-range
		for i := 0; i < numVecs; i++ {
			word := vectors[i].Data[block]

			// Kernighan's algorithm + hardware TZCNT (Trailing Zeros Count) instruction.
			// Instead of iterating 64 times asking "is this bit 1?", we jump directly to the bits that ARE 1.
			for word != 0 {
				bitPos := bits.TrailingZeros64(word)
				counts[bitPos]++
				word &= word - 1 // Turn off the least significant bit that is on
			}
		}

		// We reconstruct the resulting 64-bit block by applying the majority rule
		var resBlock uint64
		for bit := 0; bit < 64; bit++ {
			if counts[bit] > threshold {
				resBlock |= (1 << bit)
			}
		}
		result.Data[block] = resBlock
	}

	return result
}

// Similarity compares two vectors and returns the similarity (0.0 to 1.0) using Hamming distance
func Similarity(a, b HVector) float64 {
	diffBits := 0
	for i := 0; i < NumBlocks; i++ {
		xor := a.Data[i] ^ b.Data[i]
		diffBits += bits.OnesCount64(xor)
	}
	return 1.0 - (float64(diffBits) / float64(Dimensions))
}

package domain

import (
	"sync"
)

// KnowledgeBase unifies everything the engine needs to remember.
// Backward compatible: files that only contain Baseline will still decode
// correctly because Baselines and SuggestedThreshold default to zero values.
type KnowledgeBase struct {
	ItemMemory         map[rune]HVector // Immutable seed vectors for characters
	Baseline           HVector          // Single-cluster baseline (legacy / k=1)
	Baselines          []HVector        // Multi-cluster baselines (Phase 3.1)
	SuggestedThreshold float64          // Auto-tuned threshold (Phase 3.2); 0 means not set
	mu                 sync.RWMutex
}

// NewKnowledgeBase initializes an empty KB
func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{
		ItemMemory: make(map[rune]HVector),
	}
}

// GetLetterVector retrieves the vector for a character or generates a new one if it does not exist
func (kb *KnowledgeBase) GetLetterVector(char rune) HVector {
	kb.mu.RLock()
	if val, exists := kb.ItemMemory[char]; exists {
		kb.mu.RUnlock()
		return val
	}
	kb.mu.RUnlock()

	kb.mu.Lock()
	defer kb.mu.Unlock()

	// Double-check after acquiring write lock
	if val, exists := kb.ItemMemory[char]; exists {
		return val
	}

	v := GenerateRandomVector()
	kb.ItemMemory[char] = v
	return v
}

// BestSimilarity returns the highest similarity score between vec and any baseline.
// If multi-cluster baselines exist (Phase 3.1), it compares against each cluster
// and returns the maximum. Falls back to the single Baseline for backward compatibility.
func (kb *KnowledgeBase) BestSimilarity(vec HVector) float64 {
	if len(kb.Baselines) > 0 {
		best := 0.0
		for _, b := range kb.Baselines {
			if s := Similarity(vec, b); s > best {
				best = s
			}
		}
		return best
	}
	return Similarity(vec, kb.Baseline)
}

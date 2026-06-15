package domain

// KnowledgeBase unifies everything the engine needs to remember
type KnowledgeBase struct {
	ItemMemory map[rune]HVector // Immutable seed vectors for characters
	Baseline   HVector          // Combined fingerprint representing the "healthy" log state
}

// NewKnowledgeBase initializes an empty KB
func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{
		ItemMemory: make(map[rune]HVector),
	}
}

// GetLetterVector retrieves the vector for a character or generates a new one if it does not exist
func (kb *KnowledgeBase) GetLetterVector(char rune) HVector {
	if val, exists := kb.ItemMemory[char]; exists {
		return val
	}
	v := GenerateRandomVector()
	kb.ItemMemory[char] = v
	return v
}

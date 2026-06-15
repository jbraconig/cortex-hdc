package hdc

import (
	"strings"

	"github.com/jbraconig/cortex-hdc/internal/domain"
)

// HDCEncoder implements the domain Encoder interface
type HDCEncoder struct{}

func NewHDCEncoder() *HDCEncoder {
	return &HDCEncoder{}
}

// EncodeLine converts an entire line of text into a single vector using trigrams
func (e *HDCEncoder) EncodeLine(kb *domain.KnowledgeBase, line string) domain.HVector {
	// Basic normalization
	line = "^" + strings.TrimSpace(strings.ToLower(line)) + "$"
	var ngrams []domain.HVector

	// Convert to runes to handle special characters correctly
	runes := []rune(line)
	
	if len(runes) < 3 {
		return domain.GenerateRandomVector()
	}

	for i := 0; i < len(runes)-2; i++ {
		v1 := domain.Rotate(kb.GetLetterVector(runes[i]), 2)
		v2 := domain.Rotate(kb.GetLetterVector(runes[i+1]), 1)
		v3 := kb.GetLetterVector(runes[i+2])

		ngramVec := domain.Bind(domain.Bind(v1, v2), v3)
		ngrams = append(ngrams, ngramVec)
	}

	if len(ngrams) == 0 {
		return domain.GenerateRandomVector()
	}

	return domain.Bundle(ngrams)
}

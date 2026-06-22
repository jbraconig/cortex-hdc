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
	// Clean timestamps using fast heuristics (avoiding RegEx)
	cleaned := cleanTimestamp(line)
	
	// Basic normalization
	cleaned = "^" + strings.TrimSpace(strings.ToLower(cleaned)) + "$"
	var ngrams []domain.HVector

	// Convert to runes to handle special characters correctly
	runes := []rune(cleaned)
	
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

func cleanTimestamp(line string) string {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return line
	}

	// Heuristic 1: Bracketed timestamp [2026-06-22 13:05:33] or [22/Jun/2026...]
	if line[0] == '[' {
		endIdx := strings.IndexByte(line, ']')
		if endIdx > 0 && endIdx < 40 && endIdx+1 < len(line) {
			return strings.TrimSpace(line[endIdx+1:])
		}
	}

	// Heuristic 2: ISO8601 single token timestamp (e.g. 2026-06-22T13:05:33-05:00)
	// Check if the first token looks like an ISO date-time (contains 'T', ':', and '-' or '+')
	firstSpace := strings.IndexByte(line, ' ')
	if firstSpace > 0 && firstSpace < 35 {
		firstToken := line[:firstSpace]
		if strings.Contains(firstToken, "T") && strings.IndexByte(firstToken, ':') >= 0 && (strings.IndexByte(firstToken, '-') >= 0 || strings.IndexByte(firstToken, '+') >= 0 || strings.HasSuffix(firstToken, "Z")) {
			return strings.TrimSpace(line[firstSpace+1:])
		}

		// Heuristic 3: Double token timestamp (e.g. "2026-06-22 13:05:33,123 ...")
		// Check if first token contains '-' or '/' (date) and second token contains ':' (time)
		secondSpace := strings.IndexByte(line[firstSpace+1:], ' ')
		if secondSpace > 0 && secondSpace < 25 {
			secondSpaceIdx := firstSpace + 1 + secondSpace
			firstToken := line[:firstSpace]
			secondToken := line[firstSpace+1 : secondSpaceIdx]

			isDate := strings.ContainsAny(firstToken, "-/")
			isTime := strings.IndexByte(secondToken, ':') >= 0
			if isDate && isTime {
				return strings.TrimSpace(line[secondSpaceIdx+1:])
			}
		}
	}

	// Heuristic 4: Syslog format (e.g. "Jun 22 13:05:33 ...")
	// Format is <Month> <Day> <Time>
	if len(line) > 15 {
		if line[3] == ' ' {
			nextSpace := strings.IndexByte(line[4:], ' ')
			if nextSpace > 0 && nextSpace < 5 {
				daySpaceIdx := 4 + nextSpace
				timeSpace := strings.IndexByte(line[daySpaceIdx+1:], ' ')
				if timeSpace > 0 && timeSpace < 12 {
					timeSpaceIdx := daySpaceIdx + 1 + timeSpace
					timeToken := line[daySpaceIdx+1 : timeSpaceIdx]
					if strings.IndexByte(timeToken, ':') >= 0 {
						return strings.TrimSpace(line[timeSpaceIdx+1:])
					}
				}
			}
		}
	}

	return line
}


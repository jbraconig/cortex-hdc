package storage

import (
	"encoding/gob"
	"os"

	"github.com/jbraconig/cortex-hdc/internal/domain"
)

// GobStore implements the Persistence interface
type GobStore struct{}

func NewGobStore() *GobStore {
	return &GobStore{}
}

// Save saves the entire knowledge base into a binary file
func (s *GobStore) Save(kb *domain.KnowledgeBase, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(kb)
}

// Load retrieves the knowledge base from the file
func (s *GobStore) Load(filepath string) (*domain.KnowledgeBase, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var kb domain.KnowledgeBase
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&kb)
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

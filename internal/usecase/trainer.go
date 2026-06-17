package usecase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jbraconig/cortex-hdc/internal/domain"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/logreader"
)

// Trainer orchestrates the training phase
type Trainer struct {
	Encoder     domain.Encoder
	Persistence domain.Persistence
}

func NewTrainer(encoder domain.Encoder, persistence domain.Persistence) *Trainer {
	return &Trainer{
		Encoder:     encoder,
		Persistence: persistence,
	}
}

// TrainFromFile reads a massive log file, encodes it, and generates a baseline
func (t *Trainer) TrainFromFile(filepath string, outputDb string) error {
	fmt.Printf("Starting training phase from: %s\n", filepath)
	startTime := time.Now()

	kb := domain.NewKnowledgeBase()
	
	// We use ReadStaticLogs to read until EOF instead of blocking
	ch, err := logreader.ReadStaticLogs(filepath)
	if err != nil {
		return fmt.Errorf("error reading training file: %w", err)
	}

	var allVectors []domain.HVector
	var linesProcessed int

	for line := range ch {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		vec := t.Encoder.EncodeLine(kb, line)
		allVectors = append(allVectors, vec)
		linesProcessed++
		
		if linesProcessed%10000 == 0 {
			fmt.Printf("Processed %d lines...\n", linesProcessed)
		}
	}

	if len(allVectors) == 0 {
		return fmt.Errorf("training file is empty or contained no valid lines")
	}

	fmt.Printf("Generating healthy Baseline from %d vectors...\n", len(allVectors))
	kb.Baseline = domain.Bundle(allVectors)

	fmt.Printf("Saving memory (KnowledgeBase) to: %s\n", outputDb)
	if err := t.Persistence.Save(kb, outputDb); err != nil {
		return fmt.Errorf("error saving DB: %w", err)
	}

	fmt.Printf("Training completed in %v!\n", time.Since(startTime))
	return nil
}

// TrainFromDirectory reads all files in a directory, encodes them, and generates a baseline
func (t *Trainer) TrainFromDirectory(dirPath string, outputDb string) error {
	fmt.Printf("Starting auto-training phase from directory: %s\n", dirPath)
	startTime := time.Now()

	kb := domain.NewKnowledgeBase()
	var allVectors []domain.HVector
	var totalLinesProcessed int
	var filesProcessed int

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		fmt.Printf("Processing file: %s\n", path)
		ch, err := logreader.ReadStaticLogs(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %w", path, err)
		}

		var fileLines int
		for line := range ch {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			vec := t.Encoder.EncodeLine(kb, line)
			allVectors = append(allVectors, vec)
			fileLines++
			totalLinesProcessed++

			if totalLinesProcessed%10000 == 0 {
				fmt.Printf("Processed %d lines in total...\n", totalLinesProcessed)
			}
		}
		fmt.Printf("Finished file: %s (%d lines)\n", path, fileLines)
		filesProcessed++
		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory: %w", err)
	}

	if len(allVectors) == 0 {
		return fmt.Errorf("no valid log lines found in directory %s", dirPath)
	}

	fmt.Printf("Generating healthy Baseline from %d vectors across %d files...\n", len(allVectors), filesProcessed)
	kb.Baseline = domain.Bundle(allVectors)

	fmt.Printf("Saving memory (KnowledgeBase) to: %s\n", outputDb)
	if err := t.Persistence.Save(kb, outputDb); err != nil {
		return fmt.Errorf("error saving DB: %w", err)
	}

	fmt.Printf("Auto-training completed in %v!\n", time.Since(startTime))
	return nil
}

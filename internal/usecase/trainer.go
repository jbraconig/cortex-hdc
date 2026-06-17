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

// collectVectors reads all lines from a channel and encodes them into HVectors
func (t *Trainer) collectVectors(kb *domain.KnowledgeBase, ch <-chan string, label string) ([]domain.HVector, error) {
	var vectors []domain.HVector
	var linesProcessed int

	for line := range ch {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		vec := t.Encoder.EncodeLine(kb, line)
		vectors = append(vectors, vec)
		linesProcessed++

		if linesProcessed%10000 == 0 {
			fmt.Printf("[%s] Processed %d lines...\n", label, linesProcessed)
		}
	}
	return vectors, nil
}

// applyBaselines assigns either a single Bundle baseline or K cluster baselines,
// then computes the Auto-Tuning threshold and logs suggestions.
func applyBaselines(kb *domain.KnowledgeBase, allVectors []domain.HVector, numClusters int) {
	if numClusters >= 2 {
		fmt.Printf("Clustering %d vectors into %d baselines...\n", len(allVectors), numClusters)
		kb.Baselines = domain.ClusterBaselines(allVectors, numClusters, 50)
		fmt.Printf("Generated %d cluster baselines.\n", len(kb.Baselines))
	} else {
		fmt.Printf("Generating healthy Baseline from %d vectors...\n", len(allVectors))
		kb.Baseline = domain.Bundle(allVectors)
	}

	// --- Auto-Tuning: second pass over training vectors ---
	var similarities []float64
	for _, vec := range allVectors {
		similarities = append(similarities, kb.BestSimilarity(vec))
	}
	suggested := domain.SuggestThreshold(similarities)
	kb.SuggestedThreshold = suggested

	fmt.Printf("[AUTO-TUNE] Distribution: mean=%.4f  stddev=%.4f\n",
		domain.Mean(similarities), domain.StdDev(similarities))
	fmt.Printf("[AUTO-TUNE] Suggested threshold: %.4f  (use --threshold %.4f or set CORTEX_THRESHOLD=%.4f)\n",
		suggested, suggested, suggested)
}

// TrainFromFile reads a log file, encodes it, and generates a baseline.
// numClusters=0 or 1 → single Bundle baseline (default).
// numClusters>=2     → K-Means clustering into K baselines.
func (t *Trainer) TrainFromFile(filePath string, outputDb string, numClusters int) error {
	fmt.Printf("Starting training phase from: %s\n", filePath)
	startTime := time.Now()

	kb := domain.NewKnowledgeBase()

	ch, err := logreader.ReadStaticLogs(filePath)
	if err != nil {
		return fmt.Errorf("error reading training file: %w", err)
	}

	allVectors, err := t.collectVectors(kb, ch, "TRAIN")
	if err != nil {
		return err
	}
	if len(allVectors) == 0 {
		return fmt.Errorf("training file is empty or contained no valid lines")
	}

	applyBaselines(kb, allVectors, numClusters)

	fmt.Printf("Saving memory (KnowledgeBase) to: %s\n", outputDb)
	if err := t.Persistence.Save(kb, outputDb); err != nil {
		return fmt.Errorf("error saving DB: %w", err)
	}

	fmt.Printf("Training completed in %v!\n", time.Since(startTime))
	return nil
}

// TrainFromDirectory reads all files in a directory, encodes them, and generates a baseline.
// Supports multi-cluster baselines via numClusters.
func (t *Trainer) TrainFromDirectory(dirPath string, outputDb string, numClusters int) error {
	fmt.Printf("Starting auto-training phase from directory: %s\n", dirPath)
	startTime := time.Now()

	kb := domain.NewKnowledgeBase()
	var allVectors []domain.HVector
	var totalLines, filesProcessed int

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

		fileVectors, err := t.collectVectors(kb, ch, "AUTO")
		if err != nil {
			return err
		}
		allVectors = append(allVectors, fileVectors...)
		totalLines += len(fileVectors)
		filesProcessed++
		fmt.Printf("Finished file: %s (%d lines)\n", path, len(fileVectors))
		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory: %w", err)
	}
	if len(allVectors) == 0 {
		return fmt.Errorf("no valid log lines found in directory %s", dirPath)
	}

	fmt.Printf("Total: %d lines across %d files.\n", totalLines, filesProcessed)
	applyBaselines(kb, allVectors, numClusters)

	fmt.Printf("Saving memory (KnowledgeBase) to: %s\n", outputDb)
	if err := t.Persistence.Save(kb, outputDb); err != nil {
		return fmt.Errorf("error saving DB: %w", err)
	}

	fmt.Printf("Auto-training completed in %v!\n", time.Since(startTime))
	return nil
}

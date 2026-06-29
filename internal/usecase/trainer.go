package usecase

import (
	"fmt"
	"math"
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
	prefix      string
	maxLines    int
	sanitizer   *logreader.LogSanitizer
}

func NewTrainer(encoder domain.Encoder, persistence domain.Persistence, prefix string, maxLines int, sanitizer *logreader.LogSanitizer) *Trainer {
	return &Trainer{
		Encoder:     encoder,
		Persistence: persistence,
		prefix:      prefix,
		maxLines:    maxLines,
		sanitizer:   sanitizer,
	}
}

type welford struct {
	count int
	mean  float64
	m2    float64
}

func (w *welford) update(x float64) {
	w.count++
	delta := x - w.mean
	w.mean += delta / float64(w.count)
	delta2 := x - w.mean
	w.m2 += delta * delta2
}

func (w *welford) stats() (float64, float64) {
	if w.count == 0 {
		return 0, 0
	}
	if w.count < 2 {
		return w.mean, 0
	}
	variance := w.m2 / float64(w.count)
	return w.mean, math.Sqrt(variance)
}

// processVectors reads and encodes logs from a file or directory, invoking a callback on each HVector.
func (t *Trainer) processVectors(kb *domain.KnowledgeBase, path string, callback func(domain.HVector)) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(path, func(subPath string, subInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if subInfo.IsDir() {
				return nil
			}
			return t.processFileVectors(kb, subPath, callback)
		})
	}

	return t.processFileVectors(kb, path, callback)
}

func (t *Trainer) processFileVectors(kb *domain.KnowledgeBase, filePath string, callback func(domain.HVector)) error {
	ch, err := logreader.ReadStaticLogs(filePath, t.prefix, t.maxLines)
	if err != nil {
		return err
	}
	for line := range ch {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sanitizedLine := line
		if t.sanitizer != nil {
			sanitizedLine = t.sanitizer.Sanitize(line)
		}
		vec := t.Encoder.EncodeLine(kb, sanitizedLine)
		callback(vec)
	}
	return nil
}

// train encapsulates the logic of doing streaming/mini-batch passes on the files.
func (t *Trainer) train(path string, outputDb string, numClusters int) error {
	kb := domain.NewKnowledgeBase()

	// Pass 1: Model training / clustering
	var linesProcessed int
	if numClusters >= 2 {
		fmt.Printf("Clustering vectors into %d baselines...\n", numClusters)
		mb := domain.NewMiniBatchKMeans(numClusters)
		batch := make([]domain.HVector, 0, 5000)

		err := t.processVectors(kb, path, func(vec domain.HVector) {
			batch = append(batch, vec)
			linesProcessed++
			if len(batch) >= 5000 {
				mb.ProcessBatch(batch)
				batch = batch[:0] // Reuse capacity to save allocation overhead
			}
			if linesProcessed%10000 == 0 {
				fmt.Printf("[TRAIN] Processed %d lines...\n", linesProcessed)
			}
		})
		if err != nil {
			return err
		}
		if len(batch) > 0 {
			mb.ProcessBatch(batch)
		}

		kb.Baselines = mb.Centroids()
		if len(kb.Baselines) == 0 {
			return fmt.Errorf("training data is empty or contained no valid lines")
		}
		fmt.Printf("Generated %d cluster baselines.\n", len(kb.Baselines))
	} else {
		fmt.Println("Generating healthy Baseline...")
		accumulator := domain.NewBundleAccumulator()

		err := t.processVectors(kb, path, func(vec domain.HVector) {
			accumulator.Add(vec)
			linesProcessed++
			if linesProcessed%10000 == 0 {
				fmt.Printf("[TRAIN] Processed %d lines...\n", linesProcessed)
			}
		})
		if err != nil {
			return err
		}
		if linesProcessed == 0 {
			return fmt.Errorf("training data is empty or contained no valid lines")
		}
		kb.Baseline = accumulator.Result()
	}

	// Pass 2: Auto-Tuning / threshold suggestion
	fmt.Println("Auto-tuning threshold (Pass 2)...")
	var w welford
	err := t.processVectors(kb, path, func(vec domain.HVector) {
		sim := kb.BestSimilarity(vec)
		w.update(sim)
	})
	if err != nil {
		return err
	}

	mean, stddev := w.stats()
	suggested := mean - 2*stddev
	if suggested < 0.30 {
		suggested = 0.30
	}
	if suggested > 0.90 {
		suggested = 0.90
	}
	kb.SuggestedThreshold = suggested

	fmt.Printf("[AUTO-TUNE] Distribution: mean=%.4f  stddev=%.4f\n", mean, stddev)
	fmt.Printf("[AUTO-TUNE] Suggested threshold: %.4f  (use --threshold %.4f or set CORTEX_THRESHOLD=%.4f)\n",
		suggested, suggested, suggested)

	fmt.Printf("Saving memory (KnowledgeBase) to: %s\n", outputDb)
	if err := t.Persistence.Save(kb, outputDb); err != nil {
		return fmt.Errorf("error saving DB: %w", err)
	}

	return nil
}

// TrainFromFile reads a log file in streaming chunks and generates a baseline/baselines database.
func (t *Trainer) TrainFromFile(filePath string, outputDb string, numClusters int) error {
	fmt.Printf("Starting training phase from: %s\n", filePath)
	startTime := time.Now()
	if err := t.train(filePath, outputDb, numClusters); err != nil {
		return err
	}
	fmt.Printf("Training completed in %v!\n", time.Since(startTime))
	return nil
}

// TrainFromDirectory reads all files in a directory in streaming chunks and generates a baseline/baselines database.
func (t *Trainer) TrainFromDirectory(dirPath string, outputDb string, numClusters int) error {
	fmt.Printf("Starting auto-training phase from directory: %s\n", dirPath)
	startTime := time.Now()
	if err := t.train(dirPath, outputDb, numClusters); err != nil {
		return err
	}
	fmt.Printf("Auto-training completed in %v!\n", time.Since(startTime))
	return nil
}

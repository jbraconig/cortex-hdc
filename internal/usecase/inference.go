package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jbraconig/cortex-hdc/internal/domain"
)

// Inference orchestrates real-time detection
type Inference struct {
	Encoder   domain.Encoder
	LogReader domain.LogReader
	Notifier  domain.AlertNotifier
	Threshold float64
}

func NewInference(encoder domain.Encoder, logReader domain.LogReader, notifier domain.AlertNotifier, threshold float64) *Inference {
	return &Inference{
		Encoder:   encoder,
		LogReader: logReader,
		Notifier:  notifier,
		Threshold: threshold,
	}
}

// Run starts the log reading and worker pool
func (i *Inference) Run(ctx context.Context, kb *domain.KnowledgeBase, logFile string, numWorkers int) error {
	fmt.Printf("Starting Inference (Workers: %d, Threshold: %.2f) on: %s\n", numWorkers, i.Threshold, logFile)
	
	logsStream, err := i.LogReader.ReadLogs(ctx, logFile)
	if err != nil {
		return fmt.Errorf("could not open log stream: %w", err)
	}

	var wg sync.WaitGroup

	// Start Worker Pool
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for logLine := range logsStream {
				logLine = strings.TrimSpace(logLine)
				if logLine == "" {
					continue
				}

				// Convert line to high-dimensional vector
				vectorLog := i.Encoder.EncodeLine(kb, logLine)

				// Compare against the healthy baseline state
				similitud := domain.Similarity(vectorLog, kb.Baseline)

				// If it falls below the threshold, it is an anomaly
				if similitud < i.Threshold {
					_ = i.Notifier.Notify(logLine, similitud)
				}
			}
		}(w)
	}

	// Wait (this will block indefinitely unless the channel is closed by the reader)
	wg.Wait()
	return nil
}

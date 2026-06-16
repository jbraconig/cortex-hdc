package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jbraconig/cortex-hdc/internal/domain"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/metrics"
)

// Inference orchestrates real-time detection
type Inference struct {
	Encoder   domain.Encoder
	LogReader domain.LogReader
	Notifier  domain.AlertNotifier
	Threshold float64
	Verbose   bool
}

func NewInference(encoder domain.Encoder, logReader domain.LogReader, notifier domain.AlertNotifier, threshold float64, verbose bool) *Inference {
	return &Inference{
		Encoder:   encoder,
		LogReader: logReader,
		Notifier:  notifier,
		Threshold: threshold,
		Verbose:   verbose,
	}
}

// Run starts the log reading and worker pool
func (i *Inference) Run(ctx context.Context, kb *domain.KnowledgeBase, logFile string, numWorkers int) error {
	fmt.Printf("Starting Inference (Workers: %d, Threshold: %.2f, Verbose: %v) on: %s\n", numWorkers, i.Threshold, i.Verbose, logFile)
	
	logsStream, err := i.LogReader.ReadLogs(ctx, logFile)
	if err != nil {
		return fmt.Errorf("could not open log stream: %w", err)
	}

	var wg sync.WaitGroup

	// ANSI Colors
	colorRed := "\033[31m"
	colorReset := "\033[0m"
	colorGray := "\033[90m"

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

				if metrics.GlobalMetrics != nil {
					metrics.GlobalMetrics.LogsProcessed.Inc()
				}

				// Convert line to high-dimensional vector
				vectorLog := i.Encoder.EncodeLine(kb, logLine)

				// Compare against the healthy baseline state
				similitud := domain.Similarity(vectorLog, kb.Baseline)

				if metrics.GlobalMetrics != nil {
					metrics.GlobalMetrics.SimilarityScore.Observe(similitud)
				}

				// If it falls below the threshold, it is an anomaly
				if similitud < i.Threshold {
					if metrics.GlobalMetrics != nil {
						metrics.GlobalMetrics.AnomaliesDetected.Inc()
					}
					fmt.Printf("%s[ANOMALY: %.2f%%] %s%s\n", colorRed, similitud*100, logLine, colorReset)
					_ = i.Notifier.Notify(logLine, similitud)
				} else if i.Verbose {
					fmt.Printf("%s[OK: %.2f%%] %s%s\n", colorGray, similitud*100, logLine, colorReset)
				}
			}
		}(w)
	}

	// Wait (this will block indefinitely unless the channel is closed by the reader)
	wg.Wait()
	return nil
}

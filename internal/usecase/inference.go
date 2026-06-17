package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jbraconig/cortex-hdc/internal/domain"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/metrics"
)

// Inference orchestrates real-time detection
type Inference struct {
	Encoder    domain.Encoder
	LogReader  domain.LogReader
	Notifier   domain.AlertNotifier
	Store      domain.Persistence
	Threshold  float64
	Verbose    bool
	DecayRate  float64 // 0 = disabled; 0.001 = slow; 0.01 = moderate
	mu         sync.Mutex
}

func NewInference(
	encoder domain.Encoder,
	logReader domain.LogReader,
	notifier domain.AlertNotifier,
	store domain.Persistence,
	threshold float64,
	verbose bool,
	decayRate float64,
) *Inference {
	return &Inference{
		Encoder:   encoder,
		LogReader: logReader,
		Notifier:  notifier,
		Store:     store,
		Threshold: threshold,
		Verbose:   verbose,
		DecayRate: decayRate,
	}
}

// Run starts the log reading and worker pool
func (i *Inference) Run(ctx context.Context, kb *domain.KnowledgeBase, logFiles []string, numWorkers int, dbFile string) error {
	fmt.Printf("Starting Inference (Workers: %d, Threshold: %.2f, Verbose: %v, Decay: %.4f) on %d file(s):\n",
		numWorkers, i.Threshold, i.Verbose, i.DecayRate, len(logFiles))
	for _, f := range logFiles {
		fmt.Printf("  - %s\n", f)
	}

	logsStream, err := i.LogReader.ReadLogs(ctx, logFiles)
	if err != nil {
		return fmt.Errorf("could not open log stream: %w", err)
	}

	// --- Auto-save goroutine for Memory Decay ---
	if i.DecayRate > 0 && i.Store != nil && dbFile != "" {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					i.mu.Lock()
					_ = i.Store.Save(kb, dbFile)
					i.mu.Unlock()
					fmt.Println("[CORTEX] Baseline auto-saved on shutdown (decay checkpoint)")
					return
				case <-ticker.C:
					i.mu.Lock()
					_ = i.Store.Save(kb, dbFile)
					i.mu.Unlock()
					fmt.Println("[CORTEX] Baseline auto-saved (decay checkpoint)")
				}
			}
		}()
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

				// Compare against the best matching baseline (single or multi-cluster)
				similitud := kb.BestSimilarity(vectorLog)

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
				} else {
					if i.Verbose {
						fmt.Printf("%s[OK: %.2f%%] %s%s\n", colorGray, similitud*100, logLine, colorReset)
					}

					// --- Memory Decay (Phase 3.3): adapt baseline toward healthy logs ---
					if i.DecayRate > 0 {
						i.mu.Lock()
						if len(kb.Baselines) > 0 {
							// Update the nearest cluster baseline
							bestIdx := domain.AssignToCluster(vectorLog, kb.Baselines)
							kb.Baselines[bestIdx] = domain.DecayBlend(kb.Baselines[bestIdx], vectorLog, i.DecayRate)
						} else {
							kb.Baseline = domain.DecayBlend(kb.Baseline, vectorLog, i.DecayRate)
						}
						i.mu.Unlock()
					}
				}
			}
		}(w)
	}

	wg.Wait()
	return nil
}

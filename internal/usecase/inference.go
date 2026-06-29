package usecase

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jbraconig/cortex-hdc/internal/domain"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/logreader"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/metrics"
)

// Inference orchestrates real-time detection
type Inference struct {
	Encoder     domain.Encoder
	LogReader   domain.LogReader
	Notifier    domain.AlertNotifier
	Store       domain.Persistence
	Threshold   float64
	Verbose     bool
	DecayRate   float64 // 0 = disabled; 0.001 = slow; 0.01 = moderate
	ClusterSync domain.ClusterSync
	Telemetry         domain.TelemetryClient
	SendRawLogs       bool
	HeartbeatInterval int
	Sanitizer         *logreader.LogSanitizer
	mu                sync.Mutex
}

func NewInference(
	encoder domain.Encoder,
	logReader domain.LogReader,
	notifier domain.AlertNotifier,
	store domain.Persistence,
	threshold float64,
	verbose bool,
	decayRate float64,
	clusterSync domain.ClusterSync,
	telemetry domain.TelemetryClient,
	sendRawLogs bool,
	heartbeatInterval int,
	sanitizer *logreader.LogSanitizer,
) *Inference {
	return &Inference{
		Encoder:           encoder,
		LogReader:         logReader,
		Notifier:          notifier,
		Store:             store,
		Threshold:         threshold,
		Verbose:           verbose,
		DecayRate:         decayRate,
		ClusterSync:       clusterSync,
		Telemetry:         telemetry,
		SendRawLogs:       sendRawLogs,
		HeartbeatInterval: heartbeatInterval,
		Sanitizer:         sanitizer,
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

	// --- Heartbeat goroutine ---
	if i.Telemetry != nil && i.HeartbeatInterval > 0 {
		go func() {
			nodeID := "local-agent"
			if i.ClusterSync != nil {
				nodeID = i.ClusterSync.NodeName()
			} else {
				if hostname, err := os.Hostname(); err == nil {
					nodeID = hostname
				}
			}

			// Send initial heartbeat immediately
			i.Telemetry.SendHeartbeat(nodeID)

			ticker := time.NewTicker(time.Duration(i.HeartbeatInterval) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					i.Telemetry.SendHeartbeat(nodeID)
				}
			}
		}()
	}

	// --- Auto-save goroutine for Memory Decay ---
	if i.DecayRate > 0 && i.Store != nil && dbFile != "" {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
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

	var decayUpdates chan domain.HVector
	var decayWg sync.WaitGroup
	if i.DecayRate > 0 {
		decayUpdates = make(chan domain.HVector, 2000)
		decayWg.Add(1)
		go func() {
			defer decayWg.Done()
			for vec := range decayUpdates {
				i.mu.Lock()
				kb.UpdateBaseline(vec, i.DecayRate)
				i.mu.Unlock()

				// Phase 4.3: Broadcast baseline update to cluster asynchronously
				if i.ClusterSync != nil {
					_ = i.ClusterSync.BroadcastBaseline(vec, i.DecayRate)
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

				sanitizedLog := logLine
				if i.Sanitizer != nil {
					sanitizedLog = i.Sanitizer.Sanitize(logLine)
				}

				// Convert line to high-dimensional vector
				vectorLog := i.Encoder.EncodeLine(kb, sanitizedLog)

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
					fmt.Printf("%s[ANOMALY: %.2f%%] %s%s\n", colorRed, similitud*100, sanitizedLog, colorReset)
					_ = i.Notifier.Notify(logLine, similitud)

					// Report anomaly to SaaS via gRPC
					if i.Telemetry != nil {
						vecBytes := vectorLog.Serialize()
						nodeID := "local-agent"
						if i.ClusterSync != nil {
							nodeID = i.ClusterSync.NodeName()
						} else {
							if hostname, err := os.Hostname(); err == nil {
								nodeID = hostname
							}
						}
						
						rawLogToSend := ""
						if i.SendRawLogs {
							rawLogToSend = logLine
						}
						i.Telemetry.ReportAnomaly(nodeID, similitud, time.Now().Unix(), vecBytes, rawLogToSend, i.Threshold)
					}
				} else {
					if i.Verbose {
						fmt.Printf("%s[OK: %.2f%%] %s%s\n", colorGray, similitud*100, sanitizedLog, colorReset)
					}

					// --- Memory Decay (Phase 3.3): adapt baseline toward healthy logs ---
					if i.DecayRate > 0 {
						select {
						case decayUpdates <- vectorLog:
						default:
							// Drop update if channel is saturated to prioritize throughput
						}
					}
				}
			}
		}(w)
	}

	wg.Wait()
	if i.DecayRate > 0 {
		close(decayUpdates)
		decayWg.Wait()
	}

	// Save baseline synchronously at the very end of inference run
	if i.Store != nil && dbFile != "" {
		i.mu.Lock()
		_ = i.Store.Save(kb, dbFile)
		i.mu.Unlock()
		fmt.Println("[CORTEX] Baseline auto-saved on shutdown (decay checkpoint)")
	}
	return nil
}

package metrics

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CortexMetrics holds the prometheus instruments
type CortexMetrics struct {
	LogsProcessed     prometheus.Counter
	AnomaliesDetected prometheus.Counter
	SimilarityScore   prometheus.Histogram
}

var (
	GlobalMetrics *CortexMetrics
)

// InitMetrics initializes and starts the prometheus HTTP server
func InitMetrics(port int) {
	GlobalMetrics = &CortexMetrics{
		LogsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cortex_logs_processed_total",
			Help: "The total number of log lines analyzed",
		}),
		AnomaliesDetected: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cortex_anomalies_detected_total",
			Help: "The total number of anomalies detected (below threshold)",
		}),
		SimilarityScore: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "cortex_similarity_score",
			Help:    "Distribution of similarity scores of processed logs",
			Buckets: prometheus.LinearBuckets(0.0, 0.1, 10), // from 0.0 to 1.0
		}),
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("[METRICS] Prometheus server listening on :%d", port)
		if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
			log.Printf("[METRICS] Failed to start prometheus server: %v", err)
		}
	}()
}

package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/logreader"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/metrics"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/notifier"
	"github.com/jbraconig/cortex-hdc/internal/usecase"
)

type InferCommand struct {
	file        string
	workers     int
	threshold   float64
	webhook     string
	verbose     bool
	metricsPort int
	decayRate   float64
}

func (c *InferCommand) Name() string {
	return "infer"
}

func (c *InferCommand) Parse(args []string, cfg *config.Config) {
	inferCmd := flag.NewFlagSet("infer", flag.ExitOnError)
	fileFlag := inferCmd.String("file", "", "Path to the log file to monitor in real time")
	workersFlag := inferCmd.Int("workers", 4, "Number of goroutines for the worker pool")
	thresholdFlag := inferCmd.Float64("threshold", 0.65, "Similarity threshold (0.0 - 1.0) below which an alert is triggered")
	webhookFlag := inferCmd.String("webhook", "", "Webhook URL to send HTTP JSON alerts (optional)")
	verboseFlag := inferCmd.Bool("verbose", false, "Print all log lines, not just anomalies")
	decayRateFlag := inferCmd.Float64("decay-rate", 0, "Decay rate for gradual baseline adaptation (0=disabled, 0.001=slow, 0.01=moderate)")
	inferCmd.Parse(args)

	c.file = cfg.File
	c.workers = cfg.Workers
	c.threshold = cfg.Threshold
	c.webhook = cfg.Webhook
	c.verbose = cfg.Verbose
	c.metricsPort = cfg.MetricsPort
	c.decayRate = cfg.DecayRate

	inferCmd.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "file":
			c.file = *fileFlag
		case "workers":
			c.workers = *workersFlag
		case "threshold":
			c.threshold = *thresholdFlag
		case "webhook":
			c.webhook = *webhookFlag
		case "verbose":
			c.verbose = *verboseFlag
		case "decay-rate":
			c.decayRate = *decayRateFlag
		}
	})

	if c.file == "" {
		fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
		inferCmd.PrintDefaults()
		os.Exit(1)
	}
}

func (c *InferCommand) Execute(deps Dependencies) error {
	const dbFile = "cortex.kv"
	kb, err := deps.Store.Load(dbFile)
	if err != nil {
		fmt.Printf("Error: Could not load the knowledge base (%s). Run 'train' first.\n", dbFile)
		fmt.Printf("Detail: %v\n", err)
		os.Exit(1)
	}

	// Honor auto-tuned threshold if no explicit one was provided
	if kb.SuggestedThreshold > 0 && c.threshold == 0.65 {
		fmt.Printf("[AUTO-TUNE] Using stored suggested threshold: %.4f\n", kb.SuggestedThreshold)
		c.threshold = kb.SuggestedThreshold
	}

	metrics.InitMetrics(c.metricsPort)
	reader := logreader.NewRobustTailReader()
	httpNotifier := notifier.NewHTTPNotifier(c.webhook)
	inference := usecase.NewInference(deps.Encoder, reader, httpNotifier, deps.Store, c.threshold, c.verbose, c.decayRate)

	// Parse comma-separated files
	rawPaths := strings.Split(c.file, ",")
	var logFiles []string
	for _, p := range rawPaths {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			logFiles = append(logFiles, trimmed)
		}
	}

	if len(logFiles) == 0 {
		fmt.Println("Error: Specifying at least one log file via --file or CORTEX_FILE environment variable is required")
		os.Exit(1)
	}

	return RunWithGracefulShutdown(func(ctx context.Context) error {
		return inference.Run(ctx, kb, logFiles, c.workers, dbFile)
	})
}

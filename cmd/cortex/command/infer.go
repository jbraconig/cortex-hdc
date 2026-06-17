package command

import (
	"context"
	"flag"
	"fmt"
	"os"

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
	inferCmd.Parse(args)

	c.file = cfg.File
	c.workers = cfg.Workers
	c.threshold = cfg.Threshold
	c.webhook = cfg.Webhook
	c.verbose = cfg.Verbose
	c.metricsPort = cfg.MetricsPort

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
		}
	})

	if c.file == "" {
		fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
		inferCmd.PrintDefaults()
		os.Exit(1)
	}
}

func (c *InferCommand) Execute(deps Dependencies) error {
	kb, err := deps.Store.Load("cortex.kv")
	if err != nil {
		fmt.Printf("Error: Could not load the knowledge base (%s). Run 'train' first.\n", "cortex.kv")
		fmt.Printf("Detail: %v\n", err)
		os.Exit(1)
	}

	metrics.InitMetrics(c.metricsPort)
	reader := logreader.NewRobustTailReader()
	httpNotifier := notifier.NewHTTPNotifier(c.webhook)
	inference := usecase.NewInference(deps.Encoder, reader, httpNotifier, c.threshold, c.verbose)

	return RunWithGracefulShutdown(func(ctx context.Context) error {
		return inference.Run(ctx, kb, c.file, c.workers)
	})
}

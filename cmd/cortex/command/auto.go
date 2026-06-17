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

type AutoCommand struct {
	file        string
	workers     int
	threshold   float64
	webhook     string
	verbose     bool
	metricsPort int
	initLogs    string
}

func (c *AutoCommand) Name() string {
	return "auto"
}

func (c *AutoCommand) Parse(args []string, cfg *config.Config) {
	autoCmd := flag.NewFlagSet("auto", flag.ExitOnError)
	fileFlag := autoCmd.String("file", "", "Path to the log file to monitor in real time")
	workersFlag := autoCmd.Int("workers", 4, "Number of goroutines for the worker pool")
	thresholdFlag := autoCmd.Float64("threshold", 0.65, "Similarity threshold (0.0 - 1.0) below which an alert is triggered")
	webhookFlag := autoCmd.String("webhook", "", "Webhook URL to send HTTP JSON alerts (optional)")
	verboseFlag := autoCmd.Bool("verbose", false, "Print all log lines, not just anomalies")
	initLogsFlag := autoCmd.String("init-logs", "/data/init-logs/", "Directory containing baseline logs for auto-training")
	autoCmd.Parse(args)

	c.file = cfg.File
	c.workers = cfg.Workers
	c.threshold = cfg.Threshold
	c.webhook = cfg.Webhook
	c.verbose = cfg.Verbose
	c.metricsPort = cfg.MetricsPort
	c.initLogs = cfg.InitLogs

	autoCmd.Visit(func(f *flag.Flag) {
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
		case "init-logs":
			c.initLogs = *initLogsFlag
		}
	})

	if c.file == "" {
		fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
		autoCmd.PrintDefaults()
		os.Exit(1)
	}
}

func (c *AutoCommand) Execute(deps Dependencies) error {
	dbFile := "cortex.kv"

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		fmt.Printf("[CORTEX] No knowledge base found at %s. Auto-training from %s ...\n", dbFile, c.initLogs)
		if info, err := os.Stat(c.initLogs); err == nil && info.IsDir() {
			trainer := usecase.NewTrainer(deps.Encoder, deps.Store)
			if err := trainer.TrainFromDirectory(c.initLogs, dbFile); err != nil {
				fmt.Printf("Error during auto-training: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("Error: init-logs directory '%s' not found or invalid. Mount your baseline logs or run 'train' first.\n", c.initLogs)
			os.Exit(1)
		}
	} else {
		fmt.Printf("[CORTEX] Knowledge base %s found. Skipping auto-training.\n", dbFile)
	}

	kb, err := deps.Store.Load(dbFile)
	if err != nil {
		fmt.Printf("Error: Could not load the knowledge base (%s).\nDetail: %v\n", dbFile, err)
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

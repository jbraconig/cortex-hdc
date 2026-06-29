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

type AutoCommand struct {
	file              string
	workers           int
	threshold         float64
	webhook           string
	verbose           bool
	metricsPort       int
	initLogs          string
	clusters          int
	decayRate         float64
	multilinePrefix   string
	multilineTimeout  int
	multilineMaxLines int
	dateRegex         string
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
	clustersFlag := autoCmd.Int("clusters", 0, "Number of baseline clusters (0=single, >=2=multi-cluster)")
	decayRateFlag := autoCmd.Float64("decay-rate", 0, "Decay rate for gradual baseline adaptation (0=disabled, 0.001=slow, 0.01=moderate)")
	multilinePrefixFlag := autoCmd.String("multiline-prefix", "", "Prefix to detect start of log lines")
	multilineTimeoutFlag := autoCmd.Int("multiline-timeout", 500, "Timeout in ms to flush buffer")
	multilineMaxLinesFlag := autoCmd.Int("multiline-max-lines", 5, "Max lines to buffer per log")
	dateRegexFlag := autoCmd.String("date-regex", "", "Regex to match and mask dates/timestamps")
	autoCmd.Parse(args)

	c.file = cfg.File
	c.workers = cfg.Workers
	c.threshold = cfg.Threshold
	c.webhook = cfg.Webhook
	c.verbose = cfg.Verbose
	c.metricsPort = cfg.MetricsPort
	c.initLogs = cfg.InitLogs
	c.clusters = cfg.Clusters
	c.decayRate = cfg.DecayRate
	c.multilinePrefix = cfg.MultilinePrefix
	c.multilineTimeout = cfg.MultilineTimeout
	c.multilineMaxLines = cfg.MultilineMaxLines
	c.dateRegex = cfg.DateRegex

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
		case "clusters":
			c.clusters = *clustersFlag
		case "decay-rate":
			c.decayRate = *decayRateFlag
		case "multiline-prefix":
			c.multilinePrefix = *multilinePrefixFlag
		case "multiline-timeout":
			c.multilineTimeout = *multilineTimeoutFlag
		case "multiline-max-lines":
			c.multilineMaxLines = *multilineMaxLinesFlag
		case "date-regex":
			c.dateRegex = *dateRegexFlag
		}
	})

	if c.file == "" {
		fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
		autoCmd.PrintDefaults()
		os.Exit(1)
	}
}

func (c *AutoCommand) Execute(deps Dependencies) error {
	const dbFile = "cortex.kv"

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		fmt.Printf("[CORTEX] No knowledge base found at %s. Auto-training from %s ...\n", dbFile, c.initLogs)
		if info, err := os.Stat(c.initLogs); err == nil && info.IsDir() {
			sanitizer := logreader.NewLogSanitizer(c.dateRegex)
			trainer := usecase.NewTrainer(deps.Encoder, deps.Store, c.multilinePrefix, c.multilineMaxLines, sanitizer)
			if err := trainer.TrainFromDirectory(c.initLogs, dbFile, c.clusters); err != nil {
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

	// Honor auto-tuned threshold if no explicit threshold was provided
	if kb.SuggestedThreshold > 0 && c.threshold == 0.65 {
		fmt.Printf("[AUTO-TUNE] Using stored suggested threshold: %.4f\n", kb.SuggestedThreshold)
		c.threshold = kb.SuggestedThreshold
	}

	metrics.InitMetrics(c.metricsPort)
	sanitizer := logreader.NewLogSanitizer(c.dateRegex)
	reader := logreader.NewRobustTailReader(c.multilinePrefix, c.multilineTimeout, c.multilineMaxLines)
	httpNotifier := notifier.NewHTTPNotifier(c.webhook)
	inference := usecase.NewInference(deps.Encoder, reader, httpNotifier, deps.Store, c.threshold, c.verbose, c.decayRate, nil, nil, false, 0, sanitizer)

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

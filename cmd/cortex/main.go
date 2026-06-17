package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/hdc"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/logreader"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/metrics"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/notifier"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/storage"
	"github.com/jbraconig/cortex-hdc/internal/usecase"
)

const DBFile = "cortex.kv"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	// Common dependencies
	encoder := hdc.NewHDCEncoder()
	store := storage.NewGobStore()

	// Load Viper configuration (Environment variables)
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "train":
		trainCmd := flag.NewFlagSet("train", flag.ExitOnError)
		fileFlag := trainCmd.String("file", "", "Path to the healthy log file to train the baseline")
		trainCmd.Parse(os.Args[2:])

		file := cfg.File
		trainCmd.Visit(func(f *flag.Flag) {
			if f.Name == "file" {
				file = *fileFlag
			}
		})

		if file == "" {
			fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
			trainCmd.PrintDefaults()
			os.Exit(1)
		}

		trainer := usecase.NewTrainer(encoder, store)
		if err := trainer.TrainFromFile(file, DBFile); err != nil {
			fmt.Printf("Error during training: %v\n", err)
			os.Exit(1)
		}

	case "auto":
		autoCmd := flag.NewFlagSet("auto", flag.ExitOnError)
		fileFlag := autoCmd.String("file", "", "Path to the log file to monitor in real time")
		workersFlag := autoCmd.Int("workers", 4, "Number of goroutines for the worker pool")
		thresholdFlag := autoCmd.Float64("threshold", 0.65, "Similarity threshold (0.0 - 1.0) below which an alert is triggered")
		webhookFlag := autoCmd.String("webhook", "", "Webhook URL to send HTTP JSON alerts (optional)")
		verboseFlag := autoCmd.Bool("verbose", false, "Print all log lines, not just anomalies")
		initLogsFlag := autoCmd.String("init-logs", "/data/init-logs/", "Directory containing baseline logs for auto-training")
		autoCmd.Parse(os.Args[2:])

		file := cfg.File
		workers := cfg.Workers
		threshold := cfg.Threshold
		webhook := cfg.Webhook
		verbose := cfg.Verbose
		metricsPort := cfg.MetricsPort
		initLogs := cfg.InitLogs

		autoCmd.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "file":
				file = *fileFlag
			case "workers":
				workers = *workersFlag
			case "threshold":
				threshold = *thresholdFlag
			case "webhook":
				webhook = *webhookFlag
			case "verbose":
				verbose = *verboseFlag
			case "init-logs":
				initLogs = *initLogsFlag
			}
		})

		if file == "" {
			fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
			autoCmd.PrintDefaults()
			os.Exit(1)
		}

		if _, err := os.Stat(DBFile); os.IsNotExist(err) {
			fmt.Printf("[CORTEX] No knowledge base found at %s. Auto-training from %s ...\n", DBFile, initLogs)
			if info, err := os.Stat(initLogs); err == nil && info.IsDir() {
				trainer := usecase.NewTrainer(encoder, store)
				if err := trainer.TrainFromDirectory(initLogs, DBFile); err != nil {
					fmt.Printf("Error during auto-training: %v\n", err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("Error: init-logs directory '%s' not found or invalid. Mount your baseline logs or run 'train' first.\n", initLogs)
				os.Exit(1)
			}
		} else {
			fmt.Printf("[CORTEX] Knowledge base %s found. Skipping auto-training.\n", DBFile)
		}

		kb, err := store.Load(DBFile)
		if err != nil {
			fmt.Printf("Error: Could not load the knowledge base (%s).\nDetail: %v\n", DBFile, err)
			os.Exit(1)
		}

		metrics.InitMetrics(metricsPort)
		reader := logreader.NewRobustTailReader()
		httpNotifier := notifier.NewHTTPNotifier(webhook)
		inference := usecase.NewInference(encoder, reader, httpNotifier, threshold, verbose)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			sig := <-sigChan
			fmt.Printf("\n[CORTEX] Signal %v received. Starting graceful shutdown...\n", sig)
			cancel()
		}()

		if err := inference.Run(ctx, kb, file, workers); err != nil {
			fmt.Printf("Error during inference: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[CORTEX] Graceful shutdown completed.")

	case "infer":
		inferCmd := flag.NewFlagSet("infer", flag.ExitOnError)
		fileFlag := inferCmd.String("file", "", "Path to the log file to monitor in real time")
		workersFlag := inferCmd.Int("workers", 4, "Number of goroutines for the worker pool")
		thresholdFlag := inferCmd.Float64("threshold", 0.65, "Similarity threshold (0.0 - 1.0) below which an alert is triggered")
		webhookFlag := inferCmd.String("webhook", "", "Webhook URL to send HTTP JSON alerts (optional)")
		verboseFlag := inferCmd.Bool("verbose", false, "Print all log lines, not just anomalies")
		inferCmd.Parse(os.Args[2:])

		// Verify which flags were explicitly provided
		file := cfg.File
		workers := cfg.Workers
		threshold := cfg.Threshold
		webhook := cfg.Webhook
		verbose := cfg.Verbose
		metricsPort := cfg.MetricsPort

		inferCmd.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "file":
				file = *fileFlag
			case "workers":
				workers = *workersFlag
			case "threshold":
				threshold = *thresholdFlag
			case "webhook":
				webhook = *webhookFlag
			case "verbose":
				verbose = *verboseFlag
			}
		})

		if file == "" {
			fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
			inferCmd.PrintDefaults()
			os.Exit(1)
		}

		kb, err := store.Load(DBFile)
		if err != nil {
			fmt.Printf("Error: Could not load the knowledge base (%s). Run 'train' first.\n", DBFile)
			fmt.Printf("Detail: %v\n", err)
			os.Exit(1)
		}

		// Initialize Prometheus Metrics
		metrics.InitMetrics(metricsPort)

		reader := logreader.NewRobustTailReader()
		httpNotifier := notifier.NewHTTPNotifier(webhook)
		inference := usecase.NewInference(encoder, reader, httpNotifier, threshold, verbose)

		// Context for Graceful Shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			sig := <-sigChan
			fmt.Printf("\n[CORTEX] Signal %v received. Starting graceful shutdown...\n", sig)
			cancel()
		}()

		if err := inference.Run(ctx, kb, file, workers); err != nil {
			fmt.Printf("Error during inference: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[CORTEX] Graceful shutdown completed.")

	default:
		fmt.Printf("Unknown command: '%s'\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Cortex HDC - Log Anomaly Detection Engine")
	fmt.Println("Usage:")
	fmt.Println("  cortex <command> [flags]")
	fmt.Println("\nCommands:")
	fmt.Println("  auto     Runs real-time analysis, but auto-trains first if knowledge base is missing.")
	fmt.Println("           Ex: cortex auto --file /var/log/syslog --init-logs /data/init-logs/")
	fmt.Println("  train    Trains the baseline from a healthy log.")
	fmt.Println("           Ex: cortex train --file /var/log/syslog.healthy")
	fmt.Println("  infer    Runs real-time analysis.")
	fmt.Println("           Ex: cortex infer --file /var/log/syslog --workers 8 --threshold 0.65 --webhook http://... --verbose")
}

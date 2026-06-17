package main

import (
	"fmt"
	"os"

	"github.com/jbraconig/cortex-hdc/cmd/cortex/command"
	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/hdc"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/storage"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Common dependencies
	encoder := hdc.NewHDCEncoder()
	store := storage.NewGobStore()

	// Load Viper configuration (Environment variables)
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	deps := command.Dependencies{
		Encoder: encoder,
		Store:   store,
		Config:  cfg,
	}

	commands := map[string]command.Command{
		"train": &command.TrainCommand{},
		"infer": &command.InferCommand{},
		"auto":  &command.AutoCommand{},
	}

	cmdName := os.Args[1]
	cmd, ok := commands[cmdName]
	if !ok {
		fmt.Printf("Unknown command: '%s'\n", cmdName)
		printUsage()
		os.Exit(1)
	}

	cmd.Parse(os.Args[2:], cfg)
	if err := cmd.Execute(deps); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Cortex HDC - Log Anomaly Detection Engine")
	fmt.Println("Usage:")
	fmt.Println("  cortex <command> [flags]")
	fmt.Println("\nCommands:")
	fmt.Println("  auto     Auto-trains on first boot if knowledge base is missing, then monitors.")
	fmt.Println("           Ex: cortex auto --file /var/log/syslog --init-logs /data/init-logs/ --clusters 3 --decay-rate 0.001")
	fmt.Println("  train    Trains the baseline from a healthy log file.")
	fmt.Println("           Ex: cortex train --file /var/log/syslog.healthy --clusters 3")
	fmt.Println("  infer    Runs real-time anomaly detection.")
	fmt.Println("           Ex: cortex infer --file /var/log/syslog --workers 8 --threshold 0.65 --decay-rate 0.001 --webhook http://... --verbose")
}

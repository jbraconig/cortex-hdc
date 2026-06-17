package command

import (
	"flag"
	"fmt"
	"os"

	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/usecase"
)

type TrainCommand struct {
	file string
}

func (c *TrainCommand) Name() string {
	return "train"
}

func (c *TrainCommand) Parse(args []string, cfg *config.Config) {
	trainCmd := flag.NewFlagSet("train", flag.ExitOnError)
	fileFlag := trainCmd.String("file", "", "Path to the healthy log file to train the baseline")
	trainCmd.Parse(args)

	c.file = cfg.File
	trainCmd.Visit(func(f *flag.Flag) {
		if f.Name == "file" {
			c.file = *fileFlag
		}
	})

	if c.file == "" {
		fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
		trainCmd.PrintDefaults()
		os.Exit(1)
	}
}

func (c *TrainCommand) Execute(deps Dependencies) error {
	trainer := usecase.NewTrainer(deps.Encoder, deps.Store)
	return trainer.TrainFromFile(c.file, "cortex.kv")
}

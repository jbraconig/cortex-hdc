package command

import (
	"flag"
	"fmt"
	"os"

	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/logreader"
	"github.com/jbraconig/cortex-hdc/internal/usecase"
)

type TrainCommand struct {
	file              string
	clusters          int
	multilinePrefix   string
	multilineMaxLines int
	dateRegex         string
}

func (c *TrainCommand) Name() string {
	return "train"
}

func (c *TrainCommand) Parse(args []string, cfg *config.Config) {
	trainCmd := flag.NewFlagSet("train", flag.ExitOnError)
	fileFlag := trainCmd.String("file", "", "Path to the healthy log file to train the baseline")
	clustersFlag := trainCmd.Int("clusters", 0, "Number of baseline clusters (0=single, >=2=multi-cluster)")
	multilinePrefixFlag := trainCmd.String("multiline-prefix", "", "Prefix to detect start of log lines")
	multilineMaxLinesFlag := trainCmd.Int("multiline-max-lines", 5, "Max lines to buffer per log")
	dateRegexFlag := trainCmd.String("date-regex", "", "Regex to match and mask dates/timestamps")
	trainCmd.Parse(args)

	c.file = cfg.File
	c.clusters = cfg.Clusters
	c.multilinePrefix = cfg.MultilinePrefix
	c.multilineMaxLines = cfg.MultilineMaxLines
	c.dateRegex = cfg.DateRegex

	trainCmd.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "file":
			c.file = *fileFlag
		case "clusters":
			c.clusters = *clustersFlag
		case "multiline-prefix":
			c.multilinePrefix = *multilinePrefixFlag
		case "multiline-max-lines":
			c.multilineMaxLines = *multilineMaxLinesFlag
		case "date-regex":
			c.dateRegex = *dateRegexFlag
		}
	})

	if c.file == "" {
		fmt.Println("Error: Specifying a log file via --file or CORTEX_FILE environment variable is required")
		trainCmd.PrintDefaults()
		os.Exit(1)
	}
}

func (c *TrainCommand) Execute(deps Dependencies) error {
	sanitizer := logreader.NewLogSanitizer(c.dateRegex)
	trainer := usecase.NewTrainer(deps.Encoder, deps.Store, c.multilinePrefix, c.multilineMaxLines, sanitizer)
	return trainer.TrainFromFile(c.file, "cortex.kv", c.clusters)
}

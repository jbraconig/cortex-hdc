package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/domain"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/grpc"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/logreader"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/metrics"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/notifier"
	"github.com/jbraconig/cortex-hdc/internal/infrastructure/p2p"
	"github.com/jbraconig/cortex-hdc/internal/usecase"
)

type InferCommand struct {
	file         string
	workers      int
	threshold    float64
	webhook      string
	verbose      bool
	metricsPort  int
	decayRate    float64
	p2pEnabled   bool
	p2pBindPort  int
	p2pJoinAddrs string
	saasEndpoint string
	saasToken    string
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
	p2pFlag := inferCmd.Bool("p2p", false, "Enable P2P cluster synchronization")
	p2pBindPortFlag := inferCmd.Int("p2p-bind", 7946, "Local P2P bind port for gossip communication")
	p2pJoinAddrsFlag := inferCmd.String("p2p-join", "", "Comma-separated cluster seed addresses to join")
	saasEndpointFlag := inferCmd.String("saas-endpoint", "", "SaaS Control Plane gRPC endpoint (e.g. localhost:50051)")
	saasTokenFlag := inferCmd.String("saas-token", "", "Authentication token for SaaS Control Plane")
	inferCmd.Parse(args)

	c.file = cfg.File
	c.workers = cfg.Workers
	c.threshold = cfg.Threshold
	c.webhook = cfg.Webhook
	c.verbose = cfg.Verbose
	c.metricsPort = cfg.MetricsPort
	c.decayRate = cfg.DecayRate
	c.p2pEnabled = cfg.P2P
	c.p2pBindPort = cfg.P2PBindPort
	c.p2pJoinAddrs = cfg.P2PJoinAddrs
	c.saasEndpoint = cfg.SaaSEndpoint
	c.saasToken = cfg.SaaSToken

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
		case "p2p":
			c.p2pEnabled = *p2pFlag
		case "p2p-bind":
			c.p2pBindPort = *p2pBindPortFlag
		case "p2p-join":
			c.p2pJoinAddrs = *p2pJoinAddrsFlag
		case "saas-endpoint":
			c.saasEndpoint = *saasEndpointFlag
		case "saas-token":
			c.saasToken = *saasTokenFlag
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

	// Initialize P2P synchronization if explicitly enabled
	var gossipNode domain.ClusterSync
	if c.p2pEnabled {
		var joinAddrs []string
		if c.p2pJoinAddrs != "" {
			for _, addr := range strings.Split(c.p2pJoinAddrs, ",") {
				trimmed := strings.TrimSpace(addr)
				if trimmed != "" {
					joinAddrs = append(joinAddrs, trimmed)
				}
			}
		}
		gn, err := p2p.NewGossipNode(c.p2pBindPort, joinAddrs, kb)
		if err != nil {
			return fmt.Errorf("failed to start Gossip node: %w", err)
		}
		gossipNode = gn
	}

	// Initialize Telemetry Client
	var telemetryClient domain.TelemetryClient
	if c.saasEndpoint != "" && c.saasToken != "" {
		tc, err := grpc.NewRealTelemetryClient(c.saasEndpoint, c.saasToken)
		if err != nil {
			return fmt.Errorf("failed to start SaaS telemetry client: %w", err)
		}
		telemetryClient = tc
		fmt.Printf("[SAAS] Telemetry enabled. Reporting to %s\n", c.saasEndpoint)
	} else {
		telemetryClient = grpc.NewNoOpTelemetryClient()
	}

	inference := usecase.NewInference(deps.Encoder, reader, httpNotifier, deps.Store, c.threshold, c.verbose, c.decayRate, gossipNode, telemetryClient)

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
		if gossipNode != nil {
			defer gossipNode.Shutdown()
		}
		defer telemetryClient.Close()
		return inference.Run(ctx, kb, logFiles, c.workers, dbFile)
	})
}

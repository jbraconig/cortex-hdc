package command

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbraconig/cortex-hdc/internal/config"
	"github.com/jbraconig/cortex-hdc/internal/domain"
)

// Command defines the interface for all CLI subcommands
type Command interface {
	Name() string
	Parse(args []string, cfg *config.Config)
	Execute(deps Dependencies) error
}

// Dependencies groups all common services needed by commands
type Dependencies struct {
	Encoder domain.Encoder
	Store   domain.Persistence
	Config  *config.Config
}

// RunWithGracefulShutdown is a helper for commands that need to run continuously
// until interrupted
func RunWithGracefulShutdown(fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\n[CORTEX] Signal %v received. Starting graceful shutdown...\n", sig)
		cancel()
	}()

	err := fn(ctx)
	fmt.Println("[CORTEX] Graceful shutdown completed.")
	return err
}

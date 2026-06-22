package logreader

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/nxadm/tail"
)

// RobustTailReader implements the domain.LogReader interface using robust tailing
type RobustTailReader struct{}

func NewRobustTailReader() *RobustTailReader {
	return &RobustTailReader{}
}

// ReadLogs reads log lines in real time supporting multiple files and rotation.
// It multiplexes (fan-in) all streams into a single high-performance channel.
func (r *RobustTailReader) ReadLogs(ctx context.Context, filePaths []string) (<-chan string, error) {
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no log files specified for tailing")
	}

	// Use a large buffer to ensure high performance and prevent blocking
	// when multiple files burst logs simultaneously.
	ch := make(chan string, 10000)
	var wg sync.WaitGroup

	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true,                                          // Reopens the file if rotated/moved
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd}, // Starts from the end like tail -f
		MustExist: false,                                         // Does not fail if the file does not exist yet at startup
		Logger:    tail.DiscardingLogger,                         // Keep terminal clean
	}

	// Track successfully started tails in case we need to clean them up on an early error
	var activeTails []*tail.Tail

	for _, path := range filePaths {
		t, err := tail.TailFile(path, cfg)
		if err != nil {
			// Clean up any previously started tails before returning
			for _, active := range activeTails {
				active.Stop()
				active.Cleanup()
			}
			return nil, fmt.Errorf("failed to tail file %s: %w", path, err)
		}
		activeTails = append(activeTails, t)

		wg.Add(1)
		go func(tailer *tail.Tail) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					tailer.Stop()
					tailer.Cleanup()
					return
				case line, ok := <-tailer.Lines:
					if !ok {
						return
					}
					if line.Err != nil {
						continue
					}
					// Send line to the multiplexed channel with context check
					select {
					case <-ctx.Done():
						tailer.Stop()
						tailer.Cleanup()
						return
					case ch <- line.Text:
					}
				}
			}
		}(t)
	}

	// Background routine to close the channel once all tails have finished (e.g., context cancelled)
	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch, nil
}

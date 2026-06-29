package logreader

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nxadm/tail"
)

// RobustTailReader implements the domain.LogReader interface using robust tailing
type RobustTailReader struct {
	prefix    string
	timeoutMs int
	maxLines  int
}

func NewRobustTailReader(prefix string, timeoutMs int, maxLines int) *RobustTailReader {
	return &RobustTailReader{
		prefix:    prefix,
		timeoutMs: timeoutMs,
		maxLines:  maxLines,
	}
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
		Poll:      true,                                          // Use polling instead of inotify for container compatibility
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

			var logBuffer strings.Builder
			var lineCount int

			flushTimeout := time.Duration(r.timeoutMs) * time.Millisecond
			ticker := time.NewTicker(flushTimeout)
			defer ticker.Stop()

			flushBuffer := func() {
				if logBuffer.Len() > 0 {
					finalLog := logBuffer.String()
					select {
					case ch <- finalLog:
					case <-ctx.Done():
					}
					logBuffer.Reset()
					lineCount = 0
				}
			}

			for {
				select {
				case <-ctx.Done():
					flushBuffer()
					tailer.Stop()
					tailer.Cleanup()
					return
				case <-ticker.C:
					flushBuffer()
				case line, ok := <-tailer.Lines:
					if !ok {
						flushBuffer()
						return
					}
					if line.Err != nil {
						continue
					}

					text := line.Text

					if r.prefix == "" {
						select {
						case ch <- text:
						case <-ctx.Done():
							tailer.Stop()
							tailer.Cleanup()
							return
						}
						continue
					}

					if strings.HasPrefix(text, r.prefix) {
						flushBuffer()
						logBuffer.WriteString(text)
						lineCount = 1
					} else {
						if r.maxLines > 0 && lineCount >= r.maxLines {
							continue
						}
						if logBuffer.Len() > 0 {
							logBuffer.WriteString("\n")
							logBuffer.WriteString(text)
							lineCount++
						} else {
							logBuffer.WriteString(text)
							lineCount = 1
						}
					}
					ticker.Reset(flushTimeout)
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

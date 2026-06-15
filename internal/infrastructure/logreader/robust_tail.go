package logreader

import (
	"context"
	"io"

	"github.com/nxadm/tail"
)

// RobustTailReader implements the domain.LogReader interface using robust tailing
type RobustTailReader struct{}

func NewRobustTailReader() *RobustTailReader {
	return &RobustTailReader{}
}

// ReadLogs reads log lines in real time supporting file rotation
func (r *RobustTailReader) ReadLogs(ctx context.Context, filepath string) (<-chan string, error) {
	ch := make(chan string, 1000)

	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true,                                                 // Reopens the file if rotated/moved
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd},       // Starts from the end like tail -f
		MustExist: false,                                                // Does not fail if the file does not exist yet at startup
	}

	t, err := tail.TailFile(filepath, cfg)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				t.Cleanup()
				return
			case line, ok := <-t.Lines:
				if !ok {
					return
				}
				if line.Err != nil {
					continue
				}
				ch <- line.Text
			}
		}
	}()

	return ch, nil
}

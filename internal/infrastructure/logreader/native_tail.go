package logreader

import (
	"bufio"
	"os"
)

// ReadStaticLogs reads a complete file until EOF and closes the channel
func ReadStaticLogs(filepath string) (<-chan string, error) {
	ch := make(chan string, 1000)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}

	go func() {
		defer file.Close()
		defer close(ch)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
	}()

	return ch, nil
}

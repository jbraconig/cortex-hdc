package logreader

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// ReadStaticLogs reads a complete file until EOF and closes the channel.
// It applies multiline logic based on prefix and maxLines.
func ReadStaticLogs(filepath string, prefix string, maxLines int) (<-chan string, error) {
	ch := make(chan string, 1000)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}

	go func() {
		defer file.Close()
		defer close(ch)

		scanner := bufio.NewScanner(file)
		var logBuffer strings.Builder
		var lineCount int

		flushBuffer := func() {
			if logBuffer.Len() > 0 {
				finalLog := logBuffer.String()
				ch <- finalLog
				logBuffer.Reset()
				lineCount = 0
			}
		}

		for scanner.Scan() {
			text := scanner.Text()

			if prefix == "" {
				ch <- text
				continue
			}

			if strings.HasPrefix(text, prefix) {
				flushBuffer()
				logBuffer.WriteString(text)
				lineCount = 1
			} else {
				if maxLines > 0 && lineCount >= maxLines {
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
		}
		flushBuffer()

		if err := scanner.Err(); err != nil {
			log.Printf("Error scanning file %s: %v", filepath, err)
		}
	}()

	return ch, nil
}

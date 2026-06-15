package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPNotifier implements the AlertNotifier interface
type HTTPNotifier struct {
	WebhookURL string
}

func NewHTTPNotifier(url string) *HTTPNotifier {
	return &HTTPNotifier{
		WebhookURL: url,
	}
}

// Payload represents the JSON structure of the alert
type Payload struct {
	Timestamp  string  `json:"timestamp"`
	Level      string  `json:"level"`
	Message    string  `json:"message"`
	Similarity float64 `json:"similarity"`
	LogLine    string  `json:"log_line"`
}

// Notify sends the alert via HTTP POST
func (n *HTTPNotifier) Notify(logLine string, similarity float64) error {
	// We also always print it to the console as a fallback
	fmt.Printf("[CRITICAL ALERT] Low similarity (%.2f%%): %s\n", similarity*100, logLine)

	if n.WebhookURL == "" {
		return nil // If there is no URL, it only logs to the console
	}

	payload := Payload{
		Timestamp:  time.Now().Format(time.RFC3339),
		Level:      "CRITICAL",
		Message:    "Anomaly detected in logs by HDC engine",
		Similarity: similarity,
		LogLine:    logLine,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("error sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned non-success status: %d", resp.StatusCode)
	}

	return nil
}

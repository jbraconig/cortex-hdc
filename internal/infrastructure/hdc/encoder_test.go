package hdc

import (
	"testing"
)

func TestCleanTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ISO8601 with Space",
			input:    "2026-06-22 13:05:33,123 INFO something happened",
			expected: "INFO something happened",
		},
		{
			name:     "ISO8601 with T",
			input:    "2026-06-22T13:05:33-05:00 ERROR connection lost",
			expected: "ERROR connection lost",
		},
		{
			name:     "ISO8601 with Z",
			input:    "2026-06-22T13:05:33Z WARN disk almost full",
			expected: "WARN disk almost full",
		},
		{
			name:     "Syslog format",
			input:    "Jun 22 13:05:33 host-server daemon: started successfully",
			expected: "host-server daemon: started successfully",
		},
		{
			name:     "Apache/Nginx bracketed format",
			input:    "[22/Jun/2026:13:05:33 -0500] GET /index.html HTTP/1.1",
			expected: "GET /index.html HTTP/1.1",
		},
		{
			name:     "No timestamp format",
			input:    "Simple log message without any timestamp",
			expected: "Simple log message without any timestamp",
		},
		{
			name:     "Empty line",
			input:    "",
			expected: "",
		},
		{
			name:     "Just spaces",
			input:    "    ",
			expected: "",
		},
		{
			name:     "Corrupted date but normal log",
			input:    "2026-99-99 12:34:56.789 OK log output",
			expected: "OK log output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanTimestamp(tt.input)
			if got != tt.expected {
				t.Errorf("cleanTimestamp(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

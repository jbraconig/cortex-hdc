package logreader

import (
	"testing"
)

func TestSanitizeLogCustom(t *testing.T) {
	s := NewLogSanitizer(`^\d{4}-\d{2}-\d{2}`)

	// Using custom date pattern overrides default formatting, leaving the rest of the numbers inside the timestamp
	// to be replaced by numRegex.
	input := "2026-06-29T12:28:16-05:00 [INFO] Hello Custom"
	expected := "<DATE>T12:<NUM>:<NUM>-<NUM>:<NUM> [INFO] Hello Custom"
	got := s.Sanitize(input)
	if got != expected {
		t.Errorf("Sanitize custom failed: got %q, want %q", got, expected)
	}
}

func TestSanitizeLogDefault(t *testing.T) {
	s := NewLogSanitizer("")

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "2026-06-29 [INFO] Hello World",
			expected: "<DATE> [INFO] Hello World",
		},
		{
			input:    "2026-06-29T12:28:16-05:00 [INFO] RFC3339",
			expected: "<DATE> [INFO] RFC3339",
		},
		{
			input:    "2026/06/29 12:28:09.123 [INFO] Slash milliseconds",
			expected: "<DATE> [INFO] Slash milliseconds",
		},
		{
			input:    "Jun 29 12:28:16 [INFO] Syslog",
			expected: "<DATE> [INFO] Syslog",
		},
		{
			input:    "29/Jun/2026:12:28:16 -0500 [INFO] CLF format",
			expected: "<DATE> [INFO] CLF format",
		},
		{
			input:    "Error on http://domain.com:8080/path?query=1",
			expected: "Error on <URL>",
		},
		{
			input:    "Error on https://localhost:88484",
			expected: "Error on <URL>",
		},
		{
			input:    "Connected to 192.168.1.1",
			expected: "Connected to <IP>",
		},
		{
			input:    "Session f81d4fae-7dec-11d0-a765-00a0c91e6bf6 active",
			expected: "Session <UUID> active",
		},
		{
			input:    "Memory address 0x7ffd5f9c4f7c pointer",
			expected: "Memory address <HEX> pointer",
		},
		{
			input:    "PID 12345 thread 99",
			expected: "PID <NUM> thread <NUM>",
		},
	}

	for _, tc := range tests {
		got := s.Sanitize(tc.input)
		if got != tc.expected {
			t.Errorf("Sanitize(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

package logreader

import (
	"os"
	"testing"
)

func TestReadStaticLogsMultiline(t *testing.T) {
	tempFile, err := os.CreateTemp("", "cortex-test-logs")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	content := `2026-06-29 INFO start
  at line 1
  at line 2
2026-06-29 ERROR failed
  Caused by: NullPointer
2026-06-29 INFO end`

	if _, err := tempFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	sanitizer := NewLogSanitizer(`^\d{4}-\d{2}-\d{2}`)
	ch, err := ReadStaticLogs(tempFile.Name(), "2026-06-29", 5)
	if err != nil {
		t.Fatalf("ReadStaticLogs failed: %v", err)
	}

	var results []string
	for line := range ch {
		sanitizedLine := sanitizer.Sanitize(line)
		results = append(results, sanitizedLine)
	}

	expected := []string{
		"<DATE> INFO start\n  at line <NUM>\n  at line <NUM>",
		"<DATE> ERROR failed\n  Caused by: NullPointer",
		"<DATE> INFO end",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d logs, got %d. Got results: %v", len(expected), len(results), results)
	}

	for i, got := range results {
		if got != expected[i] {
			t.Errorf("Log %d: got %q, want %q", i, got, expected[i])
		}
	}
}

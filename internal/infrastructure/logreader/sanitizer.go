package logreader

import "regexp"

var (
	// Unified Date/Time regex (covers RFC3339, ISO8601, SQL dates with dashes/slashes, and log4j with comma milliseconds)
	// Matches: 2026-06-29T12:28:16-05:00, 2026/06/29 12:28:09.123, 2026-06-29 12:28:16,123
	defaultDateRegex = regexp.MustCompile(`\b\d{4}[-/]\d{2}[-/]\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:[+-]\d{2}:?\d{2}|Z)?\b`)

	// Simple Date YYYY-MM-DD or YYYY/MM/DD (e.g. 2026-06-29)
	simpleDateRegex = regexp.MustCompile(`\b\d{4}[-/]\d{2}[-/]\d{2}\b`)

	// Syslog RFC3164 (e.g. Jun 29 12:28:16)
	syslogDateRegex = regexp.MustCompile(`\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?\b`)

	// Common Log Format (Apache/Nginx, e.g. 29/Jun/2026:12:28:16 -0500)
	clfDateRegex = regexp.MustCompile(`\b\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2}(?:\s+[+-]\d{4})?\b`)

	urlRegex  = regexp.MustCompile(`(?i)https?://[^\s"']+`) // Matches URLs (http://domain.com:8080/path)
	ipRegex   = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	uuidRegex = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	hexRegex  = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	numRegex  = regexp.MustCompile(`\b\d+\b`)
)

type LogSanitizer struct {
	dateRegex *regexp.Regexp
}

func NewLogSanitizer(datePattern string) *LogSanitizer {
	var dr *regexp.Regexp
	if datePattern != "" {
		if re, err := regexp.Compile(datePattern); err == nil {
			dr = re
		}
	}
	return &LogSanitizer{dateRegex: dr}
}

// Sanitize removes dynamic noise from log lines
func (s *LogSanitizer) Sanitize(log string) string {
	if s.dateRegex != nil {
		log = s.dateRegex.ReplaceAllString(log, "<DATE>")
	}
	log = defaultDateRegex.ReplaceAllString(log, "<DATE>")
	log = simpleDateRegex.ReplaceAllString(log, "<DATE>")
	log = syslogDateRegex.ReplaceAllString(log, "<DATE>")
	log = clfDateRegex.ReplaceAllString(log, "<DATE>")

	// The order is important: URLs first, then IPs, then general numbers
	log = urlRegex.ReplaceAllString(log, "<URL>")
	log = ipRegex.ReplaceAllString(log, "<IP>")
	log = uuidRegex.ReplaceAllString(log, "<UUID>")
	log = hexRegex.ReplaceAllString(log, "<HEX>")
	log = numRegex.ReplaceAllString(log, "<NUM>")
	return log
}

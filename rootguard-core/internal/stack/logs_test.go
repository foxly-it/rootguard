package stack

import (
	"strings"
	"testing"
)

func TestSanitizeLogsBoundsAndRedactsOutput(t *testing.T) {
	lines := []string{
		"service ready",
		"Authorization: Bearer abc.def.ghi",
		"password=hunter2 token:secret-value",
		"\x1b[31mfailed\x1b[0m",
	}
	logs := sanitizeLogs("adguard", []byte(strings.Join(lines, "\n")))
	joined := strings.Join(logs.Lines, "\n")
	if strings.Contains(joined, "hunter2") || strings.Contains(joined, "abc.def") || strings.Contains(joined, "secret-value") {
		t.Fatalf("sensitive value survived redaction: %s", joined)
	}
	if strings.Contains(joined, "\x1b") {
		t.Fatalf("control character survived sanitization: %q", joined)
	}
	if !logs.Redacted || logs.Tail != 100 || logs.Since != "30m" {
		t.Fatalf("unexpected log metadata: %+v", logs)
	}
}

func TestSanitizeLogsKeepsOnlyLastHundredLines(t *testing.T) {
	var lines []string
	for index := 0; index < 120; index++ {
		lines = append(lines, "line")
	}
	logs := sanitizeLogs("unbound", []byte(strings.Join(lines, "\n")))
	if len(logs.Lines) != 100 || !logs.Truncated {
		t.Fatalf("logs were not bounded: %+v", logs)
	}
}

package stack

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReadServiceLogsUsesDedicatedManagedContainerAllowlist(t *testing.T) {
	original := dockerCommand
	t.Cleanup(func() { dockerCommand = original })
	called := make([]string, 0, len(logContainers))
	dockerCommand = func(_ context.Context, arguments ...string) ([]byte, error) {
		called = append(called, arguments[len(arguments)-1])
		return []byte("ready\n"), nil
	}

	for _, service := range []string{"core", "webapp", "updater", "adguard", "unbound"} {
		logs, err := ReadServiceLogs(context.Background(), service)
		if err != nil || logs.Service != service || len(logs.Lines) != 1 {
			t.Fatalf("read %s logs: %+v, %v", service, logs, err)
		}
	}
	for index, service := range []string{"core", "webapp", "updater", "adguard", "unbound"} {
		if called[index] != logContainers[service] {
			t.Fatalf("service %s used container %q", service, called[index])
		}
	}
}

func TestReadServiceLogsRejectsUnknownService(t *testing.T) {
	if _, err := ReadServiceLogs(context.Background(), "arbitrary"); !errors.Is(err, ErrUnknownService) {
		t.Fatalf("expected unknown-service error, got %v", err)
	}
}

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

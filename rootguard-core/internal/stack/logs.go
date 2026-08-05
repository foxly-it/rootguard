package stack

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"unicode"
)

const (
	logTailLines = 100
	logSince     = "30m"
	maxLogBytes  = 64 << 10
)

var (
	sensitiveAssignment = regexp.MustCompile(`(?i)\b(authorization|token|password|secret|api[_-]?key)(\s*[:=]\s*)([^\s,;]+)`)
	bearerCredential    = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/-]+=*`)
)

type ServiceLogs struct {
	Service     string   `json:"service"`
	Lines       []string `json:"lines"`
	Tail        int      `json:"tail"`
	Since       string   `json:"since"`
	Truncated   bool     `json:"truncated"`
	Redacted    bool     `json:"redacted"`
	Description string   `json:"description"`
}

func ReadServiceLogs(ctx context.Context, service string) (ServiceLogs, error) {
	container, ok := serviceContainers[service]
	if !ok {
		return ServiceLogs{}, ErrUnknownService
	}
	output, err := dockerCommand(ctx, "logs", "--tail", "100", "--since", logSince, container)
	if err != nil {
		return ServiceLogs{}, err
	}
	return sanitizeLogs(service, output), nil
}

func sanitizeLogs(service string, output []byte) ServiceLogs {
	truncated := len(output) > maxLogBytes
	if truncated {
		output = output[len(output)-maxLogBytes:]
	}
	rawLines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	redacted := false
	for _, raw := range rawLines {
		if raw == "" {
			continue
		}
		line := strings.Map(func(r rune) rune {
			if r == '\t' || !unicode.IsControl(r) {
				return r
			}
			return -1
		}, raw)
		next := bearerCredential.ReplaceAllString(line, "Bearer [REDACTED]")
		next = sensitiveAssignment.ReplaceAllString(next, "$1$2[REDACTED]")
		if next != line {
			redacted = true
		}
		lines = append(lines, next)
	}
	if len(lines) > logTailLines {
		lines = lines[len(lines)-logTailLines:]
		truncated = true
	}
	return ServiceLogs{
		Service: service, Lines: lines, Tail: logTailLines, Since: logSince,
		Truncated: truncated, Redacted: redacted,
		Description: "Last 100 lines from the previous 30 minutes; common credentials are redacted.",
	}
}

var dockerCommand = func(ctx context.Context, arguments ...string) ([]byte, error) {
	return commandOutput(ctx, arguments...)
}

func commandOutput(ctx context.Context, arguments ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := execCommandContext(ctx, "docker", arguments...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	stdout.Write(stderr.Bytes())
	return stdout.Bytes(), nil
}

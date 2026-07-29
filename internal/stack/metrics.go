package stack

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var metricContainers = []string{
	"rootguard-core",
	"rootguard-webapp",
	"rootguard-updater",
	"rootguard-adguard",
	"rootguard-unbound",
}

type Metrics struct {
	Available   bool    `json:"available"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
}

type dockerStatsLine struct {
	CPUPercent string `json:"CPUPerc"`
	Memory     string `json:"MemUsage"`
}

func CollectMetrics(ctx context.Context) Metrics {
	statsContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	arguments := append([]string{"stats", "--no-stream", "--format", "{{json .}}"}, metricContainers...)
	output, err := exec.CommandContext(statsContext, "docker", arguments...).Output()
	if err != nil {
		return Metrics{}
	}
	metrics, err := decodeMetrics(output)
	if err != nil {
		return Metrics{}
	}
	return metrics
}

func decodeMetrics(payload []byte) (Metrics, error) {
	var metrics Metrics
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	for scanner.Scan() {
		var line dockerStatsLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			return Metrics{}, fmt.Errorf("decode docker stats: %w", err)
		}
		cpu, err := parsePercent(line.CPUPercent)
		if err != nil {
			return Metrics{}, err
		}
		usage := strings.TrimSpace(strings.SplitN(line.Memory, "/", 2)[0])
		memory, err := parseDockerSize(usage)
		if err != nil {
			return Metrics{}, err
		}
		metrics.CPUPercent += cpu
		metrics.MemoryBytes += memory
		metrics.Available = true
	}
	if err := scanner.Err(); err != nil {
		return Metrics{}, err
	}
	return metrics, nil
}

func parsePercent(value string) (float64, error) {
	number := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	result, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, fmt.Errorf("parse docker percentage %q: %w", value, err)
	}
	return result, nil
}

func parseDockerSize(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	units := []struct {
		suffix string
		factor float64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
		{"GB", 1_000_000_000}, {"MB", 1_000_000}, {"kB", 1_000}, {"B", 1},
	}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil || parsed < 0 {
			return 0, fmt.Errorf("parse docker memory %q", value)
		}
		return uint64(parsed * unit.factor), nil
	}
	return 0, fmt.Errorf("unsupported docker memory size %q", value)
}

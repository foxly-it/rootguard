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
	"sync"
	"time"
)

// metricsRefreshInterval paces the background collector, not individual
// HTTP requests - `docker stats --no-stream` needs the Docker daemon to
// take two internal CPU-accounting samples before it can answer at all,
// which measured live costs ~1-2s per invocation regardless of who's
// asking or how often. Collecting on a fixed background cadence instead of
// per-request means CollectMetrics (called from dashboardHandler) never
// blocks an HTTP response on that latency - it just hands back whatever
// was most recently collected.
const metricsRefreshInterval = 1 * time.Second

var metricsCache = struct {
	sync.RWMutex
	value Metrics
}{}

// StartMetricsCollector runs the first collection synchronously (so a
// request arriving immediately after startup still gets real data almost
// as soon as it's available, rather than "not available" until the first
// tick) and then keeps collecting on metricsRefreshInterval for as long as
// ctx stays alive. Call once, near process startup.
func StartMetricsCollector(ctx context.Context) {
	refreshMetricsCache(ctx)
	go func() {
		ticker := time.NewTicker(metricsRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshMetricsCache(ctx)
			}
		}
	}()
}

func refreshMetricsCache(ctx context.Context) {
	metrics := collectMetricsNow(ctx)
	metricsCache.Lock()
	metricsCache.value = metrics
	metricsCache.Unlock()
}

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

// CollectMetrics returns whatever the background collector (see
// StartMetricsCollector) most recently gathered - it never itself invokes
// `docker stats`, so it never blocks a caller (dashboardHandler) on that
// command's ~1-2s inherent latency. If StartMetricsCollector was never
// called (e.g. in a test that exercises dashboardHandler directly), this
// returns the zero Metrics{} (Available: false), the same graceful
// "unavailable" state a real collection failure already produces.
func CollectMetrics(_ context.Context) Metrics {
	metricsCache.RLock()
	defer metricsCache.RUnlock()
	return metricsCache.value
}

func collectMetricsNow(ctx context.Context) Metrics {
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

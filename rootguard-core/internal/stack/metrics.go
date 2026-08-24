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

// dashboardRefreshInterval paces the background collectors for both metrics
// (this file) and container status (status.go), not individual HTTP
// requests - `docker stats --no-stream` needs the Docker daemon to take two
// internal CPU-accounting samples before it can answer at all, which
// measured live costs ~1-2s per invocation regardless of who's asking or
// how often. Collecting on a fixed background cadence instead of per-request
// means CollectMetrics/CollectStatus (both called from dashboardHandler on
// every poll) never block an HTTP response on that latency - they just hand
// back whatever was most recently collected.
const dashboardRefreshInterval = 1 * time.Second

// dashboardIdleTimeout gates the collectors on actual demand: without this, a
// 1s ticker whose own collection takes ~1-2s runs back-to-back forever with
// no idle gap - confirmed live (`docker stats` measured directly against
// the daemon, bypassing this code entirely) as a real, sustained ~14% CPU
// cost on rootguard-core itself, running whether or not anyone had the
// dashboard open. The dashboard's own frontend already pauses polling once
// its tab is hidden or closed (Overview.tsx's visibilitychange handler); this
// mirrors that same idea on the Core side - CollectMetrics/CollectStatus
// stamp dashboardLastRequested on every call, and the background loops skip
// actually refreshing once nothing has asked for a while.
const dashboardIdleTimeout = 5 * time.Second

// dashboardStaleThreshold bounds how old a cached value CollectMetrics or
// CollectStatus is allowed to hand back without first trying to refresh it -
// see CollectMetrics for why this exists (the idle-skip above means a cache
// can otherwise sit untouched for as long as the dashboard was closed).
const dashboardStaleThreshold = 2 * dashboardRefreshInterval

var dashboardLastRequested = struct {
	sync.Mutex
	at time.Time
}{}

// markDashboardRequested is called by both CollectMetrics and CollectStatus
// on every invocation (harmless if stamped twice per dashboardHandler call)
// so a single idle timer covers both background collectors - they're always
// polled together in practice, so there's no reason to track two.
func markDashboardRequested() {
	dashboardLastRequested.Lock()
	dashboardLastRequested.at = time.Now()
	dashboardLastRequested.Unlock()
}

func dashboardRecentlyRequested() bool {
	dashboardLastRequested.Lock()
	defer dashboardLastRequested.Unlock()
	return time.Since(dashboardLastRequested.at) < dashboardIdleTimeout
}

var metricsCache = struct {
	sync.RWMutex
	value     Metrics
	updatedAt time.Time
}{}

var metricsRefreshGroup refreshGroup

// StartMetricsCollector runs the first collection synchronously (so a
// request arriving immediately after startup still gets real data almost
// as soon as it's available, rather than "not available" until the first
// tick) and then keeps collecting on dashboardRefreshInterval for as long as
// ctx stays alive, but only while CollectMetrics has actually been called
// recently. Call once, near process startup.
func StartMetricsCollector(ctx context.Context) {
	metricsRefreshGroup.do(func() { refreshMetricsCache(ctx) })
	go func() {
		ticker := time.NewTicker(dashboardRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !dashboardRecentlyRequested() {
					continue
				}
				metricsRefreshGroup.do(func() { refreshMetricsCache(ctx) })
			}
		}
	}()
}

// collectMetricsNowFunc is swapped out in tests so CollectMetrics's
// staleness logic (does it correctly trigger a synchronous refresh when the
// cache is old, and skip one when it isn't) can be verified without a real
// docker binary - mirrors the attestationRun seam in attestation.go.
var collectMetricsNowFunc = collectMetricsNow

func refreshMetricsCache(ctx context.Context) {
	metrics := collectMetricsNowFunc(ctx)
	metricsCache.Lock()
	metricsCache.value = metrics
	metricsCache.updatedAt = time.Now()
	metricsCache.Unlock()
}

func isMetricsCacheStale() bool {
	metricsCache.RLock()
	defer metricsCache.RUnlock()
	return metricsCache.updatedAt.IsZero() || time.Since(metricsCache.updatedAt) > dashboardStaleThreshold
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
	// CollectedAt is when the underlying `docker stats` sample was actually
	// taken (unix ms), not when this particular caller happened to read the
	// cache - see the frontend's pushHistory, which uses this to avoid
	// recording the same cached sample twice under two different ages just
	// because it polled faster than the cache refreshes.
	CollectedAt int64 `json:"collected_at"`
}

type dockerStatsLine struct {
	CPUPercent string `json:"CPUPerc"`
	Memory     string `json:"MemUsage"`
}

// CollectMetrics normally returns whatever the background collector (see
// StartMetricsCollector) most recently gathered, without itself invoking
// `docker stats` - so it doesn't block a caller (dashboardHandler) on that
// command's ~1-2s inherent latency. If StartMetricsCollector was never
// called (e.g. in a test that exercises dashboardHandler directly), this
// returns the zero Metrics{} (Available: false), the same graceful
// "unavailable" state a real collection failure already produces.
//
// Also stamps dashboardLastRequested so the background collector knows
// someone is actually asking - see dashboardIdleTimeout. That idle-skip is
// exactly what makes the cache capable of going stale in the first place:
// once nothing has asked for dashboardIdleTimeout, the background loop stops
// refreshing entirely, so whatever was cached before going idle just sits
// there - confirmed live, a dashboard reopened after sitting idle showed a
// CPU% far higher than the LXC's actual current load per Proxmox, because
// it was serving a reading from whenever the cache was last touched, not
// from now. If the cache is older than dashboardStaleThreshold, this pays the
// real collection cost once, synchronously, before answering - the same
// latency every request used to pay before caching existed, just now
// limited to this one "waking back up" case instead of every single call.
//
// Concurrent callers that all see a stale cache at once (several browser
// tabs reopening together, or a request racing the background ticker) share
// a single real refresh via metricsRefreshGroup instead of each starting
// their own `docker stats` process - see refresh.go.
func CollectMetrics(ctx context.Context) Metrics {
	markDashboardRequested()

	if isMetricsCacheStale() {
		metricsRefreshGroup.do(func() { refreshMetricsCache(ctx) })
	}

	metricsCache.RLock()
	defer metricsCache.RUnlock()
	result := metricsCache.value
	result.CollectedAt = metricsCache.updatedAt.UnixMilli()
	return result
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

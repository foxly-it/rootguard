package stack

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestCollectMetricsReadsTheBackgroundCollectorCacheWithoutInvokingDocker(t *testing.T) {
	// CollectMetrics itself must never shell out - it only reads whatever
	// StartMetricsCollector's background loop last stored, which is the
	// whole point of decoupling the HTTP response from `docker stats`'s
	// ~1-2s inherent latency. Manipulating metricsCache directly (instead
	// of calling StartMetricsCollector) keeps this test independent of a
	// real docker binary being available.
	t.Cleanup(func() {
		metricsCache.Lock()
		metricsCache.value = Metrics{}
		metricsCache.Unlock()
	})

	metricsCache.Lock()
	metricsCache.value = Metrics{}
	metricsCache.Unlock()
	if got := CollectMetrics(context.Background()); got.Available {
		t.Fatalf("expected unavailable metrics before any collection, got %#v", got)
	}

	want := Metrics{Available: true, CPUPercent: 4.5, MemoryBytes: 1024}
	metricsCache.Lock()
	metricsCache.value = want
	metricsCache.Unlock()
	if got := CollectMetrics(context.Background()); got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestCollectMetricsStampsLastRequestedForIdleDetection(t *testing.T) {
	t.Cleanup(func() {
		metricsLastRequested.Lock()
		metricsLastRequested.at = time.Time{}
		metricsLastRequested.Unlock()
	})

	metricsLastRequested.Lock()
	metricsLastRequested.at = time.Time{}
	metricsLastRequested.Unlock()
	if metricsRecentlyRequested() {
		t.Fatal("expected not recently requested before any CollectMetrics call")
	}

	CollectMetrics(context.Background())
	if !metricsRecentlyRequested() {
		t.Fatal("expected CollectMetrics to mark itself as a recent request")
	}
}

func TestMetricsRecentlyRequestedExpiresAfterIdleTimeout(t *testing.T) {
	t.Cleanup(func() {
		metricsLastRequested.Lock()
		metricsLastRequested.at = time.Time{}
		metricsLastRequested.Unlock()
	})

	metricsLastRequested.Lock()
	metricsLastRequested.at = time.Now().Add(-metricsIdleTimeout - time.Second)
	metricsLastRequested.Unlock()
	if metricsRecentlyRequested() {
		t.Fatal("expected a request older than metricsIdleTimeout to count as idle")
	}
}

func TestDecodeMetricsAggregatesAllowlistedContainerStats(t *testing.T) {
	payload := []byte(
		"{\"CPUPerc\":\"0.25%\",\"MemUsage\":\"18.5MiB / 2GiB\"}\n" +
			"{\"CPUPerc\":\"1.75%\",\"MemUsage\":\"32MiB / 2GiB\"}\n",
	)
	metrics, err := decodeMetrics(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !metrics.Available || math.Abs(metrics.CPUPercent-2) > 0.001 {
		t.Fatalf("unexpected CPU metrics: %#v", metrics)
	}
	const expectedMemory = uint64(50.5 * (1 << 20))
	if metrics.MemoryBytes != expectedMemory {
		t.Fatalf("expected %d memory bytes, got %d", expectedMemory, metrics.MemoryBytes)
	}
}

func TestDecodeMetricsRejectsMalformedStats(t *testing.T) {
	if _, err := decodeMetrics([]byte("{\"CPUPerc\":\"invalid\",\"MemUsage\":\"2MiB / 1GiB\"}\n")); err == nil {
		t.Fatal("expected malformed CPU percentage to be rejected")
	}
	if _, err := decodeMetrics([]byte("{\"CPUPerc\":\"1%\",\"MemUsage\":\"2watts / 1GiB\"}\n")); err == nil {
		t.Fatal("expected unsupported memory unit to be rejected")
	}
}

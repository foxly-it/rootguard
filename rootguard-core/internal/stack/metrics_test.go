package stack

import (
	"context"
	"math"
	"testing"
	"time"
)

func resetMetricsCacheForTest(t *testing.T) {
	t.Helper()
	original := collectMetricsNowFunc
	t.Cleanup(func() {
		collectMetricsNowFunc = original
		metricsCache.Lock()
		metricsCache.value = Metrics{}
		metricsCache.updatedAt = time.Time{}
		metricsCache.Unlock()
	})
}

func TestCollectMetricsReadsAFreshCacheWithoutInvokingDocker(t *testing.T) {
	// CollectMetrics must not shell out when the cache is fresh - it just
	// reads whatever StartMetricsCollector's background loop last stored,
	// which is the whole point of decoupling the HTTP response from
	// `docker stats`'s ~1-2s inherent latency. Swapping collectMetricsNowFunc
	// to fail the test if it's ever called (instead of just leaving the real
	// one in place) keeps this test's guarantee explicit rather than
	// incidental.
	resetMetricsCacheForTest(t)
	collectMetricsNowFunc = func(context.Context) Metrics {
		t.Fatal("collectMetricsNowFunc must not be called for a fresh cache")
		return Metrics{}
	}

	want := Metrics{Available: true, CPUPercent: 4.5, MemoryBytes: 1024}
	metricsCache.Lock()
	metricsCache.value = want
	metricsCache.updatedAt = time.Now()
	metricsCache.Unlock()

	if got := CollectMetrics(context.Background()); got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestCollectMetricsRefreshesSynchronouslyWhenCacheIsStale(t *testing.T) {
	// The idle-skip in StartMetricsCollector (metricsIdleTimeout) is exactly
	// what lets the cache go stale in the first place: once nothing has
	// asked for a while, the background loop stops touching it entirely.
	// Confirmed live: reopening the dashboard after it sat idle showed a
	// CPU% far higher than the LXC's actual current load, because it was
	// serving whatever was cached from before going idle. CollectMetrics
	// must catch up itself when that's happened, rather than handing back
	// an arbitrarily old number forever.
	resetMetricsCacheForTest(t)
	calls := 0
	collectMetricsNowFunc = func(context.Context) Metrics {
		calls++
		return Metrics{Available: true, CPUPercent: 7, MemoryBytes: 99}
	}

	metricsCache.Lock()
	metricsCache.value = Metrics{Available: true, CPUPercent: 999}
	metricsCache.updatedAt = time.Now().Add(-metricsStaleThreshold - time.Second)
	metricsCache.Unlock()

	got := CollectMetrics(context.Background())
	if calls != 1 {
		t.Fatalf("expected exactly one synchronous refresh for a stale cache, got %d calls", calls)
	}
	want := Metrics{Available: true, CPUPercent: 7, MemoryBytes: 99}
	if got != want {
		t.Fatalf("expected the freshly collected value %#v, got %#v", want, got)
	}
}

func TestCollectMetricsTreatsAnUntouchedCacheAsStale(t *testing.T) {
	// The zero value of updatedAt (StartMetricsCollector never having run,
	// e.g. in a test exercising dashboardHandler directly) must count as
	// stale, not as "just refreshed a very very long time ago" - IsZero()
	// is checked explicitly for this rather than relying on time.Since of
	// the zero Time producing a suitably large duration by coincidence.
	resetMetricsCacheForTest(t)
	calls := 0
	collectMetricsNowFunc = func(context.Context) Metrics {
		calls++
		return Metrics{Available: true, CPUPercent: 1}
	}

	if got := CollectMetrics(context.Background()); calls != 1 || !got.Available {
		t.Fatalf("expected an untouched cache to trigger exactly one refresh, got %d calls, result %#v", calls, got)
	}
}

func TestCollectMetricsStampsLastRequestedForIdleDetection(t *testing.T) {
	resetMetricsCacheForTest(t)
	collectMetricsNowFunc = func(context.Context) Metrics { return Metrics{} }
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

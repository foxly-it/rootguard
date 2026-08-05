package stack

import (
	"math"
	"testing"
)

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

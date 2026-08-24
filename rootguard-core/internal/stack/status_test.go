package stack

import (
	"reflect"
	"testing"
	"time"
)

func resetStatusCacheForTest(t *testing.T) {
	t.Helper()
	original := collectStackStatusFunc
	t.Cleanup(func() {
		collectStackStatusFunc = original
		statusCache.Lock()
		statusCache.value = StackStatus{}
		statusCache.updatedAt = time.Time{}
		statusCache.Unlock()
	})
}

func TestCollectStatusReadsAFreshCacheWithoutInvokingDocker(t *testing.T) {
	resetStatusCacheForTest(t)
	collectStackStatusFunc = func() StackStatus {
		t.Fatal("collectStackStatusFunc must not be called for a fresh cache")
		return StackStatus{}
	}

	want := StackStatus{AdGuard: ContainerInfo{Running: true}, Unbound: ContainerInfo{Running: true}}
	statusCache.Lock()
	statusCache.value = want
	statusCache.updatedAt = time.Now()
	statusCache.Unlock()

	if got := CollectStatus(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestCollectStatusRefreshesSynchronouslyWhenCacheIsStale(t *testing.T) {
	resetStatusCacheForTest(t)
	calls := 0
	collectStackStatusFunc = func() StackStatus {
		calls++
		return StackStatus{AdGuard: ContainerInfo{Running: true}, Unbound: ContainerInfo{Running: true}}
	}

	statusCache.Lock()
	statusCache.value = StackStatus{}
	statusCache.updatedAt = time.Now().Add(-dashboardStaleThreshold - time.Second)
	statusCache.Unlock()

	got := CollectStatus()
	if calls != 1 {
		t.Fatalf("expected exactly one synchronous refresh for a stale cache, got %d calls", calls)
	}
	if !got.AdGuard.Running || !got.Unbound.Running {
		t.Fatalf("expected the freshly collected value, got %#v", got)
	}
}

func TestCollectStatusTreatsAnUntouchedCacheAsStale(t *testing.T) {
	resetStatusCacheForTest(t)
	calls := 0
	collectStackStatusFunc = func() StackStatus {
		calls++
		return StackStatus{AdGuard: ContainerInfo{Running: true}}
	}

	if got := CollectStatus(); calls != 1 || !got.AdGuard.Running {
		t.Fatalf("expected an untouched cache to trigger exactly one refresh, got %d calls, result %#v", calls, got)
	}
}

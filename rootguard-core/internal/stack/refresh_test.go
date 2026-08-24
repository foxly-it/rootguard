package stack

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefreshGroupCollapsesConcurrentCallersIntoOneRun(t *testing.T) {
	// Found via code review: without this, several callers all seeing a
	// stale cache at once (the background ticker plus multiple requests
	// after an idle period, or several open dashboard tabs) could each
	// start their own expensive collection concurrently. A refreshGroup
	// must ensure only one actually runs while the rest wait for it.
	var group refreshGroup
	var running int32
	var calls int32
	var maxConcurrent int32

	start := make(chan struct{})
	var wait sync.WaitGroup
	const callers = 20
	wait.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wait.Done()
			<-start
			group.do(func() {
				atomic.AddInt32(&calls, 1)
				current := atomic.AddInt32(&running, 1)
				for {
					previous := atomic.LoadInt32(&maxConcurrent)
					if current <= previous || atomic.CompareAndSwapInt32(&maxConcurrent, previous, current) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&running, -1)
			})
		}()
	}
	close(start)
	wait.Wait()

	if got := atomic.LoadInt32(&maxConcurrent); got != 1 {
		t.Fatalf("expected at most one concurrent run, saw %d overlapping", got)
	}
	if got := atomic.LoadInt32(&calls); got < 1 || got > callers {
		t.Fatalf("expected between 1 and %d actual runs, got %d", callers, got)
	}
}

func TestRefreshGroupRunsAgainAfterCompleting(t *testing.T) {
	var group refreshGroup
	calls := 0
	group.do(func() { calls++ })
	group.do(func() { calls++ })
	if calls != 2 {
		t.Fatalf("expected two separate runs once the first completed, got %d", calls)
	}
}

package stack

import "sync"

// refreshGroup collapses concurrent triggers for the same expensive refresh
// - the background ticker, a request that sees a stale cache, several such
// requests at once, multiple open dashboard tabs - into a single in-flight
// call. A caller arriving while one is already running waits for it to
// finish instead of starting its own redundant copy. Found via code review:
// without this, an idle period ending could start several concurrent
// `docker stats`/`docker inspect` processes at once, and one that started
// earlier but finished later could overwrite a newer result.
type refreshGroup struct {
	mu      sync.Mutex
	pending chan struct{}
}

func (g *refreshGroup) do(work func()) {
	g.mu.Lock()
	if g.pending != nil {
		wait := g.pending
		g.mu.Unlock()
		<-wait
		return
	}
	done := make(chan struct{})
	g.pending = done
	g.mu.Unlock()

	work()

	g.mu.Lock()
	g.pending = nil
	g.mu.Unlock()
	close(done)
}

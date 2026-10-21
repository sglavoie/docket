package bridge

import "sync"

// requestGate serializes bridge requests, giving waiting local work precedence
// over sync. An import yields between complete transactions, never during one.
// The zero value is ready to use.
type requestGate struct {
	mu           sync.Mutex
	changed      *sync.Cond
	active       bool
	localWaiting int
}

func (g *requestGate) enter(background bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.changed == nil {
		g.changed = sync.NewCond(&g.mu)
	}
	if !background {
		g.localWaiting++
	}
	for g.active || (background && g.localWaiting > 0) {
		g.changed.Wait()
	}
	if !background {
		g.localWaiting--
	}
	g.active = true
}

func (g *requestGate) leave() {
	g.mu.Lock()
	g.active = false
	g.changed.Broadcast()
	g.mu.Unlock()
}

func (g *requestGate) yieldBackground() {
	g.leave()
	g.enter(true)
}

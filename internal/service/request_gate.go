package service

import "sync/atomic"

const requestsStopped uint64 = 1 << 63

// requestGate counts admitted requests without serializing their work. The
// stopped bit and count share one atomic value so shutdown cannot miss an entry.
type requestGate struct {
	state   atomic.Uint64
	drained chan struct{}
}

func newRequestGate() *requestGate { return &requestGate{drained: make(chan struct{})} }

func (g *requestGate) acquire() bool {
	for {
		state := g.state.Load()
		if state&requestsStopped != 0 {
			return false
		}
		if g.state.CompareAndSwap(state, state+1) {
			return true
		}
	}
}

func (g *requestGate) release() {
	if g.state.Add(^uint64(0)) == requestsStopped {
		close(g.drained)
	}
}

func (g *requestGate) stop() {
	if g.state.Or(requestsStopped) == 0 {
		close(g.drained)
	}
}

func (g *requestGate) wait() { <-g.drained }

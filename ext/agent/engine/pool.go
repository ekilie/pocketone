package engine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// pool is a persistent set of worker goroutines. A dispatch runs fn(worker)
// on every worker (the caller is worker 0) and returns when all are done.
// Workers spin briefly between dispatches (decode issues hundreds of small
// dispatches per token, and parking costs tens of microseconds), then park.
type pool struct {
	n       int
	mu      sync.Mutex // one dispatch at a time (several generations share the pool)
	fn      func(w int)
	gen     atomic.Uint64
	pending atomic.Int64
	sleepMu sync.Mutex
	cond    *sync.Cond
	asleep  int
	closed  atomic.Bool
	cursors []cursor
}

const spinFor = 300 * time.Microsecond

func newPool(n int) *pool {
	if n < 1 {
		n = 1
	}
	// Spinning workers need their own Ps.
	n = min(n, runtime.GOMAXPROCS(0))
	p := &pool{n: n, cursors: make([]cursor, n)}
	p.cond = sync.NewCond(&p.sleepMu)
	for w := 1; w < n; w++ {
		go p.worker(w)
	}
	return p
}

func (p *pool) worker(w int) {
	// Generation 0 is "no dispatch yet". Reading gen here instead would race
	// with a dispatch issued before this goroutine first ran, and hang it.
	seen := uint64(0)
	for {
		// spin, then park
		start := time.Time{}
		spins := 0
		for p.gen.Load() == seen {
			if p.closed.Load() {
				return
			}
			spins++
			if spins&63 == 0 {
				if start.IsZero() {
					start = time.Now()
				} else if time.Since(start) > spinFor {
					p.sleepMu.Lock()
					p.asleep++
					for p.gen.Load() == seen && !p.closed.Load() {
						p.cond.Wait()
					}
					p.asleep--
					p.sleepMu.Unlock()
					start = time.Time{}
				}
			}
		}
		if p.closed.Load() {
			return
		}
		seen = p.gen.Load()
		p.fn(w)
		p.pending.Add(-1)
	}
}

// run executes fn on all workers and waits.
func (p *pool) run(fn func(w int)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runLocked(fn)
}

func (p *pool) runLocked(fn func(w int)) {
	if p.n == 1 {
		fn(0)
		return
	}
	p.fn = fn
	p.pending.Store(int64(p.n - 1))
	p.gen.Add(1)
	p.sleepMu.Lock()
	if p.asleep > 0 {
		p.cond.Broadcast()
	}
	p.sleepMu.Unlock()
	fn(0)
	for i := 0; p.pending.Load() != 0; i++ {
		if i&0xfffff == 0xfffff {
			runtime.Gosched() // only if a worker was descheduled
		}
	}
	p.fn = nil
}

// parallelFor splits [0,n) into chunks of size chunk handed out dynamically.
func (p *pool) parallelFor(n, chunk int, body func(lo, hi int)) {
	p.parallelForW(n, chunk, func(_, lo, hi int) { body(lo, hi) })
}

// parallelForW is parallelFor with the worker index passed to body.
// Each worker owns a contiguous share of the chunks with its own cursor (no
// shared counter bouncing between core clusters); a worker that finishes its
// share steals chunks from the others.
func (p *pool) parallelForW(n, chunk int, body func(w, lo, hi int)) {
	if n <= 0 {
		return
	}
	if chunk < 1 {
		chunk = 1
	}
	nw := p.n
	if nw == 1 {
		body(0, 0, n)
		return
	}
	nc := (n + chunk - 1) / chunk
	p.mu.Lock()
	defer p.mu.Unlock()
	cur := p.cursors[:nw]
	for w := range cur {
		cur[w].next.Store(int64(w * nc / nw))
		cur[w].end = int64((w + 1) * nc / nw)
	}
	p.runLocked(func(w int) {
		for k := 0; k < nw; k++ {
			c := &cur[(w+k)%nw]
			for {
				i := c.next.Add(1) - 1
				if i >= c.end {
					break
				}
				lo := int(i) * chunk
				body(w, lo, min(lo+chunk, n))
			}
		}
	})
}

type cursor struct {
	next atomic.Int64
	end  int64
	_    [112]byte // own cache line (Apple cores use 128-byte lines)
}

func (p *pool) close() {
	p.closed.Store(true)
	p.sleepMu.Lock()
	p.cond.Broadcast()
	p.sleepMu.Unlock()
}

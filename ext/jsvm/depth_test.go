//go:build cgo

package jsvm

import (
	"sync"
	"testing"
	"time"
)

func TestPumpDepthLimit(t *testing.T) {
	for _, n := range []int{100, 300} {
		cfg := pumpCfg()
		cfg.MaxPumpDepth = 10000
		_, vm, h := pbApp(t, cfg, hooksJS)
		start := time.Now()
		var wg sync.WaitGroup
		var mu sync.Mutex
		fails := 0
		first := ""
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if code, body := get(t, h, "GET", "/slow-sync"); code != 200 {
					mu.Lock()
					fails++
					if first == "" {
						first = body
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		t.Logf("%d concurrent: %v, max depth %d, fails %d %s", n, time.Since(start), vm.statMaxPump.Load(), fails, first)
	}
}

package cache

import (
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"testing"
	"time"

	cachev1 "github.com/pockethost/pocketone/ext/contracts/cache/v1"
)

// BenchmarkParallel is many instances using the store at once: 32 spaces, 80% reads, values of 1-8 KiB from a set
// small enough that some are shared. "disk" makes memory small so reads keep bringing values back from the disk tier.
func BenchmarkParallel(b *testing.B) {
	for _, mode := range []string{"memory", "disk"} {
		b.Run(mode, func(b *testing.B) {
			lim := poolLimits{eviction: cachev1.LRU, maxMem: 64 << 20}
			if mode == "disk" {
				lim.maxMem, lim.dir, lim.maxDisk = 2<<20, b.TempDir(), 1<<30
			}
			s := newTestStore(b, lim)
			spaces := make([]*space, 32)
			for i := range spaces {
				spaces[i] = s.open(fmt.Sprintf("inst%d/b", i), spaceLimits{eviction: cachev1.LRU})
			}
			values := make([][]byte, 256)
			for i := range values {
				values[i] = make([]byte, 1024*(1+i%8))
				values[i][0], values[i][1] = byte(i), byte(i>>8)
			}
			for i := range 4096 { // warm
				sp := spaces[i%len(spaces)]
				s.set(sp, fmt.Sprint(i%512), values[i%len(values)], time.Hour)
			}
			var seed atomic.Uint64
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				r := rand.New(rand.NewPCG(seed.Add(1), 0))
				for pb.Next() {
					sp := spaces[r.IntN(len(spaces))]
					k := fmt.Sprint(r.IntN(512))
					if r.IntN(10) < 8 {
						s.get(sp, k)
					} else {
						s.set(sp, k, values[r.IntN(len(values))], time.Hour)
					}
				}
			})
			b.StopTimer()
			s.flush()
		})
	}
}

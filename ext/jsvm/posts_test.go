package jsvm

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestJobRunsItsPosts: the posts a job leaves (finalizers' frees, async completions) run before the loop takes the
// next job. The loop used to pick a job or a post at random, one at a time, so under steady load it ran about one
// post per job, the queue stayed full and the process's finalizer goroutine blocked posting frees: JS values Go had
// dropped were never freed and a busy app's VM ran out of memory.
func TestJobRunsItsPosts(t *testing.T) {
	vm := testVM(t)
	const posts = 1000
	var ran atomic.Int64
	seen := make(chan int64, 1)
	err := vm.Do(func() {
		for range posts {
			vm.post(func() { ran.Add(1) })
		}
		// queue the next job before this one ends, so the loop has both to choose from
		go func() { _ = vm.Do(func() { seen <- ran.Load() }) }()
		deadline := time.Now().Add(5 * time.Second)
		for len(vm.jobs) == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if len(vm.jobs) == 0 {
			t.Error("the next job was never queued")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-seen:
		if n != posts {
			t.Fatalf("the next job ran with %d of %d posts done", n, posts)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the next job never ran")
	}
}

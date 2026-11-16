package cron

import (
	"runtime"
	"testing"
	"time"
)

// PocketOne: Stop must not race the ticker goroutine. Since tickerDone became buffered, Stop no longer waits for the
// goroutine to receive, so the goroutine read c.ticker.C while Stop set c.ticker to nil (go test -race catches it).
func TestCronStopDoesNotRaceTicker(t *testing.T) {
	for i := 0; i < 20; i++ {
		c := New()
		c.SetInterval(time.Millisecond)
		c.Start()
		time.Sleep(5 * time.Millisecond) // let the start timer fire and a few ticks run
		c.Stop()
		time.Sleep(2 * time.Millisecond) // let the goroutine loop once more after Stop
	}
}

// PocketOne: quick Stop/Start cycles must not hand one start's stop signal to another start's loop, which would leave
// a loop running forever on a stopped ticker.
func TestCronStopStartCyclesLeaveNoLoops(t *testing.T) {
	before := runtime.NumGoroutine()

	c := New()
	c.SetInterval(time.Millisecond)
	for i := 0; i < 200; i++ {
		c.Start()
		time.Sleep(time.Duration(i%3) * time.Millisecond)
	}
	c.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines left running after Stop (had %d before)", runtime.NumGoroutine()-before, before)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

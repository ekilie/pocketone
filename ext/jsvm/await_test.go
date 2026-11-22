package jsvm

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Routes that await sleep.async in 50 ms steps, counting the steps they finish in a global.
const awaitHooksJS = `
let steps = 0
routerAdd("GET", "/await/{n}", async (e) => {
	const n = Number(e.request.pathValue("n"))
	for (let i = 0; i < n; i++) {
		await sleep.async(50)
		steps++
	}
	return e.string(200, "done " + n)
})
routerAdd("GET", "/detach/{n}", (e) => {
	const n = Number(e.request.pathValue("n"))
	;(async () => {
		for (let i = 0; i < n; i++) {
			await sleep.async(50)
			steps++
		}
	})()
	return e.string(200, "started")
})
routerAdd("GET", "/spin", async (e) => {
	await sleep.async(10)
	while (true) {}
})
routerAdd("GET", "/steps", (e) => e.string(200, String(steps)))
`

func stepCount(t *testing.T, vmh func(string) (int, string)) int {
	t.Helper()
	code, body := vmh("/steps")
	if code != 200 {
		t.Fatalf("/steps: %d %s", code, body)
	}
	n, err := strconv.Atoi(strings.TrimSpace(body))
	if err != nil {
		t.Fatalf("/steps: %q", body)
	}
	return n
}

// An async handler awaiting longer than WaitTimeout and TaskWall succeeds: awaits count only toward AwaitLimit.
func TestAwaitOutlastsWaitTimeout(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{WaitTimeout: 200 * time.Millisecond, TaskWall: 200 * time.Millisecond, AwaitLimit: 10 * time.Second}, awaitHooksJS)
	start := time.Now()
	code, body := get(t, h, "GET", "/await/12") // ~600 ms of awaits
	if code != 200 || body != "done 12" {
		t.Fatalf("got %d %q after %s", code, body, time.Since(start))
	}
	if d := time.Since(start); d < 500*time.Millisecond {
		t.Fatalf("finished too soon: %s", d)
	}
}

// Past AwaitLimit the request fails, and the handler stops at its next await instead of running on.
func TestAwaitLimitStopsHandler(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{AwaitLimit: 300 * time.Millisecond}, awaitHooksJS)
	q := func(u string) (int, string) { return get(t, h, "GET", u) }
	start := time.Now()
	code, body := q("/await/100") // would take 5 s
	if code == 200 {
		t.Fatalf("got 200 %q, want a failure", body)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("failed after %s, want ~300ms", d)
	}
	at := stepCount(t, q)
	time.Sleep(400 * time.Millisecond)
	if after := stepCount(t, q); after > at+1 { // the step in flight when it gave up may still land
		t.Fatalf("handler kept running after its request failed: %d steps, then %d", at, after)
	}
}

// When the client goes away, the handler stops too.
func TestAbandonedRequestStopsHandler(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{}, awaitHooksJS)
	q := func(u string) (int, string) { return get(t, h, "GET", u) }
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(250 * time.Millisecond); cancel() }()
	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/await/100", nil).WithContext(ctx))
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("request returned after %s, want ~250ms", d)
	}
	at := stepCount(t, q)
	time.Sleep(400 * time.Millisecond)
	if after := stepCount(t, q); after > at+1 {
		t.Fatalf("handler kept running after the client left: %d steps, then %d", at, after)
	}
}

// A detached promise keeps running after its handler answered (the long-job pattern).
func TestDetachedWorkOutlivesRequest(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{AwaitLimit: 100 * time.Millisecond}, awaitHooksJS)
	q := func(u string) (int, string) { return get(t, h, "GET", u) }
	if code, body := q("/detach/8"); code != 200 {
		t.Fatalf("/detach: %d %s", code, body)
	}
	deadline := time.Now().Add(3 * time.Second)
	for stepCount(t, q) < 8 {
		if time.Now().After(deadline) {
			t.Fatalf("detached work stopped at %d of 8 steps", stepCount(t, q))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// CPU-bound JS after an await still hits TaskBudget, however generous AwaitLimit is.
func TestTaskBudgetAfterAwait(t *testing.T) {
	_, _, h := pbApp(t, VMConfig{TaskBudget: 200 * time.Millisecond, AwaitLimit: time.Minute}, awaitHooksJS)
	start := time.Now()
	code, body := get(t, h, "GET", "/spin")
	if code == 200 {
		t.Fatalf("got 200 %q", body)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("spin stopped after %s, want ~200ms", d)
	}
}

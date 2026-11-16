package jsvm

import (
	"os"
	"testing"
	"time"
)

const txHooksJS = `
routerAdd("POST", "/tx-async", async (e) => {
	await $app.runInTransaction.async(async (txApp) => {
		const r = new Record(txApp.findCollectionByNameOrId("demo2"))
		r.set("title", "txa")
		await txApp.save.async(r)
		await sleep.async(200) // the transaction stays open across this await
	})
	return e.string(200, "ok")
})
routerAdd("POST", "/write-sync", (e) => {
	const r = new Record($app.findCollectionByNameOrId("demo2"))
	r.set("title", "write")
	$app.save(r) // blocks the loop until the write lock is free
	return e.string(200, "ok")
})
`

// An async transaction holds SQLite's single write connection across an await; a sync write then blocks the loop
// waiting for it, and the transaction's continuation needs the loop: deadlock unless sync calls pump the loop.
func TestTxDeadlock(t *testing.T) {
	if os.Getenv("ASYNCVM_DEADLOCK") == "" && !pumpOn {
		t.Skip("leaves a stuck loop thread; run with ASYNCVM_DEADLOCK=1")
	}
	cfg := VMConfig{WaitTimeout: 3 * time.Second}
	if pumpOn {
		cfg.PumpSyncCalls, cfg.Blocking = true, PBBlocking
	}
	_, _, h := pbAppNoClose(t, cfg, txHooksJS)
	done := make(chan int, 2)
	go func() { c, b := get(t, h, "POST", "/tx-async"); t.Log("tx-async", c, b); done <- c }()
	time.Sleep(50 * time.Millisecond)
	go func() { c, b := get(t, h, "POST", "/write-sync"); t.Log("write-sync", c, b); done <- c }()
	var codes []int
	timeout := time.After(5 * time.Second)
	for len(codes) < 2 {
		select {
		case c := <-done:
			codes = append(codes, c)
		case <-timeout:
			t.Fatalf("deadlock: only %v finished after 5s", codes)
		}
	}
	t.Logf("codes %v", codes)
	for _, c := range codes {
		if c != 200 {
			t.Fatalf("codes %v", codes)
		}
	}
}

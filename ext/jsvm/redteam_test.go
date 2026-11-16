package jsvm

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

var attacks = []struct{ name, js string }{
	{"string doubling", `let s = "x"; for (;;) s += s`},
	{"array of strings", `let a = []; for (;;) a.push("x".repeat(1000) + a.length)`},
	{"object graph", `let a = []; for (;;) a.push({ x: { y: [1, 2, 3], z: "abc" + a.length } })`},
	{"closures", `let a = []; for (;;) { const i = a.length; a.push(() => i) }`},
	{"map growth", `const m = new Map(); for (let i = 0; ; i++) m.set(i, { i })`},
	{"typed array 1GB", `new Uint8Array(1 << 30)`},
	{"array buffer loop", `let a = []; for (;;) a.push(new ArrayBuffer(1 << 20))`},
	{"repeat 1G", `"x".repeat(1 << 30)`},
	{"sparse fill", `new Array(1e9).fill(0)`},
	{"bigint shift", `1n << 8000000000n`},
	{"bigint asUintN", `BigInt.asUintN(9e9, -1n)`},
	{"json parse big", `const s = "[" + "1,".repeat(1 << 26) + "1]"; JSON.parse(s)`},
	{"regex backtracking", `/(a+)+$/.test("a".repeat(40) + "b")`},
	{"promise flood", `for (;;) Promise.resolve().then(() => {})`},
	{"microtask chain", `function f() { Promise.resolve().then(f) }; f()`},
	{"spin", `for (;;) {}`},
	{"deep recursion", `function f(n) { return f(n + 1) + 1 }; f(0)`},
	{"deep recursion via getter", `const o = {}; Object.defineProperty(o, "x", { get() { return o.x } }); o.x`},
	{"deep proxy chain", `let p = {}; for (let i = 0; i < 1e6; i++) p = new Proxy(p, {}); p.x`},
	{"catch OOM and continue", `let a = []; for (;;) { try { a.push("x".repeat(1 << 20)) } catch (e) { a.length = a.length - 1 } }`},
	{"generator pile", `let a = []; function* g() { yield 1 }; for (;;) a.push(g())`},
	{"go handles: Record", `let a = []; for (;;) a.push(new Record())`},
	{"go handles: $app", `let a = []; for (;;) a.push($app)`},
	{"go handles: arrayOf", `let a = []; for (;;) a.push(arrayOf(new Record()))`},
	{"go handles: method values", `let a = []; for (;;) a.push($app.findRecordById)`},
	{"go: big Go result", `let a = []; for (;;) a.push($app.findAllCollections())`},
	{"async go call flood", `for (;;) sleep.async(10000)`},
}

func rssBytes() int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0
	}
	kb, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kb << 10
}

// 50 VMs in one process; for each attack, 10 run it at once while 40 answer pings.
func TestRedTeam(t *testing.T) {
	if os.Getenv("ASYNCVM_REDTEAM") == "" {
		t.Skip("set ASYNCVM_REDTEAM=1")
	}
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	const nVM, nAttack, limit = 50, 10, 32 << 20
	cfg := VMConfig{MemoryLimit: limit, TaskBudget: 2 * time.Second, TaskWall: 5 * time.Second, WaitTimeout: 20 * time.Second, PumpSyncCalls: true, Blocking: PBBlocking, Shared: PBShared}
	only := os.Getenv("ASYNCVM_ATTACK")
	for _, at := range attacks {
		if only != "" && !strings.Contains(at.name, only) {
			continue
		}
		vms := make([]*VM, nVM)
		for i := range vms {
			vm, err := New(cfg, func(vm *VM) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			vm.Log = func(...any) {}
			if err := BindApp(vm, app); err != nil {
				t.Fatal(err)
			}
			vms[i] = vm
		}
		runtime.GC()
		base := rssBytes()
		var peak atomic.Int64
		stop := make(chan struct{})
		go func() {
			for {
				select {
				case <-stop:
					return
				case <-time.After(25 * time.Millisecond):
					if r := rssBytes(); r > peak.Load() {
						peak.Store(r)
					}
				}
			}
		}()
		// healthy VMs ping while the attack runs
		var pings, slowPings, badPings atomic.Int64
		var maxPing atomic.Int64
		pingStop := make(chan struct{})
		var pwg sync.WaitGroup
		for _, vm := range vms[nAttack:] {
			pwg.Add(1)
			go func(vm *VM) {
				defer pwg.Done()
				for {
					select {
					case <-pingStop:
						return
					default:
					}
					s := time.Now()
					if err := vm.Eval(`1 + 1`, "ping.js"); err != nil {
						badPings.Add(1)
					}
					d := time.Since(s)
					pings.Add(1)
					if d > 50*time.Millisecond {
						slowPings.Add(1)
					}
					if int64(d) > maxPing.Load() {
						maxPing.Store(int64(d))
					}
					time.Sleep(5 * time.Millisecond)
				}
			}(vm)
		}
		start := time.Now()
		var wg sync.WaitGroup
		errs := make([]string, nAttack)
		durs := make([]time.Duration, nAttack)
		for i := 0; i < nAttack; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				s := time.Now()
				err := vms[i].Eval(at.js, "attack.js")
				durs[i] = time.Since(s)
				if err != nil {
					errs[i] = err.Error()
				} else {
					errs[i] = "(no error)"
				}
			}(i)
		}
		wg.Wait()
		elapsed := time.Since(start)
		// attackers must still work afterwards
		afterOK := 0
		for _, vm := range vms[:nAttack] {
			if vm.Eval(`1 + 1`, "after.js") == nil {
				afterOK++
			}
		}
		var gob int64
		for _, vm := range vms[:nAttack] {
			gob += vm.statGoBytes.Load()
		}
		close(pingStop)
		pwg.Wait()
		close(stop)
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		msg := errs[0]
		if len(msg) > 70 {
			msg = msg[:70]
		}
		t.Logf("%-28s %6.2fs  err=%q  peakRSS+%4dMB  goHeap=%4dMB  goCharged=%3dMB  attackersOK=%d/%d  pings=%d slow=%d bad=%d max=%v",
			at.name, elapsed.Seconds(), msg, (peak.Load()-base)>>20, ms.HeapInuse>>20, gob>>20, afterOK, nAttack, pings.Load(), slowPings.Load(), badPings.Load(), time.Duration(maxPing.Load()).Round(time.Millisecond))
		for _, vm := range vms {
			vm.Close()
		}
		runtime.GC()
		_ = fmt.Sprint
	}
}

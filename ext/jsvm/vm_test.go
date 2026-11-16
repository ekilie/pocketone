package jsvm

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type Post struct {
	Title string
	Tags  []string
	Meta  map[string]any
}

func (p *Post) Upper() string         { return strings.ToUpper(p.Title) }
func (p *Post) SetTitle(s string)     { p.Title = s }
func (p *Post) Fail(msg string) error { return errors.New(msg) }
func (p *Post) Slow(ms int) string    { time.Sleep(time.Duration(ms) * time.Millisecond); return p.Title }
func (p *Post) Pair() (string, error) { return p.Title, nil }
func (p *Post) Each(f func(int) error) error {
	for i := 0; i < 3; i++ {
		if err := f(i); err != nil {
			return err
		}
	}
	return nil
}

func newVM(t testing.TB, cfg VMConfig) *VM {
	t.Helper()
	vm, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vm.Close)
	return vm
}

func evalOK(t testing.TB, vm *VM, code string) {
	t.Helper()
	if err := vm.Eval(code, "test.js"); err != nil {
		t.Fatalf("eval: %v\n%s", err, code)
	}
}

func TestGlobalsAndClosures(t *testing.T) {
	vm := newVM(t, VMConfig{})
	var got []string
	vm.Log = func(a ...any) { got = append(got, a[0].(string)) }
	evalOK(t, vm, `
		const greet = (n) => "hi " + n
		let count = 0
		globalThis.handler = () => { count++; console.log(greet("x") + " " + count) }
	`)
	evalOK(t, vm, `handler(); handler()`)
	if strings.Join(got, ",") != "hi x 1,hi x 2" {
		t.Fatal(got)
	}
}

func TestGoObjectLive(t *testing.T) {
	vm := newVM(t, VMConfig{})
	p := &Post{Title: "hello", Tags: []string{"a", "b"}, Meta: map[string]any{"views": 3}}
	if err := vm.Set("post", p); err != nil {
		t.Fatal(err)
	}
	evalOK(t, vm, `
		if (post.title !== "hello") throw new Error("title " + post.title)
		if (post.upper() !== "HELLO") throw new Error("upper")
		post.setTitle("changed")
		post.meta.views = 4
		post.meta.extra = "yes"
		if (post.tags.length !== 2 || post.tags[1] !== "b") throw new Error("tags")
		let seen = []
		for (const t of post.tags) seen.push(t)
		if (seen.join() !== "a,b") throw new Error("iter " + seen)
		if (JSON.stringify(post) !== '{"Title":"changed","Tags":["a","b"],"Meta":{"extra":"yes","views":4}}') throw new Error(JSON.stringify(post))
		try { post.fail("boom"); throw new Error("no throw") } catch (e) { if (e.message !== "boom") throw e }
		if (post.pair() !== "changed") throw new Error("pair")
		let calls = []
		post.each((i) => { calls.push(i) })
		if (calls.join() !== "0,1,2") throw new Error("each " + calls)
		if (!Object.keys(post).includes("title")) throw new Error("keys " + Object.keys(post))
	`)
	if p.Title != "changed" || p.Meta["views"] != int64(4) || p.Meta["extra"] != "yes" {
		t.Fatalf("%+v", p)
	}
}

func TestAsyncGoCall(t *testing.T) {
	vm := newVM(t, VMConfig{})
	vm.Set("post", &Post{Title: "async"})
	evalOK(t, vm, `
		globalThis.out = []
		;(async () => {
			const a = post.slow.async(50)
			const b = post.slow.async(50)
			out.push(await a, await b)
		})()
	`)
	time.Sleep(200 * time.Millisecond)
	evalOK(t, vm, `if (out.join() !== "async,async") throw new Error("out " + out)`)
}

// Go calls a JS function that returns a promise; Go waits, the loop stays free.
func TestCallAsyncHandlerFromGo(t *testing.T) {
	vm := newVM(t, VMConfig{})
	vm.Set("post", &Post{Title: "p"})
	evalOK(t, vm, `globalThis.h = async (n) => { await post.slow.async(30); return n * 2 }`)
	var ref *jsRef
	vm.Do(func() { ref = vm.globalRef("h") })
	ft := reflect.TypeOf(func(int) (int, error) { return 0, nil })
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := vm.CallRef(ref, []reflect.Value{reflect.ValueOf(i)}, ft)
			if err != nil || res.Int() != int64(i*2) {
				t.Errorf("%d: %v %v", i, res, err)
			}
		}(i)
	}
	wg.Wait()
	// 20 calls that each wait 30ms in Go overlap on one VM
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("not concurrent: %v", d)
	}
}

func TestMemoryCap(t *testing.T) {
	vm := newVM(t, VMConfig{MemoryLimit: 16 << 20})
	err := vm.Eval(`let a = []; for (;;) a.push("x".repeat(1000) + a.length)`, "bomb.js")
	if err == nil || !strings.Contains(err.Error(), "out of memory") {
		t.Fatalf("want OOM, got %v", err)
	}
	// The script's data keeps this VM full; the policy is to recycle it. Other VMs are unaffected.
	evalOK(t, newVM(t, VMConfig{MemoryLimit: 16 << 20}), `globalThis.ok = 1`)
}

func TestCPUBudget(t *testing.T) {
	vm := newVM(t, VMConfig{TaskBudget: 100 * time.Millisecond})
	start := time.Now()
	err := vm.Eval(`for (;;) {}`, "spin.js")
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("want interrupt, got %v after %v", err, time.Since(start))
	}
	evalOK(t, vm, `globalThis.ok = 1`)
}

func TestDeepRecursion(t *testing.T) {
	vm := newVM(t, VMConfig{})
	err := vm.Eval(`function f(n) { return f(n + 1) + 1 }; f(0)`, "rec.js")
	if err == nil || !strings.Contains(err.Error(), "stack") {
		t.Fatalf("want stack overflow, got %v", err)
	}
}

func TestGoCallTimeDoesNotCount(t *testing.T) {
	vm := newVM(t, VMConfig{TaskBudget: 100 * time.Millisecond})
	vm.Set("post", &Post{Title: "p"})
	evalOK(t, vm, `post.slow(150); post.slow(150)`)
}

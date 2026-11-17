package pages

import (
	"net/http/httptest"
	"testing"

	"github.com/pockethost/pocketone/ext/extkit/js"
	"github.com/pockethost/pocketone/ext/jsvm"
)

// TestRequestsDontAccumulate: what a request leaves in the app's VM is freed once the request is done. Each request's
// rt object has methods that are Go closures over the request; while the request kept rt, that was a cycle across the
// Go and JS heaps that neither GC frees, so every page render kept about 55 handles and its objects until the VM ran
// out of memory (pocketone.dev, 2026-11-17: QuickJS "out of memory" on every page after a few hours).
func TestRequestsDontAccumulate(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"+layout.ejs": `<html><body><%- slot %></body></html>`,
		"index.ejs":   `<h1><%= ctx.request.url %></h1><%- include('part.ejs', { n: 1 }) %>`,
		"part.ejs":    `<p><%= n %></p>`,
		"doc.md":      "---\ntitle: Doc\n---\n# Doc\n\nSome *text*.\n",
		"api/+api.js": `module.exports = { get(ctx) { return ctx.res.json({ ok: true }) } }`,
		"boom.ejs":    `<% throw new Error('boom') %>`,
		"+error.ejs":  `<p>error</p>`,
	})
	h, p := newServer(t, dir)
	var vm *jsvm.VM
	if err := js.Run(p.app, func(rt js.Runtime) error { vm = rt.Engine().(*jsvm.VM); return nil }); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/", "/doc", "/api", "/boom"}
	serve := func(n int) {
		for range n {
			for _, path := range paths {
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
			}
		}
	}
	// settled is the VM's handle count once dropped wrappers are freed (finalizers land asynchronously).
	settled := func() int64 {
		last := int64(-1)
		for range 20 {
			vm.GC()
			n := vm.Stats().Handles
			if n == last {
				return n
			}
			last = n
		}
		return last
	}

	serve(5) // warm: compiled templates, the driver and its caches
	before := settled()
	serve(50)
	after := settled()
	if after-before > 20 {
		t.Fatalf("handles %d → %d after 200 more requests: requests stay in the VM", before, after)
	}
}

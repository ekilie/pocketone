package jsvm

import (
	"strings"
	"sync"
	"testing"
)

// TypeScript hooks, modules and migrations: .ts and .mts are ES modules, .cts is CommonJS; error positions are in
// the .ts source.

var tsHooks = map[string]string{
	"pb_hooks/lib/store.mts": `
		export interface State {
			hits: number
		}
		export const state: State = { hits: 0 }
		export const bump = (): number => ++state.hits
	`,
	// a .ts module is an ES module, like .mts
	"pb_hooks/lib/math.ts": `
		export function add(a: number, b: number): number {
			return a + b
		}
	`,
	"pb_hooks/lib/util.cts": `
		const greet = (n: string): string => "hi " + n
		module.exports = { greet }
	`,
	"pb_hooks/a.pb.mts": `
		import { bump, state, type State } from "./lib/store.mjs" // names the compiled file: resolves to store.mts
		import { add } from "./lib/math.js"                       // resolves to math.ts
		import util from "./lib/util.cts"

		enum Color { Red = "red", Blue = "blue" }
		const label: string = "typed"
		const first: State = await Promise.resolve(state)

		routerAdd("GET", "/ts/bump", (e) => e.json(200, { n: bump() }))
		routerAdd("GET", "/ts/kinds", (e) => e.json(200, {
			sum: add(2, 3),
			hi: util.greet("ben"),
			color: Color.Blue,
			label,
			same: first === state,
		}))
	`,
	"pb_hooks/b.pb.ts": `
		import type { State } from "./lib/store.mts" // a .pb.ts hook is an ES module, like .pb.mts
		import { add as plus } from "./lib/math"

		function twice(n: number): number {
			return plus(n, n)
		}
		const mine: State = { hits: -1 } // private to this file
		routerAdd("GET", "/ts/script", (e) => e.json(200, { n: twice(21) }))
		routerAdd("GET", "/ts/script-bump", async (e) => {
			const m = await import("./lib/store.mts")
			return e.json(200, { n: m.bump() })
		})
	`,
	// a .cts hook is a script in the global scope; require() loads a .ts ES module (no top-level await) as in Node 22
	"pb_hooks/c.pb.cts": `
		const ok: boolean = true
		const { add } = require("./lib/math")
		const { state: required } = require("./lib/store.mts")
		routerAdd("GET", "/ts/cts", async (e: any) => {
			const { state: imported } = await import("./lib/store.mts")
			return e.json(200, { ok, sum: add(1, 2), same: required === imported, mine: typeof mine })
		})
	`,
}

func TestTSKinds(t *testing.T) {
	_, h, err := esmAppWith(t, tsHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	var k struct {
		Sum       int
		Hi, Color string
		Label     string
		Same      bool
	}
	if code := getJSON(t, h, "/ts/kinds", &k); code != 200 {
		t.Fatalf("/ts/kinds: %d", code)
	}
	if k.Sum != 5 || k.Hi != "hi ben" || k.Color != "blue" || k.Label != "typed" || !k.Same {
		t.Fatalf("/ts/kinds = %+v", k)
	}
	var s struct{ N int }
	if getJSON(t, h, "/ts/script", &s); s.N != 42 {
		t.Fatalf(".pb.ts script: %+v", s)
	}
	var c struct {
		OK, Same bool
		Sum      int
		Mine     string
	}
	if getJSON(t, h, "/ts/cts", &c); !c.OK || c.Sum != 3 || !c.Same {
		t.Fatalf(".pb.cts script with require() of ES modules: %+v", c)
	}
	if c.Mine != "undefined" {
		t.Fatalf("b.pb.ts's top-level const leaked into the global scope: typeof mine = %q", c.Mine)
	}
}

// One TypeScript module instance serves every file and request.
func TestTSStateSharedByConcurrentRequests(t *testing.T) {
	_, h, err := esmAppWith(t, tsHooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	const per = 200
	seen := make(chan int, 2*per)
	var wg sync.WaitGroup
	for i := 0; i < per; i++ {
		for _, p := range []string{"/ts/bump", "/ts/script-bump"} {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				var r struct{ N int }
				if code := getJSON(t, h, p, &r); code != 200 {
					t.Errorf("%s: %d", p, code)
				}
				seen <- r.N
			}(p)
		}
	}
	wg.Wait()
	close(seen)
	distinct := map[int]bool{}
	for n := range seen {
		if distinct[n] {
			t.Fatalf("two requests got %d", n)
		}
		distinct[n] = true
	}
	if len(distinct) != 2*per {
		t.Fatalf("%d distinct counts, want %d", len(distinct), 2*per)
	}
}

// Errors point at the TypeScript source: the types above the throw are stripped, so the generated line differs.
func TestTSErrorPositions(t *testing.T) {
	typesAbove := "interface A {\n\tx: number\n\ty: string\n}\ntype B = A & {\n\tz: boolean\n}\n"
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"hook script (.cts)", map[string]string{"pb_hooks/z.pb.cts": typesAbove + "const n: number = 1\nthrow new Error(\"script boom \" + n)"}, "pb_hooks/z.pb.cts:9:"},
		{"hook module (.ts)", map[string]string{"pb_hooks/z.pb.ts": typesAbove + "const n: number = 1\nthrow new Error(\"module boom \" + n)"}, "pb_hooks/z.pb.ts:9:"},
		{"hook module (.mts)", map[string]string{"pb_hooks/z.pb.mts": typesAbove + "const n: number = 1\nthrow new Error(\"module boom \" + n)"}, "pb_hooks/z.pb.mts:9:"},
		{"required module", map[string]string{
			"pb_hooks/lib/bad.ts": typesAbove + "export function boom(): never {\n\tthrow new Error(\"lib boom\")\n}",
			"pb_hooks/z.pb.cts":   `require("./lib/bad").boom()`,
		}, "lib/bad.ts:9:"},
		{"require() of a module with top-level await", map[string]string{
			"pb_hooks/lib/slow.ts": "await sleep.async(1)\nexport const x = 1",
			"pb_hooks/z.pb.cts":    `require("./lib/slow")`,
		}, "uses top-level await: use await import()"},
		{"syntax error", map[string]string{"pb_hooks/z.pb.ts": "const x: = 1"}, "pb_hooks/z.pb.ts:1:"},
		{"migration script (.cts)", map[string]string{"pb_migrations/1700000000_bad.cts": typesAbove + "const n: number = 1\nthrow new Error(\"migration boom\")"}, "pb_migrations/1700000000_bad.cts:9:"},
		{"migration module (.ts)", map[string]string{"pb_migrations/1700000000_bad.ts": typesAbove + "const n: number = 1\nthrow new Error(\"migration boom\")"}, "pb_migrations/1700000000_bad.ts:9:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := esmAppWith(t, nil, c.files)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an error at %q, got %v", c.want, err)
			}
		})
	}
}

// TypeScript migrations: .ts and .mts as modules with imports, .cts as a script in its own function scope.
func TestTSMigrations(t *testing.T) {
	app, _, err := esmAppWith(t, nil, map[string]string{
		"pb_migrations/lib/names.mts": `export const table: string = "ts_migrated"`,
		"pb_migrations/1700000000_create.mts": `
			import { table } from "./lib/names.mjs"
			await sleep.async(1)
			migrate((app) => {
				app.save(new Collection({ name: table, type: "base", fields: [{ name: "title", type: "text" }] }))
			})
		`,
		"pb_migrations/1700000001_more.cts": `
			interface Field { name: string; type: string }
			const fields: Field[] = [{ name: "body", type: "text" }]
			migrate((app) => {
				app.save(new Collection({ name: "ts_migrated2", type: "base", fields }))
			})
		`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.RunAppMigrations(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ts_migrated", "ts_migrated2"} {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

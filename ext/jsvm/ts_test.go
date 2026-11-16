package jsvm

import (
	"strings"
	"sync"
	"testing"
)

// TypeScript hooks, modules and migrations: .mts (ES module), .cts (CommonJS) and .ts (like .js), with error
// positions in the .ts source.

var tsHooks = map[string]string{
	"pb_hooks/lib/store.mts": `
		export interface State {
			hits: number
		}
		export const state: State = { hits: 0 }
		export const bump = (): number => ++state.hits
	`,
	// a .ts module is CommonJS (like .js), but may still be written with import/export
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
		import type { State } from "./lib/store.mts" // type-only: gone after stripping, so this stays a script
		const { add: plus } = require("./lib/math")

		function twice(n: number): number {
			return plus(n, n)
		}
		routerAdd("GET", "/ts/script", (e) => e.json(200, { n: twice(21) }))
		routerAdd("GET", "/ts/script-bump", async (e) => {
			const m = await import("./lib/store.mts")
			return e.json(200, { n: m.bump() })
		})
	`,
	"pb_hooks/c.pb.cts": `
		const ok: boolean = true
		routerAdd("GET", "/ts/cts", (e: any) => e.json(200, { ok }))
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
	var c struct{ OK bool }
	if getJSON(t, h, "/ts/cts", &c); !c.OK {
		t.Fatal(".pb.cts script didn't run")
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
		{"hook script", map[string]string{"pb_hooks/z.pb.ts": typesAbove + "const n: number = 1\nthrow new Error(\"script boom \" + n)"}, "pb_hooks/z.pb.ts:9:"},
		{"hook module", map[string]string{"pb_hooks/z.pb.mts": typesAbove + "const n: number = 1\nthrow new Error(\"module boom \" + n)"}, "pb_hooks/z.pb.mts:9:"},
		{"required module", map[string]string{
			"pb_hooks/lib/bad.ts": typesAbove + "export function boom(): never {\n\tthrow new Error(\"lib boom\")\n}",
			"pb_hooks/z.pb.ts":    `require("./lib/bad").boom()`,
		}, "lib/bad.ts:9:"},
		{"syntax error", map[string]string{"pb_hooks/z.pb.ts": "const x: = 1"}, "pb_hooks/z.pb.ts:1:"},
		{"migration script", map[string]string{"pb_migrations/1700000000_bad.ts": typesAbove + "const n: number = 1\nthrow new Error(\"migration boom\")"}, "pb_migrations/1700000000_bad.ts:9:"},
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

// TypeScript migrations: .ts as a script in its own scope, .mts as a module with imports.
func TestTSMigrations(t *testing.T) {
	app, _, err := esmAppWith(t, nil, map[string]string{
		"pb_migrations/lib/names.mts": `export const table: string = "ts_migrated"`,
		"pb_migrations/1700000000_create.mts": `
			import { table } from "./lib/names.mjs"
			migrate((app) => {
				app.save(new Collection({ name: table, type: "base", fields: [{ name: "title", type: "text" }] }))
			})
		`,
		"pb_migrations/1700000001_more.ts": `
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

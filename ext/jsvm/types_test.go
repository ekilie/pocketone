package jsvm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
)

// The embedded types.d.ts declares every hook of this repo's core: a core hook added without `go generate` fails here.
func TestTypesCoverCoreHooks(t *testing.T) {
	appType := reflect.TypeOf(struct{ core.App }{})
	for i := 0; i < appType.NumMethod(); i++ {
		m := appType.Method(i)
		if !strings.HasPrefix(m.Name, "On") || m.Name == "OnServe" {
			continue
		}
		decl := "declare function " + jsName(m.Name) + "("
		if !strings.Contains(string(typesFile), decl) {
			t.Errorf("types.d.ts has no %q: run go generate in ext/jsvm", decl)
		}
	}
}

// types.d.ts ends with the current pocketone.d.ts: an edit to it without `go generate` fails here.
func TestTypesIncludePocketOne(t *testing.T) {
	src, err := os.ReadFile("pocketone.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(typesFile), string(src)) {
		t.Fatal("types.d.ts doesn't end with pocketone.d.ts: run go generate in ext/jsvm")
	}
}

type typerExt struct{ decl string }

func (x typerExt) TypeScript() string { return x.decl }

// types.d.ts is the engine's declarations plus every extension's (js.Typer).
func TestTypesFileWithExtensions(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	dir := t.TempDir()
	ext := typerExt{"declare var $demo: { ping(): string };"}
	if err := Register(app, Config{HooksDir: filepath.Join(dir, "pb_hooks"), TypesDir: dir, Extensions: []any{ext}}); err != nil {
		t.Fatal(err)
	}
	if err := app.Bootstrap(); err != nil { // the file is written after bootstrap
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "types.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), string(typesFile)) || !strings.Contains(string(got), ext.decl) {
		t.Fatal("types.d.ts should be the engine's declarations followed by the extension's")
	}
}

// tsc type-checks hooks written for this engine against the declarations: ES module and CommonJS hooks, .async,
// top-level await, require, Buffer, process, import.meta, a PocketOne core hook and an extension's global. Skipped
// when tsc isn't installed.
func TestTypesCheckWithTSC(t *testing.T) {
	ext := typerExt{"declare var $demo: { ping(): string };"}
	files := map[string]string{
		"pb_hooks/lib/store.ts": `
			export const state = { hits: 0 }
			export const bump = (): number => ++state.hits
		`,
		"pb_hooks/main.pb.ts": `
			import { bump } from "./lib/store.ts"
			const posts = await $app.findCollectionByNameOrId.async("posts")
			await sleep.async(10)
			routerAdd("GET", "/hello/{id}", async (e) => {
				const rec = await $app.findRecordById.async("posts", e.request!.pathValue("id"))
				const title: string = rec.getString("title")
				return e.json(200, { title, n: bump(), url: import.meta.url, col: posts.name, demo: $demo.ping() })
			})
			onFilesystemCreate((e) => { e.next() })
		`,
		"pb_hooks/legacy.pb.cts": `
			const lib = require("./lib/legacy.cjs")
			const b64: string = Buffer.from("hi").toString("base64")
			const home: string | undefined = process.env.HOME
			console.log(lib, b64, home)
			onRecordCreate((e) => { e.next() }, "posts")
		`,
		"pb_migrations/1700000000_init.ts": `
			migrate((app) => {
				app.save(new Collection({ name: "notes", type: "base" }))
			})
		`,
	}
	out, err := runTSC(t, ext, files)
	if err != nil {
		t.Fatalf("tsc: %v\n%s", err, out)
	}

	// .async is typed, not any: its promise isn't a Record
	files["pb_hooks/wrong.pb.ts"] = `
		const r: core.Record = $app.findRecordById.async("posts", "x")
	`
	out, err = runTSC(t, ext, files)
	if err == nil || !strings.Contains(out, "Promise<") {
		t.Fatalf("tsc should reject a Promise used as a Record:\n%s", out)
	}
}

func runTSC(t *testing.T, ext typerExt, files map[string]string) (string, error) {
	t.Helper()
	tsc, err := exec.LookPath("tsc")
	if err != nil {
		t.Skip("tsc isn't installed")
	}
	dir := t.TempDir()
	p := &plugin{config: Config{Extensions: []any{ext}, TypesDir: dir,
		HooksDir: filepath.Join(dir, "pb_hooks"), MigrationsDir: filepath.Join(dir, "pb_migrations")}}
	all := map[string]string{
		"types.d.ts": string(p.typesContent()),
		// the tsconfig.json the engine writes (skipLibCheck: PocketBase's generated declarations have two type
		// errors of their own)
		"tsconfig.json": string(p.tsconfigContent()),
	}
	for k, v := range files {
		all[k] = v
	}
	writeFiles(t, dir, all)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tsc, "-p", dir)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"pocketone.dev/ext/jsvm"
)

// appTypes is the types.d.ts jsvm writes for an app with these extensions.
func appTypes(t *testing.T, exts ...any) string {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	dir := t.TempDir()
	if err := jsvm.Register(app, jsvm.Config{HooksDir: filepath.Join(dir, "pb_hooks"), TypesDir: dir, Extensions: exts}); err != nil {
		t.Fatal(err)
	}
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "types.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// has reports whether ns declares member name with a usable type (anything but never).
func has(decl, ns, name string) bool { usable, _ := declared(decl, ns, name); return usable }

// declared reports how ns declares member name, as the generator writes members: with a usable type, as never.
func declared(decl, ns, name string) (usable, never bool) {
	re := regexp.MustCompile(`^  (?:let |const )?` + regexp.QuoteMeta(name) + `\??\s*[:(<]`)
	in := false
	for _, line := range strings.Split(decl, "\n") {
		switch {
		case strings.HasPrefix(line, "declare namespace "+ns+" {"):
			in = true
		case line == "}":
			in = false
		case in && re.MatchString(line):
			if strings.HasSuffix(line, ": never") {
				never = true
			} else {
				usable = true
			}
		}
	}
	return
}

// What the sandbox disables or hides is gone from the app's types; what it keeps (confined) stays.
func TestFilterTypes(t *testing.T) {
	plain := appTypes(t)
	got := appTypes(t, &Sandbox{Dir: t.TempDir()})

	gone := map[string][]string{
		"$os":         {"exec", "cmd", "exit"},
		"$filesystem": {"local"},
		"core":        {"restart", "bootstrap", "unsafeWithoutHooks", "onServe", "onTerminate", "cron", "server", "installerFunc"},
		"hook":        {"bind", "bindFunc", "unbind", "trigger"},
	}
	for ns, names := range gone {
		for _, n := range names {
			if !has(plain, ns, n) {
				t.Errorf("%s.%s: the unfiltered types should declare it (has the generator's format changed?)", ns, n)
			}
			if usable, never := declared(got, ns, n); usable || !never {
				t.Errorf("%s.%s: under the sandbox it should be declared only as never (usable %v, never %v)", ns, n, usable, never)
			}
		}
	}
	// every name the sandbox hides is gone, wherever the generator declared it
	for ns, names := range hiddenMembers() {
		for n := range names {
			if has(got, ns, n) {
				t.Errorf("%s.%s: hidden by the sandbox but still declared", ns, n)
			}
		}
	}
	kept := map[string][]string{
		"$os":         {"readFile", "getenv", "args", "mkdirAll"},
		"$filesystem": {"fileFromPath", "s3"},
		"core":        {"findRecordById", "save", "db", "onRecordCreate"},
		"router":      {"bindFunc"}, // a router's bindFunc is not hook.Hook's
	}
	for ns, names := range kept {
		for _, n := range names {
			if has(plain, ns, n) && !has(got, ns, n) {
				t.Errorf("%s.%s: the sandbox keeps it, so its declaration should stay", ns, n)
			}
		}
	}
	if strings.Contains(got, "(sql.DB)") {
		t.Error("a method returning *sql.DB is hidden, so its declaration should go")
	}
	if len(got) < len(plain)*95/100 {
		t.Errorf("filtering removed too much: %d of %d bytes left", len(got), len(plain))
	}
}

// tsc accepts hooks the sandbox runs and rejects calls it refuses. Skipped when tsc isn't installed.
func TestFilterTypesCheckWithTSC(t *testing.T) {
	tscPath, err := exec.LookPath("tsc")
	if err != nil {
		t.Skip("tsc isn't installed")
	}
	types := appTypes(t, &Sandbox{Dir: t.TempDir()})
	run := func(hook string) (string, error) {
		dir := t.TempDir()
		for name, content := range map[string]string{
			"pb_data/types.d.ts": types,
			"tsconfig.json": `{"compilerOptions": {"target": "ES2022", "module": "ESNext", "moduleResolution": "Bundler",
				"lib": ["ES2022"], "strict": true, "noEmit": true, "skipLibCheck": true},
				"include": ["pb_data/types.d.ts", "pb_hooks/**/*"]}`,
			"pb_hooks/main.pb.ts": hook,
		} {
			p := filepath.Join(dir, name)
			os.MkdirAll(filepath.Dir(p), 0o755)
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, tscPath, "-p", dir).CombinedOutput()
		return string(out), err
	}

	if out, err := run(`
		routerAdd("GET", "/hi", (e) => {
			const text = toString($os.readFile("notes.txt"))
			const rec = $app.findRecordById("posts", "x")
			return e.json(200, { text, key: $os.getenv("APP_KEY"), id: rec.id })
		})
		onRecordCreate((e) => { e.next() }, "posts")
	`); err != nil {
		t.Fatalf("tsc should accept what the sandbox runs: %v\n%s", err, out)
	}
	for _, bad := range []string{`$os.exec("ls")`, `$os.cmd("ls")`, `$app.restart()`, `$app.unsafeWithoutHooks()`,
		`$filesystem.local("/")`, `const h = null! as hook.Hook<core.RecordEvent>; h.bindFunc((e: core.RecordEvent) => e.next())`} {
		out, err := run(bad)
		if err == nil || strings.Count(out, "error TS") != 1 || !strings.Contains(out, "TS2349") {
			t.Errorf("tsc should reject %s under the sandbox:\n%s", bad, out)
		}
	}
}

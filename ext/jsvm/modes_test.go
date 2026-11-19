package jsvm_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// pb_hooks, live, as a single app and as an instance on a host: a script (.pb.js), an ES module (.pb.mjs) and
// TypeScript (.pb.ts) side by side in the app's one VM, with top-level state that lasts across requests. An edit
// restarts the app (a single app's process, or the instance on its host) into the new code; a file that fails to load
// is logged with its position and skipped while the rest serves (dev mode), and serves again once fixed. A single app
// with --hooksWatch=false refuses to start with the same broken file.
func TestModesHooks(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{{ID: "app", Files: map[string]string{
		"pb_hooks/a.pb.js":  hookA("v1"),
		"pb_hooks/b.pb.mjs": hookB,
		"pb_hooks/c.pb.ts":  hookC,
	}}}}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")

		modetest.Step(t, "three formats, one VM")
		wantBody(t, s, "/a", "a v1")
		wantBody(t, s, "/b", "b sees: shared from a")
		wantBody(t, s, "/c", "c: 42")
		wantBody(t, s, "/hits", "1")
		wantBody(t, s, "/hits", "2")

		modetest.Step(t, "a good edit: the app restarts into the new code")
		s.Write("pb_hooks/a.pb.js", hookA("v2"))
		s.WaitRestarted("edit a.pb.js", 20*time.Second)
		wantBody(t, s, "/a", "a v2")
		wantBody(t, s, "/hits", "1") // a new app: state starts over

		modetest.Step(t, "a broken edit: logged with its position, skipped, the rest serves")
		s.Write("pb_hooks/b.pb.mjs", "import { x } from './nowhere.mjs'\n\nrouterAdd('GET', '/b', (e) => e.string(200, 'never'))\n")
		s.WaitRestarted("break b.pb.mjs", 20*time.Second)
		wantBody(t, s, "/a", "a v2")
		// a single app logs PocketBase's own line; Pocker writes its own to the instance's exec.log
		loadErr := map[modetest.Mode]string{modetest.Single: "failed to execute b.pb.mjs", modetest.Host: "pb_hooks/b.pb.mjs failed to load; skipped it (dev mode)"}[w.Mode]
		modetest.WaitFor(t, "the load error for b.pb.mjs in the log", 5*time.Second, func() (bool, string) {
			log := s.Logs()
			return strings.Contains(log, loadErr) && strings.Contains(log, "nowhere.mjs"), modetest.Tail(log, 5)
		})
		wantBody(t, s, "/c", "c: 42")
		if code, b := s.Raw(http.MethodGet, "/b", "", nil); code != 404 {
			t.Fatalf("GET /b with b.pb.mjs broken: %d %s, want 404", code, b)
		}

		modetest.Step(t, "fixed: it serves again")
		s.Write("pb_hooks/b.pb.mjs", hookB)
		s.WaitRestarted("fix b.pb.mjs", 20*time.Second)
		wantBody(t, s, "/b", "b sees: shared from a")

		if w.Mode != modetest.Single {
			return
		}
		modetest.Step(t, "--hooksWatch=false: a broken file stops startup, naming it")
		w.Stop("app")
		s.Write("pb_hooks/c.pb.ts", "const n: number = 1\nthrow new Error('c is broken')\n")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		out, err := modetest.Command(ctx, t, s.Dir, s.Proc.Env, "serve", "--dir=pb_data", "--http="+modetest.FreeAddr(t), "--hooksWatch=false").CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("serve with a broken hook kept running instead of stopping:\n%s", modetest.Tail(string(out), 30))
		}
		if err == nil || !strings.Contains(string(out), "c.pb.ts") || !strings.Contains(string(out), "c is broken") {
			t.Fatalf("serve --hooksWatch=false with a broken c.pb.ts: %v\n%s", err, modetest.Tail(string(out), 30))
		}
		if !strings.Contains(string(out), "c.pb.ts:2:") {
			t.Fatalf("the startup error doesn't name the line (c.pb.ts:2:):\n%s", modetest.Tail(string(out), 30))
		}
	})
}

// hookA is a script: its top-level names are the app's globals.
func hookA(version string) string {
	return `let hits = 0
function shared() { return "shared from a" }

routerAdd("GET", "/a", (e) => e.string(200, "a ` + version + `"))
routerAdd("GET", "/hits", (e) => e.string(200, String(++hits)))
`
}

// hookB is an ES module that calls a script's global at request time.
const hookB = `const label = "b sees: "

routerAdd("GET", "/b", (e) => e.string(200, label + globalThis.shared()))
`

// hookC is TypeScript, types stripped on load.
const hookC = `type Answer = { value: number }
const answer: Answer = { value: 6 * 7 }

routerAdd("GET", "/c", (e) => e.string(200, "c: " + answer.value))
`

func wantBody(t *testing.T, s *modetest.Site, path, want string) {
	t.Helper()
	code, b := s.Raw(http.MethodGet, path, "", nil)
	if code != 200 || string(b) != want {
		t.Fatalf("GET %s: %d %q, want %q\n%s", path, code, b, want, modetest.Tail(s.Logs(), 30))
	}
}

// pb_migrations, as a single app and as an instance on a host, in each format the engine reads: a CommonJS script
// (.js), an ES module that imports a helper (.mjs) and TypeScript (.ts). The first serve applies them in order; a
// restart applies nothing again; `migrate down` in the app's folder reverts the newest and the next serve applies them
// once more. The extensions' own tables (tokens' _tokens_tokens) are there without leaving a file in pb_migrations, while a
// collection made over the API writes one (automigrate, in both modes, as on a PocketHost docker instance) without
// restarting the app, so the empty result means something.
func TestModesMigrations(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{{ID: "app", Files: migrations}}}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		ours := []string{"1700000000_books.js", "1700000001_pages.mjs", "1700000002_seed.ts"}

		modetest.Step(t, "first serve: every migration applied, in order")
		if got := books(t, s); got != "Dune:412" {
			t.Fatalf("books after the first serve: %q", got)
		}
		if code, b := s.Raw(http.MethodGet, "/api/collections/_tokens_tokens", s.JWT(), nil); code != 200 {
			t.Fatalf("tokens' _tokens_tokens collection: %d %s", code, b)
		}
		if got := migrationFiles(t, s.Dir); strings.Join(got, ",") != strings.Join(ours, ",") {
			t.Fatalf("pb_migrations after the first serve: %v, want only ours %v", got, ours)
		}

		modetest.Step(t, "a collection made over the API writes its own migration (automigrate)")
		s.MustJSON(http.MethodPost, "/api/collections", s.JWT(), map[string]any{
			"name": "shelves", "type": "base", "fields": []map[string]any{{"name": "label", "type": "text"}},
		}, 200, nil)
		var shelvesFile string
		modetest.WaitFor(t, "the shelves migration file", 5*time.Second, func() (bool, string) {
			for _, f := range migrationFiles(t, s.Dir) {
				if strings.HasSuffix(f, "_created_shelves.js") {
					shelvesFile = f
					return true, f
				}
			}
			return false, strings.Join(migrationFiles(t, s.Dir), ",")
		})
		if s.Restarted() {
			t.Fatalf("writing the migration restarted the app (%d restarts)", s.Restarts())
		}

		modetest.Step(t, "restart: nothing applied twice")
		w.Restart("app")
		if got := books(t, s); got != "Dune:412" {
			t.Fatalf("books after a restart: %q", got)
		}

		modetest.Step(t, "migrate down 2: the shelves and seed migrations revert, newest first")
		w.Stop("app")
		down := modetest.Command(context.Background(), t, s.Dir, s.Proc.Env, "migrate", "down", "2", "--dir=pb_data", "--dev=false")
		down.Stdin = strings.NewReader("y\n")
		out, err := down.CombinedOutput()
		if err != nil {
			t.Fatalf("migrate down 2: %v\n%s", err, out)
		}
		rs, rd := strings.Index(string(out), "Reverted "+shelvesFile), strings.Index(string(out), "Reverted 1700000002_seed.ts")
		if rs < 0 || rd < 0 || rs > rd {
			t.Fatalf("migrate down 2 didn't revert %s then the seed:\n%s", shelvesFile, out)
		}

		modetest.Step(t, "serve again: both applied once more")
		w.Start("app")
		if got := books(t, s); got != "Dune:412" {
			t.Fatalf("books after migrate down and serve: %q", got)
		}
		if code, b := s.Raw(http.MethodGet, "/api/collections/shelves", s.JWT(), nil); code != 200 {
			t.Fatalf("shelves after migrate down and serve: %d %s", code, b)
		}
	})
}

var migrations = map[string]string{
	"pb_migrations/1700000000_books.js": `
migrate((app) => {
	app.save(new Collection({ name: "books", type: "base", listRule: "", fields: [{ name: "title", type: "text" }] }))
}, (app) => {
	app.delete(app.findCollectionByNameOrId("books"))
})
`,
	"pb_migrations/lib/fields.mjs": `export const pagesField = { name: "pages", type: "number" }
`,
	"pb_migrations/1700000001_pages.mjs": `
import { pagesField } from "./lib/fields.mjs"

migrate((app) => {
	const books = app.findCollectionByNameOrId("books")
	books.fields.add(new NumberField(pagesField))
	app.save(books)
}, (app) => {
	const books = app.findCollectionByNameOrId("books")
	books.fields.removeByName(pagesField.name)
	app.save(books)
})
`,
	"pb_migrations/1700000002_seed.ts": `
type Book = { title: string; pages: number }
const books: Book[] = [{ title: "Dune", pages: 412 }]

migrate((app) => {
	const c = app.findCollectionByNameOrId("books")
	for (const b of books) {
		const r = new Record(c)
		r.set("title", b.title)
		r.set("pages", b.pages)
		app.save(r)
	}
}, (app) => {
	for (const b of books) {
		app.delete(app.findFirstRecordByData("books", "title", b.title))
	}
})
`,
}

// books lists title:pages for every book, sorted.
func books(t *testing.T, s *modetest.Site) string {
	t.Helper()
	var out struct {
		Items []struct {
			Title string  `json:"title"`
			Pages float64 `json:"pages"`
		} `json:"items"`
	}
	s.MustJSON(http.MethodGet, "/api/collections/books/records?perPage=200", "", nil, 200, &out)
	var got []string
	for _, b := range out.Items {
		got = append(got, b.Title+":"+strconv.FormatFloat(b.Pages, 'f', -1, 64))
	}
	sort.Strings(got)
	return strings.Join(got, ",")
}

// migrationFiles lists the files directly in the app's pb_migrations, sorted.
func migrationFiles(t *testing.T, appDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(appDir, "pb_migrations"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

package main

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
)

// An app's pb_migrations on the real binary, in each format the engine reads: a CommonJS script (.js), an ES module
// that imports a helper (.mjs) and TypeScript (.ts). The first serve applies them in order; a restart applies nothing
// again; `migrate down` reverts the newest and the next serve applies them once more. The extensions' own tables
// (tokens' _tokens) are there without leaving a file in pb_migrations, while a collection made over the API in dev mode
// does write one (automigrate), so the empty result means something.
func TestE2EMigrations(t *testing.T) {
	appDir := t.TempDir()
	ours := map[string]string{
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
	for rel, content := range ours {
		write(t, filepath.Join(appDir, rel), content)
	}
	ourFiles := migrationFiles(t, appDir)

	step(t, "first serve: every migration applied, in order")
	s := startSite(t, siteOptions{Dir: appDir})
	if got := books(t, s); got != "Dune:412" {
		t.Fatalf("books after the first serve: %q", got)
	}
	if code, b := s.raw(http.MethodGet, "/api/collections/_tokens", s.JWT(), nil); code != 200 {
		t.Fatalf("tokens' _tokens collection: %d %s", code, b)
	}
	if got := migrationFiles(t, appDir); strings.Join(got, ",") != strings.Join(ourFiles, ",") {
		t.Fatalf("pb_migrations after the first serve: %v, want only ours %v", got, ourFiles)
	}

	step(t, "a collection made over the API writes its own migration (dev mode)")
	s.mustJSON(http.MethodPost, "/api/collections", s.JWT(), map[string]any{
		"name": "shelves", "type": "base", "fields": []map[string]any{{"name": "label", "type": "text"}},
	}, 200, nil)
	var shelvesFile string
	waitFor(t, context.Background(), "the shelves migration file", 5*time.Second, func() (bool, string) {
		for _, f := range migrationFiles(t, appDir) {
			if strings.HasSuffix(f, "_created_shelves.js") {
				shelvesFile = f
				return true, f
			}
		}
		return false, strings.Join(migrationFiles(t, appDir), ",")
	})

	step(t, "restart: nothing applied twice")
	s.stop()
	s = startSite(t, siteOptions{Dir: appDir})
	if got := books(t, s); got != "Dune:412" {
		t.Fatalf("books after a restart: %q", got)
	}

	step(t, "migrate down 2: the shelves and seed migrations revert, newest first")
	s.stop()
	down := runBin(context.Background(), pocketoneBin(t), appDir, s.Env, "migrate", "down", "2", "--dir=pb_data", "--dev=false")
	down.Stdin = strings.NewReader("y\n")
	out, err := down.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate down 2: %v\n%s", err, out)
	}
	rs, rd := strings.Index(string(out), "Reverted "+shelvesFile), strings.Index(string(out), "Reverted 1700000002_seed.ts")
	if rs < 0 || rd < 0 || rs > rd {
		t.Fatalf("migrate down 2 didn't revert %s then the seed:\n%s", shelvesFile, out)
	}

	step(t, "serve again: both applied once more")
	s = startSite(t, siteOptions{Dir: appDir})
	if got := books(t, s); got != "Dune:412" {
		t.Fatalf("books after migrate down and serve: %q", got)
	}
	if code, b := s.raw(http.MethodGet, "/api/collections/shelves", s.JWT(), nil); code != 200 {
		t.Fatalf("shelves after migrate down and serve: %d %s", code, b)
	}
}

// books lists title:pages for every book, sorted.
func books(t *testing.T, s *site) string {
	t.Helper()
	var out struct {
		Items []struct {
			Title string  `json:"title"`
			Pages float64 `json:"pages"`
		} `json:"items"`
	}
	s.mustJSON(http.MethodGet, "/api/collections/books/records?perPage=200", "", nil, 200, &out)
	var got []string
	for _, b := range out.Items {
		got = append(got, b.Title+":"+strconv.FormatFloat(b.Pages, 'f', -1, 64))
	}
	sort.Strings(got)
	return strings.Join(got, ",")
}

// migrationFiles lists the files directly in pb_migrations, sorted.
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

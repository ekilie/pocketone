package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	deployv1 "pocketone.dev/ext/contracts/deploy/v1"
	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"slices"
	"strings"
	"sync"
	"testing"
)

const testPAT = "ph_pat_abcdefghij12345_BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"

// fakeTok is a token the fake control plane knows.
type fakeTok struct {
	UserID, Label string
	Scopes        []string
	InstanceIDs   []string // nil: every instance the user owns
}

type fakeBackend struct {
	mu    sync.Mutex
	insts map[string]Instance // by host
	pats  map[string]fakeTok
	down  bool
}

func (b *fakeBackend) Lookup(r *http.Request) (Instance, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i, ok := b.insts[hostOf(r)]
	return i, ok
}

func (b *fakeBackend) Authenticate(r *http.Request, inst Instance) (Auth, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	h := r.Header.Get("Authorization")
	tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	if tok == "" {
		return Auth{}, &AuthError{401, "Missing token. Set POCKETHOST_TOKEN or run 'pockethost token set'."}
	}
	if strings.HasPrefix(tok, "jwt-") {
		if tok != "jwt-user1" {
			return Auth{}, &AuthError{401, "Your login has expired. Run 'pockethost login' or use a token."}
		}
		if inst.OwnerID != "user1" {
			return Auth{}, ErrNoAccess
		}
		return Auth{UserID: "user1", Scopes: ReadWrite, Via: "jwt"}, nil
	}
	if b.down {
		return Auth{}, errors.New("mothership unreachable")
	}
	p, ok := b.pats[tok]
	if !ok {
		return Auth{}, &AuthError{401, "Invalid or expired token."}
	}
	if p.UserID != inst.OwnerID || (p.InstanceIDs != nil && !slices.Contains(p.InstanceIDs, inst.ID)) {
		return Auth{}, ErrNoAccess
	}
	return Auth{UserID: p.UserID, Scopes: p.Scopes, Via: "pat", TokenLabel: p.Label}, nil
}

type fakeInstances struct {
	mu       sync.Mutex
	dirs     map[string]string
	running  map[string]bool
	stops    int
	starts   int
	startErr error
}

func (f *fakeInstances) Running(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running[id]
}

func (f *fakeInstances) InstanceDir(id string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.dirs[id]
	return d, ok
}

func (f *fakeInstances) WithInstanceStopped(_ context.Context, id string, fn func(string) error) error {
	f.mu.Lock()
	f.stops++
	f.running[id] = false
	dir := f.dirs[id]
	f.mu.Unlock()
	return fn(dir)
}

func (f *fakeInstances) StartInstance(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.startErr != nil {
		return f.startErr
	}
	f.running[id] = true
	return nil
}

// instanceManager is what a multisite host's manager offers (pocker's Manager, fakeInstances); managerFiles adapts
// it to instance.fs/v1 for these tests, as a host's provider would.
type instanceManager interface {
	Running(id string) bool
	InstanceDir(id string) (string, bool)
	WithInstanceStopped(ctx context.Context, id string, fn func(dir string) error) error
	StartInstance(ctx context.Context, id string) error
}

type managerFiles struct {
	m                instanceManager
	pbRoot, stateDir string
	mu               sync.Mutex
	fs               map[string]*fsv1.Local
}

func newManagerFiles(m instanceManager, pbRoot, stateDir string) *managerFiles {
	return &managerFiles{m: m, pbRoot: pbRoot, stateDir: stateDir, fs: map[string]*fsv1.Local{}}
}

func (f *managerFiles) Instance(_ context.Context, id string) (fsv1.FS, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if l, ok := f.fs[id]; ok {
		return l, nil
	}
	l := &fsv1.Local{
		Folder: func() string {
			if d, ok := f.m.InstanceDir(id); ok {
				return d
			}
			return filepath.Join(f.pbRoot, id)
		},
		State:     func(string) string { return filepath.Join(f.stateDir, id) },
		IsRunning: func() bool { return f.m.Running(id) },
		Stopped: func(ctx context.Context, fn func(context.Context) error) error {
			return f.m.WithInstanceStopped(ctx, id, func(string) error { return fn(ctx) })
		},
		Start: func(ctx context.Context) error { return f.m.StartInstance(ctx, id) },
	}
	f.fs[id] = l
	return l, nil
}

type apiFixture struct {
	t    *testing.T
	root string
	be   *fakeBackend
	mgr  *fakeInstances
	api  *API
	srv  *httptest.Server
}

func newAPIFixture(t *testing.T) *apiFixture {
	root := t.TempDir()
	inst := Instance{ID: "inst1", Subdomain: "myapp", OwnerID: "user1", Power: true}
	be := &fakeBackend{
		insts: map[string]Instance{"myapp.example.com": inst},
		pats:  map[string]fakeTok{testPAT: {UserID: "user1", Label: "CI", Scopes: []string{"read", "write"}}},
	}
	mgr := &fakeInstances{dirs: map[string]string{"inst1": filepath.Join(root, "instances", "inst1")}, running: map[string]bool{"inst1": true}}
	files := newManagerFiles(mgr, filepath.Join(root, "instances"), filepath.Join(root, "deploys"))
	api := New(Options{Prefix: Prefix, Backend: be, Files: func() fsv1.Files { return files },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(299) })
	srv := httptest.NewServer(api.Wrap(next))
	t.Cleanup(srv.Close)
	return &apiFixture{t, root, be, mgr, api, srv}
}

func (f *apiFixture) instFile(rel string) string {
	return filepath.Join(f.root, "instances", "inst1", rel)
}

func (f *apiFixture) put(rel, content string) {
	p := f.instFile(rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

func (f *apiFixture) read(rel string) string {
	b, _ := os.ReadFile(f.instFile(rel))
	return string(b)
}

type resp struct {
	status int
	body   map[string]any
	raw    []byte
	header http.Header
}

func (f *apiFixture) do(method, path string, body any, opts ...func(*http.Request)) resp {
	f.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rd = bytes.NewReader(b)
	default:
		j, _ := json.Marshal(b)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, f.srv.URL+path, rd)
	req.Host = "myapp.example.com"
	req.Header.Set("Authorization", "Bearer "+testPAT)
	for _, o := range opts {
		o(req)
	}
	res, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, raw: raw, header: res.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func withToken(tok string) func(*http.Request) {
	return func(r *http.Request) {
		if tok == "" {
			r.Header.Del("Authorization")
		} else {
			r.Header.Set("Authorization", "Bearer "+tok)
		}
	}
}

func withHost(h string) func(*http.Request) { return func(r *http.Request) { r.Host = h } }

func files(m map[string]string) (map[string]string, [][]byte) {
	out := map[string]string{}
	var blobs [][]byte
	for p, c := range m {
		out[p] = sha(c)
		blobs = append(blobs, []byte(c))
	}
	return out, blobs
}

func (f *apiFixture) deploy(content map[string]string, extra map[string]any) resp {
	f.t.Helper()
	m, blobs := files(content)
	body := map[string]any{"files": m}
	for k, v := range extra {
		body[k] = v
	}
	chk := f.do("POST", Prefix+"/check", body)
	if chk.status != 200 {
		f.t.Fatalf("check: %d %s", chk.status, chk.raw)
	}
	if missing := chk.body["missing"].([]any); len(missing) > 0 {
		var up [][]byte
		want := map[string]bool{}
		for _, h := range missing {
			want[h.(string)] = true
		}
		for _, b := range blobs {
			if want[sha(string(b))] {
				up = append(up, b)
				delete(want, sha(string(b)))
			}
		}
		if r := f.do("POST", Prefix+"/blobs", encodeBundle(up...)); r.status != 200 {
			f.t.Fatalf("blobs: %d %s", r.status, r.raw)
		}
	}
	return f.do("POST", Prefix+"/commit", body)
}

// Prefix is where the deploy API is served: deploy/v1's.
var Prefix = deployv1.Prefix

func num0(v any) int { f, _ := v.(float64); return int(f) }

func TestAPIEndToEnd(t *testing.T) {
	f := newAPIFixture(t)
	f.put("pb_data/data.db", "REMOTE DB")
	f.put("pb_hooks/old.pb.js", "old")

	if who := f.do("GET", Prefix+"/tree", nil); who.status != 200 {
		t.Fatalf("files: %d %s", who.status, who.raw)
	}

	r := f.deploy(map[string]string{"pb_hooks/main.pb.js": "routerAdd()", "pb_public/index.html": "<h1>v1</h1>", "package.json": "{}"}, nil)
	if r.status != 200 {
		t.Fatalf("commit: %d %s", r.status, r.raw)
	}
	if _, ok := r.body["deployId"]; ok || num0(r.body["writtenCount"]) != 3 || num0(r.body["deletedCount"]) != 1 ||
		r.body["wasRunning"] != false || r.body["restarted"] != false || r.body["nothingChanged"] != false ||
		r.body["restartNeeded"] != nil || f.mgr.stops != 0 {
		t.Fatalf("commit body (hooks reload by themselves, no restart): %s", r.raw)
	}
	if _, ok := r.body["startError"]; ok {
		t.Fatal("startError should be absent")
	}
	if f.read("pb_hooks/main.pb.js") != "routerAdd()" || f.read("pb_data/data.db") != "REMOTE DB" {
		t.Fatal("files")
	}
	if num0(r.body["totalBytes"]) != len("routerAdd()<h1>v1</h1>{}") {
		t.Fatalf("totalBytes: %s", r.raw)
	}
	if ms, _ := os.ReadDir(filepath.Join(f.root, "deploys", "inst1", "manifests")); len(ms) != 1 {
		t.Fatal("manifest not recorded")
	}

	// pb_public-only: no stop
	stops := f.mgr.stops
	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": "routerAdd()", "pb_public/index.html": "<h1>v2</h1>", "package.json": "{}"}, nil)
	if r.status != 200 || f.mgr.stops != stops || r.body["restarted"] != false || num0(r.body["writtenCount"]) != 1 {
		t.Fatalf("pb_public deploy: %s stops=%d", r.raw, f.mgr.stops)
	}

	// nothing changed
	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": "routerAdd()", "pb_public/index.html": "<h1>v2</h1>", "package.json": "{}"}, nil)
	if r.body["nothingChanged"] != true || num0(r.body["writtenCount"]) != 0 {
		t.Fatalf("no-op: %s", r.raw)
	}

	// a broken start is reported
	f.mgr.startErr = errors.New("hook syntax error")
	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": "broken(", "package.json": "{}", "pb_migrations/0.js": "m"}, map[string]any{"restart": true})
	if r.body["restarted"] != false || r.body["startError"] != "hook syntax error" {
		t.Fatalf("startError: %s", r.raw)
	}

	f.mgr.startErr = nil
	r = f.deploy(map[string]string{"pb_hooks/main.pb.js": "ok", "package.json": "{}"}, nil)

	// a migration waits for a restart: the commit says so and leaves the app alone...
	stops = f.mgr.stops
	f.mgr.running["inst1"] = true // started again after the broken start above
	files := map[string]string{"pb_hooks/main.pb.js": "routerAdd()", "pb_public/index.html": "<h1>v2</h1>", "package.json": "{}", "pb_migrations/1.js": "m"}
	r = f.deploy(files, nil)
	if r.status != 200 || f.mgr.stops != stops || r.body["restarted"] != false || r.body["restartNeeded"] != true {
		t.Fatalf("migration: %d %s", r.status, r.raw)
	}
	// ...until the client asks on the commit (or restarts through remote/v1)
	files["pb_migrations/2.js"] = "m2"
	r = f.deploy(files, map[string]any{"restart": true})
	if r.status != 200 || r.body["restarted"] != true || r.body["restartNeeded"] != nil || f.mgr.stops != stops+1 {
		t.Fatalf("commit with restart: %d %s", r.status, r.raw)
	}
	// "restart" without a migration or packages changes nothing
	files["pb_public/index.html"] = "<h1>v3</h1>"
	r = f.deploy(files, map[string]any{"restart": true})
	if r.status != 200 || r.body["restarted"] != false || f.mgr.stops != stops+1 {
		t.Fatalf("commit with restart, nothing to apply: %d %s", r.status, r.raw)
	}

}

func TestAPICommitBeforeUpload(t *testing.T) {
	f := newAPIFixture(t)
	m, _ := files(map[string]string{"pb_hooks/a.pb.js": "A"})
	r := f.do("POST", Prefix+"/commit", map[string]any{"files": m})
	if r.status != 409 || r.body["missing"].([]any)[0] != sha("A") || r.body["message"] != "1 file(s) must be uploaded first" {
		t.Fatalf("%d %s", r.status, r.raw)
	}
}

func TestAPIAuth(t *testing.T) {
	f := newAPIFixture(t)
	ro := "ph_pat_readonly0000000000_CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	other := "ph_pat_otheruser000000000_DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"
	scoped := "ph_pat_scoped00000000000_EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
	f.be.pats[ro] = fakeTok{UserID: "user1", Scopes: []string{"read"}}
	f.be.pats[other] = fakeTok{UserID: "user2", Scopes: []string{"read", "write"}}
	f.be.pats[scoped] = fakeTok{UserID: "user1", Scopes: []string{"read", "write"}, InstanceIDs: []string{"other"}}
	check := func(name string, r resp, status int, msg string) {
		t.Helper()
		if r.status != status || !strings.Contains(fmt.Sprint(r.body["message"]), msg) || num0(r.body["status"]) != status {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	body := map[string]any{"files": map[string]any{}}
	check("missing", f.do("POST", Prefix+"/check", body, withToken("")), 401, "Missing token. Set POCKETHOST_TOKEN")
	check("bad", f.do("POST", Prefix+"/check", body, withToken("ph_pat_nope")), 401, "Invalid or expired token.")
	check("other owner", f.do("POST", Prefix+"/check", body, withToken(other)), 404, "myapp.example.com not found")
	check("scope", f.do("POST", Prefix+"/check", body, withToken(scoped)), 404, "myapp.example.com not found")
	check("read-only", f.do("POST", Prefix+"/check", body, withToken(ro)), 403, "This token is read-only.")
	if r := f.do("GET", Prefix+"/tree", nil, withToken(ro)); r.status != 200 {
		t.Fatalf("read-only files: %s", r.raw)
	}
	if r := f.do("GET", Prefix+"/tree", nil, withToken("jwt-user1")); r.status != 200 {
		t.Fatalf("jwt: %s", r.raw)
	}
	check("bad jwt", f.do("GET", Prefix+"/tree", nil, withToken("jwt-nope")), 401, "Your login has expired")
	check("unknown host", f.do("GET", Prefix+"/tree", nil, withHost("nope.example.com")), 404, "nope.example.com not found")

	// an outage is a 500
	f.be.down = true
	check("outage", f.do("GET", Prefix+"/tree", nil, withToken("ph_pat_outage")), 500, "mothership unreachable")
	check("outage again", f.do("GET", Prefix+"/tree", nil, withToken("ph_pat_outage")), 500, "mothership unreachable")
	f.be.down = false

	// suspensions (owner first), other edge
	inst := f.be.insts["myapp.example.com"]
	inst.Suspension, inst.OwnerSuspension = "instance hold", "account hold"
	f.be.insts["myapp.example.com"] = inst
	check("owner suspension", f.do("GET", Prefix+"/tree", nil), 403, "account hold")
	inst.OwnerSuspension = ""
	f.be.insts["myapp.example.com"] = inst
	check("instance suspension", f.do("GET", Prefix+"/tree", nil), 403, "instance hold")
	inst.Suspension, inst.Elsewhere = "", "myapp.example.com is served by another PocketHost edge"
	f.be.insts["myapp.example.com"] = inst
	check("elsewhere", f.do("GET", Prefix+"/tree", nil), 421, "another PocketHost edge")

	// unknown deploy route is a JSON 404; other paths go to the instance
	check("route", f.do("GET", Prefix+"/nope", nil), 404, "Not found")
	if r := f.do("GET", "/api/health", nil); r.status != 299 {
		t.Fatalf("passthrough: %d", r.status)
	}
}

func TestAPIBadInput(t *testing.T) {
	f := newAPIFixture(t)
	if r := f.do("POST", Prefix+"/check", map[string]any{"files": map[string]any{"pb_data/data.db": sha("x")}}); r.status != 400 || !strings.Contains(r.body["message"].(string), "outside the deployable set") {
		t.Fatalf("%d %s", r.status, r.raw)
	}
	if r := f.do("POST", Prefix+"/blobs", []byte("junk")); r.status != 400 || !strings.HasPrefix(r.body["message"].(string), "Could not decompress bundle") {
		t.Fatalf("%d %s", r.status, r.raw)
	}
	if r := f.do("POST", Prefix+"/blobs", []byte{}); r.status != 400 || r.body["message"] != "Empty upload" {
		t.Fatalf("%d %s", r.status, r.raw)
	}
	if r := f.do("POST", Prefix+"/check", []byte("{nope")); r.status != 400 {
		t.Fatalf("%d %s", r.status, r.raw)
	}
	if r := f.do("POST", Prefix+"/check", bytes.Repeat([]byte(" "), maxJSONBytes+1)); r.status != 413 || r.body["message"] != "Request too large" {
		t.Fatalf("%d %s", r.status, r.raw)
	}

	if r := f.do("POST", Prefix+"/check", map[string]any{"files": map[string]any{"pb_hooks/node_modules/x.js": sha("x")}}); r.status != 400 {
		t.Fatalf("%d %s", r.status, r.raw)
	}
}

func TestAPIFailedApply(t *testing.T) {
	f := newAPIFixture(t)
	f.put("pb_hooks/a.pb.js", "A")
	f.put("pb_public/blocker/node_modules/x.js", "kept")
	r := f.deploy(map[string]string{"pb_hooks/a.pb.js": "A2", "pb_public/blocker": "X"}, nil)
	if r.status != 409 || f.read("pb_hooks/a.pb.js") != "A" {
		t.Fatalf("%d %s", r.status, r.raw)
	}
	if !strings.Contains(string(r.raw), "pb_public/blocker is a directory") {
		t.Fatalf("%s", r.raw)
	}
}

func TestAPINodeModules(t *testing.T) {
	f := newAPIFixture(t)
	f.put("node_modules/old/index.js", "old")
	r := f.deploy(map[string]string{"pb_hooks/main.pb.js": "h"}, map[string]any{"nodeModules": true, "restart": true})
	if r.status != 200 || num0(r.body["deletedCount"]) != 1 || r.body["restarted"] != true {
		t.Fatalf("%s", r.raw)
	}
	if _, err := os.Stat(f.instFile("node_modules/old")); err == nil {
		t.Fatal("old package kept")
	}
}

func TestAPIInstanceUnknownToManager(t *testing.T) {
	// e.g. an unverified owner: the instance isn't served, but its owner can still deploy files to it
	f := newAPIFixture(t)
	delete(f.mgr.dirs, "inst1")
	r := f.deploy(map[string]string{"pb_hooks/main.pb.js": "h"}, nil)
	if r.status != 200 || r.body["wasRunning"] != false || f.read("pb_hooks/main.pb.js") != "h" {
		t.Fatalf("%s", r.raw)
	}
}

func TestAPIFiles(t *testing.T) {
	f := newAPIFixture(t)
	f.put("pb_hooks/main.pb.js", "hook")
	f.put("pb_data/data.db", "db")
	f.put("notes.txt", "hidden")
	f.put("pb_public/x.ph-upload", "partial")

	r := f.do("GET", Prefix+"/files?recursive=1", nil)
	var paths []string
	for _, e := range r.body["entries"].([]any) {
		paths = append(paths, e.(map[string]any)["path"].(string))
	}
	if strings.Join(paths, ",") != "pb_hooks,pb_hooks/main.pb.js,pb_public" {
		t.Fatalf("list: %v", paths)
	}
	r = f.do("GET", Prefix+"/files/raw?path=pb_hooks/main.pb.js", nil)
	if r.status != 200 || string(r.raw) != "hook" || r.header.Get("X-File-Mtime") == "" {
		t.Fatalf("raw: %d %s", r.status, r.raw)
	}
	if r = f.do("GET", Prefix+"/files/raw?path=pb_data/data.db", nil); r.status != 400 {
		t.Fatalf("pb_data: %d", r.status)
	}
	if r = f.do("GET", Prefix+"/files/raw?path=notes.txt", nil); r.status != 400 {
		t.Fatalf("outside: %d", r.status)
	}

	// chunked upload
	put := func(q string, body string) resp { return f.do("PUT", Prefix+"/files/raw?"+q, []byte(body)) }
	if r = put("path=pb_hooks/new.js&offset=0", "abc"); r.status != 200 || num0(r.body["size"]) != 3 {
		t.Fatalf("%s", r.raw)
	}
	if r = put("path=pb_hooks/new.js&offset=5", "x"); r.status != 409 || !strings.Contains(r.body["message"].(string), "server has 3 bytes, got offset 5") {
		t.Fatalf("%s", r.raw)
	}
	if r = put("path=pb_hooks/new.js&offset=3&final=1", "def"); r.status != 200 || r.body["final"] != true || f.read("pb_hooks/new.js") != "abcdef" {
		t.Fatalf("%s", r.raw)
	}
	if r = put("path=pb_hooks/new.js&offset=-1", "x"); r.status != 400 {
		t.Fatal(r.status)
	}

	// pb_data is out of reach (instance.fs/v1), powered off too
	inst := f.be.insts["myapp.example.com"]
	inst.Power = false
	f.be.insts["myapp.example.com"] = inst
	if r = put("path=pb_data/data.db&final=1", "new db"); r.status != 400 || f.read("pb_data/data.db") != "db" {
		t.Fatalf("%d %s", r.status, r.raw)
	}

	// deletes
	if r = f.do("DELETE", Prefix+"/files?path=pb_hooks", nil); r.status != 400 {
		t.Fatal(r.status)
	}
	if r = f.do("DELETE", Prefix+"/files?path=pb_hooks/sub", nil); r.status != 404 {
		t.Fatal(r.status)
	}
	f.put("pb_hooks/sub/a.js", "a")
	if r = f.do("DELETE", Prefix+"/files?path=pb_hooks/sub", nil); r.status != 409 {
		t.Fatal(r.status)
	}
	if r = f.do("DELETE", Prefix+"/files?path=pb_hooks/sub&recursive=1", nil); r.status != 200 || r.body["deleted"] != true {
		t.Fatal(r.status)
	}

	// symlinks out of the instance
	_ = os.Symlink(f.root, f.instFile("pb_public/out"))
	if r = f.do("GET", Prefix+"/files/raw?path=pb_public/out/deploys", nil); r.status != 400 || !strings.Contains(r.body["message"].(string), "symbolic link") {
		t.Fatalf("%d %s", r.status, r.raw)
	}

	// read-only tokens can't write
	f.be.pats["ph_pat_ro"] = fakeTok{UserID: "user1", Scopes: []string{"read"}}
	if r = f.do("PUT", Prefix+"/files/raw?path=pb_hooks/x.js", []byte("x"), withToken("ph_pat_ro")); r.status != 403 {
		t.Fatal(r.status)
	}
}

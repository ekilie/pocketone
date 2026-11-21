package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	cachev1 "pocketone.dev/ext/contracts/cache/v1"
	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
	watchv1 "pocketone.dev/ext/contracts/instance/watch/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/mothership"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type ms struct {
	app     *tests.TestApp
	srv     *httptest.Server
	url     string
	watches atomic.Int32
	unwatch atomic.Int32
}

// mothershipApp is a real PocketOne mothership on a local test server: the default schema and the watch server.
func mothershipApp(t *testing.T) *ms {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	on := true
	extkit.MustRegister(app, &mothership.Schema{Enabled: &on, Apex: "example.test"}, &mothership.Server{Enabled: &on, Logger: quiet})
	if err := mothership.EnsureCollections(app); err != nil {
		t.Fatal(err)
	}
	sc, _ := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	su := core.NewRecord(sc)
	su.SetEmail("ms@test.local")
	su.SetPassword("pw-12345678")
	if err := app.Save(su); err != nil {
		t.Fatal(err)
	}
	m := &ms{app: app}
	watchv1.OnWatch(app, func(e *extkit.CallEvent, in *watchv1.WatchInput) error {
		if in.Conn != "" {
			m.watches.Add(1)
		}
		return e.Next()
	})
	watchv1.OnUnwatch(app, func(e *extkit.CallEvent, in *watchv1.UnwatchInput) error {
		m.unwatch.Add(1)
		return e.Next()
	})
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	err = app.OnServe().Trigger(&core.ServeEvent{App: app, Router: router}, func(e *core.ServeEvent) error {
		mux, err := e.Router.BuildMux()
		if err != nil {
			return err
		}
		m.srv = httptest.NewServer(mux)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	m.url = m.srv.URL
	t.Cleanup(func() { m.srv.CloseClientConnections(); m.srv.Close() })
	return m
}

func save(t *testing.T, app core.App, coll string, id string, fields map[string]any) *core.Record {
	t.Helper()
	var r *core.Record
	if id != "" {
		r, _ = app.FindRecordById(coll, id)
	}
	if r == nil {
		c, err := app.FindCollectionByNameOrId(coll)
		if err != nil {
			t.Fatal(err)
		}
		r = core.NewRecord(c)
	}
	for k, v := range fields {
		r.Set(k, v)
	}
	if err := app.Save(r); err != nil {
		t.Fatalf("save %s: %v", coll, err)
	}
	return r
}

// memBucket is a bucket of bytes (entries go through MarshalCache, as on a disk tier).
type memBucket struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (b *memBucket) Get(_ context.Context, k string) ([]byte, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.m[k]
	return v, ok, nil
}
func (b *memBucket) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m[k] = v
	return nil
}
func (b *memBucket) Delete(_ context.Context, k string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, k)
	return nil
}
func (b *memBucket) Clear(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m = map[string][]byte{}
	return nil
}
func (b *memBucket) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.m)
}

var _ cachev1.Bucket = (*memBucket)(nil)

func newClient(t *testing.T, m *ms, self string) (*Client, *memBucket, <-chan sourcev1.Change, context.CancelFunc) {
	return newClientCfg(t, m, self, Config{})
}

func newClientCfg(t *testing.T, m *ms, self string, extra Config) (*Client, *memBucket, <-chan sourcev1.Change, context.CancelFunc) {
	t.Helper()
	c := New(Config{URL: m.url, Email: "ms@test.local", Password: "pw-12345678", Self: self, InternalURL: "http://10.0.0.5:8080",
		Heartbeat: time.Hour, HelloWait: 5 * time.Second, AskTimeout: 5 * time.Second, StreamIdle: extra.StreamIdle, Logger: quiet})
	b := &memBucket{m: map[string][]byte{}}
	c.UseBucket(b)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if _, err := c.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	ch, _ := c.Watch(ctx)
	return c, b, ch, cancel
}

func lookup(t *testing.T, c *Client, host string) (sourcev1.Instance, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := c.Lookup(ctx, host)
	if err != nil {
		t.Fatalf("lookup %s: %v", host, err)
	}
	return res.Instance, res.Found
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not within 5s", what)
		}
	}
}

func nextChange(t *testing.T, ch <-chan sourcev1.Change, id string) sourcev1.Change {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case c := <-ch:
			if c.Instance.ID == id {
				return c
			}
		case <-timeout:
			t.Fatalf("no change for %s within 5s", id)
		}
	}
}

func TestLazyMirror(t *testing.T) {
	m := mothershipApp(t)
	sgp := save(t, m.app, "edges", "", map[string]any{"edge_id": "sgp-1", "internal_url": "http://10.0.0.4:8080", "status": "online"})

	c, bucket, changes, _ := newClient(t, m, "lax-1")
	self := c.SelfRecord()
	if self == "" {
		t.Fatal("hello didn't name this edge (it registers itself)")
	}
	if e, ok := c.Resolve("edge:" + sgp.Id); !ok || e.ID != "sgp-1" || e.InternalURL != "http://10.0.0.4:8080" {
		t.Fatalf("sgp-1 from the hello: %+v %v", e, ok)
	}

	a := save(t, m.app, "instances", "", map[string]any{"name": "a", "hosts": []string{"a.custom.test"}, "edge": self, "power": true,
		"secrets": map[string]string{"API_KEY": "k"}, "limits": map[string]any{"max_sleep": "5s"}})
	b := save(t, m.app, "instances", "", map[string]any{"name": "b", "edge": sgp.Id, "power": true, "secrets": map[string]string{"B": "secret"}})
	save(t, m.app, "instances", "", map[string]any{"name": "c", "power": true}) // not placed

	// a full record for this edge's instance, by name and by extra host
	ia, ok := lookup(t, c, "A.example.test")
	if !ok || ia.ID != a.Id || ia.Remote != "" || ia.Secrets["API_KEY"] != "k" || ia.Limits.MaxSleep != 5*time.Second || len(ia.Hosts) != 2 {
		t.Fatalf("a: %+v %v", ia, ok)
	}
	if got := nextChange(t, changes, a.Id); got.Kind != sourcev1.Upsert {
		t.Fatalf("the host isn't told about a: %+v", got)
	}
	if ic, ok := lookup(t, c, "a.custom.test"); !ok || ic.ID != a.Id {
		t.Fatalf("a by its custom host: %+v %v", ic, ok)
	}
	// another edge's instance: routing only
	if ib, ok := lookup(t, c, "b.example.test"); !ok || ib.Remote != "edge:"+sgp.Id || ib.Secrets != nil {
		t.Fatalf("b: %+v %v", ib, ok)
	}
	// not placed, and unknown: not found (and cached as such)
	if _, ok := lookup(t, c, "c.example.test"); ok {
		t.Fatal("an unplaced instance is served")
	}
	if _, ok := lookup(t, c, "new.example.test"); ok {
		t.Fatal("unknown host found")
	}
	asked := m.watches.Load()
	for i := 0; i < 3; i++ {
		lookup(t, c, "a.example.test")
		lookup(t, c, "new.example.test")
	}
	if m.watches.Load() != asked {
		t.Fatal("a cached answer (found or not) asked again")
	}

	// changes come down the stream: the cache and the host follow, with no new ask
	save(t, m.app, "instances", a.Id, map[string]any{"secrets": map[string]string{"API_KEY": "rotated"}})
	waitFor(t, "rotated secret", func() bool { i, _ := lookup(t, c, "a.example.test"); return i.Secrets["API_KEY"] == "rotated" })
	if got := nextChange(t, changes, a.Id); got.Instance.Secrets["API_KEY"] != "rotated" {
		t.Fatalf("host change: %+v", got)
	}
	// a name that didn't exist is watched too: the instance that takes it arrives
	n := save(t, m.app, "instances", "", map[string]any{"name": "new", "edge": self, "power": true})
	waitFor(t, "new instance", func() bool { i, ok := lookup(t, c, "new.example.test"); return ok && i.ID == n.Id })
	// moved here: remote becomes full
	save(t, m.app, "instances", b.Id, map[string]any{"edge": self})
	waitFor(t, "b moved here", func() bool { i, _ := lookup(t, c, "b.example.test"); return i.Remote == "" && i.Secrets["B"] == "secret" })
	// renamed away: the old name is gone, a's custom host still answers
	save(t, m.app, "instances", a.Id, map[string]any{"name": "a2"})
	waitFor(t, "a renamed", func() bool { _, ok := lookup(t, c, "a.example.test"); return !ok })
	if i, ok := lookup(t, c, "a.custom.test"); !ok || i.ID != a.Id {
		t.Fatalf("a's custom host after the rename: %+v %v", i, ok)
	}
	// deleted: not found, and the host is told to drop it
	if err := m.app.Delete(n); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "new deleted", func() bool { _, ok := lookup(t, c, "new.example.test"); return !ok })
	if got := nextChange(t, changes, n.Id); got.Kind != sourcev1.Delete {
		// the first change for n is its upsert; the delete follows
		if got = nextChange(t, changes, n.Id); got.Kind != sourcev1.Delete {
			t.Fatalf("host not told of the delete: %+v", got)
		}
	}
	if asked2 := m.watches.Load(); asked2 != asked {
		t.Fatalf("changes asked again (%d asks, was %d)", asked2, asked)
	}

	// a hostname the cache dropped is unwatched at its next change
	for k := range bucket.m {
		if k == key(1, "b.example.test") {
			_ = bucket.Delete(context.Background(), k)
		}
	}
	save(t, m.app, "instances", b.Id, map[string]any{"dev": true})
	waitFor(t, "unwatch", func() bool { return m.unwatch.Load() == 1 })

	// edges stay current
	nyc := save(t, m.app, "edges", "", map[string]any{"edge_id": "nyc-1", "internal_url": "http://10.0.0.9:8080"})
	waitFor(t, "nyc-1", func() bool { _, ok := c.Resolve("edge:" + nyc.Id); return ok })
}

// 50 requests for a cold hostname make one ask.
func TestLookupCoalesces(t *testing.T) {
	m := mothershipApp(t)
	c, _, _, _ := newClient(t, m, "lax-1")
	save(t, m.app, "instances", "", map[string]any{"name": "hot", "edge": c.SelfRecord(), "power": true})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := lookup(t, c, "hot.example.test"); !ok {
				t.Error("not found")
			}
		}()
	}
	wg.Wait()
	if n := m.watches.Load(); n != 1 {
		t.Fatalf("%d asks for one hostname, want 1", n)
	}
}

// While the stream is down the cache serves; a miss is answered once and not cached; a new stream starts over.
func TestStreamDown(t *testing.T) {
	m := mothershipApp(t)
	c, bucket, _, _ := newClientCfg(t, m, "lax-1", Config{StreamIdle: 2 * time.Second})
	self := c.SelfRecord()
	save(t, m.app, "instances", "", map[string]any{"name": "x", "edge": self, "power": true})
	save(t, m.app, "instances", "", map[string]any{"name": "y", "edge": self, "power": true})
	if _, ok := lookup(t, c, "x.example.test"); !ok {
		t.Fatal("x")
	}

	// cut the stream: block new ones for a moment so the down state is observable
	block := make(chan struct{})
	var blocked atomic.Bool
	blocked.Store(true)
	watchv1.OnStream(m.app, func(e *extkit.CallEvent, in *watchv1.StreamInput) error {
		if blocked.Load() {
			<-block
		}
		return e.Next()
	})
	m.srv.CloseClientConnections()
	waitFor(t, "stream down", func() bool { return !c.Connected() })

	if _, ok := lookup(t, c, "x.example.test"); !ok {
		t.Fatal("cached x not served while down")
	}
	before := bucket.len()
	if i, ok := lookup(t, c, "y.example.test"); !ok || i.ID == "" {
		t.Fatal("y not answered in the reply while down")
	}
	if bucket.len() != before {
		t.Fatal("an answer from the reply was cached")
	}

	blocked.Store(false)
	close(block)
	waitFor(t, "stream back", func() bool { return c.Connected() })
	waitFor(t, "old generation cleared", func() bool { return bucket.len() == 0 })
	asked := m.watches.Load()
	if _, ok := lookup(t, c, "x.example.test"); !ok {
		t.Fatal("x after reconnect")
	}
	if m.watches.Load() != asked+1 {
		t.Fatal("after reconnect, x wasn't asked again")
	}
	if _, ok := bucket.m[key(2, "x.example.test")]; !ok {
		t.Fatal(fmt.Sprintf("x not cached in generation 2: %v", bucket.m))
	}
}

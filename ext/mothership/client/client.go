// Package client is the edge side of ext/mothership: it follows a mothership lazily over instance.watch/v1 and
// serves what it learns to a multisite host as instance.lookup/v1 (what a hostname serves), instance.source/v1 (the
// standing set, and changes to instances it handed out) and an edge.EdgeSource (where the other edges are).
//
//   - One stream per edge. Its hello carries the connection id, the edges and this edge's standing set.
//   - A hostname the host asks for and the cache doesn't hold is watched on the stream; the answer comes back on the
//     stream and is cached (cache/v1 bucket mothership.hosts). Requests for one hostname share one ask.
//   - Changes to watched hostnames come down the stream: a cached one is updated, one the cache dropped is unwatched.
//   - While the stream is down the cache keeps serving; a miss is asked in the reply, used once and never cached.
//   - A new stream starts a new generation: everything cached before it is no longer read (the cache key carries the
//     generation, so an answer from the old stream can't land in the new one), and the old entries are cleared.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cachev1 "pocketone.dev/ext/contracts/cache/v1"
	lookupv1 "pocketone.dev/ext/contracts/instance/lookup/v1"
	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
	watchv1 "pocketone.dev/ext/contracts/instance/watch/v1"
	"pocketone.dev/ext/edge"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/mothership"
	"pocketone.dev/ext/multisite"
)

// BucketName is the cache/v1 bucket the answers are kept in.
const BucketName = "mothership.hosts"

// Config configures a [Client].
type Config struct {
	URL      string // the mothership, e.g. http://10.88.0.10:8090
	Email    string // superuser login (the edge's local config carries it)
	Password string

	Self        string // this edge's edge_id
	InternalURL string // where other edges reach this one; set = register the edge record when it is missing
	Region      string // recorded when registering

	// Heartbeat is how often this edge marks its record online (PATCH edges/<id>: the default schema's); default
	// 15s, negative = never (a control plane with its own heartbeat).
	Heartbeat time.Duration
	// MaxHosts caps the hostnames kept in the cache bucket (default 50000; the cache section may override it).
	MaxHosts int
	// HelloWait is how long Snapshot waits for the first stream (default 10s); after that the host starts with an
	// empty standing set and the stream keeps trying.
	HelloWait time.Duration
	// AskTimeout bounds one ask for a hostname (default 15s).
	AskTimeout time.Duration
	// StreamIdle is how long the stream may stay silent (the server pings every 25s) before it is taken as dead
	// (default 60s).
	StreamIdle time.Duration

	Logger     *slog.Logger
	HTTPClient *http.Client // default: its own transport (an edge's egress policy guards Go's default one)
}

// Client follows a mothership (see the package doc). Create it with [New]; register it on a host (it is a
// multisite.Extension) or call Snapshot yourself.
type Client struct {
	cfg Config
	log *slog.Logger
	hc  *http.Client

	scope   *extkit.Scope // the host's, for cache/v1
	itemsMu sync.Mutex
	items   *cachev1.Items[*entry]
	inBand  noStore

	tokMu sync.Mutex
	token string

	mu        sync.Mutex
	gen       uint64                     // the cache generation: one per stream
	conn      string                     // the live stream's connection id; "" while down
	connDone  chan struct{}              // closed when the live stream ends
	pending   map[string]*ask            // hostname -> its ask in flight (this generation)
	delivered atomic.Int32               // asks answered but not yet reconciled (see reconcile)
	byID      map[string]map[string]bool // instance id -> cached hostnames that answer it (this generation)
	edges     []watchv1.Edge
	self      *watchv1.Edge
	standing  []sourcev1.Instance
	served    map[string]sourcev1.Instance // what the host was told (Snapshot and changes), by id
	hello     chan struct{}                // closed at the first hello
	helloOnce sync.Once

	started atomic.Bool
	runOnce sync.Once
	out     chan sourcev1.Change
	done    <-chan struct{}
	onStand []func([]sourcev1.Instance)
}

// ask is one hostname's ask: the answer arrives on the stream.
type ask struct {
	conn      string
	ch        chan struct{} // closed when ans or err is set
	ans       *watchv1.Answer
	err       error
	delivered bool
	latest    *watchv1.Answer // a change that came after the answer, before the answer was stored
}

var (
	_ sourcev1.Source     = (*Client)(nil)
	_ lookupv1.Lookup     = (*Client)(nil)
	_ edge.EdgeSource     = (*Client)(nil)
	_ multisite.Extension = (*Client)(nil)
)

var (
	errReset   = errors.New("mothership: the stream changed while asking")
	errNoCache = errors.New("mothership: nothing provides cache/v1 on the host")
)

// New returns a client for cfg; nothing connects until Snapshot.
func New(cfg Config) *Client {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConnsPerHost: 16,
			IdleConnTimeout:     90 * time.Second,
		}}
	}
	if cfg.MaxHosts <= 0 {
		cfg.MaxHosts = 50000
	}
	if cfg.HelloWait <= 0 {
		cfg.HelloWait = 10 * time.Second
	}
	if cfg.AskTimeout <= 0 {
		cfg.AskTimeout = 15 * time.Second
	}
	if cfg.StreamIdle <= 0 {
		cfg.StreamIdle = 60 * time.Second
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return &Client{cfg: cfg, log: log, hc: hc, pending: map[string]*ask{}, byID: map[string]map[string]bool{},
		served: map[string]sourcev1.Instance{}, hello: make(chan struct{}), out: make(chan sourcev1.Change, 1024)}
}

// FromSection returns a client for pocketone.yaml's `mothership` section, or nil when the section names no
// mothership to follow (no url).
func FromSection(log *slog.Logger) (*Client, error) {
	fc, found := mothership.Section.Get()
	if !found || fc.URL == "" {
		return nil, nil
	}
	if fc.Self == "" {
		e, _ := edge.Section.Get()
		fc.Self = e.Self
	}
	if fc.Self == "" {
		return nil, fmt.Errorf("mothership: self (this edge's id) is required with url")
	}
	return New(Config{URL: fc.URL, Email: fc.Email, Password: fc.Password, Self: fc.Self, InternalURL: fc.InternalURL,
		Region: fc.Region, Heartbeat: fc.Heartbeat, MaxHosts: fc.MaxHosts, Logger: log}), nil
}

// Name implements multisite.Extension.
func (c *Client) Name() string { return "mothership" }

// Contracts implements extkit.Declarer.
func (c *Client) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("instance.source/v1", "instance.lookup/v1"), Requires: extkit.IDs("cache/v1")}
}

// RegisterHost makes the client the host's instance.source/v1 and instance.lookup/v1.
func (c *Client) RegisterHost(h *multisite.Host) error {
	c.scope = h.Scope
	sourcev1.Contract.ProvideIn(h.Scope, c)
	lookupv1.Contract.ProvideIn(h.Scope, c)
	return nil
}

// UseBucket makes the client keep its answers in b instead of the host's cache/v1 (tests, or a host without a scope).
func (c *Client) UseBucket(b cachev1.Bucket) {
	c.itemsMu.Lock()
	c.items = cachev1.NewItems(b, entryVersion, decodeEntry)
	c.itemsMu.Unlock()
}

func (c *Client) bucket() (*cachev1.Items[*entry], error) {
	c.itemsMu.Lock()
	defer c.itemsMu.Unlock()
	if c.items != nil {
		return c.items, nil
	}
	if c.scope == nil {
		return nil, errNoCache
	}
	cache, ok := cachev1.Contract.In(c.scope)
	if !ok {
		return nil, errNoCache
	}
	b, err := cache.Bucket(BucketName, cachev1.Options{MaxItems: c.cfg.MaxHosts})
	if err != nil {
		return nil, err
	}
	c.items = cachev1.NewItems(b, entryVersion, decodeEntry)
	return c.items, nil
}

// ---------------------------------------------------------------------------------------------------------------
// cache entries

const entryVersion = "mhost/1"

// entry is one cached answer.
type entry struct {
	Found    bool               `json:"found"`
	Instance *sourcev1.Instance `json:"instance,omitempty"`
	size     int64
}

func newEntry(a *watchv1.Answer) *entry {
	e := &entry{Found: a.Found && a.Instance != nil, Instance: a.Instance}
	if !e.Found {
		e.Instance = nil
	}
	b, _ := json.Marshal(e)
	e.size = int64(len(b)) + 64
	return e
}

func (e *entry) CacheSize() int64              { return e.size }
func (e *entry) MarshalCache() ([]byte, error) { return json.Marshal(e) }
func (e *entry) CacheVersion() string          { return entryVersion }

func decodeEntry(b []byte) (*entry, error) {
	var e entry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, err
	}
	e.size = int64(len(b)) + 64
	return &e, nil
}

func (e *entry) result() lookupv1.Result {
	if !e.Found || e.Instance == nil {
		return lookupv1.Result{}
	}
	return lookupv1.Result{Found: true, Instance: *e.Instance}
}

// key is a hostname's cache key in generation gen.
func key(gen uint64, host string) string { return strconv.FormatUint(gen, 36) + "|" + host }

// noStore keeps nothing: Load over it coalesces asks without storing them (answers in the reply, while the stream is
// down). Its own type, so its flights never meet another bucket's.
type noStore struct{ cachev1.None }

// ---------------------------------------------------------------------------------------------------------------
// instance.lookup/v1

// Lookup implements [lookupv1.Lookup].
func (c *Client) Lookup(ctx context.Context, host string) (lookupv1.Result, error) {
	host = multisite.NormalizeHost(host)
	items, err := c.bucket()
	if err != nil {
		return lookupv1.Result{}, err
	}
	for attempt := 0; ; attempt++ {
		c.mu.Lock()
		gen, conn := c.gen, c.conn
		c.mu.Unlock()
		k := key(gen, host)
		if e, ok, _ := items.Get(ctx, k); ok {
			if e2 := c.reconcile(ctx, items, gen, host); e2 != nil {
				e = e2
			}
			return e.result(), nil
		}
		if conn == "" || attempt >= 2 {
			return c.askInBand(ctx, host)
		}
		e, err := items.Load(ctx, k, func(lctx context.Context) (*entry, error) { return c.ask(lctx, gen, conn, host) })
		if errors.Is(err, errReset) {
			continue // the stream changed under the ask: ask again (on the new stream, or in the reply)
		}
		if err != nil {
			return lookupv1.Result{}, err
		}
		if e2 := c.reconcile(ctx, items, gen, host); e2 != nil {
			e = e2
		}
		return e.result(), nil
	}
}

// ask watches host on the stream conn and waits for its answer there.
func (c *Client) ask(ctx context.Context, gen uint64, conn, host string) (*entry, error) {
	c.mu.Lock()
	if c.gen != gen || c.conn != conn {
		c.mu.Unlock()
		return nil, errReset
	}
	a := c.pending[host]
	if a != nil && a.delivered {
		// answered on this stream already (the cache dropped it since): still watched, and the newest answer is here
		delete(c.pending, host)
		c.delivered.Add(-1)
		ans := a.ans
		if a.latest != nil {
			ans = a.latest
		}
		e := newEntry(ans)
		c.noteServedLocked(gen, host, e)
		c.mu.Unlock()
		return e, nil
	}
	if a == nil || a.err != nil {
		a = &ask{conn: conn, ch: make(chan struct{})}
		c.pending[host] = a
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, c.cfg.AskTimeout)
	defer cancel()
	var res watchv1.WatchResult
	if err := c.call(ctx, "watch", watchv1.WatchInput{Conn: conn, Edge: c.cfg.Self, Host: host}, &res); err != nil {
		c.dropAsk(host, a)
		return nil, err
	}
	if !res.Registered {
		c.dropAsk(host, a)
		return nil, errReset // the mothership no longer knows conn: the stream is about to end here too
	}
	select {
	case <-a.ch:
	case <-ctx.Done():
		c.dropAsk(host, a)
		return nil, fmt.Errorf("mothership: no answer for %s within %s", host, c.cfg.AskTimeout)
	}
	if a.err != nil {
		return nil, a.err
	}
	e := newEntry(a.ans)
	c.noteServed(gen, host, e)
	return e, nil
}

func (c *Client) dropAsk(host string, a *ask) {
	c.mu.Lock()
	if c.pending[host] == a {
		delete(c.pending, host)
		if a.delivered {
			c.delivered.Add(-1)
		}
	}
	c.mu.Unlock()
}

// reconcile finishes an answered ask once its answer is stored: a change that came on the stream between the answer
// and the store is stored now (it is newer). It returns that newer entry, or nil.
func (c *Client) reconcile(ctx context.Context, items *cachev1.Items[*entry], gen uint64, host string) *entry {
	if c.delivered.Load() == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.pending[host]
	if a == nil || !a.delivered || c.gen != gen {
		return nil
	}
	delete(c.pending, host)
	c.delivered.Add(-1)
	if a.latest == nil {
		return nil
	}
	e := newEntry(a.latest)
	_ = items.Set(ctx, key(gen, host), e, 0)
	c.noteServedLocked(gen, host, e)
	return e
}

// askInBand asks without a stream: the answer is used for this request only, never cached (nothing watches it).
// Concurrent asks for one hostname share one call.
func (c *Client) askInBand(ctx context.Context, host string) (lookupv1.Result, error) {
	b, err := cachev1.Load(ctx, c.inBand, host, func(lctx context.Context) ([]byte, error) {
		lctx, cancel := context.WithTimeout(lctx, c.cfg.AskTimeout)
		defer cancel()
		var res watchv1.WatchResult
		if err := c.call(lctx, "watch", watchv1.WatchInput{Edge: c.cfg.Self, Host: host}, &res); err != nil {
			return nil, err
		}
		if res.Answer == nil {
			return nil, fmt.Errorf("mothership: no answer for %s", host)
		}
		return json.Marshal(newEntry(res.Answer))
	})
	if err != nil {
		return lookupv1.Result{}, err
	}
	e, err := decodeEntry(b)
	if err != nil {
		return lookupv1.Result{}, err
	}
	return e.result(), nil
}

// noteServed records which hostnames answer an instance, and tells the host when an instance it holds changed.
func (c *Client) noteServed(gen uint64, host string, e *entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.noteServedLocked(gen, host, e)
}

func (c *Client) noteServedLocked(gen uint64, host string, e *entry) {
	if gen != c.gen {
		return
	}
	for id, hosts := range c.byID {
		if hosts[host] && (!e.Found || e.Instance.ID != id) {
			delete(hosts, host)
			if len(hosts) == 0 {
				delete(c.byID, id)
				if _, ok := c.served[id]; ok && !c.isStandingLocked(id) {
					delete(c.served, id)
					c.emitLocked(sourcev1.Change{Kind: sourcev1.Delete, Instance: sourcev1.Instance{ID: id}})
				}
			}
		}
	}
	if !e.Found {
		return
	}
	inst := *e.Instance
	if c.byID[inst.ID] == nil {
		c.byID[inst.ID] = map[string]bool{}
	}
	c.byID[inst.ID][host] = true
	if prev, ok := c.served[inst.ID]; !ok || !reflect.DeepEqual(prev, inst) {
		c.served[inst.ID] = inst
		c.emitLocked(sourcev1.Change{Kind: sourcev1.Upsert, Instance: inst})
	}
}

func (c *Client) isStandingLocked(id string) bool {
	for _, s := range c.standing {
		if s.ID == id {
			return true
		}
	}
	return false
}

// emitLocked tells the host about a change (after Watch; before it, Snapshot's list carries the state).
func (c *Client) emitLocked(ch sourcev1.Change) {
	if !c.started.Load() {
		return
	}
	select {
	case c.out <- ch:
	default:
		// the host applies changes as they come; a full queue means it's stuck, and the next request for the
		// instance hands the runtime the current record anyway
		c.log.Warn("mothership: change queue full; dropped", "instance", ch.Instance.ID)
	}
}

// ---------------------------------------------------------------------------------------------------------------
// instance.source/v1

// Snapshot implements [sourcev1.Source]: it starts following the mothership, waits up to HelloWait for the first
// stream and returns the standing set (empty if the mothership can't be reached yet; it arrives as changes then).
func (c *Client) Snapshot(ctx context.Context) ([]sourcev1.Instance, error) {
	if c.cfg.URL == "" || c.cfg.Self == "" {
		return nil, errors.New("mothership: url and self are required")
	}
	if _, err := c.bucket(); err != nil {
		return nil, err
	}
	c.runOnce.Do(func() {
		c.done = ctx.Done()
		go c.run(ctx)
		if c.cfg.Heartbeat >= 0 {
			go c.heartbeatLoop(ctx)
		}
	})
	select {
	case <-c.hello:
	case <-time.After(c.cfg.HelloWait):
		c.log.Warn("mothership: no stream yet; starting without the standing set", "url", c.cfg.URL, "waited", c.cfg.HelloWait)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]sourcev1.Instance(nil), c.standing...)
	for _, s := range out {
		c.served[s.ID] = s
	}
	return out, nil
}

// Watch implements [sourcev1.Source]: changes to instances the host was handed (an answer changed, the standing set
// changed) until ctx is done.
func (c *Client) Watch(ctx context.Context) (<-chan sourcev1.Change, error) {
	c.started.Store(true)
	return c.out, nil
}

// Standing is this edge's standing set as last sent (instances it must hold whatever its traffic).
func (c *Client) Standing() []sourcev1.Instance {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]sourcev1.Instance(nil), c.standing...)
}

// OnStanding calls fn (on the stream's goroutine; it must not block) each time the standing set arrives.
func (c *Client) OnStanding(fn func([]sourcev1.Instance)) {
	c.mu.Lock()
	c.onStand = append(c.onStand, fn)
	c.mu.Unlock()
}

// Connected reports whether a stream is live.
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != ""
}

// SelfRecord is this edge's record id at the mothership ("" until a hello names it).
func (c *Client) SelfRecord() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.self == nil {
		return ""
	}
	return c.self.ID
}

// ---------------------------------------------------------------------------------------------------------------
// edge.EdgeSource

// Self implements [edge.EdgeSource].
func (c *Client) Self() string { return c.cfg.Self }

// EdgeRecords returns the edges as the stream last sent them (with record ids).
func (c *Client) EdgeRecords() []watchv1.Edge {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]watchv1.Edge(nil), c.edges...)
}

// Resolve implements [edge.EdgeSource]: "edge:<record id>", "edge:@<edge_id>" or "<edge_id>".
func (c *Client) Resolve(remote string) (edge.Edge, bool) {
	ref := strings.TrimPrefix(remote, "edge:")
	c.mu.Lock()
	defer c.mu.Unlock()
	if !strings.HasPrefix(ref, "@") {
		for _, e := range c.edges {
			if e.ID == ref {
				return toEdge(e), true
			}
		}
	}
	name := strings.TrimPrefix(ref, "@")
	for _, e := range c.edges {
		if e.Name == name {
			return toEdge(e), true
		}
	}
	return edge.Edge{}, false
}

// Edges implements [edge.EdgeSource].
func (c *Client) Edges() []edge.Edge {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]edge.Edge, 0, len(c.edges))
	for _, e := range c.edges {
		out = append(out, toEdge(e))
	}
	return out
}

func toEdge(e watchv1.Edge) edge.Edge {
	id := e.Name
	if id == "" {
		id = e.ID
	}
	return edge.Edge{ID: id, InternalURL: e.InternalURL, Region: e.Region, Status: e.Status, LastSeen: e.LastSeen}
}

// ---------------------------------------------------------------------------------------------------------------
// the stream

// run keeps a stream open until ctx is done, reconnecting with backoff.
func (c *Client) run(ctx context.Context) {
	if c.cfg.InternalURL != "" {
		for ctx.Err() == nil {
			if err := c.register(ctx); err == nil {
				break
			} else {
				c.log.Warn("mothership: register failed; retrying", "error", err)
			}
			sleep(ctx, 5*time.Second)
		}
	}
	wait := time.Second
	for attempt := 1; ctx.Err() == nil; attempt++ {
		start := time.Now()
		err := c.stream(ctx)
		c.down()
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > 30*time.Second {
			wait, attempt = time.Second, 1 // it was up for a while: start the backoff over
		}
		c.log.Warn("mothership: stream ended; reconnecting", "attempt", attempt, "in", wait, "error", err)
		sleep(ctx, wait)
		if wait < 30*time.Second {
			wait *= 2
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// stream opens one stream and reads it until it ends.
func (c *Client) stream(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	token, err := c.auth(ctx)
	if err != nil {
		return err
	}
	q := url.Values{"edge": {c.cfg.Self}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.URL+extkit.ContractPrefix+"instance.watch/v1/stream?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", token)
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		c.resetToken()
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("stream: %d %s", res.StatusCode, bytes.TrimSpace(b))
	}
	idle := time.AfterFunc(c.cfg.StreamIdle, cancel)
	defer idle.Stop()
	r := bufio.NewReaderSize(res.Body, 64<<10)
	var data bytes.Buffer
	var seq uint64
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil && errors.Is(context.Cause(ctx), context.Canceled) {
				return fmt.Errorf("stream silent for %s", c.cfg.StreamIdle)
			}
			return err
		}
		idle.Reset(c.cfg.StreamIdle)
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if data.Len() == 0 {
				continue
			}
			var ev watchv1.Event
			if err := json.Unmarshal(data.Bytes(), &ev); err != nil {
				return fmt.Errorf("stream: bad event: %w", err)
			}
			data.Reset()
			if ev.Seq != seq+1 {
				return fmt.Errorf("stream: event %d after %d (missed some)", ev.Seq, seq)
			}
			seq = ev.Seq
			if seq == 1 && ev.Kind != watchv1.KindHello {
				return fmt.Errorf("stream: first event is %q, not hello", ev.Kind)
			}
			c.apply(ctx, ev)
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(line[len("data:"):], " "))
		}
	}
}

// down marks the stream gone: the cache keeps serving; asks waiting on it fail (and aren't stored).
func (c *Client) down() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == "" {
		return
	}
	c.conn = ""
	for h, a := range c.pending {
		if !a.delivered {
			a.err = errReset
			close(a.ch)
			delete(c.pending, h)
		}
	}
}

// apply handles one stream event, in order.
func (c *Client) apply(ctx context.Context, ev watchv1.Event) {
	switch ev.Kind {
	case watchv1.KindHello:
		c.mu.Lock()
		old := c.gen
		c.gen++
		c.conn = ev.Conn
		for h, a := range c.pending {
			if !a.delivered {
				a.err = errReset
				close(a.ch)
			}
			delete(c.pending, h)
		}
		c.delivered.Store(0)
		c.byID = map[string]map[string]bool{}
		c.edges, c.self = ev.Edges, ev.Self
		c.mu.Unlock()
		c.setStanding(ev.Standing)
		if old > 0 {
			// the previous generation's entries are never read again; free them
			if items, err := c.bucket(); err == nil {
				_ = items.Bucket().Clear(ctx)
			}
		}
		c.helloOnce.Do(func() { close(c.hello) })
		c.log.Info("mothership: stream open", "conn", ev.Conn, "generation", old+1, "edges", len(ev.Edges), "standing", len(ev.Standing))
	case watchv1.KindEdges:
		c.mu.Lock()
		c.edges = ev.Edges
		for i := range ev.Edges {
			if ev.Edges[i].Name == c.cfg.Self {
				e := ev.Edges[i]
				c.self = &e
			}
		}
		c.mu.Unlock()
	case watchv1.KindStanding:
		c.setStanding(ev.Standing)
	case watchv1.KindHost:
		c.hostEvent(ctx, ev)
	}
}

func (c *Client) setStanding(list []sourcev1.Instance) {
	c.mu.Lock()
	c.standing = list
	for _, inst := range list {
		if prev, ok := c.served[inst.ID]; !ok || !reflect.DeepEqual(prev, inst) {
			c.served[inst.ID] = inst
			c.emitLocked(sourcev1.Change{Kind: sourcev1.Upsert, Instance: inst})
		}
	}
	fns := append([]func([]sourcev1.Instance){}, c.onStand...)
	c.mu.Unlock()
	for _, fn := range fns {
		fn(list)
	}
}

// hostEvent is an answer (to an ask) or a change to a watched hostname.
func (c *Client) hostEvent(ctx context.Context, ev watchv1.Event) {
	c.mu.Lock()
	gen, conn := c.gen, c.conn
	if a := c.pending[ev.Host]; a != nil {
		switch {
		case !a.delivered && ev.Answer == nil:
			a.err = fmt.Errorf("mothership: %s couldn't be resolved", ev.Host)
			close(a.ch)
			delete(c.pending, ev.Host)
		case !a.delivered:
			a.ans, a.delivered = ev.Answer, true
			c.delivered.Add(1)
			close(a.ch)
		case ev.Answer != nil:
			if items, err := c.bucket(); err == nil {
				if _, stored, _ := items.Get(ctx, key(gen, ev.Host)); stored {
					// the answer is stored: this change replaces it, and the ask is done
					e := newEntry(ev.Answer)
					_ = items.Set(ctx, key(gen, ev.Host), e, 0)
					delete(c.pending, ev.Host)
					c.delivered.Add(-1)
					c.noteServedLocked(gen, ev.Host, e)
					break
				}
			}
			a.latest = ev.Answer // the answer isn't stored yet: reconcile stores this after it
		}
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	if ev.Answer == nil {
		return
	}
	items, err := c.bucket()
	if err != nil {
		return
	}
	k := key(gen, ev.Host)
	if _, ok, _ := items.Get(ctx, k); !ok {
		// the cache dropped it (or never stored it): stop the changes
		go func() {
			uctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = c.call(uctx, "unwatch", watchv1.UnwatchInput{Conn: conn, Host: ev.Host}, nil)
		}()
		return
	}
	e := newEntry(ev.Answer)
	_ = items.Set(ctx, k, e, 0)
	c.noteServed(gen, ev.Host, e)
}

// ---------------------------------------------------------------------------------------------------------------
// HTTP

func (c *Client) auth(ctx context.Context) (string, error) {
	c.tokMu.Lock()
	defer c.tokMu.Unlock()
	if c.token != "" {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"identity": c.cfg.Email, "password": c.cfg.Password})
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL+"/api/collections/_superusers/auth-with-password", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("mothership login: %d %s", res.StatusCode, bytes.TrimSpace(b))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	c.token = out.Token
	return c.token, nil
}

func (c *Client) resetToken() {
	c.tokMu.Lock()
	c.token = ""
	c.tokMu.Unlock()
}

// call sends an instance.watch/v1 operation (logging in again once on 401).
func (c *Client) call(ctx context.Context, op string, in, out any) error {
	for attempt := 0; ; attempt++ {
		token, err := c.auth(ctx)
		if err != nil {
			return err
		}
		err = watchv1.W.Client(c.cfg.URL, token, c.hc).Call(ctx, http.MethodPost, op, in, out)
		var ce *extkit.Error
		if errors.As(err, &ce) && ce.Status == http.StatusUnauthorized && attempt == 0 {
			c.resetToken()
			continue
		}
		return err
	}
}

// rest sends a PocketBase API request as the superuser (logging in again once on 401).
func (c *Client) rest(ctx context.Context, method, path string, in, out any) error {
	for attempt := 0; ; attempt++ {
		token, err := c.auth(ctx)
		if err != nil {
			return err
		}
		var body io.Reader
		if in != nil {
			b, _ := json.Marshal(in)
			body = bytes.NewReader(b)
		}
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(rctx, method, c.cfg.URL+path, body)
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Authorization", token)
		if in != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.hc.Do(req)
		if err != nil {
			cancel()
			return err
		}
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		cancel()
		if res.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.resetToken()
			continue
		}
		if res.StatusCode >= 300 {
			return fmt.Errorf("%s %s: %d %s", method, path, res.StatusCode, bytes.TrimSpace(b))
		}
		if out != nil {
			return json.Unmarshal(b, out)
		}
		return nil
	}
}

// register creates this edge's record (default schema) when the mothership has none.
func (c *Client) register(ctx context.Context) error {
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	q := url.Values{"filter": {fmt.Sprintf("edge_id = %q", c.cfg.Self)}, "fields": {"id"}, "skipTotal": {"1"}}
	if err := c.rest(ctx, http.MethodGet, "/api/collections/edges/records?"+q.Encode(), nil, &list); err != nil {
		return err
	}
	if len(list.Items) > 0 {
		return nil
	}
	body := map[string]any{"edge_id": c.cfg.Self, "internal_url": c.cfg.InternalURL, "region": c.cfg.Region, "status": "online"}
	if err := c.rest(ctx, http.MethodPost, "/api/collections/edges/records", body, nil); err != nil {
		return fmt.Errorf("register edge %q: %w", c.cfg.Self, err)
	}
	c.log.Info("mothership: registered this edge", "edge", c.cfg.Self)
	return nil
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	every := c.cfg.Heartbeat
	if every == 0 {
		every = 15 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		c.heartbeat(sctx, "offline")
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.heartbeat(ctx, "online")
		}
	}
}

func (c *Client) heartbeat(ctx context.Context, status string) {
	id := c.SelfRecord()
	if id == "" {
		return
	}
	body := map[string]any{"status": status, "last_seen": time.Now().UTC().Format("2006-01-02 15:04:05.000Z")}
	if err := c.rest(ctx, http.MethodPatch, "/api/collections/edges/records/"+id, body, nil); err != nil && ctx.Err() == nil {
		c.log.Warn("mothership: heartbeat", "status", status, "error", err)
	}
}

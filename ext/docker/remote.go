package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	remotev1 "github.com/pockethost/pocketone/ext/contracts/remote/v1"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/multisite"
)

// Docker runs stock PocketBase images, which have no PocketOne extensions, so the host answers remote/v1 for them
// (host-provided contracts): whoami, status, restart and logs, as one of the host's instance routes (after the
// firewall, rate limits and the blocked answer, before the container). [Provider] wires it up; a host built by hand
// does the same with [Runtime.Remote], [Runtime.SuperuserAuth] and remotev1.W.HostHandler in multisite.Host's
// InstanceRoutes.

// Remote is remote/v1 for the runtime's instances, for a host scope to provide:
//
//	remotev1.Contract.ProvideForInstances(host, rt.Remote(host, dir))
//
// Restart replaces the container (the next request, or the restart itself, starts a fresh one); logs are the
// container's output since this host started following it (the last 1000 lines per instance).
func (rt *Runtime) Remote(host *extkit.Scope, dir *multisite.Directory) func(instance string) remotev1.Remote {
	return func(id string) remotev1.Remote { return &instanceRemote{rt: rt, host: host, dir: dir, id: id} }
}

type instanceRemote struct {
	rt   *Runtime
	host *extkit.Scope
	dir  *multisite.Directory
	id   string
}

func (r *instanceRemote) Whoami(_ context.Context, a extkit.Auth) (remotev1.Whoami, error) {
	w := remotev1.Whoami{Scopes: []string{}, App: remotev1.App{Name: r.id}}
	for _, id := range r.host.HostProvided() {
		w.Contracts = append(w.Contracts, string(id))
	}
	if a.Kind == "" {
		return w, nil
	}
	if a.RecordID != "" {
		w.Auth = &remotev1.Identity{Collection: a.Collection, ID: a.RecordID, Email: a.Email}
	}
	w.Via = "jwt"
	return w, nil
}

func (r *instanceRemote) Status(context.Context, extkit.Auth) (remotev1.Status, error) {
	c := r.rt.get(r.id)
	s := remotev1.Status{Running: c.running.Load()}
	if pb, ok := c.poweredBy.Load().(string); ok {
		s.PocketBase = strings.TrimPrefix(pb, "PocketBase ")
		s.Version = pb
	}
	if s.Running {
		if t := c.started.Load(); t > 0 {
			s.Uptime = time.Since(time.Unix(0, t)).Round(time.Second).String()
		}
	}
	return s, nil
}

func (r *instanceRemote) Restart(context.Context, extkit.Auth) (remotev1.RestartResult, error) {
	if !r.rt.get(r.id).running.Load() {
		return remotev1.RestartResult{Message: "not running; it starts with the new code on its next request"}, nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := r.rt.WithInstanceStopped(ctx, r.id, func(string) error { return nil }); err != nil {
			r.rt.logLine(r.id, "stderr", "restart: "+err.Error())
			return
		}
		if inst, ok := r.dir.Get(r.id); ok && inst.Blocked == nil { // a blocked instance stays stopped
			if err := r.rt.StartInstance(ctx, inst); err != nil {
				r.rt.logLine(r.id, "stderr", "restart: the container failed to start: "+err.Error())
			}
		}
	}()
	return remotev1.RestartResult{Restarting: true, Message: "restarting; requests in flight finish first"}, nil
}

func (r *instanceRemote) Logs(_ context.Context, _ extkit.Auth, in remotev1.LogsInput) (extkit.Stream, error) {
	n := in.N
	if n <= 0 {
		n = 100
	}
	ring := r.rt.ring(r.id)
	initial, next := ring.last(min(n, logRingSize))
	return extkit.Stream{Initial: initial, Poll: func() []any {
		var out []any
		out, next = ring.since(next)
		return out
	}}, nil
}

// SuperuserAuth authenticates a remote/v1 call at the host, inside the host's instance routes (the instance is
// multisite.InstanceOf's): a request whose Authorization is a superuser token of that instance (checked by the
// instance itself, which starts it if it's asleep) is a superuser; anything else is anonymous.
func (rt *Runtime) SuperuserAuth() func(*http.Request) extkit.Auth {
	return func(r *http.Request) extkit.Auth {
		token := r.Header.Get("Authorization")
		inst, ok := multisite.InstanceOf(r)
		if token == "" || !ok || inst.Blocked != nil { // never start a blocked instance to check a token
			return extkit.Auth{}
		}
		ctx, cancel := context.WithTimeout(r.Context(), rt.cfg.StartTimeout+5*time.Second)
		defer cancel()
		port, err := rt.ensure(ctx, rt.get(inst.ID), inst)
		if err != nil {
			return extkit.Auth{}
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/settings", nil)
		req.Header.Set("Authorization", token)
		res, err := rt.http.Do(req)
		if err != nil {
			return extkit.Auth{}
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return extkit.Auth{}
		}
		return extkit.Auth{Kind: "superuser", Collection: "_superusers", RecordID: jwtID(token)}
	}
}

// jwtID reads the record id from a token the instance already accepted.
func jwtID(token string) string {
	parts := strings.Split(strings.TrimPrefix(token, "Bearer "), ".")
	if len(parts) != 3 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &claims)
	return claims.ID
}

// serveRemote is an instance route (multisite.Host's InstanceRoutes) answering remote/v1 for the instances, after
// the host's gates and before next, the container.
func (rt *Runtime) serveRemote(dir *multisite.Directory) func(http.Handler) http.Handler {
	host := extkit.NewScope("docker")
	host.Registering("docker")
	remotev1.Contract.ProvideForInstances(host, rt.Remote(host, dir))
	return func(next http.Handler) http.Handler {
		return remotev1.W.HostHandler(host, multisite.InstanceID, rt.SuperuserAuth(), next)
	}
}

// logRingSize is how many output lines the host keeps per instance, for remote/v1's logs.
const logRingSize = 1000

type logRing struct {
	mu      sync.Mutex
	entries []remotev1.LogEntry
	first   int // sequence number of entries[0]
}

func (rt *Runtime) ring(id string) *logRing {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.logs == nil {
		rt.logs = map[string]*logRing{}
	}
	l, ok := rt.logs[id]
	if !ok {
		l = &logRing{}
		rt.logs[id] = l
	}
	return l
}

func (rt *Runtime) logLine(id, stream, msg string) {
	rt.ring(id).add(remotev1.LogEntry{Time: time.Now().UTC().Format(time.RFC3339Nano), Stream: stream, Message: msg})
}

func (l *logRing) add(e remotev1.LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
	if drop := len(l.entries) - logRingSize; drop > 0 {
		l.entries = append([]remotev1.LogEntry(nil), l.entries[drop:]...)
		l.first += drop
	}
}

// last returns the last n entries and the sequence number after them.
func (l *logRing) last(n int) ([]any, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	start := max(len(l.entries)-n, 0)
	return toAny(l.entries[start:]), l.first + len(l.entries)
}

// since returns the entries from sequence number seq on (those dropped meanwhile are skipped) and the next one.
func (l *logRing) since(seq int) ([]any, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	start := max(seq-l.first, 0)
	if start > len(l.entries) {
		start = len(l.entries)
	}
	return toAny(l.entries[start:]), l.first + len(l.entries)
}

func toAny(es []remotev1.LogEntry) []any {
	out := make([]any, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

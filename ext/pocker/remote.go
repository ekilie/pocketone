package pocker

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	remotev1 "pocketone.dev/ext/contracts/remote/v1"
	"pocketone.dev/ext/extkit"
)

// InstanceApp returns an instance's running app and when it started; ok is false when it isn't running.
func (m *Manager) InstanceApp(id string) (app core.App, started time.Time, ok bool) {
	m.mu.RLock()
	s := m.apps[id]
	m.mu.RUnlock()
	if s == nil {
		return nil, time.Time{}, false
	}
	select {
	case <-s.done:
	default:
		return nil, time.Time{}, false // still starting
	}
	if s.app == nil {
		return nil, time.Time{}, false
	}
	return s.app.app, s.app.started, true
}

// Remote is remote/v1 for a host's Pocker instances, for the host to provide:
//
//	remotev1.Contract.ProvideForInstances(host, pocker.Remote(m, host))
//	srv.InstanceRoutes = append(srv.InstanceRoutes, func(next http.Handler) http.Handler {
//		return remotev1.W.HostHandler(host, multisite.InstanceID, auth, next) // after the gates
//	})
//
// Restart goes through the Manager (only that instance), logs are the instance's logs/exec.log (its output, hook
// console and app log). host is the host's contract scope, for whoami's contract list.
func Remote(m *Manager, host *extkit.Scope) func(instance string) remotev1.Remote {
	return func(id string) remotev1.Remote { return &instanceRemote{m: m, host: host, id: id} }
}

type instanceRemote struct {
	m    *Manager
	host *extkit.Scope
	id   string
}

func (r *instanceRemote) Whoami(_ context.Context, a extkit.Auth) (remotev1.Whoami, error) {
	w := remotev1.Whoami{Scopes: []string{}, App: remotev1.App{Name: r.id}}
	seen := map[string]bool{}
	for _, id := range r.host.HostProvided() {
		seen[string(id)] = true
		w.Contracts = append(w.Contracts, string(id))
	}
	app, _, running := r.m.InstanceApp(r.id)
	if running {
		w.App = remotev1.App{Name: app.Settings().Meta.AppName, URL: app.Settings().Meta.AppURL}
		for _, id := range extkit.Provided(app) {
			if !seen[string(id)] {
				w.Contracts = append(w.Contracts, string(id))
			}
		}
	}
	if a.Kind == "" {
		return w, nil
	}
	if a.RecordID != "" {
		w.Auth = &remotev1.Identity{Collection: a.Collection, ID: a.RecordID, Email: a.Email}
	}
	w.Via = "jwt"
	if a.Kind == "token" {
		w.Via, w.TokenID = "token", a.ID
		if a.Scopes != nil {
			w.Scopes = a.Scopes
		}
	}
	if running {
		w.Extensions = extkit.Extensions(app)
	}
	return w, nil
}

func (r *instanceRemote) Status(context.Context, extkit.Auth) (remotev1.Status, error) {
	s := remotev1.Status{Version: extkit.Version, PocketBase: pocketbase.Version}
	if app, started, ok := r.m.InstanceApp(r.id); ok {
		s.Running, s.Dev = true, app.IsDev()
		s.Uptime = time.Since(started).Round(time.Second).String()
	}
	return s, nil
}

func (r *instanceRemote) Restart(context.Context, extkit.Auth) (remotev1.RestartResult, error) {
	if !r.m.Running(r.id) {
		return remotev1.RestartResult{Message: "not running; it starts with the new code on its next request"}, nil
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := r.m.WithInstanceStopped(ctx, r.id, func(string) error { return nil }); err != nil {
			r.m.LogInstance(r.id, "stderr", "restart: "+err.Error())
			return
		}
		if err := r.m.StartInstance(ctx, r.id); err != nil {
			r.m.LogInstance(r.id, "stderr", "restart: the app failed to start: "+err.Error())
		}
	}()
	return remotev1.RestartResult{Restarting: true, Message: "restarting; requests in flight finish first"}, nil
}

func (r *instanceRemote) Logs(_ context.Context, _ extkit.Auth, in remotev1.LogsInput) (extkit.Stream, error) {
	dir, ok := r.m.InstanceDir(r.id)
	if !ok {
		return extkit.Stream{}, &extkit.Error{Code: "not_found", Message: "unknown instance", Status: 404}
	}
	n := in.N
	if n <= 0 {
		n = 100
	}
	t := &logTail{path: filepath.Join(dir, "logs", "exec.log")}
	return extkit.Stream{Initial: t.last(min(n, 1000)), Poll: t.poll}, nil
}

// logTail follows an instance's exec.log: JSON lines ({message, stream, time}), rotated by size.
type logTail struct {
	path   string
	offset int64
}

func (t *logTail) last(n int) []any {
	f, err := os.Open(t.path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		lines = append(lines, entry(sc.Bytes()))
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	t.offset, _ = f.Seek(0, io.SeekCurrent)
	return lines
}

func (t *logTail) poll() []any {
	f, err := os.Open(t.path)
	if err != nil {
		return nil
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() < t.offset {
		t.offset = 0 // rotated
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil
	}
	var out []any
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break // a partial last line waits for the next poll
		}
		t.offset += int64(len(line))
		out = append(out, entry(line))
	}
	return out
}

func entry(line []byte) any {
	var m map[string]any
	if json.Unmarshal(line, &m) != nil {
		return map[string]any{"stream": "stdout", "message": string(line)}
	}
	return m
}

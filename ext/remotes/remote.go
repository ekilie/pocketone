package remotes

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
	remotev1 "github.com/pockethost/pocketone/ext/contracts/remote/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// remote is remote/v1 on a standalone app, or on an instance whose multisite host doesn't provide it.
type remote struct {
	app     core.App
	restart func(core.App) error
}

var _ remotev1.Remote = (*remote)(nil)

func (r *remote) Whoami(_ context.Context, a extkit.Auth) (remotev1.Whoami, error) {
	w := remotev1.Whoami{
		Scopes: []string{},
		App:    remotev1.App{Name: r.app.Settings().Meta.AppName, URL: r.app.Settings().Meta.AppURL},
	}
	for _, id := range extkit.Provided(r.app) {
		w.Contracts = append(w.Contracts, string(id))
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
	w.Extensions = extkit.Extensions(r.app)
	return w, nil
}

func (r *remote) Status(context.Context, extkit.Auth) (remotev1.Status, error) {
	s := remotev1.Status{Running: true, Version: extkit.Version, PocketBase: pocketbase.Version, Dev: r.app.IsDev()}
	if t := extkit.Started(r.app); !t.IsZero() {
		s.Uptime = time.Since(t).Round(time.Second).String()
	}
	return s, nil
}

func (r *remote) Restart(context.Context, extkit.Auth) (remotev1.RestartResult, error) {
	restart := r.restart
	if restart == nil {
		restart = func(app core.App) error { return app.Restart() } // an isolate (Pocker): only this one
	}
	go func() {
		time.Sleep(200 * time.Millisecond) // let the answer go out first
		if err := restart(r.app); err != nil {
			r.app.Logger().Error("remote/v1: restart failed", "error", err)
		}
	}()
	return remotev1.RestartResult{Restarting: true, Message: "restarting; requests in flight finish first"}, nil
}

func (r *remote) Logs(_ context.Context, _ extkit.Auth, in remotev1.LogsInput) (extkit.Stream, error) {
	n := in.N
	if n <= 0 {
		n = 100
	}
	n = min(n, 1000)
	var initial []*core.Log
	if err := r.app.LogQuery().OrderBy("created DESC", "rowid DESC").Limit(int64(n)).All(&initial); err != nil {
		return extkit.Stream{}, err
	}
	var since types.DateTime
	entries := make([]any, 0, len(initial))
	for i := len(initial) - 1; i >= 0; i-- { // oldest first
		entries = append(entries, logEntry(initial[i]))
		since = initial[i].Created
	}
	seen := map[string]bool{}
	for _, l := range initial {
		if l.Created.Equal(since) {
			seen[l.Id] = true
		}
	}
	poll := func() []any {
		var fresh []*core.Log
		q := r.app.LogQuery().OrderBy("created ASC", "rowid ASC").Limit(1000)
		if !since.IsZero() {
			q = q.AndWhere(dbxGTE("created", since.String()))
		}
		if err := q.All(&fresh); err != nil {
			return nil
		}
		var out []any
		for _, l := range fresh {
			if seen[l.Id] {
				continue
			}
			if !l.Created.Equal(since) {
				since, seen = l.Created, map[string]bool{}
			}
			seen[l.Id] = true
			out = append(out, logEntry(l))
		}
		return out
	}
	return extkit.Stream{Initial: entries, Poll: poll}, nil
}

func logEntry(l *core.Log) remotev1.LogEntry {
	return remotev1.LogEntry{
		Time:    l.Created.Time().Format(time.RFC3339Nano),
		Level:   strings.ToLower(slog.Level(l.Level).String()),
		Message: l.Message,
		Data:    l.Data,
	}
}

package remotes

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	"github.com/pockethost/pocketone/ext/api-tokens"
	remotev1 "github.com/pockethost/pocketone/ext/contracts/remote/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// remoteApp is a test app with remotes and tokens, sealed, served; restart is counted instead of done.
func remoteApp(t *testing.T) (*tests.TestApp, string, *atomic.Int32) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	var restarts atomic.Int32
	cfg := Config{Restart: func(core.App) error { restarts.Add(1); return nil }}
	if _, err := extkit.Setup(app, []extkit.Extension{New(cfg), apitokens.New(apitokens.Config{})}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := apitokens.EnsureSchema(app); err != nil {
		t.Fatal(err)
	}
	return app, serve(t, app), &restarts
}

func token(t *testing.T, app core.App, scopes ...string) string {
	t.Helper()
	su, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	secret, _, err := apitokens.For(app).Create(su, apitokens.CreateOptions{Name: "t", Scopes: scopes})
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func status(err error) int {
	var ce *extkit.Error
	if errors.As(err, &ce) {
		return ce.Status
	}
	return 0
}

func TestRemoteWhoami(t *testing.T) {
	app, url, _ := remoteApp(t)
	ctx := context.Background()

	anon, err := remotev1.NewClient(url, "", nil).Whoami(ctx, extkit.Auth{})
	if err != nil || anon.Auth != nil || anon.Extensions != nil {
		t.Fatalf("anonymous: %+v %v", anon, err)
	}
	if !slices.Contains(anon.Contracts, "remote/v1") || !slices.Contains(anon.Contracts, "oauth/v1") {
		t.Fatalf("contracts: %v", anon.Contracts)
	}
	// any token may ask who it is, whatever its scopes
	w, err := remotev1.NewClient(url, token(t, app, "remote:read"), nil).Whoami(ctx, extkit.Auth{})
	if err != nil || w.Via != "token" || w.Auth == nil || w.Auth.Email != "test@example.com" || len(w.Scopes) != 1 {
		t.Fatalf("scoped token: %+v %v", w, err)
	}
	if len(w.Extensions) != 2 || !slices.Contains(w.Contracts, "api-tokens/v1") {
		t.Fatalf("signed in: extensions %v contracts %v", w.Extensions, w.Contracts)
	}
}

func TestRemoteStatusRestartAndScopes(t *testing.T) {
	app, url, restarts := remoteApp(t)
	ctx := context.Background()

	if _, err := remotev1.NewClient(url, "", nil).Status(ctx, extkit.Auth{}); status(err) != 401 {
		t.Fatalf("anonymous status: %v", err)
	}
	reader := remotev1.NewClient(url, token(t, app, "remote:read"), nil)
	s, err := reader.Status(ctx, extkit.Auth{})
	if err != nil || !s.Running || s.Version == "" || s.Uptime == "" {
		t.Fatalf("status: %+v %v", s, err)
	}
	if _, err := reader.Restart(ctx, extkit.Auth{}); status(err) != 403 {
		t.Fatalf("remote:read restarting: %v", err)
	}
	r, err := remotev1.NewClient(url, token(t, app, "remote"), nil).Restart(ctx, extkit.Auth{})
	if err != nil || !r.Restarting {
		t.Fatalf("restart: %+v %v", r, err)
	}
	for deadline := time.Now().Add(3 * time.Second); restarts.Load() == 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app wasn't restarted")
		}
	}
}

func TestRemoteLogs(t *testing.T) {
	app, url, _ := remoteApp(t)
	ctx := context.Background()
	app.Settings().Logs.MaxDays = 7 // the test app keeps no logs; real apps do
	app.Logger().Info("before the stream")

	st, err := remotev1.NewClient(url, token(t, app, "remote:read"), nil).Logs(ctx, extkit.Auth{}, remotev1.LogsInput{N: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got := func(msg string) bool {
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			for _, e := range st.Poll() {
				if m, _ := e.(map[string]any); m["message"] == msg {
					return true
				}
			}
		}
		return false
	}
	if !got("before the stream") {
		t.Fatal("the stream didn't replay the earlier entry")
	}
	app.Logger().Info("after the stream started")
	if !got("after the stream started") {
		t.Fatal("the stream didn't send the new entry")
	}
}

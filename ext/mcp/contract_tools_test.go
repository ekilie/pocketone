package mcp

import (
	"context"
	"testing"
	"time"

	restorev1 "github.com/pockethost/pocketone/ext/contracts/restore/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// fakeRestorer is a restore/v1 provider that records the moment it was asked for.
type fakeRestorer struct{ asked *time.Time }

func (f fakeRestorer) Points(context.Context, extkit.Auth) (restorev1.Points, error) {
	return restorev1.Points{Provider: "fake", Kind: restorev1.Snapshots, Snapshots: []restorev1.Snapshot{}}, nil
}

func (f fakeRestorer) RestoreTo(_ context.Context, a extkit.Auth, in restorev1.RestoreInput) (restorev1.RestoreResult, error) {
	*f.asked = in.At
	return restorev1.RestoreResult{Restoring: true, Point: in.At, Message: "via " + a.Via}, nil
}

// Every contract the app provides is on /mcp as tools: listed with a schema from its input type, called through the
// contract (so its superuser check applies), hidden from callers who couldn't use it.
func TestContractOperationsAreTools(t *testing.T) {
	var asked time.Time
	h := newHarness(t, Config{}, func(s *Server) {
		restorev1.Contract.Provide(s.App(), fakeRestorer{asked: &asked})
	})
	su := h.superuser()

	r := h.rpc(su, "tools/list", map[string]any{})
	names := toolNames(r)
	if !contains(names, "restore_points") || !contains(names, "restore_restore_to") {
		t.Fatalf("contract tools missing: %v", names)
	}
	res, _ := r.body["result"].(map[string]any)
	for _, tl := range res["tools"].([]any) {
		def := tl.(map[string]any)
		if def["name"] != "restore_restore_to" {
			continue
		}
		props, _ := def["inputSchema"].(map[string]any)["properties"].(map[string]any)
		if at, _ := props["at"].(map[string]any); at["format"] != "date-time" {
			t.Fatalf("restore_restore_to schema: %v", def["inputSchema"])
		}
	}

	out := h.callOK(su, "restore_restore_to", map[string]any{"at": "2026-11-10T15:00:00Z"})
	if asked.IsZero() || out["message"] != "via mcp" {
		t.Fatalf("restore_restore_to: %v (asked %v)", out, asked)
	}
	if p := h.callOK(su, "restore_points", map[string]any{}); p["provider"] != "fake" {
		t.Fatalf("restore_points: %v", p)
	}

	// a signed-in app user who isn't a superuser doesn't see them, and can't call them
	user := h.token("users", "test@example.com")
	if contains(toolNames(h.rpc(user, "tools/list", map[string]any{})), "restore_points") {
		t.Fatal("a user sees restore_points")
	}
	if res := h.call(user, "restore_points", map[string]any{}); res["isError"] != true {
		t.Fatalf("a user called restore_points: %v", res)
	}
}

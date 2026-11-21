package standard_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	apitokens "pocketone.dev/ext/api-tokens"
	"pocketone.dev/ext/contracts/instance/source/v1/sourcetest"
	tokensv1 "pocketone.dev/ext/contracts/instance/tokens/v1"
	"pocketone.dev/ext/pocker"
	"pocketone.dev/ext/standard"
)

// On a Pocker host, instance.tokens/v1 checks a token with the instance that issued it (its standard set's
// ext/api-tokens): valid there, with its owner and scopes; unknown to every other instance.
func TestPockerChecksAnInstancesOwnToken(t *testing.T) {
	var mu sync.Mutex
	apps := map[string]core.App{}
	m := pocker.NewManager(sourcetest.ScopeOf(
		pocker.Instance{ID: "a", Hosts: []string{"a.test"}},
		pocker.Instance{ID: "b", Hosts: []string{"b.test"}},
	), pocker.ManagerOptions{
		PBRoot:   t.TempDir(),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Standard: standard.New(),
		Setup: func(inst pocker.Instance, app core.App) error {
			mu.Lock()
			apps[inst.ID] = app
			mu.Unlock()
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	defer m.Shutdown()
	for deadline := time.Now().Add(10 * time.Second); !m.Ready(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("manager not ready")
		}
	}
	tk := m.Tokens()

	// checking starts the instance; a token it never issued is invalid, not an error
	res, err := tk.Verify(ctx, tokensv1.Input{Instance: "a", Token: "pbt_" + "x0000000000000000000000000000000000000000"[:40]})
	if err != nil || res.Valid || res.Reason != "invalid" {
		t.Fatalf("unknown token: %+v %v", res, err)
	}
	mu.Lock()
	app := apps["a"]
	mu.Unlock()
	col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	su := core.NewRecord(col)
	su.SetEmail("admin@example.com")
	su.SetPassword("password123")
	if err := app.Save(su); err != nil {
		t.Fatal(err)
	}
	secret, _, err := apitokens.For(app).Create(su, apitokens.CreateOptions{Name: "laptop", Scopes: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}

	res, err = tk.Verify(ctx, tokensv1.Input{Instance: "a", Token: secret})
	if err != nil || !res.Valid || res.Grant == nil {
		t.Fatalf("a's own token: %+v %v", res, err)
	}
	if g := res.Grant; g.OwnerCollection != core.CollectionNameSuperusers || g.OwnerID != su.Id || g.Name != "laptop" ||
		len(g.Scopes) != 1 || g.Scopes[0] != "deploy" {
		t.Fatalf("grant: %+v", g)
	}
	if res, err := tk.Verify(ctx, tokensv1.Input{Instance: "b", Token: secret}); err != nil || res.Valid {
		t.Fatalf("a's token on b: %+v %v", res, err)
	}
}

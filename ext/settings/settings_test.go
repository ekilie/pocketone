package settings

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

type fake struct{ saved json.RawMessage }

func (f *fake) Info() settingsv1.Info { return settingsv1.Info{Extension: "fake", Title: "Fake"} }
func (f *fake) Get(context.Context) (json.RawMessage, error) {
	if f.saved == nil {
		return json.RawMessage(`{}`), nil
	}
	return f.saved, nil
}
func (f *fake) Save(_ context.Context, v json.RawMessage) (json.RawMessage, error) {
	f.saved = v
	return v, nil
}
func (f *fake) Test(_ context.Context, v json.RawMessage) error {
	if string(v) == `{"bad":true}` {
		return errors.New("bad settings")
	}
	return nil
}

func TestSettingsThroughTheContract(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	f := &fake{}
	settingsv1.Add(app, f)
	if _, err := extkit.Setup(app, []extkit.Extension{New()}, nil, nil); err != nil {
		t.Fatal(err)
	}
	s := settingsv1.Contract.Must(app)
	ctx, su := context.Background(), extkit.Auth{Kind: "superuser"}
	l, err := s.List(ctx, su)
	if err != nil || len(l.Sections) != 1 || !l.Sections[0].Testable {
		t.Fatalf("list: %+v %v", l, err)
	}
	if _, err := s.Save(ctx, su, settingsv1.Change{Extension: "fake", Values: json.RawMessage(`{"a":1}`)}); err != nil {
		t.Fatal(err)
	}
	if v, err := s.Get(ctx, su, settingsv1.Ref{Extension: "fake"}); err != nil || string(v.Values) != `{"a":1}` {
		t.Fatalf("get: %s %v", v.Values, err)
	}
	if r, err := s.Test(ctx, su, settingsv1.Change{Extension: "fake", Values: json.RawMessage(`{"bad":true}`)}); err != nil || r.OK {
		t.Fatalf("test: %+v %v", r, err)
	}
	if _, err := s.Get(ctx, su, settingsv1.Ref{Extension: "nope"}); !errors.Is(err, settingsv1.ErrUnknown) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := s.List(ctx, extkit.Auth{Kind: "record", Collection: "users"}); !errors.Is(err, extkit.ErrNotSuperuser) {
		t.Fatalf("a user: %v", err)
	}
}

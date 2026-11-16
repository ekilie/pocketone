// Package settings is a PocketOne extension: it provides settings/v1, one API for the settings an app's admins change
// for its extensions. Extensions register their sections with settingsv1.Add; this serves them.
package settings

import (
	"context"
	"encoding/json"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	settingsv1 "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's name.
const Name = "settings"

// Extension provides settings/v1.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements [extkit.Extension].
func (*Extension) Name() string { return Name }

// Contracts implements [extkit.Declarer].
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("settings/v1"), Defaults: extkit.IDs("settings/v1")}
}

// Register implements [extkit.Extension].
func (*Extension) Register(app core.App, _ extkit.Config) error {
	settingsv1.Contract.ProvideDefault(app, provider{app})
	return nil
}

type provider struct{ app core.App }

func (p provider) List(context.Context, extkit.Auth) (settingsv1.List, error) {
	l := settingsv1.List{Sections: []settingsv1.Info{}}
	for _, s := range settingsv1.Sections(p.app) {
		info := s.Info()
		_, info.Testable = s.(settingsv1.Tester)
		_, canSave := s.(settingsv1.Saver)
		info.ReadOnly = !canSave
		l.Sections = append(l.Sections, info)
	}
	return l, nil
}

func (p provider) section(name string) (settingsv1.Section, error) {
	s, ok := settingsv1.SectionOf(p.app, name)
	if !ok {
		return nil, settingsv1.ErrUnknown
	}
	return s, nil
}

func (p provider) Get(ctx context.Context, _ extkit.Auth, in settingsv1.Ref) (settingsv1.Values, error) {
	s, err := p.section(in.Extension)
	if err != nil {
		return settingsv1.Values{}, err
	}
	v, err := s.Get(ctx)
	return settingsv1.Values{Extension: in.Extension, Values: v}, err
}

func (p provider) Save(ctx context.Context, _ extkit.Auth, in settingsv1.Change) (settingsv1.Values, error) {
	s, err := p.section(in.Extension)
	if err != nil {
		return settingsv1.Values{}, err
	}
	saver, ok := s.(settingsv1.Saver)
	if !ok {
		return settingsv1.Values{}, settingsv1.ErrReadOnly
	}
	if len(in.Values) == 0 {
		in.Values = json.RawMessage("{}")
	}
	v, err := saver.Save(ctx, in.Values)
	return settingsv1.Values{Extension: in.Extension, Values: v}, err
}

func (p provider) Test(ctx context.Context, _ extkit.Auth, in settingsv1.Change) (settingsv1.TestResult, error) {
	s, err := p.section(in.Extension)
	if err != nil {
		return settingsv1.TestResult{}, err
	}
	t, ok := s.(settingsv1.Tester)
	if !ok {
		return settingsv1.TestResult{}, settingsv1.ErrNotTestable
	}
	if err := t.Test(ctx, in.Values); err != nil {
		return settingsv1.TestResult{OK: false, Message: err.Error()}, nil
	}
	return settingsv1.TestResult{OK: true, Message: "ok"}, nil
}

package tokens

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's key under an instance's `extensions:`.
const Name = "tokens"

// Extension is API tokens as a PocketOne extension: on a single app with extkit.MustRegister, and under pocker every
// instance gets its own _tokens, routes and screen.
//
//	srv.Extensions = append(srv.Extensions, tokens.New(tokens.Config{}))
//
// Per instance, `extensions: {tokens: {enabled: false}}` turns it off and `collections: [...]` picks who may own
// tokens. Scopes are registered per app by the extensions that own them (they call tokens.For(app).RegisterScope).
type Extension struct{ cfg Config }

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension. cfg holds the defaults for every app.
func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [extkit.Extension].
func (p *Extension) Name() string { return Name }

type settings struct {
	Collections []string `json:"collections"`
}

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, layers extkit.Config) error {
	cfg := p.cfg
	var s settings
	if err := layers.Decode(&s); err != nil {
		return err
	}
	if len(s.Collections) > 0 {
		cfg.Collections = s.Collections
	}
	_, err := Register(app, cfg)
	return err
}

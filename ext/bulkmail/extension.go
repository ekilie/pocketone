package bulkmail

import (
	_ "embed"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// Extension is bulk mail as a PocketOne extension ([extkit.Extension]): the same value works on a standalone app and
// on every instance a multisite host runs on PocketOne. It is a member of ext/standard's set.
//
//	extkit.MustRegister(app, bulkmail.New(bulkmail.Config{})) // one app
//	set.BulkMail = bulkmail.New(bulkmail.Config{})            // the standard set: every app built from it
//
// On Pocker, an instance's `extensions.bulkmail` settings apply over the Config given to New: `rate`, `daily` (0 means this app may not send bulk mail; the screens say so) and
// `enabled`, which the host checks.
type Extension struct {
	cfg Config
}

var (
	_ extkit.Extension = (*Extension)(nil)
	_ extkit.JSBinder  = (*Extension)(nil)
)

// New returns the extension. cfg holds the defaults for every app it's registered on.
func New(cfg Config) *Extension { return &Extension{cfg: cfg} }

// Name implements [extkit.Extension].
func (p *Extension) Name() string { return Name }

// settings is the shape of the extension's settings layers.
type settings struct {
	Rate  *float64 `json:"rate"`
	Daily *int     `json:"daily"`
}

// Contracts implements [extkit.Declarer]: bulk mail's API is these contracts.
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{
		Implements: extkit.IDs("bulkmail/v1", "mail.suppression/v1", "mail.feedback/v1"),
		Defaults:   extkit.IDs("mail.suppression/v1", "mail.feedback/v1"),
		WorksWith:  extkit.IDs("settings/v1"),
	}
}

// Register implements [extkit.Extension].
func (p *Extension) Register(app core.App, cfg extkit.Config) error {
	var s settings
	if err := cfg.Decode(&s); err != nil {
		return err
	}
	c := p.cfg
	if s.Rate != nil && *s.Rate > 0 {
		c.Rate = *s.Rate
	}
	if s.Daily != nil {
		c.Daily = *s.Daily
		if *s.Daily <= 0 {
			c.Daily = NoBulkMail
		}
	}
	_, err := Register(app, c)
	return err
}

// BindJS implements [extkit.JSBinder]: $bulkmail in the app's JS hooks. It only reaches the app's own data, mail
// settings and limits, so it's safe for an untrusted tenant on a multisite host.
func (p *Extension) BindJS(app core.App, rt js.Runtime) {
	if For(app) != nil {
		BindJS(app, rt)
	}
}

//go:embed jsbind.d.ts
var jsTypes string

// TypeScript declares the extension's JS API for types.d.ts (extkit/js Typer).
func (p *Extension) TypeScript() string { return jsTypes }

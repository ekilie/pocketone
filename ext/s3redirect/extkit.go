package s3redirect

import (
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
)

var _ extkit.Extension = (*Extension)(nil)

// Section is the key of the `s3redirect` section (pocketone.yaml): the process defaults, under the values set in Go.
var Section = extkit.NewSection(Name, Config{}, "Signed S3 redirects for file downloads: enabled, URL lifetime. The --s3Redirect and --s3RedirectTTL flags win.")

// settings is the shape of an instance's `extensions.s3redirect`.
type settings struct {
	Enabled *bool            `json:"enabled"`
	TTL     *extkit.Duration `json:"ttl"`
}

// applySection fills what Go left unset from pocketone.yaml's `s3redirect` section, once.
func (p *Extension) applySection() {
	p.sectionOnce.Do(func() {
		c, found := Section.Get()
		if !found {
			return
		}
		if p.cfg.Enabled == nil {
			p.cfg.Enabled = c.Enabled
		}
		if !p.ttlSet && c.TTL > 0 {
			p.cfg.TTL = c.TTL
		}
	})
}

// Register implements [extkit.Extension]: the redirect on app's file downloads, with app's settings over the
// defaults given to [New] and the `s3redirect` section.
func (p *Extension) Register(app core.App, layers extkit.Config) error {
	p.applySection()
	var s settings
	if err := layers.Decode(&s); err != nil {
		return err
	}
	ac := AppConfig{Enabled: s.Enabled}
	if s.TTL != nil {
		ac.TTL = time.Duration(*s.TTL)
	}
	return p.SetupApp(app, ac)
}

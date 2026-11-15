package s3redirect

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// MustRegister adds signed download redirects to a single PocketBase app (call before app.Start). It panics on
// error, like PocketBase's own extensions.
func MustRegister(app core.App, cfg Config) *Extension {
	p, err := Register(app, cfg)
	if err != nil {
		panic(err)
	}
	return p
}

// Register is [MustRegister] returning the error.
func Register(app core.App, cfg Config) (*Extension, error) {
	p := New(cfg)
	if err := p.SetupApp(app, AppConfig{}); err != nil {
		return nil, err
	}
	return p, nil
}

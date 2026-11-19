// Package uicache is a PocketOne extension: Cache-Control headers for the dashboard (/_/) that let browsers and CDNs
// see changes. PocketBase sends `max-age=1209600` (14 days) on every dashboard file but index.html, including files
// whose names never change, so after an extension or PocketOne update a CDN such as Cloudflare can keep serving the old
// /_/extensions.js (every UI extension's main.js, joined) or the old libs/ for two weeks.
//
// It sets the header (unless a middleware before it already did) ahead of PocketBase's own dashboard middleware,
// which leaves a header already set alone:
//
//   - /_/extensions.js and /_/index.html: `no-cache` (checked on every load).
//   - content-hashed files under /_/assets/: left to PocketBase (their URL changes with their content).
//   - every other dashboard file (libs/, fonts/, images/, /_/extensions/<name>/...): `max-age=300`.
package uicache

import (
	"net/http"
	"regexp"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's name.
const Name = "uicache"

// Header values it sends.
const (
	Revalidate = "no-cache"
	Short      = "public, max-age=300"
)

// MiddlewareID is the id of its router middleware.
const MiddlewareID = "uicache"

// Extension sets the dashboard's Cache-Control headers.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements [extkit.Extension].
func (*Extension) Name() string { return Name }

// Register implements [extkit.Extension].
func (*Extension) Register(app core.App, _ extkit.Config) error {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.Bind(&hook.Handler[*core.RequestEvent]{
			Id: MiddlewareID,
			Func: func(e *core.RequestEvent) error {
				if !e.App.IsDev() && (e.Request.Method == http.MethodGet || e.Request.Method == http.MethodHead) {
					if v := Policy(e.Request.URL.Path); v != "" && e.Response.Header().Get("Cache-Control") == "" {
						e.Response.Header().Set("Cache-Control", v)
					}
				}
				return e.Next()
			},
		})
		return se.Next()
	})
	return nil
}

// hashed matches the file names Vite gives bundled files: name-<8 character content hash>.ext.
var hashed = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.[A-Za-z0-9]+$`)

// Policy is the Cache-Control value for a request path, or "" to leave the header to PocketBase.
func Policy(path string) string {
	rest, ok := strings.CutPrefix(path, "/_/")
	if !ok || rest == "" { // not the dashboard, or its root (index.html, which PocketBase sends without max-age)
		return ""
	}
	switch {
	case rest == "extensions.js", rest == "index.html":
		return Revalidate
	case strings.HasPrefix(rest, "assets/") && hashed.MatchString(rest):
		return ""
	}
	return Short
}

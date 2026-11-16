// The OAuth server (oauth/v1): it makes the app an OAuth 2.1 authorization server whose access tokens are this
// package's API tokens. `pocketone login` signs in through it in the browser (loopback redirect + PKCE) or with a device
// code, and MCP clients (Claude, ChatGPT connectors) can register themselves and connect. It's on with tokens
// (Config.OAuth; OAuthConfig.Disabled leaves it out).
//
// Endpoints: /.well-known/oauth-authorization-server (RFC 8414), and under /api/_c/oauth/v1/: authorize (sign-in +
// consent pages), token (authorization code + PKCE S256, refresh, device code), device (RFC 8628), revoke (RFC 7009),
// register (RFC 7591, off by default).

package tokens

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	tokensv1 "github.com/pockethost/pocketone/ext/contracts/tokens/v1"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/router"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/security"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/types"
	oauthv1 "github.com/pockethost/pocketone/ext/contracts/oauth/v1"
)

const (
	oauthStoreKey = "oauthServer"
	paramKey      = "oauth_server"

	// CLIClientId is the built-in public client `pocketone login` uses: loopback redirects and device codes.
	CLIClientId = "pocketbase-cli"

	// MetadataPath is the server's metadata (oauth/v1's; clients find oauth/v1 in remote/v1's whoami).
	MetadataPath = oauthv1.MetadataPath
)

// Branding styles the built-in pages.
type Branding struct {
	// Name shown on the pages (default: Settings → Application name).
	Name string
	// LogoURL, if set, is shown above the form.
	LogoURL string
	// Accent is the button color (CSS color, default #16161a).
	Accent string
	// Footer is plain text under the form.
	Footer string
}

// OAuthConfig is set by whoever builds the binary.
type OAuthConfig struct {
	// Disabled leaves the OAuth server out (on by default with tokens): no oauth/v1, sign-in only with pasted tokens.
	Disabled bool

	// Collections are the auth collections (names) whose records may sign in and authorize clients. Default (empty):
	// _superusers and every auth collection, resolved on each request. The sign-in page signs in app users (the
	// collections other than _superusers) by default and _superusers behind a "Sign in as a superuser" link. Their
	// records must also be allowed to own API tokens: when Config.Collections is narrowed, list them
	// there too (its default is every auth collection). A collection that can't hold tokens shows an error on the
	// page and logs a warning.
	Collections []string

	// AccessTTL is an access token's lifetime (default 30 days); RefreshTTL a refresh token's (default 180 days,
	// renewed on each use).
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	// DynamicRegistration turns on POST /oauth/register (RFC 7591), e.g. for MCP connectors. An extension can also turn
	// it on later with [OAuthServer.EnableDynamicRegistration].
	DynamicRegistration bool

	// CIMDHosts are the hosts whose https client ids are trusted as client ID metadata documents (the client id is
	// the URL of a JSON document with its name and redirect URIs). Default: claude.ai, claude.com. Set to an empty,
	// non-nil slice to turn CIMD off.
	CIMDHosts []string

	// HTTPClient fetches client metadata documents (default: 5 s timeout).
	HTTPClient *http.Client

	// Issuer is the server's public base URL (default: the request's scheme and host; https when the request is TLS or
	// X-Forwarded-Proto says so).
	Issuer string

	Branding Branding

	// Render, if set, renders a page instead of the built-in templates (return handled=false to fall back). When it
	// isn't set and ext/pages is registered with a RenderTemplate method, pages render "oauth/<page>" templates.
	Render func(e *core.RequestEvent, page string, data PageData) (handled bool, err error)

	// Now is the clock (tests).
	Now func() time.Time
}

// OAuthServer is the OAuth server of one app. Get it with [For].
type OAuthServer struct {
	app core.App
	cfg OAuthConfig

	router  *router.Router[*core.RequestEvent] // for in-process calls to the auth API
	muxOnce sync.Once
	mux     http.Handler
	muxErr  error
	warned  sync.Map // collection name -> true: "can't hold tokens" logged

	mu     sync.Mutex
	secret []byte

	limiter *limiter

	dcr  atomic.Bool
	cimd sync.Map // client id URL -> cimdEntry

	ttls sync.Map // resource path -> time.Duration (0: never expires), see SetAccessTTL
}

// SetAccessTTL sets the lifetime of access tokens issued for a resource on this server (by its URL path, e.g.
// "/mcp"); 0 means they never expire (revoke them instead). Other tokens use OAuthConfig.AccessTTL. ext/mcp uses it so an
// agent's connection isn't signed out.
func (s *OAuthServer) SetAccessTTL(resourcePath string, ttl time.Duration) {
	s.ttls.Store(strings.TrimRight(resourcePath, "/"), ttl)
}

func (s *OAuthServer) accessTTL(audience string) time.Duration {
	if audience != "" {
		if u, err := url.Parse(audience); err == nil {
			if v, ok := s.ttls.Load(strings.TrimRight(u.Path, "/")); ok {
				return v.(time.Duration)
			}
		}
	}
	return s.cfg.AccessTTL
}

// Register adds the OAuth server to app: its collections (on bootstrap), endpoints and pages (on serve), oauth/v1,
// and on a *pocketbase.PocketBase the `oauth clients` command. The app must
// also register ext/
func registerOAuth(app core.App, cfg OAuthConfig) (*OAuthServer, error) {
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 30 * 24 * time.Hour
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 180 * 24 * time.Hour
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.CIMDHosts == nil {
		cfg.CIMDHosts = []string{"claude.ai", "claude.com"}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	s := &OAuthServer{app: app, cfg: cfg, limiter: newLimiter(10, 5*time.Minute)}
	s.dcr.Store(cfg.DynamicRegistration)
	app.Store().Set(oauthStoreKey, s)
	oauthv1.Contract.ProvideDefault(app, s) // its routes under /api/_c/oauth/v1/; whoami lists it

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		return EnsureOAuthSchema(e.App)
	})
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		s.router = se.Router
		s.bindRoutes(se)
		return se.Next()
	})
	app.Cron().MustAdd("__pbOAuthCleanup__", "*/30 * * * *", func() {
		if err := s.deleteExpired(); err != nil {
			app.Logger().Warn("tokens: oauth: cleanup", "error", err)
		}
	})
	if pb, ok := app.(*pocketbase.PocketBase); ok {
		pb.RootCmd.AddCommand(NewOAuthCommand(s))
	}
	return s, nil
}

// For returns the OAuth server registered on app, or nil.
func OAuth(app core.App) *OAuthServer {
	s, _ := app.Store().Get(oauthStoreKey).(*OAuthServer)
	return s
}

func (s *OAuthServer) tokens() *Tokens { return For(s.app) }

// EnableDynamicRegistration turns on POST /oauth/register (e.g. ext/mcp does when it registers).
func (s *OAuthServer) EnableDynamicRegistration() { s.dcr.Store(true) }

// OfflineAccess is accepted in scope requests (clients ask for it to get refresh tokens, which this server always
// issues); it isn't stored on
const OfflineAccess = tokensv1.OfflineAccess

// allows: c is an auth collection whose records may sign in (OAuthConfig.Collections, or any auth collection by default).
func (s *OAuthServer) allows(c *core.Collection) bool {
	return c != nil && c.IsAuth() && (len(s.cfg.Collections) == 0 || slices.Contains(s.cfg.Collections, c.Name))
}

// key returns the server's HMAC key (generated once, stored in _params).
func (s *OAuthServer) key() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secret != nil {
		return s.secret
	}
	param := &core.Param{}
	var stored struct {
		Secret string `json:"secret"`
	}
	if err := s.app.ModelQuery(param).Model(paramKey, param); err == nil {
		_ = json.Unmarshal(param.Value, &stored)
	} else {
		param = &core.Param{}
		param.Id = paramKey
		param.MarkAsNew()
		param.Created = types.NowDateTime()
	}
	if stored.Secret == "" {
		stored.Secret = security.RandomString(48)
		raw, _ := json.Marshal(stored)
		param.Value = types.JSONRaw(raw)
		param.Updated = types.NowDateTime()
		if err := s.app.Save(param); err != nil {
			s.app.Logger().Error("tokens: oauth: save key", "error", err)
		}
	}
	s.secret = []byte(stored.Secret)
	return s.secret
}

func (s *OAuthServer) sign(parts ...string) string {
	m := hmac.New(sha256.New, s.key())
	m.Write([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(m.Sum(nil))
}

func oauthHashSecret(v string) string {
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:])
}

// ScopeInfo is a scope as the consent page shows it.
type ScopeInfo struct {
	Name        string
	Description string
	// Checked: on the consent page, the scope's box starts ticked (the client asked for it).
	Checked bool
}

// resolveScopes checks a space-separated scope request against the registered scopes the owner collection may
// hold. Empty: full rights.
func (s *OAuthServer) resolveScopes(raw string, owner *core.Collection) ([]ScopeInfo, error) {
	registered := map[string]Scope{}
	for _, sc := range s.tokens().Scopes() {
		registered[sc.Name] = sc
	}
	var out []ScopeInfo
	seen := map[string]bool{}
	for _, name := range strings.Fields(raw) {
		name = strings.ToLower(name)
		if seen[name] || name == OfflineAccess {
			continue
		}
		seen[name] = true
		sc, ok := registered[name]
		if !ok {
			return nil, fmt.Errorf("unknown scope %q", name)
		}
		if owner != nil && len(sc.Collections) > 0 && !slices.Contains(sc.Collections, owner.Name) {
			return nil, fmt.Errorf("%s accounts can't grant the %q scope", owner.Name, name)
		}
		out = append(out, ScopeInfo{Name: sc.Name, Description: sc.Description})
	}
	return out, nil
}

// scopeChoices are the scopes the consent page lets the user pick from: the ones the client asked for (ticked), then
// every other registered scope the account may hold, except infrastructure ones ([Scope.Infrastructure]).
func (s *OAuthServer) scopeChoices(requested []ScopeInfo, owner *core.Collection) []ScopeInfo {
	out := make([]ScopeInfo, 0, len(requested))
	seen := map[string]bool{}
	for _, r := range requested {
		r.Checked = true
		out = append(out, r)
		seen[r.Name] = true
	}
	for _, sc := range s.tokens().Scopes() {
		if seen[sc.Name] || sc.Infrastructure() {
			continue
		}
		if owner != nil && len(sc.Collections) > 0 && !slices.Contains(sc.Collections, owner.Name) {
			continue
		}
		out = append(out, ScopeInfo{Name: sc.Name, Description: sc.Description})
	}
	return out
}

// chosenScopes reads the consent form: "access" is "full" (no scopes: the account's full rights) or "limited" with
// the ticked "grant" boxes, each one of choices. A form without "access" (a custom page that offers no choice)
// grants what the client asked for.
func chosenScopes(e *core.RequestEvent, requested, choices []ScopeInfo) ([]ScopeInfo, error) {
	switch e.Request.PostFormValue("access") {
	case "":
		return requested, nil
	case "full":
		return nil, nil
	case "limited":
		var out []ScopeInfo
		for _, name := range e.Request.PostForm["grant"] {
			i := slices.IndexFunc(choices, func(c ScopeInfo) bool { return c.Name == name })
			if i < 0 {
				return nil, fmt.Errorf("unknown scope %q", name)
			}
			if !slices.ContainsFunc(out, func(c ScopeInfo) bool { return c.Name == name }) {
				out = append(out, choices[i])
			}
		}
		if len(out) == 0 {
			return nil, errors.New("Tick at least one permission, or choose Full access.")
		}
		return out, nil
	}
	return nil, errors.New("unknown access choice")
}

func scopeNames(in []ScopeInfo) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, s.Name)
	}
	return out
}

// limiter counts failed sign-ins per key (IP) in a window.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) >= l.max
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[key] = append(l.recent(key), time.Now())
	if len(l.hits) > 10000 {
		clear(l.hits)
	}
}

func (l *limiter) recent(key string) []time.Time {
	cut := time.Now().Add(-l.window)
	h := l.hits[key]
	i := 0
	for i < len(h) && h[i].Before(cut) {
		i++
	}
	return h[i:]
}

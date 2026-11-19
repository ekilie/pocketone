package apitokens

import (
	"embed"
	"io/fs"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	templatesv1 "pocketone.dev/ext/contracts/templates/v1"
)

// Pages the server renders.
const (
	PageSignIn  = "signin"  // the collection's sign-in methods: password, "email me a code", OAuth2 providers
	PageMFA     = "mfa"     // the second sign-in step when the collection has MFA on
	PageOTP     = "otp"     // enter the one-time code
	PageConsent = "consent" // allow or deny the client
	PageDevice  = "device"  // enter the code shown on the other device
	PageDone    = "done"    // device flow finished (or denied)
	PageError   = "error"   // a request that can't be redirected back
)

// PageData is everything a page needs. Custom renderers get the same values.
type PageData struct {
	Page string

	// Branding
	AppName, LogoURL, Accent, Footer string

	// Action is the form's URL; Hidden are the fields every form must post back (the OAuth request).
	Action string
	Hidden map[string]string
	// CSRF goes in a hidden "csrf" field of every POST form.
	CSRF string

	// The client asking, and where it will send the user afterwards.
	ClientName string
	ClientHost string
	Scopes     []ScopeInfo
	// FullAccess: no scopes requested, so the token gets the account's full rights. With ScopeChoices, it's
	// whether "Full access" starts selected.
	FullAccess bool
	// ScopeChoices, when not empty, let the user choose on consent: post "access" = "full", or "limited" with a
	// "grant" checkbox per ticked scope (Checked: ticked at first). A form that posts no "access" grants Scopes.
	ScopeChoices []ScopeInfo

	// Email is the signed-in account (consent, mfa), or the address a code went to (otp).
	Email string
	// Collection is the auth collection signing in (or signed in, on consent). Collections are the user
	// collections to choose from on the user form (a choice is shown when there are several).
	Collection  string
	Collections []string
	// Superuser: the page is about the _superusers collection (the superuser form, or consent as a superuser);
	// the pages look clearly different then. AccountKind is "user" or "superuser".
	Superuser   bool
	AccountKind string
	// SuperuserLink: the user form offers "Sign in as a superuser"; UserLink: the superuser form links back.
	SuperuserLink bool
	UserLink      bool
	// The methods the collection has on (PocketBase's auth-methods). IdentityLabel is "Email" or "Email or
	// username". Providers are the OAuth2 providers (never for superusers).
	PasswordEnabled bool
	OTPEnabled      bool
	IdentityLabel   string
	Providers       []Provider
	OTPId           string
	// MFAId is PocketBase's MFA session while the second step is pending (post it back as "mfa_id").
	MFAId string

	UserCode string
	Error    string
	Message  string
}

// Provider is an OAuth2 sign-in option. Its button posts step "oauth2:<Name>".
type Provider struct {
	Name        string
	DisplayName string
}

func (s *OAuthServer) render(e *core.RequestEvent, status int, d PageData) error {
	b := s.cfg.Branding
	if d.AppName == "" {
		d.AppName = b.Name
		if d.AppName == "" {
			d.AppName = s.app.Settings().Meta.AppName
		}
	}
	d.LogoURL, d.Footer = b.LogoURL, b.Footer
	d.Accent = b.Accent
	if d.Accent == "" {
		d.Accent = "#16161a"
	}
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("X-Frame-Options", "DENY")
	e.Response.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")

	if s.cfg.Render != nil {
		if handled, err := s.cfg.Render(e, d.Page, d); handled || err != nil {
			return err
		}
	}
	// the oauth template set: the project's copy of each file (pb_templates/oauth, through templates/v1), else the
	// built-in one
	if t, ok := templatesv1.Contract.Of(s.app); ok {
		return t.Render(e, TemplateSet, d.Page, status, d)
	}
	set, err := templatesv1.Parse(oauthTemplates(), nil)
	if err != nil {
		return err
	}
	return templatesv1.Write(e, set, d.Page, status, d)
}

// TemplateSet is the name of the OAuth pages' template set (templates/v1): `pocketone template eject oauth` copies it
// to pb_templates/oauth/.
const TemplateSet = "oauth"

//go:embed templates/oauth
var oauthFiles embed.FS

// oauthTemplates is the built-in set: one Go html/template file per page, plus the head, foot and hidden pieces.
func oauthTemplates() fs.FS {
	f, err := fs.Sub(oauthFiles, "templates/oauth")
	if err != nil {
		panic(err)
	}
	return f
}

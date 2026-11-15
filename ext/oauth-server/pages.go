package oauthserver

import (
	"bytes"
	"html/template"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
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
	// FullAccess: no scopes requested, so the token gets the account's full rights.
	FullAccess bool

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

// pagesRenderer is what ext/pages offers (duck-typed, so this module doesn't depend on it): render
// pb_pages/_private/<name> with data as ctx.data, or report there's no such template. `pocketone oauth eject`
// writes the built-in pages there.
type pagesRenderer interface {
	RenderTemplate(e *core.RequestEvent, name string, status int, data map[string]any) (handled bool, err error)
}

func (s *Server) render(e *core.RequestEvent, status int, d PageData) error {
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
	if pr, ok := s.app.Store().Get("extPages").(pagesRenderer); ok {
		if handled, err := pr.RenderTemplate(e, "oauth/"+d.Page, status, pageMap(d)); handled || err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, d.Page, d); err != nil {
		return err
	}
	return e.HTML(status, buf.String())
}

func pageMap(d PageData) map[string]any {
	if d.Hidden == nil {
		d.Hidden = map[string]string{}
	}
	if d.Collections == nil {
		d.Collections = []string{}
	}
	scopes := make([]map[string]string, 0, len(d.Scopes))
	for _, s := range d.Scopes {
		scopes = append(scopes, map[string]string{"name": s.Name, "description": s.Description})
	}
	providers := make([]map[string]string, 0, len(d.Providers))
	for _, p := range d.Providers {
		providers = append(providers, map[string]string{"name": p.Name, "displayName": p.DisplayName})
	}
	return map[string]any{
		"page": d.Page, "appName": d.AppName, "logoUrl": d.LogoURL, "accent": d.Accent, "footer": d.Footer,
		"action": d.Action, "hidden": d.Hidden, "csrf": d.CSRF, "clientName": d.ClientName, "clientHost": d.ClientHost,
		"scopes": scopes, "fullAccess": d.FullAccess, "email": d.Email, "collection": d.Collection,
		"collections": d.Collections, "superuser": d.Superuser, "accountKind": d.AccountKind,
		"superuserLink": d.SuperuserLink, "userLink": d.UserLink, "passwordEnabled": d.PasswordEnabled,
		"otpEnabled": d.OTPEnabled, "identityLabel": d.IdentityLabel, "providers": providers, "otpId": d.OTPId,
		"mfaId": d.MFAId, "userCode": d.UserCode, "error": d.Error, "message": d.Message,
	}
}

var pageTemplates = template.Must(template.New("oauth").Parse(`
{{define "head"}}<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>{{.AppName}}</title>
<style>
:root{--accent:{{.Accent}};--bg:#f4f4f6;--card:#fff;--fg:#16161a;--muted:#6b6b76;--line:#e3e3e8;--err:#c62828}
@media (prefers-color-scheme:dark){:root{--bg:#121214;--card:#1c1c20;--fg:#ececf1;--muted:#9a9aa6;--line:#2c2c33}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;
background:var(--bg);color:var(--fg);font:15px/1.5 system-ui,-apple-system,Segoe UI,Roboto,sans-serif;padding:16px}
.card{width:100%;max-width:400px;background:var(--card);border:1px solid var(--line);border-radius:14px;padding:28px}
.logo{display:block;max-height:48px;max-width:160px;margin:0 auto 12px}
h1{font-size:20px;margin:0 0 4px;text-align:center}.sub{color:var(--muted);text-align:center;margin:0 0 20px}
label{display:block;font-size:13px;color:var(--muted);margin:12px 0 4px}
input[type=email],input[type=password],input[type=text],select{width:100%;padding:10px 12px;border:1px solid var(--line);
border-radius:8px;background:transparent;color:inherit;font:inherit}
input:focus,select:focus{outline:2px solid var(--accent);outline-offset:1px}
.btn{display:block;width:100%;margin-top:18px;padding:11px;border:0;border-radius:8px;background:var(--accent);color:#fff;
font:inherit;font-weight:600;cursor:pointer}.btn.secondary{background:transparent;color:var(--fg);border:1px solid var(--line);margin-top:10px}
.link{background:none;border:0;color:var(--muted);font:inherit;text-decoration:underline;cursor:pointer;padding:0;margin-top:14px;display:block;width:100%}
.err{background:color-mix(in srgb,var(--err) 12%,transparent);color:var(--err);border-radius:8px;padding:10px 12px;margin:0 0 12px}
.msg{color:var(--muted);text-align:center}
ul.scopes{list-style:none;padding:0;margin:12px 0;border:1px solid var(--line);border-radius:8px}
ul.scopes li{padding:10px 12px;border-top:1px solid var(--line)}ul.scopes li:first-child{border-top:0}
ul.scopes b{display:block}ul.scopes span{color:var(--muted);font-size:13px}
.code{font:600 22px ui-monospace,Menlo,monospace;letter-spacing:3px;text-align:center;text-transform:uppercase}
.foot{color:var(--muted);font-size:12px;text-align:center;margin-top:18px}
.tabs{display:flex;gap:4px;padding:3px;border:1px solid var(--line);border-radius:9px}
.tab{flex:1;padding:6px 8px;border:0;border-radius:6px;background:transparent;color:var(--muted);font:inherit;font-size:13px;cursor:pointer}
.tab.on{background:var(--accent);color:#fff}
.or{display:flex;align-items:center;gap:10px;color:var(--muted);font-size:12px;margin:18px 0 0}
.or::before,.or::after{content:"";flex:1;border-top:1px solid var(--line)}
.alt{margin:22px 0 0;padding-top:4px;border-top:1px solid var(--line)}
.badge{display:block;width:max-content;margin:0 auto 10px;padding:3px 10px;border-radius:999px;background:var(--accent);
color:#fff;font-size:12px;font-weight:600;letter-spacing:.05em;text-transform:uppercase}
.warn{background:color-mix(in srgb,var(--accent) 12%,transparent);border-left:3px solid var(--accent);border-radius:6px;
padding:10px 12px;margin:0 0 12px;font-size:14px}
body.su{--accent:#c2410c;--bg:#fde8d7}body.su .card{border:2px solid var(--accent)}
@media (prefers-color-scheme:dark){body.su{--bg:#2b1708}}
</style></head><body{{if .Superuser}} class="su"{{end}}><main class="card">
{{if .LogoURL}}<img class="logo" src="{{.LogoURL}}" alt="">{{end}}{{end}}

{{define "foot"}}{{if .Footer}}<p class="foot">{{.Footer}}</p>{{end}}</main></body></html>{{end}}

{{define "hidden"}}{{range $k, $v := .Hidden}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}<input type="hidden" name="csrf" value="{{.CSRF}}">{{end}}

{{define "signin"}}{{template "head" .}}
{{if .Superuser}}<span class="badge">Superuser</span><h1>Superuser sign-in</h1>
<p class="sub">to {{.AppName}}{{if .ClientName}}, to continue to {{.ClientName}}{{end}}</p>
<p class="warn">A superuser has full control of {{.AppName}}. Only sign in here if you mean to give {{if .ClientName}}{{.ClientName}}{{else}}this app{{end}} that power.</p>
{{else}}<h1>Sign in to {{.AppName}}</h1>
<p class="sub">{{if .ClientName}}to continue to {{.ClientName}}{{else}}to continue{{end}}</p>{{end}}
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}<input type="hidden" name="collection" value="{{.Collection}}">
{{if gt (len .Collections) 1}}<div class="tabs">{{range .Collections}}<button class="tab{{if eq . $.Collection}} on{{end}}" name="step" value="use:{{.}}" formnovalidate>{{.}}</button>{{end}}</div>{{end}}
{{if or .PasswordEnabled .OTPEnabled}}<label for="email">{{.IdentityLabel}}</label><input id="email" type="{{if eq .IdentityLabel "Email"}}email{{else}}text{{end}}" name="email" value="{{.Email}}" autocomplete="username" required autofocus>{{end}}
{{if .PasswordEnabled}}<label for="password">Password</label><input id="password" type="password" name="password" autocomplete="current-password">
<button class="btn" name="step" value="password">Sign in</button>{{end}}
{{if .OTPEnabled}}<button class="{{if .PasswordEnabled}}link{{else}}btn{{end}}" name="step" value="request_otp" formnovalidate>Email me a sign-in code</button>{{end}}
{{if .Providers}}{{if or .PasswordEnabled .OTPEnabled}}<p class="or">or</p>{{end}}
{{range .Providers}}<button class="btn secondary" name="step" value="oauth2:{{.Name}}" formnovalidate>Continue with {{.DisplayName}}</button>{{end}}{{end}}
{{if .SuperuserLink}}<p class="alt"><button class="link" name="step" value="use:_superusers" formnovalidate>Sign in as a superuser</button></p>{{end}}
{{if .UserLink}}<p class="alt"><button class="link" name="step" value="use:" formnovalidate>Sign in as a user instead</button></p>{{end}}
</form>{{template "foot" .}}{{end}}

{{define "mfa"}}{{template "head" .}}
{{if .Superuser}}<span class="badge">Superuser</span>{{end}}
<h1>One more step</h1><p class="sub">{{.AppName}} needs a second sign-in method for <b>{{.Email}}</b></p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}<input type="hidden" name="collection" value="{{.Collection}}"><input type="hidden" name="mfa_id" value="{{.MFAId}}">
{{if .PasswordEnabled}}<label for="password">Password</label><input id="password" type="password" name="password" autocomplete="current-password" required autofocus>
<button class="btn" name="step" value="password">Continue</button>{{end}}
{{if .OTPEnabled}}<button class="{{if .PasswordEnabled}}link{{else}}btn{{end}}" name="step" value="request_otp" formnovalidate>Email me a sign-in code</button>{{end}}
{{range .Providers}}<button class="btn secondary" name="step" value="oauth2:{{.Name}}" formnovalidate>Continue with {{.DisplayName}}</button>{{end}}
<p class="alt"><button class="link" name="step" value="use:{{.Collection}}" formnovalidate>Start over</button></p>
</form>{{template "foot" .}}{{end}}

{{define "otp"}}{{template "head" .}}
<h1>Check your email</h1><p class="sub">We sent a code to {{.Email}}</p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}
<input type="hidden" name="collection" value="{{.Collection}}"><input type="hidden" name="email" value="{{.Email}}"><input type="hidden" name="otp_id" value="{{.OTPId}}">{{if .MFAId}}<input type="hidden" name="mfa_id" value="{{.MFAId}}">{{end}}
<label for="code">Code</label><input id="code" type="text" name="code" inputmode="numeric" autocomplete="one-time-code" required autofocus>
<button class="btn" name="step" value="otp">Continue</button></form>{{template "foot" .}}{{end}}

{{define "consent"}}{{template "head" .}}
{{if .Superuser}}<span class="badge">Superuser</span>{{end}}
<h1>{{.ClientName}}</h1><p class="sub">wants to access {{.AppName}} as <b>{{.Email}}</b></p>
{{if .Superuser}}<p class="warn"><b>Superuser account.</b> {{.ClientName}} will act as a superuser, with full control of {{.AppName}}: every record, collection and setting.</p>
{{else}}<p class="msg">User account ({{.Collection}}). {{.ClientName}} can do what this user can.</p>{{end}}
{{if .FullAccess}}<ul class="scopes"><li><b>Full access</b><span>Everything this account can do.</span></li></ul>
{{else}}<ul class="scopes">{{range .Scopes}}<li><b>{{.Name}}</b><span>{{.Description}}</span></li>{{end}}</ul>{{end}}
{{if .ClientHost}}<p class="msg">You'll be sent back to {{.ClientHost}}.</p>{{end}}
<form method="post" action="{{.Action}}">{{template "hidden" .}}
<button class="btn" name="step" value="allow">Allow</button>
<button class="btn secondary" name="step" value="deny">Deny</button>
<button class="link" name="step" value="switch">Not {{.Email}}? Use another account</button></form>{{template "foot" .}}{{end}}

{{define "device"}}{{template "head" .}}
<h1>Connect a device</h1><p class="sub">Enter the code shown on your other device.</p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="get" action="{{.Action}}">
<label for="user_code">Code</label><input id="user_code" class="code" type="text" name="user_code" value="{{.UserCode}}" placeholder="XXXX-XXXX" autocomplete="off" required autofocus>
<button class="btn">Continue</button></form>{{template "foot" .}}{{end}}

{{define "done"}}{{template "head" .}}
<h1>{{if .Error}}Not connected{{else}}All set{{end}}</h1><p class="msg">{{if .Error}}{{.Error}}{{else}}{{.Message}}{{end}}</p>{{template "foot" .}}{{end}}

{{define "error"}}{{template "head" .}}
<h1>Something's wrong</h1><p class="err">{{.Error}}</p>{{template "foot" .}}{{end}}
`))

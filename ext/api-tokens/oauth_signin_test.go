package apitokens

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
)

func authorizeQuery() url.Values {
	_, challenge := pkce()
	return url.Values{"response_type": {"code"}, "client_id": {CLIClientId}, "redirect_uri": {"http://127.0.0.1:1/cb"},
		"state": {"st"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
}

// otpFromMail waits for the OTP mail to email (PocketBase sends it in the background) and returns the code.
func otpFromMail(t *testing.T, e *env, email string, after int) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msgs := e.app.TestMailer.Messages()
		for i := len(msgs) - 1; i >= after; i-- {
			if strings.Contains(msgs[i].Subject, "OTP") && hasRecipient(msgs[i], email) {
				if m := regexp.MustCompile(`\b(\d{6,8})\b`).FindStringSubmatch(msgs[i].HTML); m != nil {
					return m[1]
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no OTP mail to %s", email)
	return ""
}

func hasRecipient(m *mailer.Message, email string) bool {
	for _, a := range m.To {
		if strings.EqualFold(a.Address, email) {
			return true
		}
	}
	return false
}

func TestSignInForms(t *testing.T) {
	e := newEnv(t, OAuthConfig{})
	b := newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	// users by default: the user collections with a sign-in method (users first), their methods, the superuser link
	if b.status != 200 || !b.has("Sign in to") || !b.has(`name="collection" value="users"`) || b.has("Superuser sign-in") ||
		!b.has(`value="use:users"`) || !b.has(`value="use:clients"`) || b.has(`value="use:nologin"`) ||
		!b.has(`value="oauth2:google"`) || !b.has(`value="oauth2:gitlab"`) || !b.has(`value="request_otp"`) ||
		!b.has("Sign in as a superuser") || b.has("Account type") || b.has(`class="su"`) {
		t.Fatalf("user form: %d %s", b.status, b.page)
	}
	// another user collection: its own methods (clients: password only)
	b.submit("use:clients", nil)
	if !b.has(`name="collection" value="clients"`) || b.has("oauth2:") || b.has(`value="request_otp"`) || !b.has(`value="password"`) {
		t.Fatalf("clients form: %s", b.page)
	}
	// the superuser form: clearly different, no providers (PocketBase never allows OAuth2 for superusers)
	b.submit("use:_superusers", nil)
	if !b.has("Superuser sign-in") || !b.has(`class="su"`) || !b.has(`name="collection" value="_superusers"`) ||
		b.has("oauth2:") || b.has("use:clients") || !b.has("Sign in as a user instead") || b.has("Sign in as a superuser") {
		t.Fatalf("superuser form: %s", b.page)
	}
	b.submit("password", map[string]string{"email": "test@example.com", "password": "1234567890"})
	if !b.has("wants to access") || !b.has("Superuser account") || !b.has(`class="su"`) {
		t.Fatalf("superuser consent: %s", b.page)
	}
	b.submit("switch", nil)
	if !b.has(`name="collection" value="users"`) {
		t.Fatalf("switch account: %s", b.page)
	}
	// back from the superuser form
	b.submit("use:_superusers", nil)
	b.submit("use:", nil)
	if !b.has(`name="collection" value="users"`) || b.has("Superuser sign-in") {
		t.Fatalf("back to users: %s", b.page)
	}

	// a user, consent says so
	b.submit("password", map[string]string{"collection": "clients", "email": "test@example.com", "password": "1234567890"})
	if !b.has("wants to access") || !b.has("User account (clients)") || b.has("Superuser account") || b.has(`class="su"`) {
		t.Fatalf("user consent: %s", b.page)
	}
	res := b.submit("allow", nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	st, tok := postToken(t, e.url, url.Values{"grant_type": {"authorization_code"}, "code": {loc.Query().Get("code")},
		"redirect_uri": {"http://127.0.0.1:1/cb"}, "client_id": {CLIClientId}, "code_verifier": {strings.Repeat("v", 50)}})
	if st != 200 {
		t.Fatalf("token: %d %v", st, tok)
	}
	info, err := e.tk.Verify(tok["access_token"].(string))
	if err != nil || info.Owner.Collection().Name != "clients" {
		t.Fatalf("token owner: %v %+v", err, info)
	}
}

func TestSignInNarrowed(t *testing.T) {
	// superusers only: their form is the only one, no link
	e := newEnv(t, OAuthConfig{Collections: []string{core.CollectionNameSuperusers}})
	b := newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	if !b.has("Superuser sign-in") || b.has("Sign in as a user") || b.has("Sign in as a superuser") {
		t.Fatalf("superusers only: %s", b.page)
	}
	b.submit("password", map[string]string{"collection": "users", "email": "test@example.com", "password": "1234567890"})
	if b.status != 401 || !b.has("can&#39;t sign in here") {
		t.Fatalf("users posted anyway: %d %s", b.status, b.page)
	}
	// users only: no superuser link, superusers refused
	e = newEnv(t, OAuthConfig{Collections: []string{"clients"}})
	b = newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	if !b.has(`name="collection" value="clients"`) || b.has("Sign in as a superuser") || b.has(`class="tabs"`) {
		t.Fatalf("clients only: %s", b.page)
	}
	b.submit("password", map[string]string{"collection": "_superusers", "email": "test@example.com", "password": "1234567890"})
	if b.status != 401 || !b.has("can&#39;t sign in here") {
		t.Fatalf("superuser posted anyway: %d %s", b.status, b.page)
	}
}

// When ext/api-tokens is narrowed to superusers, the user form says signing in isn't set up.
func TestSignInUsersWithoutTokens(t *testing.T) {
	e := newEnvTokens(t, OAuthConfig{}, Config{Collections: []string{core.CollectionNameSuperusers}})
	b := newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	if !b.has("isn&#39;t set up on this server") || b.has(`value="password"`) || !b.has("Sign in as a superuser") {
		t.Fatalf("no tokens: %s", b.page)
	}
	b.submit("password", map[string]string{"collection": "clients", "email": "test@example.com", "password": "1234567890"})
	if b.status != 401 || !b.has("can&#39;t hold API tokens") {
		t.Fatalf("posted anyway: %d %s", b.status, b.page)
	}
	b.submit("password", map[string]string{"collection": "_superusers", "email": "test@example.com", "password": "1234567890"})
	if !b.has("Superuser account") {
		t.Fatalf("superuser: %s", b.page)
	}
}

// The collection's auth rule applies: clients requires verified = true.
func TestSignInAuthRule(t *testing.T) {
	e := newEnv(t, OAuthConfig{})
	b := newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	b.submit("password", map[string]string{"collection": "clients", "email": "test2@example.com", "password": "1234567890"})
	if b.status != 401 || !b.has("requirements to sign in") || b.has("wants to access") {
		t.Fatalf("unverified: %d %s", b.status, b.page)
	}
	b.submit("password", map[string]string{"collection": "clients", "email": "test@example.com", "password": "1234567890"})
	if !b.has("User account (clients)") {
		t.Fatalf("verified: %s", b.page)
	}
}

// A method turned off disappears, and posting it anyway fails in PocketBase.
func TestSignInMethodOff(t *testing.T) {
	e := newEnv(t, OAuthConfig{})
	col, _ := e.app.FindCollectionByNameOrId("clients")
	col.PasswordAuth.Enabled = false
	col.OTP.Enabled = true
	if err := e.app.Save(col); err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	b.submit("use:clients", nil)
	if b.has(`value="password"`) || !b.has(`value="request_otp"`) {
		t.Fatalf("methods: %s", b.page)
	}
	b.submit("password", map[string]string{"email": "test@example.com", "password": "1234567890"})
	if b.status != 401 || b.has("wants to access") {
		t.Fatalf("password while off: %d %s", b.status, b.page)
	}
}

// OTP alone, then MFA (users has it on): password → code, and code → password.
func TestSignInOTPAndMFA(t *testing.T) {
	e := newEnv(t, OAuthConfig{})
	col, _ := e.app.FindCollectionByNameOrId("clients")
	col.OTP.Enabled = true
	if err := e.app.Save(col); err != nil {
		t.Fatal(err)
	}

	// OTP, no MFA
	b := newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	n := len(e.app.TestMailer.Messages())
	b.submit("request_otp", map[string]string{"collection": "clients", "email": "test@example.com"})
	if !b.has("Check your email") {
		t.Fatalf("otp page: %s", b.page)
	}
	code := otpFromMail(t, e, "test@example.com", n)
	b.submit("otp", map[string]string{"code": "00000000"})
	if b.status != 401 || !b.has("wrong or expired") {
		t.Fatalf("bad code: %d %s", b.status, b.page)
	}
	b.submit("otp", map[string]string{"code": code})
	if !b.has("User account (clients)") {
		t.Fatalf("otp sign-in: %s", b.page)
	}

	// MFA: password, then a code
	b = newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	b.submit("password", map[string]string{"email": "test@example.com", "password": "1234567890"})
	if !b.has("One more step") || !b.has(`name="mfa_id"`) || b.has(`name="password"`) || !b.has(`value="request_otp"`) || !b.has("oauth2:google") {
		t.Fatalf("mfa page after password: %s", b.page)
	}
	n = len(e.app.TestMailer.Messages())
	b.submit("request_otp", nil)
	if !b.has("Check your email") || !b.has(`name="mfa_id"`) {
		t.Fatalf("mfa otp page: %s", b.page)
	}
	b.submit("otp", map[string]string{"code": otpFromMail(t, e, "test@example.com", n)})
	if !b.has("User account (users)") {
		t.Fatalf("consent after password + code: %s", b.page)
	}

	// MFA: a code, then the password
	b = newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	n = len(e.app.TestMailer.Messages())
	b.submit("request_otp", map[string]string{"email": "test@example.com"})
	b.submit("otp", map[string]string{"code": otpFromMail(t, e, "test@example.com", n)})
	if !b.has("One more step") || !b.has(`name="password"`) || b.has(`value="request_otp"`) {
		t.Fatalf("mfa page after code: %s", b.page)
	}
	b.submit("password", map[string]string{"password": "wrong"})
	if b.status != 401 || !b.has("One more step") || !b.has("Wrong email or password") {
		t.Fatalf("mfa wrong password: %d %s", b.status, b.page)
	}
	b.submit("password", map[string]string{"password": "1234567890"})
	if !b.has("User account (users)") {
		t.Fatalf("consent after code + password: %s", b.page)
	}

	// superusers with MFA and OTP: after the password the code is the only step left, so it's sent right away
	su, _ := e.app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	su.OTP.Enabled, su.MFA.Enabled = true, true
	if err := e.app.Save(su); err != nil {
		t.Fatal(err)
	}
	b = newBrowser(t)
	b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
	n = len(e.app.TestMailer.Messages())
	b.submit("password", map[string]string{"collection": "_superusers", "email": "test@example.com", "password": "1234567890"})
	if !b.has("Check your email") || !b.has(`name="mfa_id"`) {
		t.Fatalf("superuser mfa: %s", b.page)
	}
	b.submit("otp", map[string]string{"code": otpFromMail(t, e, "test@example.com", n)})
	if !b.has("Superuser account") {
		t.Fatalf("superuser consent after MFA: %s", b.page)
	}
}

// fakeOIDC is a minimal OIDC provider: /auth (the browser would go there), /token and /userinfo.
func fakeOIDC(t *testing.T, sub, email string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.PostForm.Get("code") != "good-code" || r.PostForm.Get("code_verifier") == "" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"at","token_type":"bearer","expires_in":3600}`)
		case "/userinfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"sub": sub, "email": email, "email_verified": true, "name": "Oidc User"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSignInOAuth2(t *testing.T) {
	e := newEnv(t, OAuthConfig{})
	oidc := fakeOIDC(t, "oidc-1", "oidc@example.com")
	users, _ := e.app.FindCollectionByNameOrId("users")
	users.MFA.Enabled = false
	users.OAuth2.Providers = []core.OAuth2ProviderConfig{{Name: "oidc", ClientId: "cid", ClientSecret: "secret",
		AuthURL: oidc.URL + "/auth", TokenURL: oidc.URL + "/token", UserInfoURL: oidc.URL + "/userinfo"}}
	if err := e.app.Save(users); err != nil {
		t.Fatal(err)
	}
	callback := e.url + providerCallbackPath

	start := func(b *browser) url.Values {
		t.Helper()
		b.get(e.url + "/api/_c/oauth/v1/authorize?" + authorizeQuery().Encode())
		if !b.has(`value="oauth2:oidc"`) || b.has(`value="oauth2:google"`) {
			t.Fatalf("provider buttons: %s", b.page)
		}
		res := b.submit("oauth2:oidc", nil)
		loc, _ := url.Parse(res.Header.Get("Location"))
		q := loc.Query()
		if res.StatusCode != 302 || !strings.HasPrefix(loc.String(), oidc.URL+"/auth?") || q.Get("redirect_uri") != callback ||
			q.Get("state") == "" || q.Get("code_challenge") == "" || q.Get("client_id") != "cid" {
			t.Fatalf("to the provider: %d %s", res.StatusCode, loc)
		}
		return q
	}

	// wrong state
	b := newBrowser(t)
	start(b)
	b.get(callback + "?state=nope&code=good-code")
	if b.status != 400 || !b.has("expired or was already used") {
		t.Fatalf("wrong state: %d %s", b.status, b.page)
	}
	// the provider refuses the code
	b = newBrowser(t)
	q := start(b)
	b.get(callback + "?state=" + url.QueryEscape(q.Get("state")) + "&code=bad")
	if b.status != 401 || !b.has("failed") || b.has("wants to access") {
		t.Fatalf("bad code: %d %s", b.status, b.page)
	}

	// success: a new user signed up by PocketBase, then consent and a token
	b = newBrowser(t)
	q = start(b)
	b.get(callback + "?state=" + url.QueryEscape(q.Get("state")) + "&code=good-code")
	if b.status != 200 || !b.has("oidc@example.com") || !b.has("User account (users)") {
		t.Fatalf("consent after OAuth2: %d %s", b.status, b.page)
	}
	res := b.submit("allow", nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != 302 || loc.Query().Get("code") == "" || loc.Query().Get("state") != "st" {
		t.Fatalf("allow: %d %s", res.StatusCode, loc)
	}
	if _, err := e.app.FindAuthRecordByEmail("users", "oidc@example.com"); err != nil {
		t.Fatalf("not signed up: %v", err)
	}
	// the state is one use
	b.get(callback + "?state=" + url.QueryEscape(q.Get("state")) + "&code=good-code")
	if b.status != 400 {
		t.Fatalf("state reused: %d", b.status)
	}

	// MFA on: the provider is the first step, the page asks for another
	users.MFA.Enabled = true
	if err := e.app.Save(users); err != nil {
		t.Fatal(err)
	}
	b = newBrowser(t)
	q = start(b)
	b.get(callback + "?state=" + url.QueryEscape(q.Get("state")) + "&code=good-code")
	if !b.has("One more step") || !b.has("oidc@example.com") || b.has("oauth2:oidc") || !b.has(`value="request_otp"`) {
		t.Fatalf("mfa after OAuth2: %s", b.page)
	}

	// device flow through a provider comes back to the device page
	b = newBrowser(t)
	devRes, err := (&http.Client{Transport: &http.Transport{}}).PostForm(e.url+"/api/_c/oauth/v1/device", url.Values{"client_id": {CLIClientId}})
	if err != nil {
		t.Fatal(err)
	}
	var dev map[string]any
	json.NewDecoder(devRes.Body).Decode(&dev)
	devRes.Body.Close()
	users.MFA.Enabled = false
	if err := e.app.Save(users); err != nil {
		t.Fatal(err)
	}
	b.get(e.url + "/api/_c/oauth/v1/device/verify?user_code=" + url.QueryEscape(dev["user_code"].(string)))
	r := b.submit("oauth2:oidc", nil)
	dq, _ := url.Parse(r.Header.Get("Location"))
	b.get(callback + "?state=" + url.QueryEscape(dq.Query().Get("state")) + "&code=good-code")
	if !b.has("User account (users)") || !b.has(`action="/api/_c/oauth/v1/device/verify"`) {
		t.Fatalf("device consent after OAuth2: %s", b.page)
	}
	b.submit("allow", nil)
	if !b.has("is connected") {
		t.Fatalf("device allow: %s", b.page)
	}
}

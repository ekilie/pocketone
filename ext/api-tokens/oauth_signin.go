package apitokens

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Sign-in: who may sign in, the pages, and the steps. Every check of a password, code or provider answer is
// PocketBase's own auth endpoint, called in-process (api.go); the record it returns is the signed-in account.

const (
	providerCallbackPath = "/api/_c/oauth/v1/provider/callback"
	providerCookie       = "pb_oauth_provider"
	providerTTL          = 10 * time.Minute
)

const (
	badCredentials = "Wrong email or password."
	badCode        = "That code is wrong or expired."
	tooMany        = "Too many attempts. Wait a few minutes and try again."
)

// ---------------------------------------------------------------------------------------------------------------
// who may sign in

// hasSignInMethod: the collection has password, OTP or an OAuth2 provider turned on.
func hasSignInMethod(c *core.Collection) bool {
	return c.PasswordAuth.Enabled || c.OTP.Enabled || (c.OAuth2.Enabled && len(c.OAuth2.Providers) > 0)
}

// signInCollections resolves who may sign in right now: the user collections (allowed auth collections other than
// _superusers that have a sign-in method on, "users" first) and _superusers, if allowed.
func (s *OAuthServer) signInCollections() (users []*core.Collection, super *core.Collection) {
	var cols []*core.Collection
	if len(s.cfg.Collections) == 0 {
		all, err := s.app.FindAllCollections(core.CollectionTypeAuth)
		if err != nil {
			s.app.Logger().Warn("api-tokens: oauth: list auth collections", "error", err)
		}
		slices.SortStableFunc(all, func(a, b *core.Collection) int { return strings.Compare(a.Name, b.Name) })
		cols = all
	} else {
		for _, n := range s.cfg.Collections {
			if c, err := s.app.FindCachedCollectionByNameOrId(n); err == nil && c.IsAuth() {
				cols = append(cols, c)
			}
		}
	}
	for _, c := range cols {
		switch {
		case c.Name == core.CollectionNameSuperusers:
			super = c
		case hasSignInMethod(c):
			users = append(users, c)
		}
	}
	// the default user form is "users" when there is one
	slices.SortStableFunc(users, func(a, b *core.Collection) int {
		switch {
		case a.Name == "users" && b.Name != "users":
			return -1
		case b.Name == "users" && a.Name != "users":
			return 1
		}
		return 0
	})
	return users, super
}

// pickCollection returns the allowed collection by name; "" is the default form (the first user collection, else
// _superusers).
func (s *OAuthServer) pickCollection(name string) (*core.Collection, error) {
	users, super := s.signInCollections()
	if name == "" {
		if len(users) > 0 {
			return users[0], nil
		}
		if super != nil {
			return super, nil
		}
		return nil, errors.New("Nobody can sign in here: no account type is set up for it.")
	}
	for _, c := range users {
		if c.Name == name {
			return c, nil
		}
	}
	if super != nil && super.Name == name {
		return super, nil
	}
	return nil, errors.New("This account type can't sign in here.")
}

// stepCollection is the collection a sign-in step posts, which must be able to hold the tokens it would get.
func (s *OAuthServer) stepCollection(e *core.RequestEvent) (*core.Collection, error) {
	col, err := s.pickCollection(e.Request.PostFormValue("collection"))
	if err != nil {
		return nil, err
	}
	if !s.tokens().AllowedCollection(col) {
		return nil, errors.New(s.noTokensMessage(col))
	}
	return col, nil
}

// noTokensMessage: the collection may sign in but ext/api-tokens won't let its records own  The page says so in
// plain words; the log says what to change.
func (s *OAuthServer) noTokensMessage(col *core.Collection) string {
	if _, done := s.warned.LoadOrStore(col.Name, true); !done {
		s.app.Logger().Warn("api-tokens: oauth: this collection may sign in but can't own API tokens; add it to apitokens.Config.Collections",
			"collection", col.Name)
	}
	return "Signing in with a " + col.Name + " account isn't set up on this server yet (its accounts can't hold API tokens)."
}

// accountName is how the pages name an account: its email, else its username, else its id.
func accountName(rec *core.Record) string {
	if v := rec.Email(); v != "" {
		return v
	}
	if v := rec.GetString("username"); v != "" {
		return v
	}
	return rec.Id
}

// ---------------------------------------------------------------------------------------------------------------
// pages

// fillAccount sets the account kind fields of a page about col.
func (s *OAuthServer) fillAccount(d *PageData, col *core.Collection) {
	d.Collection = col.Name
	d.Superuser = col.Name == core.CollectionNameSuperusers
	d.AccountKind = "user"
	if d.Superuser {
		d.AccountKind = "superuser"
	}
}

// fillMethods sets the sign-in methods col offers (GET auth-methods), leaving out skip (the method an MFA sign-in
// already used). False if they can't be read.
func (s *OAuthServer) fillMethods(e *core.RequestEvent, d *PageData, col *core.Collection, skip string) bool {
	am, err := s.authMethods(e, col)
	if err != nil {
		s.app.Logger().Error("api-tokens: oauth: auth methods", "collection", col.Name, "error", err)
		return false
	}
	d.PasswordEnabled = am.Password.Enabled && skip != core.MFAMethodPassword
	d.OTPEnabled = am.OTP.Enabled && skip != core.MFAMethodOTP
	d.IdentityLabel = "Email"
	if am.Password.Enabled && slices.ContainsFunc(am.Password.IdentityFields, func(f string) bool { return f != core.FieldNameEmail }) {
		d.IdentityLabel = "Email or username"
	}
	d.Providers = nil
	if am.OAuth2.Enabled && skip != core.MFAMethodOAuth2 && col.Name != core.CollectionNameSuperusers {
		for _, p := range am.OAuth2.Providers {
			d.Providers = append(d.Providers, Provider{Name: p.Name, DisplayName: p.DisplayName})
		}
	}
	return true
}

// signInPage renders the sign-in form for col (nil: the default form).
func (s *OAuthServer) signInPage(e *core.RequestEvent, d PageData, col *core.Collection, errMsg, email string) error {
	d.Page, d.Error, d.Email = PageSignIn, errMsg, email
	users, super := s.signInCollections()
	if col == nil || (d.Account != "" && col.Name != d.Account) {
		var err error
		if col, err = s.pickCollection(d.Account); err != nil {
			d.Page, d.Error = PageError, err.Error()
			return s.render(e, http.StatusForbidden, d)
		}
	}
	s.fillAccount(&d, col)
	if !d.Superuser {
		for _, c := range users {
			d.Collections = append(d.Collections, c.Name)
		}
	}
	d.SuperuserLink = !d.Superuser && super != nil
	d.UserLink = d.Superuser && len(users) > 0
	if d.Account != "" { // the client needs this account type: no other forms to switch to
		d.Collections, d.SuperuserLink, d.UserLink = nil, false, false
	}
	status := http.StatusOK
	if !s.tokens().AllowedCollection(col) {
		if d.Error == "" {
			d.Error = s.noTokensMessage(col)
		}
	} else if !s.fillMethods(e, &d, col, "") && d.Error == "" {
		d.Error = "Sign-in isn't available right now. Try again later."
	}
	if errMsg != "" {
		status = http.StatusUnauthorized
	}
	return s.render(e, status, d)
}

// mfaState is a sign-in waiting for its second factor: PocketBase's _mfas record.
type mfaState struct {
	id     string
	method string // the first method: password, otp, oauth2
	record *core.Record
}

// loadMFA reads the MFA session id; nil when there's none.
func (s *OAuthServer) loadMFA(col *core.Collection, id string) (*mfaState, error) {
	if id == "" {
		return nil, nil
	}
	expired := errors.New("Your sign-in expired. Start again.")
	m, err := s.app.FindMFAById(id)
	if err != nil || m.CollectionRef() != col.Id || m.HasExpired(col.MFA.DurationTime()) {
		return nil, expired
	}
	rec, err := s.app.FindRecordById(col, m.RecordRef())
	if err != nil {
		return nil, expired
	}
	return &mfaState{id: m.Id, method: m.Method(), record: rec}, nil
}

// mfaPage asks for the second factor: the collection's other methods. When an emailed code is the only one left,
// it's sent right away.
func (s *OAuthServer) mfaPage(e *core.RequestEvent, d PageData, col *core.Collection, m *mfaState, errMsg string) error {
	d.Page, d.Error, d.Email, d.MFAId = PageMFA, errMsg, accountName(m.record), m.id
	s.fillAccount(&d, col)
	if !s.fillMethods(e, &d, col, m.method) && d.Error == "" {
		d.Error = "Sign-in isn't available right now. Try again later."
	}
	if errMsg == "" && d.OTPEnabled && !d.PasswordEnabled && len(d.Providers) == 0 {
		return s.requestOTP(e, d, col, m.record.Email(), m)
	}
	status := http.StatusOK
	if errMsg != "" {
		status = http.StatusUnauthorized
	}
	return s.render(e, status, d)
}

func (s *OAuthServer) otpPage(e *core.RequestEvent, d PageData, col *core.Collection, email, otpId string, m *mfaState, errMsg string) error {
	d.Page, d.Error, d.Email, d.OTPId = PageOTP, errMsg, email, otpId
	s.fillAccount(&d, col)
	if m != nil {
		d.MFAId = m.id
	}
	status := http.StatusOK
	if errMsg != "" {
		status = http.StatusUnauthorized
	}
	return s.render(e, status, d)
}

// again shows a failed step's page with msg: the MFA page during MFA, else the sign-in form.
func (s *OAuthServer) again(e *core.RequestEvent, d PageData, col *core.Collection, email string, m *mfaState, msg string) error {
	if m != nil {
		return s.mfaPage(e, d, col, m, msg)
	}
	return s.signInPage(e, d, col, msg, email)
}

// ---------------------------------------------------------------------------------------------------------------
// steps

// stepStart reads what every step posts: the collection, the MFA session (if any) and the email (the form's, or
// the MFA account's).
func (s *OAuthServer) stepStart(e *core.RequestEvent, d PageData) (col *core.Collection, m *mfaState, email string, done error) {
	email = strings.TrimSpace(e.Request.PostFormValue("email"))
	col, err := s.stepCollection(e)
	if err != nil {
		return nil, nil, "", s.signInPage(e, d, nil, err.Error(), email)
	}
	if m, err = s.loadMFA(col, e.Request.PostFormValue("mfa_id")); err != nil {
		return nil, nil, "", s.signInPage(e, d, col, err.Error(), "")
	}
	if m != nil {
		email = m.record.Email()
	}
	if s.limiter.blocked(e.RealIP()) {
		return nil, nil, "", s.again(e, d, col, email, m, tooMany)
	}
	return col, m, email, nil
}

func (s *OAuthServer) stepPassword(e *core.RequestEvent, ar *authRequest, d PageData) error {
	col, m, email, done := s.stepStart(e, d)
	if col == nil {
		return done
	}
	body := map[string]any{"identity": email, "password": e.Request.PostFormValue("password")}
	if m != nil {
		body["identity"], body["identityField"] = m.identity(col)
		body["mfaId"] = m.id
	}
	res, err := s.callAPI(e, http.MethodPost, collectionAPI(col, "auth-with-password"), body)
	if err != nil {
		return err
	}
	return s.finishAuth(e, ar, d, col, res, func(msg string) error { return s.again(e, d, col, email, m, msg) },
		badCredentials)
}

// identity is what the MFA account signs in with by password: its email, else its first other identity field.
func (m *mfaState) identity(col *core.Collection) (string, string) {
	for _, f := range col.PasswordAuth.IdentityFields {
		if v := m.record.GetString(f); v != "" {
			return v, f
		}
	}
	return m.record.Email(), core.FieldNameEmail
}

func (s *OAuthServer) stepRequestOTP(e *core.RequestEvent, _ *authRequest, d PageData) error {
	col, m, email, done := s.stepStart(e, d)
	if col == nil {
		return done
	}
	return s.requestOTP(e, d, col, email, m)
}

func (s *OAuthServer) requestOTP(e *core.RequestEvent, d PageData, col *core.Collection, email string, m *mfaState) error {
	res, err := s.callAPI(e, http.MethodPost, collectionAPI(col, "request-otp"), map[string]any{"email": email})
	if err != nil {
		return err
	}
	if res.status != http.StatusOK || res.str("otpId") == "" {
		msg := apiMessage(res, "Enter a valid email address.")
		if res.status == http.StatusForbidden {
			msg = "Sign-in codes are off for this account type."
		}
		return s.again(e, d, col, email, m, msg)
	}
	return s.otpPage(e, d, col, email, res.str("otpId"), m, "")
}

func (s *OAuthServer) stepOTP(e *core.RequestEvent, ar *authRequest, d PageData) error {
	col, m, email, done := s.stepStart(e, d)
	if col == nil {
		return done
	}
	otpId := e.Request.PostFormValue("otp_id")
	body := map[string]any{"otpId": otpId, "password": strings.TrimSpace(e.Request.PostFormValue("code"))}
	if m != nil {
		body["mfaId"] = m.id
	}
	res, err := s.callAPI(e, http.MethodPost, collectionAPI(col, "auth-with-otp"), body)
	if err != nil {
		return err
	}
	return s.finishAuth(e, ar, d, col, res, func(msg string) error { return s.otpPage(e, d, col, email, otpId, m, msg) },
		badCode)
}

// finishAuth handles an auth endpoint's answer: signed in (200), a second factor wanted (401 with mfaId), or a
// failure (counted against the IP; fail renders it, with badMsg for a plain 400).
func (s *OAuthServer) finishAuth(e *core.RequestEvent, ar *authRequest, d PageData, col *core.Collection, res apiResponse,
	fail func(msg string) error, badMsg string) error {
	switch {
	case res.status == http.StatusOK:
		rec, err := s.app.FindAuthRecordByToken(res.str("token"), core.TokenTypeAuth)
		if err != nil || rec.Collection().Id != col.Id {
			return fail("Sign-in failed. Try again.")
		}
		return s.signedIn(e, ar, d, rec)
	case res.status == http.StatusUnauthorized && res.str("mfaId") != "":
		m, err := s.loadMFA(col, res.str("mfaId"))
		if err != nil {
			return fail(err.Error())
		}
		return s.mfaPage(e, d, col, m, "")
	}
	s.limiter.fail(e.RealIP())
	return fail(apiMessage(res, badMsg))
}

// apiMessage turns an auth endpoint's error into words for the page; bad is the message for a plain 400 (wrong
// password, wrong code, invalid input).
func apiMessage(res apiResponse, bad string) string {
	msg := res.str("message")
	switch res.status {
	case http.StatusBadRequest:
		if strings.Contains(msg, "MFA") || strings.Contains(msg, "different authentication method") {
			return msg
		}
		return bad
	case http.StatusForbidden:
		if strings.Contains(msg, "collection requirements") {
			return "This account doesn't meet the app's requirements to sign in (for example, its email isn't verified)."
		}
		if msg == "" {
			return "This account can't sign in from here."
		}
		return msg
	case http.StatusTooManyRequests:
		return tooMany
	}
	if msg == "" {
		msg = "Sign-in failed. Try again."
	}
	return msg
}

func (s *OAuthServer) signedIn(e *core.RequestEvent, ar *authRequest, d PageData, rec *core.Record) error {
	tok, err := rec.NewStaticAuthToken(sessionTTL)
	if err != nil {
		return err
	}
	e.SetCookie(&http.Cookie{Name: sessionCookie, Value: tok, Path: "/api/_c/oauth/v1", HttpOnly: true, Secure: e.IsTLS(),
		SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL.Seconds())})
	return s.consentPage(e, ar, d, rec)
}

// ---------------------------------------------------------------------------------------------------------------
// OAuth2 providers

// pendingProvider is a sign-in sent to a provider, kept in a signed cookie until it comes back.
type pendingProvider struct {
	State        string            `json:"state"`
	Provider     string            `json:"provider"`
	Collection   string            `json:"collection"`
	CodeVerifier string            `json:"codeVerifier"`
	RedirectURL  string            `json:"redirectURL"`
	MFAId        string            `json:"mfaId,omitempty"`
	Action       string            `json:"action"` // authorizePath or deviceVerifyPath
	Request      map[string]string `json:"request"`
}

func (s *OAuthServer) providerCookie(e *core.RequestEvent, value string, maxAge int) {
	c := &http.Cookie{Name: providerCookie, Value: value, Path: "/api/_c/oauth/v1", HttpOnly: true, Secure: e.IsTLS(),
		SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
	if e.IsTLS() {
		c.SameSite = http.SameSiteNoneMode // Apple answers with a cross-site POST (form_post)
	}
	e.SetCookie(c)
}

// stepOAuth2 sends the browser to the provider (state and PKCE from PocketBase's auth-methods).
func (s *OAuthServer) stepOAuth2(e *core.RequestEvent, d PageData, provider string) error {
	col, m, email, done := s.stepStart(e, d)
	if col == nil {
		return done
	}
	am, err := s.authMethods(e, col)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(am.OAuth2.Providers, func(p authProvider) bool { return p.Name == provider })
	if !am.OAuth2.Enabled || i < 0 || col.Name == core.CollectionNameSuperusers {
		return s.again(e, d, col, email, m, "That sign-in option isn't available.")
	}
	p := am.OAuth2.Providers[i]
	callback := s.Issuer(e) + providerCallbackPath
	st := pendingProvider{State: p.State, Provider: p.Name, Collection: col.Name, CodeVerifier: p.CodeVerifier,
		RedirectURL: callback, Action: d.Action, Request: d.Hidden}
	if m != nil {
		st.MFAId = m.id
	}
	raw, _ := json.Marshal(st)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	s.providerCookie(e, payload+"."+s.sign("provider", payload), int(providerTTL.Seconds()))
	// auth-methods' authURL ends with an empty redirect_uri for the caller to fill in
	return e.Redirect(http.StatusFound, p.AuthURL+url.QueryEscape(callback))
}

func (s *OAuthServer) readPending(e *core.RequestEvent) (*pendingProvider, bool) {
	c, err := e.Request.Cookie(providerCookie)
	if err != nil {
		return nil, false
	}
	payload, sig, ok := strings.Cut(c.Value, ".")
	if !ok || subtle.ConstantTimeCompare([]byte(sig), []byte(s.sign("provider", payload))) != 1 {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	st := &pendingProvider{}
	if err != nil || json.Unmarshal(raw, st) != nil || st.State == "" {
		return nil, false
	}
	return st, true
}

// providerCallback is where providers send the browser back: check the state, then PocketBase's auth-with-oauth2
// (account linking, sign-up, hooks), then consent as usual.
func (s *OAuthServer) providerCallback(e *core.RequestEvent) error {
	if e.Request.Method == http.MethodPost {
		_ = e.Request.ParseForm()
	}
	st, ok := s.readPending(e)
	s.providerCookie(e, "", -1) // one use
	expired := PageData{Page: PageError, Error: "This sign-in expired or was already used. Start again from the app you were connecting."}
	if !ok || subtle.ConstantTimeCompare([]byte(param(e, "state")), []byte(st.State)) != 1 {
		return s.render(e, http.StatusBadRequest, expired)
	}

	// the OAuth request it was for
	var ar *authRequest
	if uc := st.Request["user_code"]; uc != "" {
		var err error
		if ar, err = s.deviceRequest(uc); err != nil {
			expired.Error = err.Error()
			return s.render(e, http.StatusBadRequest, expired)
		}
	} else {
		var problem string
		var canRedirect bool
		ar, problem, canRedirect = s.parseAuthorizeWith(e, func(k string) string { return st.Request[k] })
		if !canRedirect {
			expired.Error = problem
			return s.render(e, http.StatusBadRequest, expired)
		}
		if problem != "" {
			return ar.errorRedirect(e, problem)
		}
	}
	d := s.basePage(e, ar, st.Action)
	col, err := s.pickCollection(st.Collection)
	if err != nil {
		return s.signInPage(e, d, nil, err.Error(), "")
	}
	m, err := s.loadMFA(col, st.MFAId)
	if err != nil {
		return s.signInPage(e, d, col, err.Error(), "")
	}
	if s.limiter.blocked(e.RealIP()) {
		return s.again(e, d, col, "", m, tooMany)
	}
	code := param(e, "code")
	if code == "" || param(e, "error") != "" {
		return s.again(e, d, col, "", m, "Sign-in with "+st.Provider+" was cancelled or failed. Try again.")
	}
	body := map[string]any{"provider": st.Provider, "code": code, "codeVerifier": st.CodeVerifier, "redirectURL": st.RedirectURL}
	if m != nil {
		body["mfaId"] = m.id
	}
	res, err := s.callAPI(e, http.MethodPost, collectionAPI(col, "auth-with-oauth2"), body)
	if err != nil {
		return err
	}
	return s.finishAuth(e, ar, d, col, res, func(msg string) error { return s.again(e, d, col, "", m, msg) },
		"Sign-in with "+st.Provider+" failed. Try again.")
}

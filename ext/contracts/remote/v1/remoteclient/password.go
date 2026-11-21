package remoteclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PasswordTokenTTL is how long a password login lasts: the session is swapped for a token of this length (PocketBase's
// impersonate, which a superuser may do for itself), so it lasts like a browser login. Changing the superuser's
// password ends it.
const PasswordTokenTTL = 30 * 24 * time.Hour

// PasswordLogin signs in as one of the remote's superusers with email and password, through PocketBase's own auth
// API. It's for a remote with no browser sign-in (stock PocketBase, such as an instance a multisite host runs in
// Docker): the host checks the superuser's token with the instance. The session token is swapped for one of
// [PasswordTokenTTL] when the remote allows it.
func PasswordLogin(ctx context.Context, c *Client, email, password string) (*LoginResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, errors.New("enter the email and password of one of the app's superusers")
	}
	anon := *c
	anon.Token = ""
	var auth struct {
		Token  string `json:"token"`
		MFAID  string `json:"mfaId"`
		Record struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"record"`
	}
	err := anon.JSON(ctx, http.MethodPost, "/api/collections/_superusers/auth-with-password",
		map[string]string{"identity": email, "password": password}, &auth)
	var ae *APIError
	switch {
	case errors.As(err, &ae) && ae.Status == http.StatusUnauthorized && strings.Contains(fmt.Sprint(ae.Body), "mfaId"):
		return nil, errors.New("this superuser has multi-factor authentication on, which the CLI can't do yet: pass a superuser token with --token")
	case errors.As(err, &ae) && (ae.Status == http.StatusBadRequest || ae.Status == http.StatusUnauthorized):
		return nil, fmt.Errorf("%s didn't accept that superuser email and password", c.Remote.URL)
	case err != nil:
		return nil, err
	case auth.MFAID != "":
		return nil, errors.New("this superuser has multi-factor authentication on, which the CLI can't do yet: pass a superuser token with --token")
	case auth.Token == "":
		return nil, errors.New("the sign-in returned no token")
	}

	token := auth.Token
	signed := anon
	signed.Token = auth.Token
	var imp struct {
		Token string `json:"token"`
	}
	if err := signed.JSON(ctx, http.MethodPost, "/api/collections/_superusers/impersonate/"+auth.Record.ID,
		map[string]int{"duration": int(PasswordTokenTTL / time.Second)}, &imp); err == nil && imp.Token != "" {
		token = imp.Token
	}
	now := time.Now().UTC()
	cred := &Credential{Token: token, Via: "password", Email: auth.Record.Email, Collection: "_superusers", Created: now,
		Expires: jwtExpiry(token)}
	if cred.Email == "" {
		cred.Email = email
	}

	// the host must take the instance's superusers on its own routes (deploy, remote): ask it, when it says who you are
	probe := *c
	probe.Token = token
	w, err := probe.Whoami(ctx)
	switch {
	case errors.Is(err, ErrNoWhoami):
	case err != nil:
		return nil, err
	case w.Auth == nil:
		return nil, fmt.Errorf("%s accepted the password, but its host doesn't take the app's superusers for deploys yet: sign in with a token from the host (--token)", c.Remote.URL)
	}
	return &LoginResult{Credential: cred}, nil
}

// jwtExpiry reads a JWT's exp claim (zero when it has none or isn't a JWT).
func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(b, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0).UTC()
}

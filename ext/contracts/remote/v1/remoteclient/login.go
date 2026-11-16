package remoteclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// LoginResult is a finished sign-in, ready to save.
type LoginResult struct {
	Credential *Credential
	// Warning, if set, is shown to the user.
	Warning string
	// TokenId is the API token's id, when the remote reports one.
	TokenId string
}

// LoginWithToken checks a pasted token (pbt_..., a PocketHost token, or a JWT) with the remote's whoami when it has
// one; a remote without whoami (a PocketHost edge today) takes it as is.
func LoginWithToken(ctx context.Context, c *Client, token string) (*LoginResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("empty token")
	}
	probe := *c
	probe.Token = token
	cred := &Credential{Token: token, Via: "token", Created: time.Now().UTC()}
	w, err := probe.Whoami(ctx)
	switch {
	case errors.Is(err, ErrNoWhoami):
		return &LoginResult{Credential: cred}, nil
	case err != nil:
		return nil, err
	case w.Auth == nil:
		return nil, fmt.Errorf("%s rejected the token", c.Remote.URL)
	}
	cred.Email, cred.Collection = w.Auth.Email, w.Auth.Collection
	if w.Via == "jwt" {
		cred.Via = "jwt"
	}
	return &LoginResult{Credential: cred, TokenId: w.TokenId}, nil
}

// SaveLogin stores a login for the client's remote URL.
func SaveLogin(c *Client, res *LoginResult) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	cred := *res.Credential
	creds.ByURL[c.Remote.URL] = &cred
	return creds.Save()
}

package remoteclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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

// LoginOptions configure [Login].
type LoginOptions struct {
	// Token, if set, is used as is (pbt_..., a PocketHost token, or a JWT).
	Token string
	// TokenFrom, if set, gives the token on its first line (--token-stdin).
	TokenFrom io.Reader
	// Browser is the OAuth sign-in tried first on a remote that offers oauth/v1 (normally [OAuthLogin]); nil skips it.
	// It may return (nil, nil) to fall back to the token prompt.
	Browser   func(ctx context.Context, c *Client, w *Whoami, opts BrowserLoginOptions) (*LoginResult, error)
	Device    bool // device code instead of a browser redirect
	NoBrowser bool // print the sign-in URL instead of opening a browser
	// Prompt is where a pasted token is read when nothing else signed in; nil means there's no terminal to ask in.
	Prompt io.Reader
	// Out gets the prompts and the result line.
	Out io.Writer
}

// Login signs in to the client's remote, saves the login and says who it signed in as: with opts.Token or
// opts.TokenFrom when given, else in the browser when the remote offers OAuth, else with a token pasted at opts.Prompt.
// It's the sign-in behind `login`, `link` and `clone`.
func Login(ctx context.Context, c *Client, opts LoginOptions) (*LoginResult, error) {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	c.Token, c.TokenSource = "", ""
	var (
		res *LoginResult
		err error
	)
	readLine := func(r io.Reader) (string, error) {
		line, err := bufio.NewReader(r).ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return line, nil
	}
	switch {
	case opts.TokenFrom != nil:
		line, rerr := readLine(opts.TokenFrom)
		if rerr != nil {
			return nil, rerr
		}
		res, err = LoginWithToken(ctx, c, line)
	case opts.Token != "":
		res, err = LoginWithToken(ctx, c, opts.Token)
	default:
		// OAuth first, when the remote offers it
		if opts.Browser != nil {
			wctx, wcancel := context.WithTimeout(ctx, 20*time.Second)
			w, werr := c.Whoami(wctx)
			wcancel()
			if werr != nil && !errors.Is(werr, ErrNoWhoami) {
				return nil, werr
			}
			if w != nil && w.Offers("oauth/v1") {
				res, err = opts.Browser(ctx, c, w, BrowserLoginOptions{Device: opts.Device, NoBrowser: opts.NoBrowser, Out: out})
				if err != nil {
					return nil, err
				}
			}
		}
		if res == nil {
			if opts.Prompt == nil {
				return nil, errors.New("no terminal to prompt in: pass --token, --token-stdin, or set " + EnvVar("TOKEN"))
			}
			fmt.Fprintf(out, "Signing in to %s.\nPaste an API token: ", c.Remote.URL)
			line, rerr := readLine(opts.Prompt)
			if rerr != nil {
				return nil, rerr
			}
			res, err = LoginWithToken(ctx, c, line)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := SaveLogin(c, res); err != nil {
		return nil, err
	}
	who := res.Credential.Email
	if who == "" {
		who = "token"
	}
	fmt.Fprintf(out, "Logged in to %s as %s (%s).\n", c.Remote.URL, who, res.Credential.Via)
	if res.Warning != "" {
		fmt.Fprintln(out, "Note: "+res.Warning)
	}
	return res, nil
}

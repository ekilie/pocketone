package remoteclient

import (
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
	// Prompt is where the sign-in is asked for when nothing else signed in (a superuser's email, or a pasted token);
	// nil means there's no terminal to ask in.
	Prompt io.Reader
	// Password reads a password without echoing it (nil: read a line from Prompt).
	Password func(prompt string) (string, error)
	// Out gets the prompts and the result line.
	Out io.Writer
}

// Login signs in to the client's remote, saves the login and says who it signed in as: with opts.Token or
// opts.TokenFrom when given, else in the browser when the remote offers OAuth, else at opts.Prompt with a superuser's
// email and password ([PasswordLogin]) or a pasted token.
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
	// one byte at a time: a buffered read could take the next line too (a password typed ahead, read without echo
	// from the same terminal afterwards)
	readLine := func(r io.Reader) (string, error) {
		var line []byte
		b := make([]byte, 1)
		for {
			n, err := r.Read(b)
			if n == 1 {
				if b[0] == '\n' {
					return string(line), nil
				}
				line = append(line, b[0])
			}
			if err == io.EOF {
				return string(line), nil
			}
			if err != nil {
				return "", err
			}
		}
	}
	switch {
	case opts.TokenFrom != nil:
		if opts.Prompt != nil { // a person at a terminal, not a pipe
			fmt.Fprint(out, "Paste the token: ")
		}
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
			fmt.Fprintf(out, "Sign in to %s as one of its superusers.\nEmail (or paste an API token): ", c.Remote.URL)
			line, rerr := readLine(opts.Prompt)
			if rerr != nil {
				return nil, rerr
			}
			line = strings.TrimSpace(line)
			if looksLikeToken(line) {
				res, err = LoginWithToken(ctx, c, line)
				break
			}
			var pw string
			if opts.Password != nil {
				pw, rerr = opts.Password("Password: ")
			} else {
				fmt.Fprint(out, "Password: ")
				pw, rerr = readLine(opts.Prompt)
				pw = strings.TrimRight(pw, "\r\n")
			}
			if rerr != nil {
				return nil, rerr
			}
			res, err = PasswordLogin(ctx, c, line, pw)
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

// looksLikeToken tells a pasted token (an API token or a JWT) from an email at the sign-in prompt.
func looksLikeToken(s string) bool {
	if strings.Contains(s, "@") {
		return false
	}
	return strings.HasPrefix(s, "pbt_") || strings.HasPrefix(s, "ph_pat_") || strings.Count(s, ".") == 2
}

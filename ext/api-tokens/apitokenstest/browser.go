// Package tokenstest plays the user in the browser for the OAuth server in ext/api-tokens, so tests can run the CLI's
// browser sign-in (`login`, `link`) end to end without a browser.
package apitokenstest

import (
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Login is who signs in on the sign-in pages.
type Login struct {
	Collection string // auth collection, e.g. "_superusers"
	Email      string
	Password   string
}

// Approve opens authURL (the URL the CLI would open in a browser), signs in, checks the consent screen names the
// client ("pocketone CLI"), clicks Allow and follows the redirect back to the CLI's loopback listener.
func Approve(authURL string, login Login) error {
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 10 * time.Second, Transport: browserTransport(),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	u, err := url.Parse(authURL)
	if err != nil {
		return err
	}
	if id := u.Query().Get("client_id"); id != "pocketone-cli" {
		return fmt.Errorf("client_id %q, want pocketone-cli", id)
	}
	page, _, err := fetch(c, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	page, _, err = submit(c, u, page, "password", map[string]string{"collection": login.Collection, "email": login.Email, "password": login.Password})
	if err != nil {
		return err
	}
	if !strings.Contains(page, "pocketone CLI") || !strings.Contains(page, "wants to access") {
		return fmt.Errorf("consent page:\n%s", page)
	}
	_, res, err := submit(c, u, page, "allow", nil)
	if err != nil {
		return err
	}
	back := res.Header.Get("Location")
	if !strings.HasPrefix(back, "http://127.0.0.1:") || !strings.Contains(back, "code=") {
		return fmt.Errorf("allow redirected to %q", back)
	}
	backURL, err := url.Parse(back)
	if err != nil {
		return err
	}
	_, res, err = fetch(c, http.MethodGet, backURL, nil)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("loopback answered %d", res.StatusCode)
	}
	return nil
}

// Fail sends an OAuth error to the CLI's redirect URI, the way the server does when the user denies, so a sign-in
// waiting on its loopback listener fails at once with cause instead of waiting out its timeout.
func Fail(authURL string, cause error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return
	}
	q := u.Query()
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		return
	}
	back.RawQuery = url.Values{"state": {q.Get("state")}, "error": {"test_browser"}, "error_description": {cause.Error()}}.Encode()
	c := &http.Client{Timeout: 5 * time.Second, Transport: browserTransport()}
	if res, err := c.Get(back.String()); err == nil {
		res.Body.Close()
	}
}

// browserTransport is the "browser"'s own connection pool. A real browser is another process, so it never shares the
// CLI's http.DefaultTransport, which a Pocker host in the same test binary rewires through its egress policy (that
// blocks loopback, and would keep Fail from ever reaching the CLI's listener).
func browserTransport() *http.Transport {
	return &http.Transport{Proxy: nil, DisableKeepAlives: true}
}

// ApproveOrFail is [Approve], then [Fail] if it didn't work. It returns Approve's error.
func ApproveOrFail(authURL string, login Login) error {
	err := Approve(authURL, login)
	if err != nil {
		Fail(authURL, err)
	}
	return err
}

var (
	hiddenRE = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)
	formRE   = regexp.MustCompile(`<form method="post" action="([^"]+)">`)
)

func submit(c *http.Client, base *url.URL, page, step string, fields map[string]string) (string, *http.Response, error) {
	m := formRE.FindStringSubmatch(page)
	if m == nil {
		return "", nil, errors.New("no form on page:\n" + page)
	}
	form := url.Values{}
	for _, h := range hiddenRE.FindAllStringSubmatch(page, -1) {
		form.Set(h[1], html.UnescapeString(h[2]))
	}
	for k, v := range fields {
		form.Set(k, v)
	}
	form.Set("step", step)
	target, err := base.Parse(html.UnescapeString(m[1]))
	if err != nil {
		return "", nil, err
	}
	return fetch(c, http.MethodPost, target, strings.NewReader(form.Encode()))
}

func fetch(c *http.Client, method string, u *url.URL, body io.Reader) (string, *http.Response, error) {
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return "", nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	res, err := c.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return string(b), res, fmt.Errorf("%s %s: %d\n%s", method, u.Path, res.StatusCode, b)
	}
	return string(b), res, err
}

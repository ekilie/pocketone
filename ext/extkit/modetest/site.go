package modetest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// Site is one instance as a client sees it.
type Site struct {
	t     *testing.T
	ID    string
	Host  string // its hostname, <id>.test
	URL   string // http://<id>.test:<port>, the same shape in every mode
	Dir   string // its folder (pb_data, pb_hooks, ... inside)
	Proc  *Proc  // the process that runs it (a single app, or its host)
	owner string // the host id that runs it ("" for a single app)
	dial  string // the address requests go to (its owner, or another host with World.Via)
	jwt   string
}

// TrapURL is the dev mail trap inbox of the process that runs the instance. On a host, every dev instance shares it.
func (s *Site) TrapURL() string { return s.Proc.TrapURL }

// Client is an HTTP client that sends every request for any hostname to this site's process, so URLs the app builds
// from its own URL (links in mail, redirects) work as they are.
func (s *Site) Client() *http.Client { return clientFor(s.dial) }

// JWT is a superuser auth token (cached).
func (s *Site) JWT() string {
	s.t.Helper()
	if s.jwt == "" {
		var out struct {
			Token string `json:"token"`
		}
		s.MustJSON(http.MethodPost, "/api/collections/_superusers/auth-with-password", "",
			map[string]string{"identity": SuperuserEmail, "password": SuperuserPassword}, 200, &out)
		s.jwt = out.Token
	}
	return s.jwt
}

// Token creates an API token for the superuser (no scopes: the owner's full rights).
func (s *Site) Token(scopes ...string) string {
	s.t.Helper()
	if scopes == nil {
		scopes = []string{}
	}
	var out struct {
		Token string `json:"token"`
	}
	s.MustJSON(http.MethodPost, "/api/_c/tokens/v1", s.JWT(), map[string]any{"name": "modetest", "scopes": scopes}, 200, &out)
	if !strings.HasPrefix(out.Token, "pbt_") {
		s.t.Fatalf("token: %q", out.Token)
	}
	return out.Token
}

// Raw sends one request (path relative to the site, or a full URL; body nil, a string, or anything JSON-encodable)
// and returns status and body. A transport error is status 0 with the error as the body.
func (s *Site) Raw(method, path, auth string, body any) (int, []byte) {
	u := path
	if !strings.Contains(path, "://") {
		u = s.URL + path
	}
	return request(s.dial, u, method, auth, body)
}

// MustJSON sends a request, fails unless the status is want, and decodes the answer into out (if not nil).
func (s *Site) MustJSON(method, path, auth string, body any, want int, out any) []byte {
	s.t.Helper()
	code, b := s.Raw(method, path, auth, body)
	if code != want {
		s.t.Fatalf("%s %s %s: %d, want %d\n%s", s.ID, method, path, code, want, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			s.t.Fatalf("%s %s %s: decode: %v\n%s", s.ID, method, path, err, b)
		}
	}
	return b
}

// Get is the body of GET path, or "<status> <body>" when it isn't 200.
func (s *Site) Get(path string) string {
	code, b := s.Raw(http.MethodGet, path, "", nil)
	if code != 200 {
		return fmt.Sprintf("%d %s", code, b)
	}
	return string(b)
}

var (
	clientsMu sync.Mutex
	clients   = map[string]*http.Client{}
)

// clientFor is a client that connects to addr whatever the URL's host, keeping the URL's Host header.
func clientFor(addr string) *http.Client {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if c, ok := clients[addr]; ok {
		return c
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	c := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return d.DialContext(ctx, network, addr)
			},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	clients[addr] = c
	return c
}

func request(addr, u, method, auth string, body any) (int, []byte) {
	var r io.Reader
	ctype := ""
	switch b := body.(type) {
	case nil:
	case string:
		r, ctype = strings.NewReader(b), "application/json"
	default:
		j, _ := json.Marshal(b)
		r, ctype = bytes.NewReader(j), "application/json"
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return 0, []byte(err.Error())
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res, err := clientFor(addr).Do(req)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

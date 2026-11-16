package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// One stock app with everything on, checked from the outside: what it says at startup, what it serves (pb_hooks,
// pb_pages, pb_public), MCP and OAuth discovery, contracts with API tokens and their scopes, settings, and a restart
// asked for over remote/v1.
func TestE2ESingleSite(t *testing.T) {
	s := startSite(t, siteOptions{Files: map[string]string{
		"pb_hooks/probe.pb.js":    `routerAdd("GET", "/probe", (e) => e.json(200, {mcp: typeof $mcp, tokens: typeof $tokens, bulkmail: typeof $bulkmail}))`,
		"pb_pages/hello-page.ejs": `<p>pages <%= 1 + 1 %></p>`,
		"pb_public/static.txt":    "static file",
	}})

	t.Run("startup lines", func(t *testing.T) {
		log := s.Log.String()
		for _, want := range []string{
			"standard: on: settings, tokens, remotes, mcp, pages, s3redirect, pitr, backups, bulkmail, deploy",
			"dev mail trap is active", s.TrapURL,
			"restore/v1 ← pitr", "oauth/v1 ← tokens", "deploy/v1 ← deploy",
		} {
			if !strings.Contains(log, want) {
				t.Errorf("startup log lacks %q", want)
			}
		}
	})

	t.Run("powered by header", func(t *testing.T) {
		res, err := http.Get(s.URL + "/api/health")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if h := res.Header.Get("X-Powered-By"); !strings.HasPrefix(h, "PocketOne ") {
			t.Fatalf("X-Powered-By %q", h)
		}
	})

	t.Run("hooks see the standard JS APIs", func(t *testing.T) {
		var got map[string]string
		s.mustJSON(http.MethodGet, "/probe", "", nil, 200, &got)
		if got["mcp"] != "object" || got["tokens"] != "object" || got["bulkmail"] != "object" {
			t.Fatalf("/probe: %v", got)
		}
	})

	t.Run("pages and public", func(t *testing.T) {
		if got := getBody(s.URL + "/hello-page"); !strings.Contains(got, "pages 2") {
			t.Fatalf("pb_pages: %s", got)
		}
		if got := getBody(s.URL + "/static.txt"); got != "static file" {
			t.Fatalf("pb_public: %s", got)
		}
	})

	t.Run("mcp and oauth discovery", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, s.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 || !strings.Contains(res.Header.Get("WWW-Authenticate"), "resource_metadata") {
			t.Fatalf("/mcp anonymous: %d %q", res.StatusCode, res.Header.Get("WWW-Authenticate"))
		}
		var md map[string]any
		s.mustJSON(http.MethodGet, "/.well-known/oauth-authorization-server", "", nil, 200, &md)
		if md["authorization_endpoint"] == nil || md["token_endpoint"] == nil {
			t.Fatalf("oauth metadata: %v", md)
		}
		tools := mcpCall(t, s, s.Token(), "tools/list", nil)
		for _, want := range []string{`"restore_points"`, `"pb_list_records"`} {
			if !strings.Contains(tools, want) {
				t.Errorf("tools/list lacks %s:\n%.2000s", want, tools)
			}
		}
	})

	t.Run("contracts with API tokens", func(t *testing.T) {
		full := s.Token()
		var w struct {
			Via       string   `json:"via"`
			Contracts []string `json:"contracts"`
		}
		s.mustJSON(http.MethodGet, "/api/_c/remote/v1", full, nil, 200, &w)
		if w.Via != "token" {
			t.Fatalf("whoami via %q", w.Via)
		}
		got := strings.Join(w.Contracts, " ")
		for _, c := range []string{"bulkmail/v1", "deploy/v1", "oauth/v1", "remote/v1", "restore/v1", "settings/v1", "tokens/v1", "mail.suppression/v1"} {
			if !strings.Contains(got, c) {
				t.Errorf("whoami contracts lack %s: %s", c, got)
			}
		}
		var st map[string]any
		s.mustJSON(http.MethodGet, "/api/_c/remote/v1/status", full, nil, 200, &st)
		if st["running"] != true {
			t.Fatalf("status %v", st)
		}

		if code, b := s.raw(http.MethodGet, "/api/_c/restore/v1", "", nil); code != 401 {
			t.Fatalf("restore/v1 anonymous: %d %s", code, b)
		}
		ro := s.Token("restore:read")
		s.mustJSON(http.MethodGet, "/api/_c/restore/v1", ro, nil, 200, nil)
		if code, b := s.raw(http.MethodPost, "/api/_c/restore/v1/restore", ro, map[string]string{"at": time.Now().Format(time.RFC3339)}); code != 403 {
			t.Fatalf("restore with a restore:read token: %d %s", code, b)
		}
		if code, b := s.raw(http.MethodGet, "/api/collections/users/records", ro, nil); code < 400 {
			t.Fatalf("records API with a scoped token: %d %s", code, b)
		}
		if code, b := s.raw(http.MethodPost, "/mcp", ro, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); code < 400 {
			t.Fatalf("/mcp with a scoped token: %d %s", code, b)
		}
	})

	t.Run("settings", func(t *testing.T) {
		var out struct {
			Sections []struct {
				Extension string `json:"extension"`
			} `json:"sections"`
		}
		s.mustJSON(http.MethodGet, "/api/_c/settings/v1", s.Token("settings:read"), nil, 200, &out)
		var names []string
		for _, sec := range out.Sections {
			names = append(names, sec.Extension)
		}
		if !strings.Contains(strings.Join(names, " "), "pitr") {
			t.Fatalf("settings sections: %v", names)
		}
	})

	// last: it restarts the process
	t.Run("restart over remote/v1", func(t *testing.T) {
		var rr map[string]any
		s.mustJSON(http.MethodPost, "/api/_c/remote/v1/restart", s.Token("remote"), map[string]any{}, 200, &rr)
		if rr["restarting"] != true {
			t.Fatalf("restart: %v", rr)
		}
		s.waitRestarted("remote/v1 restart", 20*time.Second)
		if got := getBody(s.URL + "/probe"); !strings.Contains(got, `"mcp":"object"`) {
			t.Fatalf("after restart /probe: %s", got)
		}
	})
}

// mcpCall sends one JSON-RPC call to /mcp and returns the raw answer.
func mcpCall(t *testing.T, s *site, auth, method string, params any) string {
	t.Helper()
	call := func(m string, p any, id int) (int, string) {
		body := map[string]any{"jsonrpc": "2.0", "method": m}
		if id > 0 {
			body["id"] = id
		}
		if p != nil {
			body["params"] = p
		}
		j, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, s.URL+"/mcp", strings.NewReader(string(j)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer "+auth)
		res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var sb strings.Builder
		buf := make([]byte, 32<<10)
		for {
			n, err := res.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return res.StatusCode, sb.String()
	}
	code, out := call(method, params, 1)
	if code != 200 {
		t.Fatalf("mcp %s: %d %s", method, code, out)
	}
	return out
}

// The config file: found in the working directory or named with --config; a switch in it takes effect; an unknown
// section stops startup with its name.
func TestE2EConfigFile(t *testing.T) {
	t.Run("pocketone.yaml in the app folder", func(t *testing.T) {
		s := startSite(t, siteOptions{Files: map[string]string{"pocketone.yaml": "pitr:\n  enabled: false\n"}})
		assertRestoreProvider(t, s, "backups")
	})

	t.Run("--config elsewhere", func(t *testing.T) {
		s := startSite(t, siteOptions{
			Files: map[string]string{"conf/custom.yaml": "pitr:\n  enabled: false\n"},
			Args:  []string{"--config", "conf/custom.yaml"},
		})
		assertRestoreProvider(t, s, "backups")
	})

	t.Run("pitr on by default", func(t *testing.T) {
		s := startSite(t, siteOptions{})
		assertRestoreProvider(t, s, "pitr")
	})

	t.Run("unknown section stops startup", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "pocketone.yaml"), "not_a_section:\n  x: 1\n")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		out, err := runBin(ctx, pocketoneBin(t), dir, baseEnv(t), "serve", "--dir=pb_data", "--http="+freeAddr(t)).CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("serve kept running with an unknown section:\n%s", out)
		}
		var ee *exec.ExitError
		if err == nil || !errorAs(err, &ee) || !strings.Contains(string(out), "not_a_section") {
			t.Fatalf("serve: %v\n%s", err, out)
		}
	})
}

func assertRestoreProvider(t *testing.T, s *site, want string) {
	t.Helper()
	var pts struct {
		Provider string `json:"provider"`
	}
	s.mustJSON(http.MethodGet, "/api/_c/restore/v1", s.JWT(), nil, 200, &pts)
	if pts.Provider != want {
		t.Fatalf("restore/v1 provider %q, want %q", pts.Provider, want)
	}
}

// --dev=false: no dev mail trap and no SQL log lines.
func TestE2EProductionMode(t *testing.T) {
	s := startSite(t, siteOptions{Args: []string{"--dev=false"}})
	s.mustJSON(http.MethodGet, "/api/_c/remote/v1", s.JWT(), nil, 200, nil)
	log := s.Log.String()
	if strings.Contains(log, "dev mail trap") || strings.Contains(log, "SELECT ") {
		t.Fatalf("production mode log has dev output:\n%s", tail(log, 40))
	}
	if code, _ := rawRequest(s.TrapURL, http.MethodGet, "/api/messages", "", nil); code != 0 {
		t.Fatalf("mail trap answers in production mode: %d", code)
	}
}

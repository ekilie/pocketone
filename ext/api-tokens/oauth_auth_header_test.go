package apitokens

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// PocketBase v0.42 answers 401 to any Authorization header that isn't a known PocketBase token. The OAuth server
// reads an OAuth client's "Basic" credentials (client_secret_basic) from that header, and its metadata is public, so
// both must still reach the handler.
func TestOAuthAuthorizationHeaderReachesHandler(t *testing.T) {
	e := newEnv(t, OAuthConfig{})
	var out bytes.Buffer
	cmd := NewOAuthCommand(e.srv)
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"clients", "add", "--name", "Web", "--redirect", "https://app.example.com/cb"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`client_id: (\S+)\nclient_secret: (\S+)`).FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("no client id/secret in: %s", out.String())
	}
	client := &http.Client{Transport: &http.Transport{}}

	t.Run("client_secret_basic at the token endpoint", func(t *testing.T) {
		form := url.Values{"grant_type": {"authorization_code"}, "code": {"unknown"}, "redirect_uri": {"https://app.example.com/cb"}}
		req, _ := http.NewRequest(http.MethodPost, e.url+tokenPath, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(m[1], m[2])
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		// the client authenticates; the made-up code is what fails (invalid_grant), not the Authorization header
		if !strings.Contains(string(b), "invalid_grant") {
			t.Fatalf("want invalid_grant from the handler, got %d %s", res.StatusCode, b)
		}
	})

	t.Run("client_secret_basic with a wrong secret", func(t *testing.T) {
		form := url.Values{"grant_type": {"authorization_code"}, "code": {"unknown"}}
		req, _ := http.NewRequest(http.MethodPost, e.url+tokenPath, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(m[1], "wrong")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if !strings.Contains(string(b), "invalid_client") {
			t.Fatalf("want invalid_client from the handler, got %d %s", res.StatusCode, b)
		}
	})

	t.Run("client_secret_basic at the revoke endpoint", func(t *testing.T) {
		form := url.Values{"token": {"unknown"}}
		req, _ := http.NewRequest(http.MethodPost, e.url+revokePath, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(m[1], m[2])
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		// RFC 7009: revoking an unknown token succeeds once the client authenticates
		if res.StatusCode != http.StatusOK {
			t.Fatalf("want 200 from the handler, got %d %s", res.StatusCode, b)
		}
	})

	t.Run("metadata with a stale bearer token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, e.url+MetadataPath, nil)
		req.Header.Set("Authorization", "Bearer stale")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || !strings.Contains(string(b), "token_endpoint") {
			t.Fatalf("want the metadata, got %d %.120s", res.StatusCode, b)
		}
	})
}

package remoteclient

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeJWT(exp time.Time) string {
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	return enc(map[string]string{"alg": "HS256"}) + "." + enc(map[string]any{"id": "su1", "exp": exp.Unix()}) + ".sig"
}

// fakeApp is stock PocketBase's superuser auth plus a host's whoami that knows the long token (acceptHost) or not.
func fakeApp(t *testing.T, acceptHost bool, mfa bool) (*httptest.Server, string) {
	long := fakeJWT(time.Now().Add(PasswordTokenTTL))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/collections/_superusers/auth-with-password", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		switch {
		case in["identity"] != "admin@example.com" || in["password"] != "password":
			w.WriteHeader(400)
			fmt.Fprint(w, `{"message":"Failed to authenticate.","status":400}`)
		case mfa:
			w.WriteHeader(401)
			fmt.Fprint(w, `{"mfaId":"m1"}`)
		default:
			fmt.Fprintf(w, `{"token":%q,"record":{"id":"su1","email":"admin@example.com"}}`, fakeJWT(time.Now().Add(time.Hour)))
		}
	})
	mux.HandleFunc("POST /api/collections/_superusers/impersonate/su1", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]int
		json.NewDecoder(r.Body).Decode(&in)
		if in["duration"] != int(PasswordTokenTTL/time.Second) || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("impersonate: %v %q", in, r.Header.Get("Authorization"))
		}
		fmt.Fprintf(w, `{"token":%q}`, long)
	})
	mux.HandleFunc("GET /api/_c/remote/v1/", func(w http.ResponseWriter, r *http.Request) {
		if acceptHost && r.Header.Get("Authorization") == "Bearer "+long {
			fmt.Fprint(w, `{"auth":{"collection":"_superusers","id":"su1"},"via":"jwt","contracts":["deploy/v1","remote/v1"]}`)
			return
		}
		fmt.Fprint(w, `{"auth":null,"contracts":["deploy/v1","remote/v1"]}`)
	})
	return httptest.NewServer(mux), long
}

func testClient(url string) *Client {
	return &Client{Remote: &Remote{Name: "default", URL: url}, HTTP: NewHTTPClient()}
}

func TestPasswordLogin(t *testing.T) {
	srv, long := fakeApp(t, true, false)
	defer srv.Close()
	res, err := PasswordLogin(t.Context(), testClient(srv.URL), "admin@example.com", "password")
	if err != nil {
		t.Fatal(err)
	}
	c := res.Credential
	if c.Token != long || c.Via != "password" || c.Email != "admin@example.com" || c.Collection != "_superusers" {
		t.Fatalf("credential %+v", c)
	}
	if d := time.Until(c.Expires); d < PasswordTokenTTL-time.Minute || d > PasswordTokenTTL+time.Minute {
		t.Fatalf("expires in %s", d)
	}
}

func TestPasswordLoginRefusals(t *testing.T) {
	srv, _ := fakeApp(t, false, false)
	defer srv.Close()
	if _, err := PasswordLogin(t.Context(), testClient(srv.URL), "admin@example.com", "nope"); err == nil || !strings.Contains(err.Error(), "didn't accept") {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := PasswordLogin(t.Context(), testClient(srv.URL), "admin@example.com", "password"); err == nil || !strings.Contains(err.Error(), "host doesn't take") {
		t.Fatalf("host that doesn't take superusers: %v", err)
	}
	mfa, _ := fakeApp(t, true, true)
	defer mfa.Close()
	if _, err := PasswordLogin(t.Context(), testClient(mfa.URL), "admin@example.com", "password"); err == nil || !strings.Contains(err.Error(), "multi-factor") {
		t.Fatalf("mfa: %v", err)
	}
}

func TestLooksLikeToken(t *testing.T) {
	for in, want := range map[string]bool{"pbt_abc": true, "ph_pat_x": true, "a.b.c": true, "me@x.com": false, "admin": false} {
		if looksLikeToken(in) != want {
			t.Errorf("looksLikeToken(%q) != %v", in, want)
		}
	}
}

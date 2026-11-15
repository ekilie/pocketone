package oauthserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// The sign-in pages don't check passwords or codes themselves: they call PocketBase's own auth endpoints
// (auth-with-password, request-otp, auth-with-otp, auth-with-oauth2, auth-methods) in-process, through a handler
// built from the app's router. So the collection's auth rule, MFA, rate limits, superuser IP limits and every
// onRecordAuth*Request hook apply exactly as for the REST API. The same pattern as ext/pages' ctx.pb().

// apiHandler returns a handler built from the app's router, built on first use (after every OnServe hook has
// registered its routes).
func (s *Server) apiHandler() (http.Handler, error) {
	s.muxOnce.Do(func() {
		if s.router == nil {
			s.muxErr = errors.New("oauth-server: the app isn't serving")
			return
		}
		s.mux, s.muxErr = s.router.BuildMux()
	})
	return s.mux, s.muxErr
}

// apiResponse is an in-process API answer.
type apiResponse struct {
	status int
	raw    []byte
	body   map[string]any
}

func (r apiResponse) str(key string) string {
	v, _ := r.body[key].(string)
	return v
}

// callAPI sends method path (with body as JSON) to the app's router as if the browser behind e had sent it: same
// host, client IP and forwarding headers, user agent.
func (s *Server) callAPI(e *core.RequestEvent, method, path string, body any) (apiResponse, error) {
	h, err := s.apiHandler()
	if err != nil {
		return apiResponse{}, err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return apiResponse{}, err
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequestWithContext(e.Request.Context(), method, path, rd)
	req.Host = e.Request.Host
	req.RemoteAddr = e.Request.RemoteAddr
	for _, k := range []string{"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP", "X-Forwarded-Proto", "User-Agent", "Accept-Language"} {
		if v := e.Request.Header.Get(k); v != "" {
			req.Header.Set(k, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := apiResponse{status: rec.Code, raw: rec.Body.Bytes()}
	_ = json.Unmarshal(out.raw, &out.body)
	return out, nil
}

func collectionAPI(col *core.Collection, action string) string {
	return "/api/collections/" + url.PathEscape(col.Name) + "/" + action
}

// authMethods is GET /api/collections/{c}/auth-methods.
type authMethods struct {
	Password struct {
		Enabled        bool     `json:"enabled"`
		IdentityFields []string `json:"identityFields"`
	} `json:"password"`
	OTP struct {
		Enabled bool `json:"enabled"`
	} `json:"otp"`
	OAuth2 struct {
		Enabled   bool           `json:"enabled"`
		Providers []authProvider `json:"providers"`
	} `json:"oauth2"`
}

// authProvider is an OAuth2 provider as auth-methods lists it, with a fresh state and PKCE verifier.
type authProvider struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	State        string `json:"state"`
	AuthURL      string `json:"authURL"`
	CodeVerifier string `json:"codeVerifier"`
}

func (s *Server) authMethods(e *core.RequestEvent, col *core.Collection) (*authMethods, error) {
	res, err := s.callAPI(e, http.MethodGet, collectionAPI(col, "auth-methods"), nil)
	if err != nil {
		return nil, err
	}
	if res.status != http.StatusOK {
		return nil, errors.New("auth-methods: " + http.StatusText(res.status) + ": " + string(res.raw))
	}
	am := &authMethods{}
	if err := json.Unmarshal(res.raw, am); err != nil {
		return nil, err
	}
	return am, nil
}

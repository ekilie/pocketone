package multisite

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// orderRuntime serves every instance and records that it ran.
type orderRuntime struct{ log *[]string }

func (orderRuntime) Name() string         { return "order" }
func (orderRuntime) Serves(Instance) bool { return true }
func (orderRuntime) Ready() bool          { return true }
func (rt orderRuntime) ServeInstance(w http.ResponseWriter, _ *http.Request, inst Instance) {
	*rt.log = append(*rt.log, "runtime "+inst.ID)
	_, _ = io.WriteString(w, "runtime")
}

// orderGate admits everything except instance "denied", and records each admission.
type orderGate struct{ log *[]string }

func (orderGate) AllowPeer(*http.Request) bool { return true }
func (g orderGate) Admit(w http.ResponseWriter, _ *http.Request, inst Instance) (func(), bool) {
	*g.log = append(*g.log, "gate "+inst.ID)
	if inst.ID == "denied" {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return nil, false
	}
	return func() { *g.log = append(*g.log, "release "+inst.ID) }, true
}

// Instance routes run after the gates admitted the request and before the runtime, with the resolved instance; a
// blocked instance, a refused request and a forwarded instance never reach them.
func TestInstanceRoutesRunAfterTheGates(t *testing.T) {
	var log []string
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewDirectory(quiet)
	d.Load([]Instance{
		{ID: "a", Hosts: []string{"a.test"}},
		{ID: "off", Hosts: []string{"off.test"}, Blocked: &Blocked{Status: 409, Message: "powered off"}},
		{ID: "denied", Hosts: []string{"denied.test"}},
		{ID: "far", Hosts: []string{"far.test"}, Remote: "http://far"},
	})
	route := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				inst, ok := InstanceOf(r)
				id, _ := InstanceID(r)
				log = append(log, name+" "+inst.ID)
				if !ok || id != inst.ID {
					t.Errorf("%s: no instance in the request", name)
				}
				if strings.HasPrefix(r.URL.Path, "/api/_c/") && name == "inner" {
					_, _ = io.WriteString(w, "contract")
					return
				}
				next.ServeHTTP(w, r)
			})
		}
	}
	h := &Handler{Dir: d, Gate: orderGate{&log}, Runtime: orderRuntime{&log}, Logger: quiet,
		InstanceRoutes: []func(http.Handler) http.Handler{route("outer"), route("inner")},
		Forward: func(w http.ResponseWriter, _ *http.Request, inst Instance) {
			log = append(log, "forward "+inst.ID)
		}}
	serve := func(host, path string) (int, string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://"+host+path, nil))
		return w.Code, w.Body.String()
	}
	check := func(what string, want ...string) {
		t.Helper()
		if strings.Join(log, ", ") != strings.Join(want, ", ") {
			t.Fatalf("%s: ran %q, want %q", what, log, want)
		}
		log = nil
	}

	if code, body := serve("a.test", "/api/_c/remote/v1"); code != 200 || body != "contract" {
		t.Fatalf("contract: %d %q", code, body)
	}
	check("contract", "gate a", "outer a", "inner a", "release a")
	if code, body := serve("a.test", "/api/health"); code != 200 || body != "runtime" {
		t.Fatalf("app: %d %q", code, body)
	}
	check("app", "gate a", "outer a", "inner a", "runtime a", "release a")
	if code, _ := serve("off.test", "/api/_c/remote/v1"); code != 409 {
		t.Fatalf("blocked: %d", code)
	}
	check("blocked")
	if code, _ := serve("denied.test", "/api/_c/remote/v1"); code != http.StatusTooManyRequests {
		t.Fatalf("refused: %d", code)
	}
	check("refused", "gate denied")
	serve("far.test", "/api/_c/remote/v1")
	check("forwarded", "gate far", "forward far", "release far")
}

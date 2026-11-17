package templates_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// Theming the OAuth sign-in pages, as a single app and as an instance on a host: `template list` in the app's folder
// shows the oauth set, `template eject oauth` copies its Go templates into pb_templates/oauth (again: nothing to
// write), an edit to head.tpl is never overwritten without asking and shows on the next sign-in page with no restart,
// and deleting the copy brings the built-in page back.
func TestModesEjectOAuth(t *testing.T) {
	modetest.Run(t, modetest.Topology{Instances: []modetest.Inst{{ID: "app"}}}, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")

		out, err := s.Run("template", "list", "--dev=false")
		if err != nil || !strings.Contains(out, "oauth") || !strings.Contains(out, "built in (10 files)") {
			t.Fatalf("template list: %v\n%s", err, out)
		}
		out, err = s.Run("template", "eject", "oauth", "--dev=false")
		if err != nil || !strings.Contains(out, "oauth: 10 file(s) written") {
			t.Fatalf("template eject oauth: %v\n%s", err, out)
		}
		head := filepath.Join(s.Dir, "pb_templates", "oauth", "head.tpl")
		b, err := os.ReadFile(head)
		if err != nil || !strings.Contains(string(b), "<title>{{.AppName}}</title>") {
			t.Fatalf("ejected head.tpl: %v\n%.300s", err, b)
		}
		if out, err := s.Run("template", "eject", "oauth", "--dev=false"); err != nil || !strings.Contains(out, "oauth: 0 file(s) written") {
			t.Fatalf("second eject of an unchanged copy: %v\n%s", err, out)
		}

		q := url.Values{"response_type": {"code"}, "client_id": {"pocketone-cli"}, "redirect_uri": {"http://127.0.0.1:9/cb"},
			"state": {"s"}, "code_challenge": {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGyuxcqC4s"}, "code_challenge_method": {"S256"}}
		signin := "/api/_c/oauth/v1/authorize?" + q.Encode()

		modetest.WriteFile(t, head, strings.Replace(string(b), "<title>{{.AppName}}</title>", "<title>THEMED {{.AppName}}</title>", 1))
		// a changed file: no terminal to ask in, so eject refuses and keeps it
		if out, err := s.Run("template", "eject", "oauth", "--dev=false"); err == nil || !strings.Contains(out, "head.tpl") {
			t.Fatalf("eject over a change: %v\n%s", err, out)
		}
		if out, err := s.Run("template", "list", "--dev=false"); err != nil || !strings.Contains(out, "1 changed, 9 unchanged") {
			t.Fatalf("list after a change: %v\n%s", err, out)
		}
		if code, page := s.Raw(http.MethodGet, signin, "", nil); code != 200 || !strings.Contains(string(page), "<title>THEMED ") ||
			!strings.Contains(string(page), `name="csrf"`) {
			t.Fatalf("themed sign-in page: %d\n%.500s", code, page)
		}
		if s.Restarted() {
			t.Fatalf("the app restarted for a template edit (%d restarts)", s.Restarts())
		}

		os.RemoveAll(filepath.Join(s.Dir, "pb_templates", "oauth"))
		if code, page := s.Raw(http.MethodGet, signin, "", nil); code != 200 || strings.Contains(string(page), "THEMED") ||
			!strings.Contains(string(page), "<title>") {
			t.Fatalf("built-in sign-in page after deleting the copy: %d\n%.500s", code, page)
		}
	})
}

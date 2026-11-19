package backups_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/extkit/modetest"
)

// With PITR off (its `enabled: false` setting), as a single app and as an instance on a host, restore/v1 falls back to
// the backups extension: its points are the backup zips, and restoring picks the newest one at or before the moment
// asked for.
func TestModesProviderWhenPITROff(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{{ID: "app", Extensions: map[string]any{"pitr": map[string]any{"enabled": false}}}}}
	modetest.Run(t, top, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		su := s.JWT()
		s.MustJSON(http.MethodPost, "/api/collections", su, map[string]any{
			"name": "notes", "type": "base", "fields": []map[string]any{{"name": "text", "type": "text"}},
		}, 200, nil)
		addNote := func(text string) {
			s.MustJSON(http.MethodPost, "/api/collections/notes/records", su, map[string]string{"text": text}, 200, nil)
		}
		addNote("a")
		s.MustJSON(http.MethodPost, "/api/backups", su, map[string]string{"name": "snap.zip"}, 204, nil)
		addNote("b")

		var pts struct {
			Provider string `json:"provider"`
		}
		b := s.MustJSON(http.MethodGet, "/api/_c/restore/v1", s.Token("restore:read"), nil, 200, &pts)
		if pts.Provider != "backups" || !strings.Contains(string(b), "snap.zip") {
			t.Fatalf("restore/v1 points: %s", b)
		}

		s.MustJSON(http.MethodPost, "/api/_c/restore/v1/restore", s.Token("restore"), map[string]string{"at": time.Now().UTC().Format(time.RFC3339Nano)}, 200, nil)
		s.WaitRestarted("backups restore", 30*time.Second)
		var out struct {
			Items []struct {
				Text string `json:"text"`
			} `json:"items"`
		}
		s.MustJSON(http.MethodGet, "/api/collections/notes/records", s.JWT(), nil, 200, &out)
		if len(out.Items) != 1 || out.Items[0].Text != "a" {
			t.Fatalf("notes after restoring snap.zip: %+v, want a", out.Items)
		}
	})
}

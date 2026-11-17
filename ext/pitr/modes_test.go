package pitr_test

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// PITR, as a single app and as an instance on a host: replicate, restore to a moment between two writes over
// restore/v1, and the app restarts into the restored database, keeping the one it replaced.
func TestModesRestore(t *testing.T) {
	modetest.Run(t, modetest.Topology{Instances: []modetest.Inst{{ID: "app"}}}, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		notesCollection(t, s)
		addNote(t, s, "a")
		waitWindowPast(t, s, time.Now())
		at := time.Now().UTC()
		time.Sleep(1100 * time.Millisecond)
		addNote(t, s, "b")
		waitWindowPast(t, s, time.Now())

		var res map[string]any
		s.MustJSON(http.MethodPost, "/api/_c/restore/v1/restore", s.Token("restore"), map[string]string{"at": at.Format(time.RFC3339Nano)}, 200, &res)
		if res["restoring"] != true {
			t.Fatalf("restore: %v", res)
		}
		s.WaitRestarted("PITR restore", 30*time.Second)
		if got := notes(t, s); got != "a" {
			t.Fatalf("notes after restore: %q, want a", got)
		}
		if _, err := os.Stat(filepath.Join(s.Dir, "pb_data", "litestream", "before-restore.db")); err != nil {
			t.Fatalf("the replaced database wasn't kept: %v", err)
		}
		var pts struct {
			Last *struct {
				Asked time.Time `json:"asked"`
				Point time.Time `json:"point"`
				Done  time.Time `json:"done"`
			} `json:"last"`
			Details map[string]any `json:"details"`
		}
		s.MustJSON(http.MethodGet, "/api/_c/restore/v1", s.JWT(), nil, 200, &pts)
		if pts.Last == nil || !pts.Last.Asked.Equal(at) || pts.Last.Point.IsZero() || pts.Last.Done.Before(at) || pts.Details["lastError"] != nil {
			t.Fatalf("last restore: %+v details %v (asked for %s)", pts.Last, pts.Details, at)
		}

		// replication carries on after the restore
		addNote(t, s, "c")
		waitWindowPast(t, s, time.Now())
		if got := notes(t, s); got != "a,c" {
			t.Fatalf("notes: %q", got)
		}

		// out of the window: refused, nothing restarts
		if code, b := s.Raw(http.MethodPost, "/api/_c/restore/v1/restore", s.Token("restore"), map[string]string{"at": "2000-01-01T00:00:00Z"}); code != 422 {
			t.Fatalf("restore out of the window: %d %s", code, b)
		}
	})
}

// A PocketBase backup zip restored while PITR runs, as a single app and as an instance on a host: the zip wins,
// PITR's history survives it (it's never in a zip), replication carries on, and PITR can still go back to before the
// zip restore.
func TestModesZipRestore(t *testing.T) {
	modetest.Run(t, modetest.Topology{Instances: []modetest.Inst{{ID: "app"}}}, modetest.AllApp, func(t *testing.T, w *modetest.World) {
		s := w.Instance("app")
		su := s.JWT()
		notesCollection(t, s)
		addNote(t, s, "a")
		s.MustJSON(http.MethodPost, "/api/backups", su, map[string]string{"name": "one.zip"}, 204, nil)
		addNote(t, s, "b")
		waitWindowPast(t, s, time.Now())
		beforeZip := time.Now().UTC()
		time.Sleep(1100 * time.Millisecond)

		var list []struct {
			Key string `json:"key"`
		}
		s.MustJSON(http.MethodGet, "/api/backups", su, nil, 200, &list)
		if len(list) != 1 || list[0].Key != "one.zip" {
			t.Fatalf("backups: %+v", list)
		}
		if zipHas(t, filepath.Join(s.Dir, "pb_data", "backups", "one.zip"), "litestream/") {
			t.Fatal("the backup zip carries PITR history")
		}

		modetest.Step(t, "restore the zip")
		s.MustJSON(http.MethodPost, "/api/backups/one.zip/restore", su, nil, 204, nil)
		s.WaitRestarted("zip restore", 30*time.Second)
		if got := notes(t, s); got != "a" {
			t.Fatalf("notes after the zip restore: %q, want a", got)
		}
		if _, err := os.Stat(filepath.Join(s.Dir, "pb_data", "litestream")); err != nil {
			t.Fatalf("PITR history gone after the zip restore: %v", err)
		}

		addNote(t, s, "c")
		waitWindowPast(t, s, time.Now())

		// PITR back to before the zip restore: a and b again
		s.MustJSON(http.MethodPost, "/api/_c/restore/v1/restore", s.Token("restore"), map[string]string{"at": beforeZip.Format(time.RFC3339Nano)}, 200, nil)
		s.WaitRestarted("PITR restore across the zip restore", 30*time.Second)
		if got := notes(t, s); got != "a,b" {
			t.Fatalf("notes after PITR to before the zip restore: %q, want a,b", got)
		}
	})
}

func notesCollection(t *testing.T, s *modetest.Site) {
	t.Helper()
	s.MustJSON(http.MethodPost, "/api/collections", s.JWT(), map[string]any{
		"name": "notes", "type": "base", "fields": []map[string]any{{"name": "text", "type": "text"}},
	}, 200, nil)
}

func addNote(t *testing.T, s *modetest.Site, text string) {
	t.Helper()
	s.MustJSON(http.MethodPost, "/api/collections/notes/records", s.JWT(), map[string]string{"text": text}, 200, nil)
}

// notes lists the notes' texts, sorted and comma-joined.
func notes(t *testing.T, s *modetest.Site) string {
	t.Helper()
	var out struct {
		Items []struct {
			Text string `json:"text"`
		} `json:"items"`
	}
	s.MustJSON(http.MethodGet, "/api/collections/notes/records?perPage=200", s.JWT(), nil, 200, &out)
	var texts []string
	for _, it := range out.Items {
		texts = append(texts, it.Text)
	}
	sort.Strings(texts)
	return strings.Join(texts, ",")
}

// waitWindowPast waits until PITR's restorable window reaches past moment (replication caught up).
func waitWindowPast(t *testing.T, s *modetest.Site, moment time.Time) {
	t.Helper()
	modetest.WaitFor(t, "PITR window past "+moment.Format("15:04:05.000"), 20*time.Second, func() (bool, string) {
		code, b := s.Raw(http.MethodGet, "/api/_c/restore/v1", s.JWT(), nil)
		if code != 200 {
			return false, fmt.Sprintf("%d %s", code, b)
		}
		var pts struct {
			Window *struct {
				Latest time.Time `json:"latest"`
			} `json:"window"`
		}
		_ = json.Unmarshal(b, &pts)
		if pts.Window == nil {
			return false, "no window yet"
		}
		return !pts.Window.Latest.Before(moment), "latest " + pts.Window.Latest.Format(time.RFC3339Nano)
	})
}

// zipHas reports whether the zip at path has an entry starting with prefix.
func zipHas(t *testing.T, path, prefix string) bool {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, prefix) {
			return true
		}
	}
	return false
}

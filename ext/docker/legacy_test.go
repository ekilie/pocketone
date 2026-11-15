package docker

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pockethost/pocketone/ext/host"
)

// A host can lower the upstream floor (releases it keeps in its own cache) and adjust a release's command and
// readiness check through Prepare; with neither, nothing changes.
func TestPickMin(t *testing.T) {
	rs := []Release{rel("pb", "0.8.0"), rel("pb", "0.22.21"), rel("pb", "0.36.9")}
	if _, err := Pick(rs, "0.22.*"); err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("default floor: %v", err)
	}
	for want, exp := range map[string]string{"0.22.*": "pb:0.22.21", "0.8.0": "pb:0.8.0", "0.36.*": "pb:0.36.9"} {
		if r, err := PickMin(rs, want, "0.7.0"); err != nil || r.Key() != exp {
			t.Errorf("PickMin(%q, 0.7.0) = %s, %v; want %s", want, r.Key(), err, exp)
		}
	}
	if _, err := PickMin(rs, "0.8.0", "0.9.0"); err == nil || !strings.Contains(err.Error(), "oldest served is 0.9.0") {
		t.Fatalf("below a custom floor: %v", err)
	}
	if _, err := PickMin(rs, "0.36.*", "nope"); err == nil {
		t.Fatal("bad floor accepted")
	}
}

func TestSpecCmdAndHealth(t *testing.T) {
	b := &Binaries{Root: t.TempDir()}
	rs := []Release{rel("pb", "0.8.0"), rel("pb", "0.36.9")}
	for _, r := range rs {
		p := b.Path(r.Line, r.Version)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rt, err := New(Config{Image: "alpine:3.20", DataRoot: t.TempDir(), Binaries: b, Versions: staticVersions(rs), MinUpstream: "0.7.0",
		Prepare: func(_ context.Context, inst host.Instance, version string, spec *Spec) error {
			if version != "pb:0.8.0" {
				return nil
			}
			spec.Cmd = func(def []string) []string {
				out := def[:0:0]
				for _, a := range def {
					if a != "--dev" {
						out = append(out, a)
					}
				}
				return out
			}
			spec.HealthPath, spec.HealthAnyStatus = "/", true
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	rt.releases.Store(&rs)
	ctx := context.Background()

	old, version, err := rt.containerConfig(ctx, host.Instance{ID: "old", Version: "0.8.0", Dev: true})
	if err != nil || version != "pb:0.8.0" {
		t.Fatalf("old: %s %v", version, err)
	}
	if want := []string{"./pocketbase", "serve", "--http", "0.0.0.0:8090"}; !reflect.DeepEqual(old.Cmd, want) {
		t.Fatalf("old cmd %v", old.Cmd)
	}
	if old.healthPath != "/" || !old.healthAny {
		t.Fatalf("old health %q %v", old.healthPath, old.healthAny)
	}

	cur, _, err := rt.containerConfig(ctx, host.Instance{ID: "cur", Version: "0.36.*"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"./pocketbase", "serve", "--http", "0.0.0.0:8090"}; !reflect.DeepEqual(cur.Cmd, want) { // hooks watcher on

		t.Fatalf("current cmd %v", cur.Cmd)
	}
	if cur.healthPath != "" || cur.healthAny {
		t.Fatalf("current health %q %v", cur.healthPath, cur.healthAny)
	}
}

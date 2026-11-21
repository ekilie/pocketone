package binary

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The upstream version must follow ui/.env, which every PocketBase release updates, so a merge that forgets to bump
// upstreamVersion fails here.
func TestUpstreamVersionMatchesUI(t *testing.T) {
	env, err := os.ReadFile("../../../ui/.env")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^PB_VERSION\s*=\s*"v?([^"]+)"`).FindSubmatch(env)
	if m == nil {
		t.Fatal("no PB_VERSION in ui/.env")
	}
	if got := string(m[1]); got != upstreamVersion {
		t.Fatalf("upstreamVersion = %q, ui/.env PB_VERSION = %q: bump upstreamVersion in version.go", upstreamVersion, got)
	}
	if s := versionString(); !strings.HasSuffix(s, "(PocketBase "+upstreamVersion+")") {
		t.Fatalf("versionString() = %q", s)
	}
}

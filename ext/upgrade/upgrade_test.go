package upgrade

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeptName(t *testing.T) {
	cases := []struct{ exe, v, want string }{
		{"/usr/local/bin/pocketone", "0.0.3", "/usr/local/bin/pocketone-0.0.3"},
		{"/usr/local/bin/pocketone", "v0.0.3", "/usr/local/bin/pocketone-0.0.3"},
		{"/usr/local/bin/pocketone", "(untracked)", "/usr/local/bin/pocketone.old"},
		{"/usr/local/bin/pocketone", "", "/usr/local/bin/pocketone.old"},
		{"/usr/local/bin/pocketone", "0.0.3-next", "/usr/local/bin/pocketone.old"},
		{`C:/bin/pocketone.exe`, "0.1.0", `C:/bin/pocketone-0.1.0.exe`},
	}
	for _, c := range cases {
		got := filepath.ToSlash(KeptName(filepath.FromSlash(c.exe), c.v))
		if got != c.want {
			t.Errorf("KeptName(%q, %q) = %q, want %q", c.exe, c.v, got, c.want)
		}
	}
}

func TestHomebrewFormula(t *testing.T) {
	if got := homebrewFormula("/opt/homebrew/Cellar/pocketone/0.0.3/bin/pocketone"); got != "pocketone" {
		t.Errorf("got %q", got)
	}
	if got := homebrewFormula("/usr/local/bin/pocketone"); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestScoopApp(t *testing.T) {
	for exe, want := range map[string]string{
		`C:\Users\me\scoop\apps\pocketone\current\pocketone.exe`:     "pocketone",
		`C:\ProgramData\scoop\apps\pocketone\0.0.3\pocketone.exe`:    "pocketone",
		`C:\Users\me\AppData\Local\Programs\pocketone\pocketone.exe`: "",
		"/usr/local/bin/pocketone":                                   "",
	} {
		if got := scoopApp(exe); got != want {
			t.Errorf("scoopApp(%q) = %q, want %q", exe, got, want)
		}
	}
}

func TestLinuxPackaged(t *testing.T) {
	for _, c := range []struct {
		goos, exe string
		want      bool
	}{
		{"linux", "/usr/bin/pocketone", true},
		{"linux", "/usr/local/bin/pocketone", false},
		{"linux", "/home/me/.local/bin/pocketone", false},
		{"darwin", "/usr/bin/pocketone", false},
	} {
		if got := linuxPackaged(c.goos, c.exe); got != c.want {
			t.Errorf("linuxPackaged(%q, %q) = %v, want %v", c.goos, c.exe, got, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	if compareVersions("0.0.3", "0.0.4") != 1 || compareVersions("0.1.0", "0.0.9") != -1 || compareVersions("1.0", "1.0.0") != 0 ||
		compareVersions("0.1.0-alpha.1", "0.1.0-alpha.2") != 1 || compareVersions("0.1.0", "0.1.0-alpha.9") != -1 {
		t.Fatal("compareVersions is wrong")
	}
}

// fakeReleases serves a GitHub-style latest release "v0.0.4" with one darwin/arm64 archive holding a "pocketone"
// executable whose content is body. badSum corrupts checksums.txt.
func zipWith(t *testing.T, name, body string) []byte {
	t.Helper()
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	w, _ := zw.Create(name)
	w.Write([]byte(body))
	zw.Close()
	return zbuf.Bytes()
}

func fakeReleases(t *testing.T, body string, badSum bool) *httptest.Server {
	t.Helper()
	archive := zipWith(t, "pocketone", body)
	sum := sha256.Sum256(archive)
	hexSum := hex.EncodeToString(sum[:])
	if badSum {
		hexSum = strings.Repeat("0", 64)
	}
	const name = "pocketone_0.0.4_darwin_arm64.zip"

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/pocketonehq/pocketone/releases", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Release{{Tag: "v0.0.4", Body: "notes", Assets: []Asset{
			{Name: name, DownloadURL: srv.URL + "/dl/" + name},
			{Name: "checksums.txt", DownloadURL: srv.URL + "/dl/checksums.txt"},
		}}, {Tag: "v0.0.9", Draft: true}})
	})
	mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n%s  other.zip\n", hexSum, name, strings.Repeat("1", 64))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testExtension(t *testing.T, srv *httptest.Server, current, exe string) *extension {
	cfg := Config{BaseURL: srv.URL}
	cfg.defaults()
	return &extension{cfg: cfg, current: current, executable: func() (string, error) { return exe, nil },
		goos: "darwin", goarch: "arm64"}
}

func TestUpgradeKeepsEveryOldVersion(t *testing.T) {
	srv := fakeReleases(t, "new", false)
	dir := t.TempDir()
	exe := filepath.Join(dir, "pocketone")
	// an older kept copy from a previous upgrade must survive
	os.WriteFile(filepath.Join(dir, "pocketone-0.0.2"), []byte("older"), 0o755)
	os.WriteFile(exe, []byte("old"), 0o755)

	if err := testExtension(t, srv, "0.0.3", exe).upgrade(false, false); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{exe: "new", exe + "-0.0.3": "old", exe + "-0.0.2": "older"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", filepath.Base(path), got, err, want)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Errorf("expected exactly 3 files (no temp left behind), got %d", len(entries))
	}
}

func TestUpgradeAlreadyLatest(t *testing.T) {
	srv := fakeReleases(t, "new", false)
	exe := filepath.Join(t.TempDir(), "pocketone")
	os.WriteFile(exe, []byte("current"), 0o755)
	if err := testExtension(t, srv, "0.0.4", exe).upgrade(false, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "current" {
		t.Errorf("executable changed: %q", got)
	}
}

func TestUpgradeRejectsBadChecksum(t *testing.T) {
	srv := fakeReleases(t, "new", true)
	dir := t.TempDir()
	exe := filepath.Join(dir, "pocketone")
	os.WriteFile(exe, []byte("old"), 0o755)
	err := testExtension(t, srv, "0.0.3", exe).upgrade(false, false)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum mismatch, got %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("executable changed: %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("expected only the executable left, got %d entries", len(entries))
	}
}

func TestUpgradeDevBuildKeptAsOld(t *testing.T) {
	srv := fakeReleases(t, "new", false)
	exe := filepath.Join(t.TempDir(), "pocketone")
	os.WriteFile(exe, []byte("dev"), 0o755)
	if err := testExtension(t, srv, "(untracked)", exe).upgrade(false, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe + ".old"); string(got) != "dev" {
		t.Errorf("dev build not kept as .old: %q", got)
	}
}

func TestUpgradeFor(t *testing.T) {
	rs := []Release{
		{Tag: "v0.0.1-alpha.1", Prerelease: true},
		{Tag: "v0.0.1-alpha.2", Prerelease: true},
		{Tag: "v0.0.1"},
		{Tag: "v0.0.2-alpha.1", Prerelease: true},
	}
	cases := []struct {
		current    string
		stableOnly bool
		want       string
		confirm    bool
	}{
		{"0.0.1-alpha.1", false, "v0.0.2-alpha.1", false}, // prerelease to prerelease: smooth
		{"0.0.1", false, "v0.0.2-alpha.1", true},          // stable to prerelease: asks
		{"0.0.1", true, "", false},                        // said no, and there's no newer stable release
		{"0.0.0", true, "v0.0.1", false},                  // said no: the newest stable release instead
		{"0.0.2-alpha.1", false, "", false},               // nothing newer
		{"(untracked)", false, "v0.0.2-alpha.1", false},   // a dev build: smooth
	}
	for _, c := range cases {
		got := ""
		r := UpgradeFor(c.current, rs, c.stableOnly)
		if r != nil {
			got = r.Tag
		}
		if got != c.want || NeedsConfirm(c.current, r) != c.confirm {
			t.Errorf("UpgradeFor(%q, %v) = %q (confirm %v), want %q (confirm %v)",
				c.current, c.stableOnly, got, NeedsConfirm(c.current, r), c.want, c.confirm)
		}
	}
	// ordering: alpha.2 > alpha.1, a release > its prereleases
	if UpgradeFor("0.0.1-alpha.1", rs[:3], false).Tag != "v0.0.1" || UpgradeFor("0.0.1-alpha.1", rs[:2], false).Tag != "v0.0.1-alpha.2" {
		t.Fatal("prerelease ordering is wrong")
	}
}

func TestUpgradeAsksBeforeAPrerelease(t *testing.T) {
	// the fake server's only release is v0.0.4; flagged as a prerelease here by its tag instead
	archive := zipWith(t, "pocketone", "new")
	sum := sha256.Sum256(archive)
	const name = "pocketone_0.0.4-alpha.1_darwin_arm64.zip"
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/pocketonehq/pocketone/releases", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Release{{Tag: "v0.0.4-alpha.1", Prerelease: true, Assets: []Asset{
			{Name: name, DownloadURL: srv.URL + "/dl/" + name},
			{Name: "checksums.txt", DownloadURL: srv.URL + "/dl/checksums.txt"},
		}}})
	})
	mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, c := range []struct {
		current string
		answer  string
		yes     bool
		want    string
	}{
		{"0.0.3", "n\n", false, "old"}, // stable to prerelease asks; no means no
		{"0.0.3", "", false, "old"},    // no answer (no terminal) means no
		{"0.0.3", "y\n", false, "new"},
		{"0.0.3", "", true, "new"},          // --yes answers for you
		{"0.0.3-alpha.1", "", false, "new"}, // prerelease to prerelease doesn't ask
	} {
		exe := filepath.Join(t.TempDir(), "pocketone")
		os.WriteFile(exe, []byte("old"), 0o755)
		p := testExtension(t, srv, c.current, exe)
		p.stdin = strings.NewReader(c.answer)
		if err := p.upgrade(false, c.yes); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(exe); string(got) != c.want {
			t.Errorf("%s, answer %q, --yes %v: executable is %q, want %q", c.current, c.answer, c.yes, got, c.want)
		}
	}
}

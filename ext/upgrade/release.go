package upgrade

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

// Release is a GitHub release (only the fields this package uses).
type Release struct {
	Tag        string  `json:"tag_name"`
	Body       string  `json:"body"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

// Asset is one file attached to a [Release].
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Version is the release's version without the leading "v".
func (r *Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// IsPrerelease reports whether the release is a prerelease: flagged on GitHub, or a version with a prerelease part
// ("0.1.0-alpha.1").
func (r *Release) IsPrerelease() bool { return r.Prerelease || isPrerelease(r.Version()) }

func (r *Release) asset(name string) (*Asset, error) {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("release %s has no %s", r.Tag, name)
}

// FetchRelease fetches the latest release (tag "") or the release with the given tag ("0.0.3" or "v0.0.3").
func (c *Config) FetchRelease(ctx context.Context, tag string) (*Release, error) {
	path := "releases/latest"
	if tag != "" {
		path = "releases/tags/v" + strings.TrimPrefix(tag, "v")
	}
	url := fmt.Sprintf("%s/repos/%s/%s/%s", c.BaseURL, c.Owner, c.Repo, path)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	r := &Release{}
	if err := json.Unmarshal(body, r); err != nil {
		return nil, fmt.Errorf("decoding release info: %w", err)
	}
	if r.Tag == "" {
		return nil, errors.New("release info has no tag")
	}
	return r, nil
}

// FetchReleases fetches the repository's published releases (drafts left out), prereleases included.
func (c *Config) FetchReleases(ctx context.Context) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.BaseURL, c.Owner, c.Repo)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var all []Release
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, fmt.Errorf("decoding release list: %w", err)
	}
	out := all[:0]
	for _, r := range all {
		if !r.Draft && r.Tag != "" && semver.IsValid("v"+r.Version()) {
			out = append(out, r)
		}
	}
	return out, nil
}

// UpgradeFor picks the newest release newer than current (a version such as "0.1.0" or "0.1.0-alpha.2", or a dev
// build's non-version string), prerelease or not, or nil when there's nothing newer. With stableOnly, prereleases are
// left out. Whether to ask before installing is [NeedsConfirm]'s call.
func UpgradeFor(current string, releases []Release, stableOnly bool) *Release {
	current = strings.TrimPrefix(current, "v")
	var best *Release
	for i := range releases {
		r := &releases[i]
		if stableOnly && r.IsPrerelease() {
			continue
		}
		if isRelease(current) && compareVersions(current, r.Version()) <= 0 {
			continue
		}
		if best == nil || compareVersions(best.Version(), r.Version()) > 0 {
			best = r
		}
	}
	return best
}

// NeedsConfirm reports whether moving from current to r needs a yes: only a jump from a stable release to a
// prerelease does. Prerelease to prerelease, prerelease to stable and a dev build to anything go ahead.
func NeedsConfirm(current string, r *Release) bool {
	current = strings.TrimPrefix(current, "v")
	return r != nil && r.IsPrerelease() && isRelease(current) && !isPrerelease(current)
}

// DownloadArchive downloads the release archive for this platform into dir, checks it against the release's
// checksums.txt and returns its path.
func (c *Config) DownloadArchive(ctx context.Context, r *Release, goos, goarch, dir string) (string, error) {
	suffix := archiveSuffix(goos, goarch)
	if suffix == "" {
		return "", fmt.Errorf("no release builds for %s/%s", goos, goarch)
	}
	name := fmt.Sprintf("%s_%s%s", c.ArchivePrefix, r.Version(), suffix)
	a, err := r.asset(name)
	if err != nil {
		return "", err
	}
	sums, err := r.asset("checksums.txt")
	if err != nil {
		return "", err
	}
	sumsBody, err := c.get(ctx, sums.DownloadURL)
	if err != nil {
		return "", err
	}
	want, err := checksumFor(sumsBody, name)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, name)
	got, err := c.download(ctx, a.DownloadURL, dest)
	if err != nil {
		return "", err
	}
	if got != want {
		_ = os.Remove(dest)
		return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	return dest, nil
}

func (c *Config) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %d %s", url, res.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// download saves url to dest and returns its sha256 (hex).
func (c *Config) download(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %d", url, res.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), res.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), f.Close()
}

// checksumFor finds name in a goreleaser checksums.txt ("<sha256>  <name>" per line).
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(string(sums)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

func archiveSuffix(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "linux/ppc64le", "linux/s390x",
		"darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64":
		return "_" + goos + "_" + goarch + ".zip"
	case "linux/arm":
		return "_linux_armv7.zip"
	}
	return ""
}

// compareVersions returns 1 if b is newer than a, -1 if a is newer, 0 if equal. Versions are semver without the
// "v" ("1.2.3", "1.2.3-alpha.1"; "1.2" means "1.2.0"); a prerelease is older than its release.
func compareVersions(a, b string) int {
	return semver.Compare("v"+b, "v"+a)
}

func isPrerelease(v string) bool { return semver.Prerelease("v"+strings.TrimPrefix(v, "v")) != "" }

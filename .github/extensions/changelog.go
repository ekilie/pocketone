package main

// Changelogs. PocketOne's own changes (core, the binary, the site, anything not one extension's) are in the root
// CHANGELOG.md; upstream PocketBase's are in CHANGELOG.pocketbase.md. Each extension keeps its own in
// ext/<name>/CHANGELOG.md, so a change to an extension touches only its folder, and lines go under `## Unreleased`.
//
// `release vX.Y.Z` cuts a PocketOne release: every extension with lines under its `## Unreleased` gets a new version
// (minor when it has upgrade steps, else patch; a new extension keeps its first one and is listed as new), set in its README and stamped on that section; the root changelog's
// `## Unreleased` becomes `## vX.Y.Z (date)`, with an Extensions list naming each new version and linking to it.
// `release-notes vX.Y.Z` prints that section for GitHub, with its links made absolute.

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	rootChangelog = "CHANGELOG.md"
	repoBlob      = "https://github.com/pocketonehq/pocketone/blob/"
)

// checkChangelogs: every extension with a catalogue entry has a CHANGELOG.md with a `## Unreleased` section.
func checkChangelogs() []string {
	entries, err := os.ReadDir("ext")
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, e := range entries {
		readme, err := os.ReadFile("ext/" + e.Name() + "/README.md")
		if !e.IsDir() || err != nil || readFront(string(readme))["title"] == "" {
			continue // no page of its own (a link in site/extensions.json)
		}
		f := "ext/" + e.Name() + "/CHANGELOG.md"
		raw, err := os.ReadFile(f)
		if err != nil {
			problems = append(problems, f+": missing; every extension keeps its changelog in its folder (docs/extensions.md#changelogs)")
			continue
		}
		if !unreleasedRe.Match(raw) {
			problems = append(problems, f+": needs a `## Unreleased` section, where new lines go (docs/extensions.md#changelogs)")
		}
	}
	return problems
}

var unreleasedRe = regexp.MustCompile(`(?m)^## Unreleased[ \t]*$`)

// unreleased splits a changelog around its `## Unreleased` section: what comes before the heading, the section's
// body, and the rest (from the next `## ` heading on).
func unreleased(src string) (before, body, after string, ok bool) {
	loc := unreleasedRe.FindStringIndex(src)
	if loc == nil {
		return "", "", "", false
	}
	rest := src[loc[1]:]
	end := len(rest)
	if i := strings.Index(rest, "\n## "); i >= 0 {
		end = i + 1
	}
	return src[:loc[0]], rest[:end], rest[end:], true
}

// hasLines reports whether a section body has any bullet.
func hasLines(body string) bool {
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "- ") {
			return true
		}
	}
	return false
}

var upgradingRe = regexp.MustCompile(`(?m)^### Upgrading[ \t]*$`)

// nextVersion is v raised: minor for a change with upgrade steps, else patch (0.x semver: minor is breaking).
func nextVersion(v string, breaking bool) (string, error) {
	m := semverRe.FindStringSubmatch(v)
	if m == nil {
		return "", fmt.Errorf("version %q isn't X.Y.Z", v)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	if breaking {
		return fmt.Sprintf("%d.%d.0", major, minor+1), nil
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
}

// stamp renames a changelog's `## Unreleased` to `## <heading>` and puts a new empty `## Unreleased` above it.
func stamp(src, heading string) string {
	before, body, after, _ := unreleased(src)
	return before + "## Unreleased\n\n## " + heading + "\n\n" + strings.Trim(body, "\n") + "\n\n" + strings.TrimLeft(after, "\n")
}

// setVersion replaces the `version:` line in a README's front matter.
func setVersion(readme, version string) string {
	loc := frontRe.FindStringSubmatchIndex(readme)
	if loc == nil {
		return readme
	}
	fm := readme[loc[2]:loc[3]]
	fm = regexp.MustCompile(`(?m)^version:.*$`).ReplaceAllString(fm, "version: "+version)
	return readme[:loc[2]] + fm + readme[loc[3]:]
}

// githubAnchor is the id GitHub gives a Markdown heading.
func githubAnchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// bump is one extension's new version in a release.
type bump struct {
	name, title, from, to, heading string
	upgrading                      bool
	isNew                          bool // its first release: no version is stamped in its changelog yet
}

// release cuts PocketOne version: it raises each extension with unreleased lines, stamps its changelog, and stamps
// the root changelog with an Extensions list. It returns the files it changed.
func release(version, date string) ([]string, error) {
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`).MatchString(version) {
		return nil, fmt.Errorf("%q isn't a version like v0.1.0 or v0.1.0-alpha.1", version)
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	entries, err := os.ReadDir("ext")
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	var bumps []bump
	for _, e := range entries {
		dir := "ext/" + e.Name()
		readme, err1 := os.ReadFile(dir + "/README.md")
		log, err2 := os.ReadFile(dir + "/CHANGELOG.md")
		if !e.IsDir() || err1 != nil || err2 != nil {
			continue
		}
		_, body, after, ok := unreleased(string(log))
		if !ok || !hasLines(body) {
			continue
		}
		fm := readFront(string(readme))
		b := bump{name: e.Name(), title: strings.Trim(fm["title"], `"`), from: fm["version"], upgrading: upgradingRe.MatchString(body)}
		// A new extension ships at the version its README already has; it's listed as new, not as a raise.
		if b.isNew = !strings.Contains(after, "## "); b.isNew {
			b.to = b.from
		} else if b.to, err = nextVersion(b.from, b.upgrading); err != nil {
			return nil, fmt.Errorf("%s/README.md: %w", dir, err)
		}
		b.heading = b.to + " (" + date + ")"
		files[dir+"/README.md"] = setVersion(string(readme), b.to)
		files[dir+"/CHANGELOG.md"] = stamp(string(log), b.heading)
		bumps = append(bumps, b)
	}

	raw, err := os.ReadFile(rootChangelog)
	if err != nil {
		return nil, err
	}
	root := string(raw)
	if regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(version) + `\b`).MatchString(root) {
		return nil, fmt.Errorf("%s already has a section for %s", rootChangelog, version)
	}
	before, body, after, ok := unreleased(root)
	if !ok {
		return nil, fmt.Errorf("%s: no `## Unreleased` section", rootChangelog)
	}
	if !hasLines(body) && len(bumps) == 0 {
		return nil, fmt.Errorf("nothing to release: %s and every extension's `## Unreleased` are empty", rootChangelog)
	}
	if len(bumps) > 0 {
		var list strings.Builder
		list.WriteString("\n### Extensions\n\n")
		for _, b := range bumps {
			if b.isNew {
				fmt.Fprintf(&list, "- [%s](ext/%s/CHANGELOG.md#%s) %s (new)\n", b.title, b.name, githubAnchor(b.heading), b.to)
				continue
			}
			fmt.Fprintf(&list, "- [%s](ext/%s/CHANGELOG.md#%s) %s → %s", b.title, b.name, githubAnchor(b.heading), b.from, b.to)
			if b.upgrading {
				list.WriteString(" (has upgrade steps)")
			}
			list.WriteString("\n")
		}
		body = insertBeforeHeading(body, "PocketBase", list.String())
	}
	files[rootChangelog] = stamp(before+"## Unreleased\n"+body+after, version+" ("+date+")")

	var changed []string
	for f, content := range files {
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			return nil, err
		}
		changed = append(changed, f)
	}
	for _, b := range bumps {
		if b.isNew {
			fmt.Printf("%s: %s (new)\n", b.name, b.to)
			continue
		}
		fmt.Printf("%s: %s → %s%s\n", b.name, b.from, b.to, map[bool]string{true: " (has upgrade steps)"}[b.upgrading])
	}
	return changed, nil
}

// insertBeforeHeading adds block to a section body before its `### <heading>`, or at its end.
func insertBeforeHeading(body, heading, block string) string {
	if loc := regexp.MustCompile(`(?m)^### ` + regexp.QuoteMeta(heading) + `[ \t]*$`).FindStringIndex(body); loc != nil {
		return strings.TrimRight(body[:loc[0]], "\n") + "\n" + block + "\n" + body[loc[0]:]
	}
	return strings.TrimRight(body, "\n") + "\n" + block
}

// releaseNotes prints version's section of the root changelog, with relative links pointing at the release's files on
// GitHub (release notes have no folder to resolve them against).
func releaseNotes(version string) (string, error) {
	raw, err := os.ReadFile(rootChangelog)
	if err != nil {
		return "", err
	}
	section, ok := changelogSection(string(raw), version)
	if !ok {
		return "", fmt.Errorf("no ## %s section in %s: run `go run ./.github/extensions release %s` first", version, rootChangelog, version)
	}
	return absoluteLinks(section, repoBlob+version+"/") + "\n", nil
}

var relLinkRe = regexp.MustCompile(`\]\(([^)\s#:]+\.md)(#[^)\s]*)?\)`)

func absoluteLinks(s, base string) string {
	return relLinkRe.ReplaceAllString(s, "]("+base+"$1$2)")
}

// changelogSection is the body of the `## <version>` section (the heading's first word is the version).
func changelogSection(src, version string) (string, bool) {
	var b strings.Builder
	found := false
	for _, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(l, "## ") {
			if found {
				break
			}
			if f := strings.Fields(l); len(f) > 1 && f[1] == version {
				found = true
				continue
			}
		}
		if found {
			b.WriteString(l + "\n")
		}
	}
	return strings.TrimSpace(b.String()), found && strings.TrimSpace(b.String()) != ""
}

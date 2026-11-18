package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// section is one pocketone.yaml section, as `pocketone config --json` describes it.
type section struct {
	Name, Doc, Owner, Extension string
	Keys                        []struct{ Key, Type, Default, Doc string }
}

// loadBinary builds the pocketone binary (examples/pocketone) once and asks it for its config sections
// (`pocketone config --json`) and its commands and contract APIs (`pocketone reference`).
func loadBinary(root string) ([]section, *reference, error) {
	tmp, err := os.MkdirTemp("", "site-sync-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, "pocketone")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join(root, "examples", "pocketone")
	build.Stderr = os.Stderr
	fmt.Fprintln(os.Stderr, "site/sync: building examples/pocketone for its config sections, commands and contract APIs")
	if err := build.Run(); err != nil {
		return nil, nil, fmt.Errorf("building examples/pocketone: %w", err)
	}
	// PocketBase takes a binary under the temp or Go cache dir for `go run` and defaults --dev to true; point both
	// elsewhere for the child so the reference shows a built binary's defaults
	childTmp := filepath.Join(tmp, "t")
	if err := os.MkdirAll(childTmp, 0o755); err != nil {
		return nil, nil, err
	}
	env := append(os.Environ(), "TMPDIR="+childTmp, "GOCACHE=off")
	ask := func(start string, v any, args ...string) error {
		cmd := exec.Command(bin, append(args, "--dev=false", "--dir", filepath.Join(tmp, "pb_data"))...)
		cmd.Dir = tmp
		cmd.Env = env
		cmd.Stderr = os.Stderr
		name := "pocketone " + strings.Join(args, " ")
		raw, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		// defaults computed from where the binary runs (--dir's pb_data beside it) don't belong in the docs
		for _, dir := range []string{tmp, "/private" + tmp} {
			raw = []byte(strings.ReplaceAll(string(raw), dir, "<the binary's folder>"))
		}
		// Decode only the JSON value (it starts with start alone on a line): extensions may log to stdout around it.
		i := strings.Index(string(raw), start+"\n")
		if i < 0 {
			return fmt.Errorf("%s printed no JSON", name)
		}
		if err := json.NewDecoder(strings.NewReader(string(raw[i:]))).Decode(v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
	var secs []section
	if err := ask("[", &secs, "config", "--json"); err != nil {
		return nil, nil, err
	}
	ref := &reference{}
	if err := ask("{", ref, "reference"); err != nil {
		return nil, nil, err
	}
	return secs, ref, nil
}

// requiredSections are the README sections every extension has, in this order (.github/extensions checks them).
var requiredSections = []string{"Install", "Configure", "Use", "Integrations", "Example"}

// extensionPage is /docs/extensions/<name>: the extension's README, with a jump line before its Install section and its
// pocketone.yaml sections (from the binary) added at the end of its Configure section.
func extensionPage(rw *rewriter, p *extension, secs []section) (string, error) {
	src := "ext/" + p.Name + "/README.md"
	body, _, err := rw.page(src, p.Title)
	if err != nil {
		return "", err
	}
	// the README's own front matter (owners, version) becomes a line under the title
	meta := ""
	if raw, err := os.ReadFile(filepath.Join(rw.root, src)); err == nil {
		owners, version := ownersAndVersion(string(raw))
		var parts []string
		if version != "" {
			parts = append(parts, "Version "+version)
		}
		if len(owners) > 0 {
			var links []string
			for _, o := range owners {
				links = append(links, "["+o+"](https://github.com/"+strings.TrimPrefix(o, "@")+")")
			}
			parts = append(parts, "Owners: "+strings.Join(links, ", "))
		}
		meta = strings.Join(parts, " · ")
	}
	body = stripFront(body)
	if meta != "" {
		if i := strings.Index(body, "\n"); i >= 0 && strings.HasPrefix(body, "# ") {
			body = body[:i+1] + "\n" + meta + "\n" + body[i+1:]
		}
	}
	var owned strings.Builder
	for _, s := range secs {
		if s.Extension == p.Name {
			owned.WriteString(sectionTable(s))
		}
	}

	lines := strings.Split(body, "\n")
	var out []string
	fence, inConfigure := false, false
	flushConfigure := func() {
		if inConfigure && owned.Len() > 0 {
			out = append(out, "### pocketone.yaml reference", "", strings.TrimRight(owned.String(), "\n"), "")
		}
		inConfigure = false
	}
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(l, "## ") {
			flushConfigure()
			switch strings.TrimSpace(l[3:]) {
			case "Install":
				var links []string
				for _, name := range requiredSections {
					links = append(links, fmt.Sprintf("[%s](#%s)", name, strings.ToLower(name)))
				}
				out = append(out, strings.Join(links, " · "), "")
			case "Configure":
				inConfigure = true
			}
		}
		out = append(out, l)
	}
	flushConfigure()
	return frontMatter(p.Title, p.Summary, src) + strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n", nil
}

func sectionTable(s section) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#### `%s:`\n\n%s\n\n", s.Name, escapeEJS(s.Doc))
	if len(s.Keys) == 0 {
		return b.String()
	}
	b.WriteString("| Key | Type | Default | Description |\n|---|---|---|---|\n")
	for _, k := range s.Keys {
		def := ""
		if k.Default != "" {
			def = "`" + strings.ReplaceAll(k.Default, "\n", " ") + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", k.Key, k.Type, cell(def), cell(escapeEJS(k.Doc)))
	}
	b.WriteString("\n")
	return b.String()
}

func cell(s string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", `\|`)
}

// configReference is /docs/config-reference: every section the pocketone binary understands.
func configReference(m *manifest, secs []section) string {
	titles := map[string]string{}
	for _, p := range m.Extensions {
		titles[p.Name] = p.Title
	}
	var b strings.Builder
	b.WriteString(frontMatter("pocketone.yaml reference", "Every section of the pocketone.yaml config file, generated from the pocketone binary.", ""))
	b.WriteString("# pocketone.yaml reference\n\nEvery section the `pocketone` binary understands, from `pocketone config --json`. Each extension owns its own section. `pocketone config check` checks a file. How the file works (where it's found, `.env` secrets) is on the [Config](/docs/extensions/config) page.\n\n")
	for _, s := range secs {
		b.WriteString(sectionTable(s))
		if t, ok := titles[s.Extension]; ok && s.Extension != "config" {
			fmt.Fprintf(&b, "Owned by [%s](/docs/extensions/%s).\n\n", t, s.Extension)
		}
	}
	return b.String()
}

// ownersAndVersion reads `owners: ["@a"]` and `version: X.Y.Z` from a README's front matter.
func ownersAndVersion(src string) (owners []string, version string) {
	m := frontStripRe.FindString(src)
	for _, line := range strings.Split(m, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "owners":
			for _, o := range strings.Split(strings.Trim(v, "[]"), ",") {
				if o = strings.Trim(strings.TrimSpace(o), `"'`); o != "" {
					owners = append(owners, o)
				}
			}
		case "version":
			version = v
		}
	}
	return owners, version
}

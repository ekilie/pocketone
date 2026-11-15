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

// loadSections builds the pocketone binary (examples/pocketone) and asks it for its config sections.
func loadSections(root string) ([]section, error) {
	tmp, err := os.MkdirTemp("", "site-sync-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, "pocketone")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join(root, "examples", "pocketone")
	build.Stderr = os.Stderr
	fmt.Fprintln(os.Stderr, "site/sync: building examples/pocketone for its config sections")
	if err := build.Run(); err != nil {
		return nil, fmt.Errorf("building examples/pocketone: %w", err)
	}
	cmd := exec.Command(bin, "config", "--json", "--dev=false", "--dir", filepath.Join(tmp, "pb_data"))
	cmd.Dir = tmp
	cmd.Stderr = os.Stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pocketone config --json: %w", err)
	}
	// Decode only the JSON value (it starts with "[" alone on a line): extensions may log to stdout around it.
	var secs []section
	start := strings.Index(string(raw), "[\n")
	if start < 0 {
		return nil, fmt.Errorf("pocketone config --json printed no JSON")
	}
	if err := json.NewDecoder(strings.NewReader(string(raw[start:]))).Decode(&secs); err != nil {
		return nil, fmt.Errorf("pocketone config --json: %w", err)
	}
	return secs, nil
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

func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

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

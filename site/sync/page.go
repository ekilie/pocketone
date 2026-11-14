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
	Name, Doc, Owner, Plugin string
	Keys                     []struct{ Key, Type, Default, Doc string }
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
	// Decode only the JSON value (it starts with "[" alone on a line): plugins may log to stdout around it.
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

func exists(root, p string) bool {
	_, err := os.Stat(filepath.Join(root, p))
	return err == nil
}

// pluginPage is /docs/plugins/<name>: Install, Configure and Use, or the README while docs/use.md doesn't exist.
func pluginPage(root string, rw *rewriter, p *plugin, m *manifest, secs []section) (string, error) {
	dir := "ext/" + p.Name
	if !exists(root, dir+"/docs/use.md") {
		body, _, err := rw.page(dir+"/README.md", p.Title)
		return frontMatter(p.Title, p.Summary, dir+"/README.md") + body, err
	}
	part := func(name string) (string, error) {
		src := dir + "/docs/" + name + ".md"
		if !exists(root, src) {
			return "", nil
		}
		body, _, err := rw.page(src, "")
		if err != nil {
			return "", err
		}
		return body + "\n" + editLink(src), nil
	}

	var b strings.Builder
	b.WriteString(strings.Replace(frontMatter(p.Title, p.Summary, ""), "from site/plugins.json.",
		"from "+dir+"/docs/ and site/plugins.json. Edit those, then run site/sync.", 1))
	fmt.Fprintf(&b, "# %s\n\n%s\n\n[Install](#install) · [Configure](#configure) · [Use](#use)\n", p.Title, escapeEJS(p.Summary))

	b.WriteString("\n## Install\n\n")
	install, err := part("install")
	if err != nil {
		return "", err
	}
	if install == "" {
		install = generatedInstall(root, p, m)
	}
	b.WriteString(install)

	b.WriteString("\n## Configure\n\n")
	configure, err := part("configure")
	if err != nil {
		return "", err
	}
	b.WriteString(configure)
	owned := 0
	for _, s := range secs {
		if s.Plugin == p.Name {
			owned++
		}
	}
	if owned > 0 {
		b.WriteString("\n### pocketone.yaml reference\n\n")
		for _, s := range secs {
			if s.Plugin == p.Name {
				b.WriteString(sectionTable(s))
			}
		}
	} else if configure == "" {
		b.WriteString("Nothing to configure.\n")
	}

	b.WriteString("\n## Use\n\n")
	use, err := part("use")
	if err != nil {
		return "", err
	}
	b.WriteString(use)

	b.WriteString("\n## More\n\n")
	fmt.Fprintf(&b, "- [How it works](%s/blob/main/%s/README.md): the plugin's README, for developers\n", repoURL, dir)
	for _, g := range p.guides {
		fmt.Fprintf(&b, "- [%s](/docs/plugins/%s/%s)\n", g.title, p.Name, g.slug)
	}
	fmt.Fprintf(&b, "- [Questions and ideas](%s/discussions)\n", repoURL)
	return b.String(), nil
}

func editLink(src string) string {
	return fmt.Sprintf("<p class=\"edit-page\"><a href=\"%s/edit/main/%s\">Edit this section</a></p>\n", repoURL, src)
}

func generatedInstall(root string, p *plugin, m *manifest) string {
	if p.Group == m.Groups[0].ID {
		return "Built into `pocketone`. There's nothing to install.\n"
	}
	example := "ext/" + p.Name + "/example/main.go"
	src, err := os.ReadFile(filepath.Join(root, example))
	if err != nil {
		return fmt.Sprintf("Not in the `pocketone` binary: add `github.com/pockethost/pocketone/ext/%s` to your own build. See the [README](%s/blob/main/ext/%s/README.md).\n", p.Name, repoURL, p.Name)
	}
	return fmt.Sprintf("Not in the `pocketone` binary: add it to your own Go build. A minimal example, from [`%s`](%s/blob/main/%s):\n\n```go\n%s```\n",
		example, repoURL, example, escapeEJS(strings.TrimRight(string(src), "\n")+"\n"))
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
	for _, p := range m.Plugins {
		titles[p.Name] = p.Title
	}
	var b strings.Builder
	b.WriteString(frontMatter("pocketone.yaml reference", "Every section of the pocketone.yaml config file, generated from the pocketone binary.", ""))
	b.WriteString("# pocketone.yaml reference\n\nEvery section the `pocketone` binary understands, from `pocketone config --json`. Each plugin owns its own section. `pocketone config check` checks a file. How the file works (where it's found, `.env` secrets) is on the [Config](/docs/plugins/config) page.\n\n")
	for _, s := range secs {
		b.WriteString(sectionTable(s))
		if t, ok := titles[s.Plugin]; ok && s.Plugin != "config" {
			fmt.Fprintf(&b, "Owned by [%s](/docs/plugins/%s).\n\n", t, s.Plugin)
		}
	}
	return b.String()
}

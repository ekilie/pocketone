package main

// The CLI, REST and MCP references (/docs/reference/*) and each contract's and extension's "Commands and APIs"
// section, from `pocketone reference` (clicmdv1.Reference): the standard binary's command tree, and every contract
// with an HTTP API linked into it, with its operations and who provides it there.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const referenceOut = "site/pb_pages/(main)/docs/reference"

// reference mirrors clicmdv1.Reference (site/sync is its own module and doesn't import PocketOne).
type reference struct {
	Global    []flagRef     `json:"global"`
	Commands  []commandRef  `json:"commands"`
	Contracts []contractRef `json:"contracts"`
}

type flagRef struct {
	Name, Shorthand, Type, Default, Usage string
}

type commandRef struct {
	Path, Use, Short, Long string
	Aliases                []string
	Flags                  []flagRef
	Runnable               bool
	Source                 string
}

type contractRef struct {
	ID, Prefix, Provider string
	Routed, Provided     bool
	Ops                  []opRef
}

type opRef struct {
	Name, Desc, Method, REST, Scope, Out, Command, Tool string
	RawIn, ReadOnly                                     bool
	Input                                               []fieldRef
}

type fieldRef struct {
	Name, Flag, Type string
}

func (r *reference) contract(id string) *contractRef {
	if r == nil {
		return nil
	}
	for i := range r.Contracts {
		if r.Contracts[i].ID == id {
			return &r.Contracts[i]
		}
	}
	return nil
}

// --- shared bits --------------------------------------------------------------------------------------------------

// commandLine is how an operation's command is typed: its path, then a flag per input field.
func (op opRef) commandLine() string {
	if op.Command == "" {
		return ""
	}
	s := "pocketone " + op.Command
	for _, f := range op.Input {
		s += " --" + f.Flag + " " + placeholder(f.Type)
	}
	if op.RawIn {
		s += " --file <path>"
	}
	return s
}

func placeholder(t string) string {
	switch t {
	case "bool":
		return "true"
	case "int", "float":
		return "<n>"
	case "time":
		return "<time>"
	case "strings":
		return "<a,b>"
	case "json":
		return "<json>"
	}
	return "<value>"
}

func (op opRef) restLine() string { return op.Method + " " + op.REST }

func scopeText(s string) string {
	if s == "" || s == "public" {
		return "public"
	}
	return "`" + s + "`"
}

func code(s string) string {
	if s == "" {
		return "–"
	}
	return "`" + s + "`"
}

// firstSentence is a description's first sentence, for table cells.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}

func contractURL(id string) string { return "/docs/contracts/" + id }

// extensionLink links an extension's page by its catalogue title, or names it.
func extensionLink(m *manifest, name string) string {
	for _, p := range m.Extensions {
		if p.Name == name {
			if p.Link != "" {
				return p.Title
			}
			return "[" + p.Title + "](" + readmeURL(p) + ")"
		}
	}
	return name
}

// sourceText says, for a command reference, who answers a command.
func sourceText(m *manifest, src string) string {
	switch {
	case src == "pocketbase":
		return "PocketBase"
	case src == "main":
		return "the `pocketone` binary"
	case strings.HasPrefix(src, "ext/"):
		return extensionLink(m, strings.TrimPrefix(src, "ext/"))
	case strings.HasPrefix(src, "contract:"):
		id := strings.TrimPrefix(src, "contract:")
		return "[" + id + "](" + contractURL(id) + ") (generated)"
	case strings.HasPrefix(src, "contracts/"):
		id := strings.TrimPrefix(src, "contracts/")
		return "[" + id + "](" + contractURL(id) + ")'s client"
	}
	return src
}

// groupSource is a group command's source: its runnable subcommands', when they agree.
func groupSource(r *reference, path string) string {
	src := ""
	for _, c := range r.Commands {
		if !c.Runnable || !strings.HasPrefix(c.Path, path+" ") {
			continue
		}
		s := c.Source
		if strings.HasPrefix(s, "contract:") {
			s = "contracts" // generated commands under one group come from many contracts
		}
		if src == "" {
			src = s
		} else if src != s {
			return ""
		}
	}
	return src
}

func flagTable(b *strings.Builder, flags []flagRef) {
	if len(flags) == 0 {
		return
	}
	b.WriteString("| Flag | Type | Default | |\n|---|---|---|---|\n")
	for _, f := range flags {
		name := "`--" + f.Name + "`"
		if f.Shorthand != "" {
			name = "`-" + f.Shorthand + "`, " + name
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", name, f.Type, code(f.Default), flagMentions(cell(prose(f.Usage))))
	}
	b.WriteString("\n")
}

var flagMentionRe = regexp.MustCompile(`(^|[\s(])(--[a-zA-Z][a-zA-Z0-9-]*(?:=[^\s,;)]+)?)`)

// prose escapes text from the binary (help, doc comments) for Markdown: "serve <domain>" must not read as a tag, nor
// "pb_*,pb_*/**/*" as emphasis.
func prose(s string) string {
	return strings.NewReplacer("<", "&lt;", ">", "&gt;", "*", `\*`, "_", `\_`).Replace(s)
}

// flagMentions sets the flags a usage text names (--https, --dev=false) as code, so they read as flags and don't
// break across lines.
func flagMentions(s string) string { return flagMentionRe.ReplaceAllString(s, "$1`$2`") }

// --- per contract ---------------------------------------------------------------------------------------------------

// contractSurfaces is a contract page's "Commands and APIs" section: each operation as a REST call, a command and an
// MCP tool. "" for a contract with no HTTP API.
func contractSurfaces(r *reference, id string) string {
	c := r.contract(id)
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Commands and APIs\n\n")
	if c.Routed {
		fmt.Fprintf(&b, "Its own HTTP API under `%s/`, described above. It has no generated commands or MCP tools. [Remote CLI](/docs/features/remote) explains how PocketOne apps are managed from afar.\n\n", c.Prefix)
		return b.String()
	}
	b.WriteString("Every operation is a REST endpoint, a `pocketone` command and an MCP tool, all generated from the contract and behind the same token scope check and hooks. See [Remote CLI](/docs/features/remote), or the full [CLI](/docs/reference/cli), [REST](/docs/reference/rest) and [MCP](/docs/reference/mcp) references.\n\n")
	for _, op := range c.Ops {
		fmt.Fprintf(&b, "### %s\n\n", op.Name)
		if op.Desc != "" {
			b.WriteString(prose(op.Desc) + "\n\n")
		}
		b.WriteString("| | |\n|---|---|\n")
		fmt.Fprintf(&b, "| REST | `%s` |\n", op.restLine())
		if cl := op.commandLine(); cl != "" {
			fmt.Fprintf(&b, "| CLI | `%s` |\n", cl)
		}
		if op.Tool != "" {
			fmt.Fprintf(&b, "| MCP | `%s` |\n", op.Tool)
		}
		fmt.Fprintf(&b, "| Token scope | %s |\n", scopeText(op.Scope))
		switch {
		case op.Out == "stream":
			b.WriteString("| Answer | a stream of JSON entries (server-sent events); the command prints one line each |\n")
		case op.Out == "raw":
			b.WriteString("| Answer | a file or page, not JSON |\n")
		}
		if op.RawIn {
			b.WriteString("| Body | the raw upload, not JSON |\n")
		}
		b.WriteString("\n")
		if len(op.Input) > 0 {
			b.WriteString("| Input field | Flag | Type |\n|---|---|---|\n")
			for _, f := range op.Input {
				fmt.Fprintf(&b, "| `%s` | `--%s` | %s |\n", f.Name, f.Flag, f.Type)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// insertBefore puts block before the first "## <heading>" line outside code, or at the end.
func insertBefore(body, heading, block string) string {
	if block == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	fence := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fence = !fence
		}
		if !fence && strings.TrimSpace(l) == "## "+heading {
			return strings.Join(lines[:i], "\n") + "\n" + strings.TrimRight(block, "\n") + "\n\n" + strings.Join(lines[i:], "\n")
		}
	}
	return strings.TrimRight(body, "\n") + "\n\n" + block
}

// --- per extension --------------------------------------------------------------------------------------------------

var indexRowRe = regexp.MustCompile(`^\| \[([^\]]+)\]\([^)]*\) \| (provides(?: \(default\))?) \|`)

// providedContracts reads each contract README's generated Extensions table: extension name → the contracts it
// provides.
func providedContracts(root string, cs []contractPage) map[string][]string {
	out := map[string][]string{}
	for _, c := range cs {
		raw, err := os.ReadFile(filepath.Join(root, c.Src))
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(raw), "\n") {
			if m := indexRowRe.FindStringSubmatch(l); m != nil {
				out[m[1]] = append(out[m[1]], c.ID)
			}
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// extensionSurfaces is an extension page's "Commands and APIs" section: the commands it adds to `pocketone` and the
// operations of the contracts it provides. "" when it adds none.
func extensionSurfaces(m *manifest, p *extension) string {
	r := m.ref
	if r == nil {
		return ""
	}
	var b strings.Builder
	var cmds []commandRef
	for _, c := range r.Commands {
		if c.Runnable && c.Source == "ext/"+p.Name {
			cmds = append(cmds, c)
		}
	}
	if len(cmds) > 0 {
		b.WriteString("### Commands\n\nWhat it adds to the `pocketone` binary ([CLI reference](/docs/reference/cli)):\n\n| Command | |\n|---|---|\n")
		for _, c := range cmds {
			fmt.Fprintf(&b, "| [`%s`](/docs/reference/cli#%s) | %s |\n", strings.TrimSpace("pocketone "+c.Path+" "+c.Use), anchor("pocketone "+c.Path), cell(prose(c.Short)))
		}
		b.WriteString("\n")
	}
	for _, id := range m.provides[p.Name] {
		c := r.contract(id)
		if c == nil {
			continue
		}
		fmt.Fprintf(&b, "### [%s](%s)\n\n", id, contractURL(id))
		if c.Routed {
			fmt.Fprintf(&b, "Its HTTP API under `%s/`, as the contract describes.\n\n", c.Prefix)
			continue
		}
		b.WriteString("| Operation | CLI | REST | MCP |\n|---|---|---|---|\n")
		for _, op := range c.Ops {
			fmt.Fprintf(&b, "| [%s](%s#%s) | %s | %s | %s |\n", op.Name, contractURL(id), anchor(op.Name),
				code(op.commandLine()), code(op.restLine()), code(op.Tool))
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return ""
	}
	return "## Commands and APIs\n\nGenerated from the `pocketone` binary and the contracts this extension provides.\n\n" + b.String()
}

// anchor is the id the site gives a heading (lowercase, words joined by "-").
func anchor(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// --- the references -------------------------------------------------------------------------------------------------

func cliReference(m *manifest) string {
	r := m.ref
	var b strings.Builder
	b.WriteString(refFrontMatter("CLI reference", "Every command of the pocketone binary: PocketBase's own, the standard extensions', and one per contract operation."))
	b.WriteString("# CLI reference\n\nEvery command of the `pocketone` binary with its [standard extensions](/docs/extensions/standard): PocketBase's own, the ones extensions add, and a `pocketone remote …` command for every operation of every contract, generated from it ([how](/docs/features/remote)). `pocketone <command> --help` says the same.\n\n")
	b.WriteString("## Global flags\n\nEvery command takes these.\n\n")
	flagTable(&b, r.Global)
	tops := map[string]bool{}
	var order []string
	for _, c := range r.Commands {
		top, _, _ := strings.Cut(c.Path, " ")
		if !tops[top] {
			tops[top] = true
			order = append(order, top)
		}
	}
	for _, top := range order {
		for _, c := range r.Commands {
			if c.Path != top && !strings.HasPrefix(c.Path, top+" ") {
				continue
			}
			level := "###"
			if c.Path == top {
				level = "##"
			}
			fmt.Fprintf(&b, "%s pocketone %s\n\n", level, c.Path)
			if c.Short != "" {
				b.WriteString(prose(c.Short) + "\n\n")
			}
			if !c.Runnable {
				src := groupSource(r, c.Path)
				if src != "" && src != "contracts" {
					b.WriteString("From " + sourceText(m, src) + ".\n\n")
				}
				if len(c.Flags) > 0 {
					b.WriteString("Every `pocketone " + c.Path + "` command takes these.\n\n")
					flagTable(&b, c.Flags)
				}
				continue
			}
			fmt.Fprintf(&b, "```\npocketone %s\n```\n\n", strings.TrimSpace(c.Path+" "+c.Use))
			if c.Long != "" && c.Long != c.Short {
				b.WriteString(prose(c.Long) + "\n\n")
			}
			if len(c.Aliases) > 0 {
				b.WriteString("Aliases: " + code(strings.Join(c.Aliases, "`, `")) + "\n\n")
			}
			flagTable(&b, c.Flags)
			if c.Source != "" {
				b.WriteString("From " + sourceText(m, c.Source) + ".\n\n")
			}
		}
	}
	return b.String()
}

func restReference(m *manifest) string {
	r := m.ref
	var b strings.Builder
	b.WriteString(refFrontMatter("REST reference", "Every contract API of a PocketOne app with its standard extensions: method, path, token scope and input."))
	b.WriteString("# REST reference\n\nEvery contract API a PocketOne app with its [standard extensions](/docs/extensions/standard) answers, under `/api/_c/<contract>/`. PocketBase's own API (records, collections, auth, files, settings…) is unchanged; see the [PocketBase docs](/docs/pocketbase).\n\n")
	b.WriteString("- **Auth:** an `Authorization` header with a superuser's auth token or an [API token](/docs/extensions/tokens). Each operation needs its token scope; a superuser's own auth token passes every scope.\n")
	b.WriteString("- **Discovery:** `GET /api/_c/remote/v1` says who you are and lists every contract the app offers ([remote/v1](/docs/contracts/remote/v1)).\n")
	b.WriteString("- **Input:** a JSON body for `POST`, query parameters for `GET`. **Answers** are JSON; streams are server-sent events.\n")
	b.WriteString("- **Same as the CLI and MCP:** every operation here is also a `pocketone` command and an MCP tool, behind the same checks ([how](/docs/features/remote)).\n\n")
	for _, c := range r.Contracts {
		fmt.Fprintf(&b, "## %s\n\n", c.ID)
		prov := ""
		if c.Provided && c.Provider != "" {
			prov = " Answered by " + extensionLink(m, c.Provider) + " in the standard set."
		}
		if c.Routed {
			fmt.Fprintf(&b, "Its own API under `%s/`: see [%s](%s).%s\n\n", c.Prefix, c.ID, contractURL(c.ID), prov)
			continue
		}
		fmt.Fprintf(&b, "[Contract page](%s).%s\n\n", contractURL(c.ID), prov)
		b.WriteString("| Endpoint | What it does | Token scope | Input |\n|---|---|---|---|\n")
		for _, op := range c.Ops {
			var in []string
			for _, f := range op.Input {
				in = append(in, "`"+f.Name+"` ("+f.Type+")")
			}
			if op.RawIn {
				in = append(in, "raw body")
			}
			what := firstSentence(op.Desc)
			if op.Out == "stream" {
				what += " (stream)"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", op.restLine(), cell(prose(what)), scopeText(op.Scope), strings.Join(in, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func mcpReference(m *manifest) string {
	r := m.ref
	var b strings.Builder
	b.WriteString(refFrontMatter("MCP reference", "Every MCP tool a PocketOne app's contracts add at /mcp, with its token scope and arguments."))
	b.WriteString("# MCP reference\n\nA PocketOne app serves MCP at `/mcp` ([MCP](/docs/extensions/mcp)): PocketBase's whole API as `pb_*` tools, the tools your `pb_hooks` add, and one tool per operation of every contract the app provides, listed here for the [standard set](/docs/extensions/standard). Each contract tool calls the contract, so it passes the same token scope check and hooks as the REST API and the CLI ([how](/docs/features/remote)). Uploads and downloads have no tool.\n\n")
	for _, c := range r.Contracts {
		var tools []opRef
		for _, op := range c.Ops {
			if op.Tool != "" {
				tools = append(tools, op)
			}
		}
		if len(tools) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", c.ID)
		prov := ""
		if c.Provided && c.Provider != "" {
			prov = " Answered by " + extensionLink(m, c.Provider) + " in the standard set."
		}
		fmt.Fprintf(&b, "[Contract page](%s).%s\n\n", contractURL(c.ID), prov)
		b.WriteString("| Tool | What it does | Reads only | Token scope | Arguments |\n|---|---|---|---|---|\n")
		for _, op := range tools {
			var in []string
			for _, f := range op.Input {
				in = append(in, "`"+f.Name+"` ("+f.Type+")")
			}
			ro := ""
			if op.ReadOnly {
				ro = "yes"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", op.Tool, cell(prose(firstSentence(op.Desc))), ro, scopeText(op.Scope), strings.Join(in, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func refFrontMatter(title, desc string) string {
	return strings.Replace(frontMatter(title, desc, ""), "site/extensions.json and each extension's README front matter",
		"the pocketone binary (`pocketone reference`) and each contract's README", 1)
}

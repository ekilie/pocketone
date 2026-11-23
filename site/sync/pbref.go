package main

// The PocketBase JavaScript API (/docs/reference/javascript/pocketbase and a page per Go namespace under it): the
// globals, hooks, classes and types every PocketOne app's pb_hooks and page code get from core, read from
// ext/jsvm/types.d.ts (generated from the Go code, so it is PocketOne's own JS API, hooks included, and its doc
// comments are the Go ones). Types link to their entries wherever they appear.

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// pbRefPrefixes are the PocketBase API reference's pages: on the site and in llms.txt, but too big for llms-full.txt,
// the Agent Skills and the site's ask box.
var pbRefPrefixes = []string{pbJSURL, pbGoURL}

const (
	pbTypesFile = "ext/jsvm/types.d.ts"
	pbJSURL     = "/docs/reference/javascript/pocketbase"
)

// pbType is one type in a Go namespace (core.Record), its interface blocks merged; Call is set for a Go function
// (tygoja writes those as callable interfaces), which JS reaches only through a global ($dbx.and).
type pbType struct {
	NS, Name string
	Doc      jsDocInfo
	Extends  []string
	Fields   []apiEntry
	Text     string
	Call     *tsSig
}

type pbVar struct {
	Name, Type string
	Doc        jsDocInfo
}

type pbClass struct {
	Name       string
	Ctors      []tsSig
	Implements string
	Doc        jsDocInfo
}

type pbAPI struct {
	Vars     []pbVar
	Spaces   []string              // $dbx, $apis, ... in order
	Members  map[string][]apiEntry // a space's members
	SpaceDoc map[string]jsDocInfo
	Funcs    []apiEntry
	Hooks    []apiEntry
	Classes  []pbClass
	Top      []apiType         // top-level types (PocketBase, FormData)
	EntryNS  map[string]string // a global member resolved to a Go function: that function's namespace, for its types
	NS       map[string]map[string]*pbType
	NSOrder  map[string][]string
}

var (
	pbClassRe  = regexp.MustCompile(`^declare\s+class\s+([A-Za-z_$][\w$]*)(?:\s+(?:implements|extends)\s+([^{]+?))?\s*\{`)
	pbVarRe    = regexp.MustCompile(`^declare\s+(?:var|let|const)\s+([A-Za-z_$][\w$]*)\s*:\s*(.*?);?$`)
	pbExtRe    = regexp.MustCompile(`\bextends\s+([^{]+)\{`)
	pbExportAs = regexp.MustCompile(`^export\s*\{\s*([\w$]+)\s+as\s+([\w$]+)\s*\}`)
	pbInherit  = regexp.MustCompile(`\{@inheritDoc\s+([\w$.]+)\}`)
	pbHookRe   = regexp.MustCompile(`^on[A-Z]`)
)

// block is jsBlock that also takes a declaration whose braces close on its own line ("interface X extends Y{}").
func block(lines []string, i int) ([]string, int) {
	if braceDepth(lines[i]) <= 0 {
		return nil, i
	}
	return jsBlock(lines, i)
}

// splitDoc separates a JSDoc comment written on the declaration's own line ("/** @group PocketBase */declare ...").
func splitDoc(t string) (doc, rest string) {
	if strings.HasPrefix(t, "/**") {
		if i := strings.Index(t, "*/"); i > 0 && strings.TrimSpace(t[i+2:]) != "" {
			return t[:i+2], strings.TrimSpace(t[i+2:])
		}
	}
	return "", t
}

func parsePBTypes(ts string) pbAPI {
	api := pbAPI{Members: map[string][]apiEntry{}, SpaceDoc: map[string]jsDocInfo{}, NS: map[string]map[string]*pbType{},
		NSOrder: map[string][]string{}, EntryNS: map[string]string{}}
	lines := strings.Split(ts, "\n")
	var doc []string
	inDoc := false
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		t := strings.TrimSpace(raw)
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			if inDoc {
				doc = append(doc, t)
				inDoc = !strings.Contains(t, "*/")
			}
			continue
		}
		if inDoc {
			doc = append(doc, t)
			inDoc = !strings.Contains(t, "*/")
			continue
		}
		if d, rest := splitDoc(t); d != "" {
			doc, t = []string{d}, rest
		} else if strings.HasPrefix(t, "/**") {
			doc = []string{t}
			inDoc = !strings.Contains(t, "*/")
			continue
		}
		if t == "" || strings.HasPrefix(t, "//") || t == "}" {
			continue
		}
		d := parseJSDoc(doc)
		doc = nil
		switch {
		case jsNSRe.MatchString(t):
			name := jsNSRe.FindStringSubmatch(t)[1]
			body, end := block(lines, i)
			i = end
			if strings.HasPrefix(name, "$") {
				if _, ok := api.Members[name]; !ok {
					api.Spaces = append(api.Spaces, name)
				}
				api.SpaceDoc[name] = d
				api.Members[name] = append(api.Members[name], spaceEntries(body, name)...)
			} else {
				api.readNS(name, body)
			}
		case strings.HasPrefix(t, "declare function "):
			text, end := memberText(lines, i)
			i = end
			text = text[strings.Index(text, "declare function ")+len("declare "):]
			if sig, ok := parseTSMember(text); ok {
				e := apiEntry{Path: sig.Name, Sig: sig, Doc: d}
				if pbHookRe.MatchString(sig.Name) {
					api.Hooks = append(api.Hooks, e)
				} else if !hasTag(d, "deprecated") {
					api.Funcs = append(api.Funcs, e)
				}
			}
		case pbClassRe.MatchString(t):
			m := pbClassRe.FindStringSubmatch(t)
			body, end := block(lines, i)
			i = end
			c := pbClass{Name: m[1], Implements: strings.TrimSpace(m[2]), Doc: d}
			for _, l := range body {
				if s, ok := parseTSMember(strings.TrimSpace(l)); ok && s.Name == "constructor" {
					c.Ctors = append(c.Ctors, s)
				}
			}
			api.Classes = append(api.Classes, c)
		case pbVarRe.MatchString(t):
			m := pbVarRe.FindStringSubmatch(t)
			name, typ := m[1], strings.TrimSpace(m[2])
			if strings.HasSuffix(typ, "{") { // declare const Record: { new(...): core.Record }
				body, end := block(lines, i)
				i = end
				c := pbClass{Name: name, Doc: d}
				for _, l := range body {
					l = strings.TrimSpace(l)
					if strings.HasPrefix(l, "new(") {
						if s, ok := parseTSMember("constructor" + l[3:]); ok {
							c.Ctors = append(c.Ctors, s)
							c.Implements = s.Returns
						}
					}
				}
				api.Classes = append(api.Classes, c)
				continue
			}
			if !strings.HasPrefix(name, "__") {
				api.Vars = append(api.Vars, pbVar{Name: name, Type: typ, Doc: d})
			}
		case jsInterfaceRe.MatchString(t) || strings.HasPrefix(t, "interface "):
			name := regexp.MustCompile(`^interface\s+([\w$]+)`).FindStringSubmatch(t)
			body, end := block(lines, i)
			i = end
			if name == nil || strings.Contains(raw, "// merge") || name[1] == "Function" || name[1] == "ImportMeta" {
				continue // a class's merge, or the language's own
			}
			if end < len(lines) && strings.Contains(lines[end], "// merge") {
				continue
			}
			at := apiType{Name: name[1], Doc: d, Fields: apiEntries(body, "")}
			if m := pbExtRe.FindStringSubmatch(t); m != nil {
				at.Text = "extends " + strings.TrimSpace(m[1])
			}
			api.Top = append(api.Top, at)
		case jsAliasRe.MatchString(t):
			m := jsAliasRe.FindStringSubmatch(t)
			text, end := memberText(lines, i)
			i = end
			if strings.HasPrefix(m[1], "_") || m[1] == "excludeHooks" {
				continue
			}
			api.Top = append(api.Top, apiType{Name: m[1], Doc: d, Text: strings.TrimSuffix(strings.TrimSpace(text[strings.Index(text, "=")+1:]), ";")})
		}
	}
	api.resolve()
	return api
}

func hasTag(d jsDocInfo, tag string) bool { return strings.Contains(d.Body, "@"+tag) }

// spaceEntries reads a $-namespace: its exported functions and lets, "export { _in as in }" renames applied.
func spaceEntries(body []string, space string) []apiEntry {
	entries := apiEntries(body, space+".")
	for _, l := range body {
		if m := pbExportAs.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			for i := range entries {
				if entries[i].Path == space+"."+m[1] {
					entries[i].Path, entries[i].Sig.Name = space+"."+m[2], m[2]
				}
			}
		}
	}
	return entries
}

// readNS adds a namespace block's types, merging the interface blocks of one type (tygoja writes each Go method as
// its own block).
func (api *pbAPI) readNS(ns string, body []string) {
	if api.NS[ns] == nil {
		api.NS[ns] = map[string]*pbType{}
	}
	get := func(name string) *pbType {
		t := api.NS[ns][name]
		if t == nil {
			t = &pbType{NS: ns, Name: name}
			api.NS[ns][name] = t
			api.NSOrder[ns] = append(api.NSOrder[ns], name)
		}
		return t
	}
	var doc []string
	inDoc := false
	for i := 0; i < len(body); i++ {
		t := strings.TrimSpace(body[i])
		switch {
		case inDoc:
			doc = append(doc, t)
			inDoc = !strings.Contains(t, "*/")
			continue
		case strings.HasPrefix(t, "/**"):
			doc = []string{t}
			inDoc = !strings.Contains(t, "*/")
			continue
		case t == "" || strings.HasPrefix(t, "//"):
			continue
		}
		d := parseJSDoc(doc)
		doc = nil
		if m := regexp.MustCompile(`^(?:export\s+)?interface\s+([\w$]+)`).FindStringSubmatch(t); m != nil {
			inner, end := block(body, i)
			i = end
			pt := get(m[1])
			if pt.Doc.Body == "" {
				pt.Doc = d
			}
			if e := pbExtRe.FindStringSubmatch(t); e != nil {
				for _, x := range splitTop(e[1], ',') {
					if x = strings.TrimSpace(x); x != "" && !pbHas(pt.Extends, x) {
						pt.Extends = append(pt.Extends, x)
					}
				}
			}
			// a Go function: its one member is a call signature
			for j, l := range inner {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "(") || strings.HasPrefix(l, "<") {
					text, _ := memberText(inner, j)
					if s, ok := parseTSMember(m[1] + text); ok {
						pt.Call = &s
						if cd := parseJSDoc(precedingDoc(inner, j)); cd.Body != "" {
							pt.Doc = cd
						}
					}
					break
				}
			}
			if pt.Call == nil {
				pt.Fields = append(pt.Fields, apiEntries(inner, "")...)
			}
			continue
		}
		if m := jsAliasRe.FindStringSubmatch(strings.TrimPrefix(t, "export ")); m != nil {
			text, end := memberText(body, i)
			i = end
			pt := get(m[1])
			pt.Doc = d
			pt.Text = strings.TrimSuffix(strings.TrimSpace(text[strings.Index(text, "=")+1:]), ";")
			continue
		}
		if braceDepth(body[i]) > 0 {
			_, i = jsBlock(body, i)
		}
	}
}

func pbHas(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// precedingDoc is the JSDoc block just above line j.
func precedingDoc(lines []string, j int) []string {
	end := j - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end < 0 || !strings.HasSuffix(strings.TrimSpace(lines[end]), "*/") {
		return nil
	}
	start := end
	for start >= 0 && !strings.HasPrefix(strings.TrimSpace(lines[start]), "/**") {
		start--
	}
	if start < 0 {
		return nil
	}
	var out []string
	for _, l := range lines[start : end+1] {
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

// lookup finds "core.Record" (or "Record" in namespace cur).
func (api *pbAPI) lookup(cur, name string) *pbType {
	ns, n, ok := strings.Cut(name, ".")
	if !ok {
		ns, n = cur, name
	}
	if m := api.NS[ns]; m != nil {
		return m[n]
	}
	return nil
}

// resolve turns a global's "let and: dbx.and" into the Go function it is, and fills {@inheritDoc X}.
func (api *pbAPI) resolve() {
	inherit := func(d jsDocInfo) jsDocInfo {
		if m := pbInherit.FindStringSubmatch(d.Body); m != nil {
			if t := api.lookup("", m[1]); t != nil {
				return t.Doc
			}
		}
		return d
	}
	for _, s := range api.Spaces {
		for i, e := range api.Members[s] {
			if !e.Sig.Method {
				if t := api.lookup("", strings.TrimSuffix(e.Sig.Returns, " (optional)")); t != nil && t.Call != nil {
					sig := *t.Call
					sig.Name = e.Sig.Name
					e.Sig = sig
					api.EntryNS[e.Path] = t.NS
					if e.Doc.Body == "" {
						e.Doc = t.Doc
					}
				}
			}
			e.Doc = inherit(e.Doc)
			api.Members[s][i] = e
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------
// Pages

// coreFamilies split PocketBase's core package, far too big for one page, into a page per family of types, in the
// JavaScript and the Go reference alike; the rest (App, BaseApp, helpers) stays on core's own page.
var coreFamilies = []struct{ Slug, Title string }{
	{"records", "Records and collections"}, {"fields", "Fields"}, {"events", "Events"}, {"settings", "Settings"},
}

var (
	coreRecordsRe  = regexp.MustCompile(`^(Record|Collection|BaseModel|Model|ExternalAuth|MFA|OTP|AuthOrigin|Log|Param|BaseRecord)`)
	coreSettingsRe = regexp.MustCompile(`Config|Settings|Options$`)
)

// coreFamily is the family page a core type is on ("" for core's own page).
func coreFamily(name string) string {
	switch {
	case strings.Contains(name, "Event"):
		return "events"
	case strings.HasSuffix(name, "Field") || strings.HasPrefix(name, "Field"):
		return "fields"
	case coreSettingsRe.MatchString(name):
		return "settings"
	case coreRecordsRe.MatchString(name):
		return "records"
	}
	return ""
}

// pbNSURL is a namespace's page.
func pbNSURL(ns string) string { return pbJSURL + "/" + ns }

// typeHTML renders a type expression as <code> with every known type linked (cur: the page's namespace, "" on the
// globals page). It is safe in a Markdown table cell.
func (api *pbAPI) typeHTML(cur, typ string) string {
	typ = pbTygoja.Replace(typ)
	var b strings.Builder
	last := 0
	for _, loc := range pbIdentRe.FindAllStringIndex(typ, -1) {
		b.WriteString(html.EscapeString(typ[last:loc[0]]))
		id := typ[loc[0]:loc[1]]
		if u := api.typeURL(cur, id); u != "" {
			fmt.Fprintf(&b, `<a href="%s">%s</a>`, u, html.EscapeString(id))
		} else {
			b.WriteString(html.EscapeString(id))
		}
		last = loc[1]
	}
	b.WriteString(html.EscapeString(typ[last:]))
	return "<code>" + strings.ReplaceAll(b.String(), "|", "&#124;") + "</code>"
}

// pbTygoja spells out the generator's helper types.
var pbTygoja = strings.NewReplacer("_TygojaDict", "{ [key: string]: any }", "_TygojaAny", "any")

var pbIdentRe = regexp.MustCompile(`[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*`)

func (api *pbAPI) typeURL(cur, id string) string {
	if t := api.lookup(cur, id); t != nil && t.Call == nil && !strings.HasPrefix(t.Name, "_") {
		u := pbNSURL(t.NS)
		if t.NS == "core" {
			if f := coreFamily(t.Name); f != "" {
				u += "/" + f
			}
		}
		return u + "#" + gid(t.NS+"."+t.Name)
	}
	if !strings.Contains(id, ".") {
		for _, t := range api.Top {
			if t.Name == id {
				return pbJSURL + "#" + gid(id)
			}
		}
	}
	return ""
}

// pbNamespaces is every namespace with at least one type to show, PocketBase's own first (those with a Go page).
func (api *pbAPI) namespaces() []string {
	var out []string
	for ns, order := range api.NSOrder {
		for _, n := range order {
			if t := api.NS[ns][n]; t.Call == nil && !strings.HasPrefix(n, "_") {
				out = append(out, ns)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// pbGoPage maps a JS namespace to its PocketBase Go package page, "" when it is from another Go library.
func pbGoPage(ns string, gopkgs []goRefPackage) string {
	var hit string
	for _, p := range gopkgs {
		if p.PocketBase && p.Name == ns {
			if hit != "" {
				return ""
			}
			hit = p.URL
		}
	}
	return hit
}

func (api *pbAPI) writeEntry(b *strings.Builder, e apiEntry) {
	cur := api.EntryNS[e.Path]
	heading := e.Path
	if e.Sig.Method {
		heading += "()"
	}
	fmt.Fprintf(b, "### %s\n\n", heading)
	fmt.Fprintf(b, "```ts\n%s\n```\n\n", e.Sig.display(strings.TrimSuffix(e.Path, e.Sig.Name)))
	if body := api.docLinks(cur, cleanPBDoc(e.Doc.Body)); body != "" {
		b.WriteString(body + "\n\n")
	}
	if e.Sig.Method && len(e.Sig.Params) > 0 {
		b.WriteString("| Parameter | Type |\n|---|---|\n")
		for _, p := range e.Sig.Params {
			opt := ""
			if p.Optional {
				opt = " (optional)"
			}
			fmt.Fprintf(b, "| `%s` | %s%s |\n", p.Name, api.typeHTML(cur, p.Type), opt)
		}
		b.WriteString("\n")
	}
	ret := e.Sig.Returns
	if !e.Sig.Method {
		ret = strings.TrimSuffix(ret, " (optional)")
	}
	if ret != "" && ret != "void" {
		fmt.Fprintf(b, "Returns %s.\n\n", api.typeHTML(cur, ret))
	}
}

// cleanPBDoc drops the tags the page shows elsewhere (or never: @group, @namespace).
func cleanPBDoc(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "@group") || strings.HasPrefix(t, "@namespace") || strings.HasPrefix(t, "{@inheritDoc") {
			continue
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

var pbGoLinkRe = regexp.MustCompile(`\[([A-Z][\w]*(?:\.[A-Za-z_]\w*)*|[a-z]\w*\.[A-Z]\w*(?:\.[A-Za-z_]\w*)*)\]`)

// docLinks turns Go doc links ("[App.CreateBackup]", "[router.NewApiError]") into code with JS names, linked to the
// type when the reference has it. Code blocks and Markdown links are left alone.
func (api *pbAPI) docLinks(cur, s string) string {
	lines := strings.Split(s, "\n")
	fence := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		var b strings.Builder
		last := 0
		for _, m := range pbGoLinkRe.FindAllStringSubmatchIndex(l, -1) {
			if m[1] < len(l) && (l[m[1]] == '(' || l[m[1]] == '[') || m[0] > 0 && l[m[0]-1] == ']' {
				continue
			}
			b.WriteString(l[last:m[0]])
			name := l[m[2]:m[3]]
			parts := strings.Split(name, ".")
			typeName, js := name, name
			if len(parts) > 1 { // the last part is a member: JS names start lower case
				last := parts[len(parts)-1]
				js = strings.Join(parts[:len(parts)-1], ".") + "." + strings.ToLower(last[:1]) + last[1:]
				if t := api.lookup(cur, name); t == nil || t.Call != nil {
					typeName = strings.Join(parts[:len(parts)-1], ".")
				}
			}
			if u := api.typeURL(cur, typeName); u != "" {
				fmt.Fprintf(&b, "[`%s`](%s)", js, u)
			} else {
				fmt.Fprintf(&b, "`%s`", js)
			}
			last = m[1]
		}
		b.WriteString(l[last:])
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}

// pbGlobalsPage is /docs/reference/javascript/pocketbase.
func pbGlobalsPage(api pbAPI, gopkgs []goRefPackage) string {
	var b strings.Builder
	b.WriteString(refFrontMatter("PocketBase JavaScript API", "Every global, hook and class PocketOne's core gives pb_hooks and page code: $app, $apis, $security, routerAdd, cronAdd, onRecordCreate and the rest, with signatures and linked types."))
	b.WriteString("# PocketBase JavaScript API\n\n")
	b.WriteString("What every PocketOne app's `pb_hooks` and page code get from core, before any extension adds its own: " +
		"the globals (`$app` and the `$`-helpers), the hooks, the functions and the classes. Types link to their pages, one per Go " +
		"namespace (" + "[core](" + pbNSURL("core") + "), [dbx](" + pbNSURL("dbx") + "), [router](" + pbNSURL("router") + ") and the [rest](/docs/reference/javascript#pocketbase)). " +
		"Generated from PocketOne's `types.d.ts`, the same file your editor reads, so it matches the binary you run. " +
		"Go names are written in JS form: Go's `FindRecordById` is `findRecordById`.\n\n")

	b.WriteString("## Globals\n\n")
	for _, v := range api.Vars {
		fmt.Fprintf(&b, "### %s\n\n```ts\n%s: %s\n```\n\n", v.Name, v.Name, v.Type)
		if body := cleanPBDoc(v.Doc.Body); body != "" {
			b.WriteString(body + "\n\n")
		}
		fmt.Fprintf(&b, "Type: %s.\n\n", api.typeHTML("", v.Type))
	}
	for _, s := range api.Spaces {
		fmt.Fprintf(&b, "## %s\n\n", s)
		if body := cleanPBDoc(api.SpaceDoc[s].Body); body != "" {
			b.WriteString(body + "\n\n")
		}
		for _, e := range api.Members[s] {
			if !e.Group {
				api.writeEntry(&b, e)
			}
		}
	}
	if len(api.Funcs) > 0 {
		b.WriteString("## Functions\n\n")
		for _, e := range api.Funcs {
			api.writeEntry(&b, e)
		}
	}
	if len(api.Classes) > 0 {
		b.WriteString("## Classes\n\n")
		for _, c := range api.Classes {
			fmt.Fprintf(&b, "### %s\n\n", c.Name)
			if len(c.Ctors) > 0 {
				b.WriteString("```ts\n")
				for _, s := range c.Ctors {
					s.Returns = ""
					fmt.Fprintf(&b, "new %s\n", strings.TrimPrefix(s.display(""), "constructor"))
				}
				b.WriteString("```\n\n")
			}
			if body := cleanPBDoc(c.Doc.Body); body != "" {
				b.WriteString(body + "\n\n")
			}
			if c.Implements != "" {
				fmt.Fprintf(&b, "Is a %s.\n\n", api.typeHTML("", c.Implements))
			}
		}
	}
	if len(api.Hooks) > 0 {
		b.WriteString("## Hooks\n\n")
		b.WriteString("Each hook takes a handler that gets the event; call `e.next()` to go on to the next handler (and the action " +
			"itself). Hooks marked with tags take collection names (or ids) after the handler, so the handler runs only for those.\n\n")
		b.WriteString("| Hook | Event | |\n|---|---|---|\n")
		app := api.NS["core"]["App"]
		for _, h := range api.Hooks {
			ev := ""
			if len(h.Sig.Params) > 0 {
				if m := regexp.MustCompile(`\(e:\s*([\w.]+)\)`).FindStringSubmatch(h.Sig.Params[0].Type); m != nil {
					ev = api.typeHTML("", m[1])
				}
			}
			name := h.Sig.Name + "(handler"
			if len(h.Sig.Params) > 1 {
				name += ", ...tags"
			}
			name += ")"
			desc := ""
			if app != nil {
				for _, f := range app.Fields {
					if f.Sig.Name == h.Sig.Name {
						desc = f.Doc.Summary
					}
				}
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", name, ev, cell(api.docLinks("core", desc)))
		}
		b.WriteString("\n")
	}
	if len(api.Top) > 0 {
		b.WriteString("## Types\n\n")
		for _, t := range api.Top {
			fmt.Fprintf(&b, "### %s\n\n", t.Name)
			if body := cleanPBDoc(t.Doc.Body); body != "" {
				b.WriteString(body + "\n\n")
			}
			if t.Text != "" {
				fmt.Fprintf(&b, "%s\n\n", api.typeHTML("", t.Text))
			}
			api.writeFields(&b, "", t.Fields)
		}
	}
	return escapeEJS(b.String())
}

func (api *pbAPI) writeFields(b *strings.Builder, cur string, fields []apiEntry) {
	var rows []apiEntry
	for _, f := range fields {
		if !f.Group {
			rows = append(rows, f)
		}
	}
	if len(rows) == 0 {
		return
	}
	b.WriteString("| Member | Type | |\n|---|---|---|\n")
	for _, f := range rows {
		typ := strings.TrimSuffix(f.Sig.Returns, " (optional)")
		name := f.Sig.Name
		if f.Sig.Method {
			typ = strings.TrimPrefix(f.Sig.display(""), f.Sig.Name)
			name += "()"
		} else if strings.HasSuffix(f.Sig.Returns, " (optional)") {
			name += "?"
		}
		fmt.Fprintf(b, "| `%s` | %s | %s |\n", name, api.typeHTML(cur, typ), cell(api.docLinks(cur, f.Doc.Summary)))
	}
	b.WriteString("\n")
}

// pbNSPage is one namespace's page: its types, with their members.
func pbNSPage(api pbAPI, ns string, gopkgs []goRefPackage, fam string) string {
	var b strings.Builder
	goPage := pbGoPage(ns, gopkgs)
	title := ns
	for _, f := range coreFamilies {
		if f.Slug == fam {
			title = ns + ": " + f.Title
		}
	}
	desc := "The " + title + " types PocketOne's JS reaches through $app, the other globals and hook events: each with its members, signatures and linked types."
	b.WriteString(refFrontMatter(title+" (JavaScript)", desc))
	fmt.Fprintf(&b, "# %s\n\n", title)
	if fam != "" {
		fmt.Fprintf(&b, "Part of [core](%s), PocketBase's Go package `core` as JavaScript sees it. The same types in Go: [core: %s](%s).\n\n",
			pbNSURL(ns), strings.TrimPrefix(title, ns+": "), goPage+"/"+fam)
	} else if goPage != "" {
		fmt.Fprintf(&b, "Types from PocketBase's Go package `%s`, as JavaScript sees them: you get these objects from "+
			"[`$app` and the other globals](%s) and from hook events, and call their methods by their JS names "+
			"(Go's `FindRecordById` is `findRecordById`). The same package in Go: [%s](%s).\n\n", ns, pbJSURL, ns, goPage)
	} else {
		fmt.Fprintf(&b, "Types from the Go package `%s` (a library PocketBase builds on), as JavaScript sees them: they turn up "+
			"as values of [PocketBase's types](%s) and of hook events.\n\n", ns, pbJSURL)
	}
	if ns == "core" && fam == "" {
		b.WriteString("Its types are on five pages: this one (the app, `core.App`, and what doesn't fit elsewhere)")
		for _, f := range coreFamilies {
			fmt.Fprintf(&b, ", [%s](%s)", f.Title, pbNSURL(ns)+"/"+f.Slug)
		}
		b.WriteString(".\n\n")
	}
	for _, n := range api.NSOrder[ns] {
		t := api.NS[ns][n]
		if t.Call != nil || strings.HasPrefix(n, "_") || (ns == "core" && coreFamily(n) != fam) {
			continue
		}
		fmt.Fprintf(&b, "## %s.%s\n\n", ns, n)
		if body := api.docLinks(ns, cleanPBDoc(t.Doc.Body)); body != "" {
			b.WriteString(body + "\n\n")
		}
		var embeds []string
		for _, x := range t.Extends {
			if a := api.lookup(ns, x); a != nil && strings.HasPrefix(a.Name, "_") && a.Text != "" {
				for _, y := range strings.Split(a.Text, "&") {
					embeds = append(embeds, api.typeHTML(ns, strings.TrimSpace(y)))
				}
			} else {
				embeds = append(embeds, api.typeHTML(ns, x))
			}
		}
		if len(embeds) > 0 {
			fmt.Fprintf(&b, "Has everything %s has.\n\n", strings.Join(embeds, ", "))
		}
		if t.Text != "" && len(t.Fields) == 0 {
			fmt.Fprintf(&b, "%s\n\n", api.typeHTML(ns, t.Text))
		}
		api.writeFields(&b, ns, t.Fields)
	}
	return escapeEJS(b.String())
}

// pbJSPages reads ext/jsvm/types.d.ts and renders the globals page and a page per namespace, keyed by URL.
func pbJSPages(root string, gopkgs []goRefPackage) (map[string]string, []string) {
	raw, err := os.ReadFile(filepath.Join(root, pbTypesFile))
	if err != nil {
		return nil, nil
	}
	api := parsePBTypes(string(raw))
	pages := map[string]string{pbJSURL: pbGlobalsPage(api, gopkgs)}
	nss := api.namespaces()
	for _, ns := range nss {
		pages[pbNSURL(ns)] = pbNSPage(api, ns, gopkgs, "")
		if ns == "core" {
			for _, f := range coreFamilies {
				pages[pbNSURL(ns)+"/"+f.Slug] = pbNSPage(api, ns, gopkgs, f.Slug)
			}
		}
	}
	return pages, nss
}

// gid is the id the site gives a heading with this text (goldmark's auto ids: lower case, letters, digits, _ and -),
// so links to generated headings land.
func gid(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c >= 0x80:
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_':
			b.WriteByte(c)
		case c == ' ' || c == '\t' || c == '-':
			if !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
	}
	if b.Len() == 0 {
		return "heading"
	}
	return b.String()
}

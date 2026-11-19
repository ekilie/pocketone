package sandbox

import (
	"regexp"
	"strings"
	"unicode"
)

// FilterTypes implements js.TypesFilter: what the sandbox disables or hides from JS is declared `never` in the app's
// types.d.ts, so tsc refuses a call to it and editors say why. It reads the same tables the sandbox enforces
// (hiddenMethods, hiddenFields, hiddenPackages, hiddenTypes) plus the globals OnInit disables. A member is redeclared
// rather than removed: the generator gives every Go interface a `[key: string]: any` signature, so a removed member
// would quietly become `any`. Members the sandbox keeps but confines ($os.readFile under Dir, $os.getenv over Env)
// stay as they are.
func (s *Sandbox) FilterTypes(decl string) string {
	return filterTypes(decl, hiddenMembers())
}

// disabledGlobals are the namespace members OnInit replaces with a function that always fails.
var disabledGlobals = map[string][]string{
	"$os":         {"exec", "cmd", "exit"},
	"$filesystem": {"local"},
}

// hiddenMembers maps a declaration namespace to the JS names of the members to drop. A Go package's namespace in the
// generated declarations is its last path element (net/http → http).
func hiddenMembers() map[string]map[string]bool {
	out := map[string]map[string]bool{}
	add := func(ns, name string) {
		if out[ns] == nil {
			out[ns] = map[string]bool{}
		}
		out[ns][name] = true
	}
	for ns, names := range disabledGlobals {
		for _, n := range names {
			add(ns, n)
		}
	}
	for _, table := range []map[string]map[string]bool{hiddenMethods, hiddenFields} {
		for pkg, names := range table {
			ns := pkg[strings.LastIndex(pkg, "/")+1:]
			for n := range names {
				add(ns, jsName(n))
			}
		}
	}
	return out
}

// hiddenReturn matches a declared type that names a hidden type: a member returning one is hidden (HideMethod).
var hiddenReturn = func() *regexp.Regexp {
	var alts []string
	for pkg := range hiddenPackages {
		alts = append(alts, regexp.QuoteMeta(pkg[strings.LastIndex(pkg, "/")+1:])+`\.\w`)
	}
	for t := range hiddenTypes {
		i := strings.LastIndex(t, ".")
		pkg := t[:i]
		alts = append(alts, regexp.QuoteMeta(pkg[strings.LastIndex(pkg, "/")+1:]+"."+t[i+1:])+`\b`)
	}
	return regexp.MustCompile(`(^|[^\w.$])(` + strings.Join(alts, "|") + `)`)
}()

var (
	nsOpen    = regexp.MustCompile(`^declare namespace (\S+) \{`)
	ifaceOpen = regexp.MustCompile(`^ interface ([\w$]+)`)
	// a member of an interface or namespace, as the generator writes it: two spaces, then `let name:`, `name(`,
	// `name<`, `name:` or `name?:`
	memberLine = regexp.MustCompile(`^  (let |const )?([A-Za-z_$][\w$]*)\??\s*[:(<]`)
)

// filterTypes redeclares as `never` the members in hidden (per top-level namespace) and every member whose declared
// type names a hidden type, replacing the doc comment right above each.
func filterTypes(decl string, hidden map[string]map[string]bool) string {
	lines := strings.Split(decl, "\n")
	out := make([]string, 0, len(lines))
	var pending []string // a doc comment waiting for the line it documents
	inComment := false
	ns, iface := "", ""
	replaced := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inComment || (strings.HasPrefix(trimmed, "/**") && strings.HasPrefix(line, "  ") && ns != "") {
			pending = append(pending, line)
			inComment = !strings.Contains(trimmed, "*/")
			continue
		}
		switch {
		case nsOpen.MatchString(line):
			ns = nsOpen.FindStringSubmatch(line)[1]
		case line == "}":
			ns, iface = "", ""
		case ifaceOpen.MatchString(line):
			iface = ifaceOpen.FindStringSubmatch(line)[1]
		}
		drop := false
		if ns != "" {
			if m := memberLine.FindStringSubmatch(line); m != nil {
				drop = hidden[ns][m[2]] || hiddenReturn.MatchString(declaredType(line, m[0]))
				key := ns + "." + iface + "." + m[2]
				if drop && !replaced[key] { // once per interface: its merged declarations repeat members
					replaced[key] = true
					out = append(out, "  /** Not available in this app: the sandbox hides it. */")
					if m[1] != "" {
						out = append(out, "  let "+m[2]+": never")
					} else {
						out = append(out, "  "+m[2]+": never")
					}
				}
			}
		}
		if !drop {
			out = append(out, pending...)
			out = append(out, line)
		}
		pending = pending[:0]
	}
	out = append(out, pending...)
	return strings.Join(out, "\n")
}

// declaredType is the part of a member line HideField and HideMethod judge: a field's type, a method's results.
func declaredType(line, head string) string {
	rest := line[len(head)-1:]
	if strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "<") {
		if i := strings.LastIndex(rest, "):"); i >= 0 {
			return rest[i+2:]
		}
		return ""
	}
	return rest
}

// jsName is a Go name as JS sees it: a copy of jsvm's mapper, itself plugins/jsvm's convertGoToJSName
// (FindRecordById → findRecordById, DB → db, OAuth2 → oauth2).
func jsName(name string) string {
	if name == "OAuth2" {
		return "oauth2"
	}
	upper := 0
	for _, c := range name {
		if c != '_' && !unicode.IsUpper(c) && !unicode.IsDigit(c) {
			break
		}
		upper++
	}
	if upper == len(name) {
		return strings.ToLower(name)
	}
	if upper > 1 {
		return strings.ToLower(name[0:upper-1]) + name[upper-1:]
	}
	if upper == 1 {
		return strings.ToLower(name[0:1]) + name[1:]
	}
	return name
}

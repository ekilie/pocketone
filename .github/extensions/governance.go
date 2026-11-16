package main

// Governance: who owns each extension and contract, and who must approve a change. Owners and versions live in the
// front matter of each extension's README (and each contract's), so a contributor's extension carries its own:
//
//	---
//	owners: ["@github-handle"]
//	version: 0.1.0
//	---
//
// The rules (for outside pull requests; our own work merges to main directly):
//   - core (PocketBase's own code), the standard extensions, the site, docs, examples and CI need a maintainer;
//   - a community extension needs one of its owners (or a maintainer); a new extension needs a maintainer;
//   - a contract needs one of its owners (or a maintainer); a new contract needs a maintainer.
//
// Owners are read from the base branch, never from the pull request, so a pull request can't add its author as an
// owner and approve itself. Every merge that changes an extension's code bumps its version (`versions` checks it);
// its Go module tag (ext/<name>/vX.Y.Z) follows from it.

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// maintainers approve core, standard extensions, new extensions and contracts, and anything nobody else owns.
var maintainers = []string{"@benallfree"}

var (
	handleRe = regexp.MustCompile(`^@[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	semverRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

// readFront returns a README's front matter as key → raw value ("" keys absent).
func readFront(src string) map[string]string {
	m := frontRe.FindStringSubmatch(src)
	out := map[string]string{}
	if m == nil {
		return out
	}
	for _, line := range strings.Split(m[1], "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// parseOwners reads `["@a", "@b"]`.
func parseOwners(v string) []string {
	var out []string
	for _, o := range strings.Split(strings.Trim(v, "[]"), ",") {
		if o = strings.Trim(strings.TrimSpace(o), `"'`); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// checkOwnership: every extension README has owners (GitHub handles) and a semver version; an extension with code
// (a go.mod) is at 0.0.1 or later.
func checkOwnership() []string {
	entries, err := os.ReadDir("ext")
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, e := range entries {
		name := e.Name()
		f := "ext/" + name + "/README.md"
		if !e.IsDir() || name == "contracts" {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			continue // checkDocs reports a missing README
		}
		fm := readFront(string(raw))
		owners := parseOwners(fm["owners"])
		if len(owners) == 0 {
			problems = append(problems, f+`: front matter needs owners: ["@github-handle"] (docs/extensions.md#ownership)`)
		}
		for _, o := range owners {
			if !handleRe.MatchString(o) {
				problems = append(problems, fmt.Sprintf("%s: owner %q isn't a GitHub handle (@name)", f, o))
			}
		}
		v := fm["version"]
		switch {
		case !semverRe.MatchString(v):
			problems = append(problems, f+": front matter needs version: X.Y.Z (docs/extensions.md#ownership)")
		case v == "0.0.0":
			if _, err := os.Stat("ext/" + name + "/go.mod"); err == nil {
				problems = append(problems, f+": an extension with code starts at version 0.0.1 (0.0.0 is for planned ones)")
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// gitShow returns a file at a commit ("" when it doesn't exist there).
func gitShow(rev, path string) string {
	out, err := exec.Command("git", "show", rev+":"+path).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// standardMembers lists the extensions ext/standard registers on every app, at rev.
func standardMembers(rev string) map[string]bool {
	src := gitShow(rev, "ext/standard/standard.go")
	out := map[string]bool{"standard": true, "extkit": true} // the set itself and the extension kit are core-like
	for _, m := range regexp.MustCompile(`"github\.com/pockethost/pocketone/ext/([a-z0-9-]+)"`).FindAllStringSubmatch(src, -1) {
		out[m[1]] = true
	}
	return out
}

// approval is who may approve one area of a change.
type approval struct {
	Area   string   // "core", "ext/tokens (standard)", "contract restore/v1", ...
	AnyOf  []string // any one of these approves the area
	Reason string
}

// approvers lists, for a change from base to head, the areas it touches and who may approve each, from the owners
// at base.
func approvers(base, head string) ([]approval, error) {
	out, err := exec.Command("git", "diff", "--name-only", base+"..."+head).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...%s: %w", base, head, err)
	}
	std := standardMembers(base)
	areas := map[string]approval{}
	add := func(a approval) {
		if _, ok := areas[a.Area]; !ok {
			areas[a.Area] = a
		}
	}
	contractDir := regexp.MustCompile(`^ext/contracts/((?:[a-z0-9-]+/)+v[0-9]+)/`)
	for _, f := range strings.Fields(string(out)) {
		switch {
		case contractDir.MatchString(f):
			dir := contractDir.FindStringSubmatch(f)[1]
			parts := strings.Split(dir, "/")
			id := strings.Join(parts[:len(parts)-1], ".") + "/" + parts[len(parts)-1]
			readme := gitShow(base, "ext/contracts/"+dir+"/README.md")
			if readme == "" {
				add(approval{"contract " + id + " (new)", maintainers, "a new contract"})
				continue
			}
			add(approval{"contract " + id, union(parseOwners(readFront(readme)["owners"]), maintainers), "its owners"})
		case strings.HasPrefix(f, "ext/contracts/"):
			add(approval{"ext/contracts (tooling)", maintainers, "contract tooling and catalogue"})
		case strings.HasPrefix(f, "ext/"):
			name := strings.SplitN(strings.TrimPrefix(f, "ext/"), "/", 2)[0]
			readme := gitShow(base, "ext/"+name+"/README.md")
			switch {
			case std[name]:
				add(approval{"ext/" + name + " (standard)", maintainers, "a standard extension"})
			case readme == "":
				add(approval{"ext/" + name + " (new)", maintainers, "a new extension"})
			default:
				add(approval{"ext/" + name, union(parseOwners(readFront(readme)["owners"]), maintainers), "its owners"})
			}
		default:
			add(approval{"core and project files", maintainers, "PocketBase's code, site, docs, examples or CI"})
		}
	}
	var list []approval
	for _, a := range areas {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Area < list[j].Area })
	return list, nil
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// versionBumps: every extension whose code (anything but Markdown) changed from base to head has a higher version at
// head than at base. A new extension needs no bump.
func versionBumps(base, head string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", base+"..."+head).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...%s: %w", base, head, err)
	}
	changed := map[string]bool{}
	for _, f := range strings.Fields(string(out)) {
		if !strings.HasPrefix(f, "ext/") || strings.HasPrefix(f, "ext/contracts/") || strings.HasSuffix(f, ".md") {
			continue
		}
		changed[strings.SplitN(strings.TrimPrefix(f, "ext/"), "/", 2)[0]] = true
	}
	var problems []string
	for name := range changed {
		before := readFront(gitShow(base, "ext/"+name+"/README.md"))["version"]
		after := readFront(gitShow(head, "ext/"+name+"/README.md"))["version"]
		if before == "" {
			continue // new
		}
		if !semverLess(before, after) {
			problems = append(problems, fmt.Sprintf("ext/%s: its code changed, so its version must go up (now %s at both)", name, before))
		}
	}
	sort.Strings(problems)
	return problems, nil
}

func semverLess(a, b string) bool {
	pa, pb := semverRe.FindStringSubmatch(a), semverRe.FindStringSubmatch(b)
	if pa == nil || pb == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		var x, y int
		fmt.Sscan(pa[i], &x)
		fmt.Sscan(pb[i], &y)
		if x != y {
			return x < y
		}
	}
	return false
}

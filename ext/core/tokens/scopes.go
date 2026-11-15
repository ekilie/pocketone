package tokens

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Scope is a named slice of an owner's rights that a token can be limited to. Plugins and apps register their
// scopes with [Tokens.RegisterScope] (JS: `$tokens.registerScope`); a token whose scopes are set authenticates only
// on requests its scopes match (Paths + Methods, or Match), routes guard with [RequireScope], and collection rules
// read `@request.headers.x_token_scope_<name>` (see [Scope.RuleField]).
type Scope struct {
	// Name, e.g. "deploy" or "notes:write": lower-case letters, digits, ":" and "_", starting with a letter or digit.
	Name string `json:"name"`

	// Description is shown on the consent page and in the token screens.
	Description string `json:"description"`

	// Paths are URL path prefixes (e.g. "/_pocker/deploy/") where a token with this scope authenticates as its
	// owner. Elsewhere a scoped token is refused.
	Paths []string `json:"paths,omitempty"`

	// Methods, when not empty, limits Paths to these HTTP methods (e.g. GET for a read scope; GET also covers HEAD),
	// so a read and a write scope can share a path. Empty: every method.
	Methods []string `json:"methods,omitempty"`

	// Match is an optional Go guard: a token with this scope also authenticates on any request Match returns true
	// for (e.g. one collection's records API with method rules of your own). It runs before the owner is loaded,
	// so it may look only at the request, not at e.Auth.
	Match func(e *core.RequestEvent) bool `json:"-"`

	// Collections, when not empty, limits which auth collections may hold a token with this scope (collection
	// names, e.g. "_superusers"). Empty: any collection that may hold tokens.
	Collections []string `json:"collections,omitempty"`
}

// Infrastructure reports whether the scope only reaches PocketOne's own /_pocker/ routes (deploy, logs): a client
// that needs one asks for it; consent pages and MCP don't offer it otherwise.
func (s Scope) Infrastructure() bool {
	if s.Match != nil || len(s.Paths) == 0 {
		return false
	}
	for _, p := range s.Paths {
		if !strings.HasPrefix(p, "/_pocker/") {
			return false
		}
	}
	return true
}

// HeaderPrefix starts the request headers that tell rules and hooks which scopes a request holds.
const HeaderPrefix = "X-Token-Scope-"

var scopeNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9:_]*$`)

// ValidScopeName reports whether name may be registered as a scope.
func ValidScopeName(name string) bool { return len(name) <= 64 && scopeNameRe.MatchString(name) }

// Header is the request header set to "1" on a signed-in request that holds this scope, e.g. notes:write →
// X-Token-Scope-Notes-Write.
func (s Scope) Header() string { return ScopeHeader(s.Name) }

// RuleField is how a collection API rule reads the scope, e.g. @request.headers.x_token_scope_notes_write
// (PocketBase lowercases header names and turns "-" into "_"). Compare it with "1".
func (s Scope) RuleField() string { return ScopeRuleField(s.Name) }

// ScopeHeader is [Scope.Header] for a scope name.
func ScopeHeader(name string) string {
	return http.CanonicalHeaderKey(HeaderPrefix + strings.NewReplacer(":", "-", "_", "-").Replace(name))
}

// ScopeRuleField is [Scope.RuleField] for a scope name.
func ScopeRuleField(name string) string {
	return "@request.headers." + strings.ReplaceAll(strings.ToLower(ScopeHeader(name)), "-", "_")
}

// AnyTokenPaths authenticate every token, whatever its scopes: discovery routes that only tell the caller who it is.
var AnyTokenPaths = []string{"/_pocker/whoami"}

type registry struct {
	mu     sync.RWMutex
	scopes map[string]Scope
}

func (r *registry) add(s Scope) error {
	s.Name = strings.TrimSpace(s.Name)
	if !ValidScopeName(s.Name) {
		return fmt.Errorf("tokens: invalid scope name %q (lower-case letters, digits, \":\" and \"_\")", s.Name)
	}
	methods := make([]string, 0, len(s.Methods))
	for _, m := range s.Methods {
		methods = append(methods, strings.ToUpper(strings.TrimSpace(m)))
	}
	s.Methods = methods
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scopes == nil {
		r.scopes = map[string]Scope{}
	}
	if old, ok := r.scopes[s.Name]; ok {
		// several plugins may share a scope (deploy + logs under one name): merge their paths
		for _, p := range s.Paths {
			if !slices.Contains(old.Paths, p) {
				old.Paths = append(old.Paths, p)
			}
		}
		for _, m := range s.Methods {
			if !slices.Contains(old.Methods, m) {
				old.Methods = append(old.Methods, m)
			}
		}
		if old.Description == "" {
			old.Description = s.Description
		}
		if s.Match != nil {
			prev, next := old.Match, s.Match
			if prev == nil {
				old.Match = next
			} else {
				old.Match = func(e *core.RequestEvent) bool { return prev(e) || next(e) }
			}
		}
		for _, c := range s.Collections {
			if !slices.Contains(old.Collections, c) {
				old.Collections = append(old.Collections, c)
			}
		}
		s = old
	} else {
		// notes:write and notes_write would share a header: refuse the second
		for _, o := range r.scopes {
			if o.Header() == s.Header() {
				return fmt.Errorf("tokens: scope %q collides with %q (both use the %s header)", s.Name, o.Name, s.Header())
			}
		}
	}
	r.scopes[s.Name] = s
	return nil
}

func (r *registry) get(name string) (Scope, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.scopes[name]
	return s, ok
}

func (r *registry) list() []Scope {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Scope, 0, len(r.scopes))
	for _, s := range r.scopes {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// allows reports whether a token with these scopes may authenticate on this request. No scopes: everywhere.
func (r *registry) allows(scopes []string, e *core.RequestEvent) bool {
	if len(scopes) == 0 {
		return true
	}
	path := e.Request.URL.Path
	for _, p := range AnyTokenPaths {
		if pathUnder(path, p) {
			return true
		}
	}
	for _, name := range scopes {
		s, ok := r.get(name)
		if ok && s.matches(e) {
			return true
		}
	}
	return false
}

// allowsLimit is allows for a WithScopes limit, where no scopes means nothing (not full rights).
func (r *registry) allowsLimit(limit []string, e *core.RequestEvent) bool {
	if len(limit) == 0 {
		for _, p := range AnyTokenPaths {
			if pathUnder(e.Request.URL.Path, p) {
				return true
			}
		}
		return false
	}
	return r.allows(limit, e)
}

// matches: the request is under one of the scope's paths with an allowed method, or its Match guard says yes.
func (s Scope) matches(e *core.RequestEvent) bool {
	if s.methodAllowed(e.Request.Method) {
		for _, p := range s.Paths {
			if pathUnder(e.Request.URL.Path, p) {
				return true
			}
		}
	}
	return s.Match != nil && s.Match(e)
}

func (s Scope) methodAllowed(m string) bool {
	if len(s.Methods) == 0 {
		return true
	}
	m = strings.ToUpper(m)
	return slices.Contains(s.Methods, m) || (m == http.MethodHead && slices.Contains(s.Methods, http.MethodGet))
}

func pathUnder(path, prefix string) bool {
	if prefix == "" {
		return false
	}
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// Reaches reports whether a request limited to scopes could get through on method + path pattern, where a "{name}"
// segment stands for any one segment (e.g. "/api/collections/{collection}/records"). A scope with a Match guard
// might allow anything, so it counts as reaching. It's for listing (ext/core/mcp hides tools a connection can't use);
// the middleware's check on the real request is what enforces.
func (t *Tokens) Reaches(scopes []string, method, pattern string) bool {
	for _, p := range AnyTokenPaths {
		if pathUnder(pattern, p) {
			return true
		}
	}
	for _, name := range scopes {
		s, ok := t.registry.get(name)
		if !ok {
			continue
		}
		if s.Match != nil {
			return true
		}
		if !s.methodAllowed(method) {
			continue
		}
		for _, p := range s.Paths {
			if patternUnder(pattern, p) {
				return true
			}
		}
	}
	return false
}

// patternUnder is pathUnder for a pattern whose "{name}" segments match any segment of prefix.
func patternUnder(pattern, prefix string) bool {
	if prefix == "" {
		return false
	}
	ps := strings.Split(strings.Trim(pattern, "/"), "/")
	full := strings.Split(strings.Trim(prefix, "/"), "/")
	if len(ps) < len(full) {
		return false
	}
	for i, seg := range full {
		if ps[i] != seg && !(strings.HasPrefix(ps[i], "{") && strings.HasSuffix(ps[i], "}")) {
			return false
		}
	}
	return true
}

package extkit

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Contracts is what an extension declares about the contracts it takes part in.
type Contracts struct {
	Implements []ContractID
	Defaults   []ContractID // the subset of Implements provided with ProvideDefault
	Wraps      []ContractID // decorates the existing provider; also requires one
	Requires   []ContractID // the extension is turned off when one has no provider
	WorksWith  []ContractID // used when present; never turns anything off, only orders
}

// Declarer is an extension that takes part in contracts.
type Declarer interface {
	Contracts() Contracts
}

// IDs is shorthand for a list of contract ids.
func IDs(ids ...string) []ContractID {
	out := make([]ContractID, len(ids))
	for i, id := range ids {
		out[i] = ContractID(id)
	}
	return out
}

// Plan is the resolver's answer: which extensions run, in what order, and why the others don't.
type Plan struct {
	On    []Extension
	Off   map[string]string // extension name → why
	Lines []string
}

// Resolve decides which extensions run and in what order, from their declarations alone (before any Register runs):
//
//   - an extension whose Requires (or Wraps) has no provider is turned off, repeating until nothing changes, since
//     turning one off can strand another;
//   - a contract chosen "off" under providers: counts as having no provider;
//   - hostProvided contracts always have a provider (the host);
//   - providers register before extensions that require, wrap or work with them; a cycle is an error.
func Resolve(exts []Extension, choices Choices, hostProvided func(ContractID) bool) (*Plan, error) {
	decl := map[string]Contracts{}
	for _, e := range exts {
		if d, ok := e.(Declarer); ok {
			decl[e.Name()] = d.Contracts()
		}
	}
	on := map[string]bool{}
	for _, e := range exts {
		on[e.Name()] = true
	}
	off := map[string]string{}
	provided := func(id ContractID) (bool, []string) {
		if choices[id] == "off" {
			return false, nil
		}
		// app code (pb_hooks) loads after the Go extensions register, so the resolver can't see it yet; naming it
		// under providers: says it will provide the contract, and the seal checks that it did
		if choices[id] == "pb_hooks" {
			return true, []string{"pb_hooks"}
		}
		if l, ok := asList(choices[id]); ok && slices.Contains(l, "pb_hooks") {
			return true, []string{"pb_hooks"}
		}
		if hostProvided != nil && hostProvided(id) {
			return true, []string{"host"}
		}
		var by []string
		for _, e := range exts {
			if on[e.Name()] && slices.Contains(decl[e.Name()].Implements, id) {
				by = append(by, e.Name())
			}
		}
		return len(by) > 0, by
	}
	for changed := true; changed; {
		changed = false
		for _, e := range exts {
			n := e.Name()
			if !on[n] {
				continue
			}
			need := append(slices.Clone(decl[n].Requires), decl[n].Wraps...)
			for _, id := range need {
				// an extension that provides its own default satisfies its own requirement
				if ok, _ := provided(id); !ok {
					on[n] = false
					why := fmt.Sprintf("needs %s, which nothing provides", id)
					if choices[id] == "off" {
						why = fmt.Sprintf("needs %s, which providers: turns off", id)
					}
					off[n] = why
					changed = true
					break
				}
			}
		}
	}

	// order: an extension comes after every extension that implements something it requires, wraps or works with
	var active []Extension
	for _, e := range exts {
		if on[e.Name()] {
			active = append(active, e)
		}
	}
	ordered, err := topo(active, decl)
	if err != nil {
		return nil, err
	}
	p := &Plan{On: ordered, Off: off}
	for _, e := range exts {
		if why, isOff := off[e.Name()]; isOff {
			p.Lines = append(p.Lines, fmt.Sprintf("%s off: %s", e.Name(), why))
		}
	}
	return p, nil
}

func topo(exts []Extension, decl map[string]Contracts) ([]Extension, error) {
	index := map[string]int{}
	for i, e := range exts {
		index[e.Name()] = i
	}
	// edges: provider → user
	after := make([][]int, len(exts))
	indeg := make([]int, len(exts))
	for i, e := range exts {
		d := decl[e.Name()]
		uses := append(append(slices.Clone(d.Requires), d.Wraps...), d.WorksWith...)
		for j, p := range exts {
			if i == j {
				continue
			}
			for _, id := range uses {
				if slices.Contains(decl[p.Name()].Implements, id) {
					after[j] = append(after[j], i)
					indeg[i]++
					break
				}
			}
		}
	}
	var out []Extension
	done := make([]bool, len(exts))
	for len(out) < len(exts) {
		next := -1
		for i := range exts { // lowest index first: ties keep registration order
			if !done[i] && indeg[i] == 0 {
				next = i
				break
			}
		}
		if next < 0 {
			var cyc []string
			for i, e := range exts {
				if !done[i] {
					cyc = append(cyc, e.Name())
				}
			}
			return nil, fmt.Errorf("extkit: contract cycle between %s", strings.Join(cyc, ", "))
		}
		done[next] = true
		out = append(out, exts[next])
		for _, k := range after[next] {
			indeg[k]--
		}
	}
	return out, nil
}

// Setup registers exts on app through contracts, the way the standard Set will: resolve, Register each extension that
// stays on (attributing its Provide calls to it), then seal with the providers: choices. It returns the startup lines.
// Spike stand-in for standard.Set.Register.
func Setup(app core.App, exts []Extension, layers func(name string) Config, choices Choices) ([]string, error) {
	plan, err := Prepare(app, exts, layers, choices)
	if err != nil {
		return nil, err
	}
	return Finish(app, plan, choices)
}

// Prepare resolves and registers the Go extensions. Between Prepare and Finish, jsvm loads pb_hooks; their onContracts
// handlers run at the start of Finish, and what they provide is attributed to pb_hooks.
func Prepare(app core.App, exts []Extension, layers func(name string) Config, choices Choices) (*Plan, error) {
	s := scopeOf(app)
	var hostProvided func(ContractID) bool
	if s.host != nil {
		hostProvided = s.host.HostProvides
	}
	plan, err := Resolve(exts, choices, hostProvided)
	if err != nil {
		return nil, err
	}
	for _, e := range plan.On {
		s.Registering(e.Name())
		var cfg Config
		if layers != nil {
			cfg = layers(e.Name())
		}
		if err := e.Register(app, cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	s.Registering("pb_hooks") // anything provided after this point (jsvm loading pb_hooks) is the app's own code
	return plan, nil
}

// Finish seals app's contracts and checks every extension provided what it declared. It returns the startup lines.
func Finish(app core.App, plan *Plan, choices Choices) ([]string, error) {
	s := scopeOf(app)
	if err := runJSDeclarations(app); err != nil { // pb_hooks' onContracts: provide before the seal
		return nil, err
	}
	if err := s.Seal(choices); err != nil {
		return nil, err
	}
	if err := checkJSDeclarations(app, s); err != nil {
		return nil, err
	}
	mountWires(app)
	if err := bindMigrations(app, plan.On); err != nil { // extension migrations, at bootstrap after the system ones
		return nil, err
	}
	names := make([]string, 0, len(plan.On))
	for _, e := range plan.On {
		names = append(names, e.Name())
	}
	app.Store().Set(extensionsKey, names)
	app.Store().Set(runningKey, append([]Extension(nil), plan.On...))
	app.Store().Set(startedKey, time.Now())
	// check Implements against what was provided
	var lines []string
	for _, e := range plan.On {
		d, ok := e.(Declarer)
		if !ok {
			continue
		}
		for _, id := range d.Contracts().Implements {
			if !s.providedBy(id, e.Name()) && !(s.host != nil && s.host.HostProvides(id)) {
				return nil, fmt.Errorf("extkit: %s declares it implements %s but didn't provide it", e.Name(), id)
			}
		}
	}
	lines = append(lines, plan.Lines...)
	for _, l := range s.Lines {
		lines = append(lines, l)
	}
	for i := range lines {
		lines[i] = s.name + ": " + lines[i]
	}
	return lines, nil
}

func (s *Scope) providedBy(id ContractID, ext string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sl := s.slots[id]
	if sl == nil {
		return false
	}
	for _, p := range sl.provisions {
		if p.ext == ext {
			return true
		}
	}
	return false
}

const (
	extensionsKey = "extkit.extensions"
	runningKey    = "extkit.running"
	startedKey    = "extkit.started"
)

// Extensions lists the extensions running on app, in registration order (after Finish).
func Extensions(app core.App) []string {
	names, _ := app.Store().Get(extensionsKey).([]string)
	return names
}

// Running is the extensions running on app, in registration order (after Finish): Extensions' values, for code that
// asks them something (pocketone reference collects their TypeScript declarations).
func Running(app core.App) []Extension {
	exts, _ := app.Store().Get(runningKey).([]Extension)
	return exts
}

// Started is when app's extensions were set up: when this app (instance) last started.
func Started(app core.App) time.Time {
	t, _ := app.Store().Get(startedKey).(time.Time)
	return t
}

// Package rules is the lifecycle's policy engine: named criteria an app can register or replace, combinators, rules
// (a criterion that leads to an action, with vetoes and a budget) and a report that says why each subject matched.
//
// It knows nothing about instances, plans or payments. A subject is whatever the caller evaluates: an instance's facts
// on an edge, an account with its instances on a mothership. Criteria come from code ([Registry.Register]) or from
// definitions a config source supplies ([Registry.Load]: a YAML section, a mothership), built from the kinds an app
// registered ([Registry.RegisterKind]).
package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Result is a criterion's answer for one subject, and why.
type Result struct {
	Match bool
	Why   string
}

// Criterion answers one question about a subject.
type Criterion[S any] interface {
	Check(ctx context.Context, s S) (Result, error)
}

// Func is a [Criterion] written as a function.
type Func[S any] func(ctx context.Context, s S) (Result, error)

// Check implements [Criterion].
func (f Func[S]) Check(ctx context.Context, s S) (Result, error) { return f(ctx, s) }

// Pred is a criterion from a predicate that doesn't fail: why says what it found either way (e.g. "6 running, plan 5").
func Pred[S any](f func(s S) (match bool, why string)) Criterion[S] {
	return Func[S](func(_ context.Context, s S) (Result, error) {
		m, why := f(s)
		return Result{Match: m, Why: why}, nil
	})
}

// All matches when every criterion matches (none: matches). It stops at the first that doesn't.
func All[S any](cs ...Criterion[S]) Criterion[S] {
	return Func[S](func(ctx context.Context, s S) (Result, error) {
		whys := make([]string, 0, len(cs))
		for _, c := range cs {
			r, err := c.Check(ctx, s)
			if err != nil {
				return Result{}, err
			}
			if !r.Match {
				return Result{Why: r.Why}, nil
			}
			whys = append(whys, r.Why)
		}
		return Result{Match: true, Why: join(whys)}, nil
	})
}

// Any matches when one criterion matches (none: doesn't). It stops at the first that does.
func Any[S any](cs ...Criterion[S]) Criterion[S] {
	return Func[S](func(ctx context.Context, s S) (Result, error) {
		whys := make([]string, 0, len(cs))
		for _, c := range cs {
			r, err := c.Check(ctx, s)
			if err != nil {
				return Result{}, err
			}
			if r.Match {
				return r, nil
			}
			whys = append(whys, r.Why)
		}
		return Result{Why: join(whys)}, nil
	})
}

// Not inverts a criterion; the reason stays the same.
func Not[S any](c Criterion[S]) Criterion[S] {
	return Func[S](func(ctx context.Context, s S) (Result, error) {
		r, err := c.Check(ctx, s)
		return Result{Match: !r.Match, Why: r.Why}, err
	})
}

func join(whys []string) string {
	out := whys[:0:0]
	for _, w := range whys {
		if w != "" {
			out = append(out, w)
		}
	}
	return strings.Join(out, "; ")
}

// Factory builds a criterion of one kind from a definition's params. Params of combinators name other criteria.
type Factory[S any] func(params map[string]any) (Criterion[S], error)

// Def defines a named criterion in config: a registered kind and its params, or a combinator.
//
//	stale:      { kind: idle, params: { after: 8760h } }
//	archivable: { kind: all, of: [stale, free_plan] }
//	keep:       { kind: not, of: [archivable] }
type Def struct {
	Kind   string         `json:"kind" yaml:"kind"`
	Params map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
	Of     []string       `json:"of,omitempty" yaml:"of,omitempty"` // for all, any, not
}

// Registry holds named criteria and the kinds config can build them from. Rules refer to criteria by name through
// [Registry.Ref], so a criterion replaced later (by an app, or by a config reload) takes effect in every rule.
type Registry[S any] struct {
	mu    sync.RWMutex
	named map[string]Criterion[S]
	kinds map[string]Factory[S]
}

// NewRegistry returns an empty registry.
func NewRegistry[S any]() *Registry[S] {
	return &Registry[S]{named: map[string]Criterion[S]{}, kinds: map[string]Factory[S]{}}
}

// Register adds or replaces the criterion called name.
func (r *Registry[S]) Register(name string, c Criterion[S]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.named[name] = c
}

// RegisterKind adds or replaces a kind that definitions can build criteria from.
func (r *Registry[S]) RegisterKind(kind string, f Factory[S]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[kind] = f
}

// Get returns the criterion called name.
func (r *Registry[S]) Get(name string) (Criterion[S], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.named[name]
	return c, ok
}

// Names lists the registered criteria, sorted.
func (r *Registry[S]) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.named))
	for n := range r.named {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Ref is the criterion called name, looked up each time it is checked. Checking a name nobody registered is an error.
func (r *Registry[S]) Ref(name string) Criterion[S] {
	return Func[S](func(ctx context.Context, s S) (Result, error) {
		c, ok := r.Get(name)
		if !ok {
			return Result{}, fmt.Errorf("no criterion %q", name)
		}
		return c.Check(ctx, s)
	})
}

// Load builds each definition and registers it under its name, replacing what was there. Combinators (all, any, not)
// refer to other criteria by name, defined in the same call or registered before. Nothing is registered when a
// definition is wrong.
func (r *Registry[S]) Load(defs map[string]Def) error {
	built := make(map[string]Criterion[S], len(defs))
	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		d := defs[name]
		refs := func(min int) ([]Criterion[S], error) {
			if len(d.Of) < min {
				return nil, fmt.Errorf("criterion %q: %s needs %d or more names in of", name, d.Kind, min)
			}
			out := make([]Criterion[S], len(d.Of))
			for i, n := range d.Of {
				if _, def := defs[n]; !def {
					if _, ok := r.Get(n); !ok {
						return nil, fmt.Errorf("criterion %q: no criterion %q", name, n)
					}
				}
				out[i] = r.Ref(n)
			}
			return out, nil
		}
		var c Criterion[S]
		var err error
		switch d.Kind {
		case "all", "any":
			var cs []Criterion[S]
			if cs, err = refs(1); err == nil {
				if d.Kind == "all" {
					c = All(cs...)
				} else {
					c = Any(cs...)
				}
			}
		case "not":
			var cs []Criterion[S]
			if cs, err = refs(1); err == nil {
				if len(cs) != 1 {
					err = fmt.Errorf("criterion %q: not takes one name", name)
				} else {
					c = Not(cs[0])
				}
			}
		default:
			r.mu.RLock()
			f, ok := r.kinds[d.Kind]
			r.mu.RUnlock()
			if !ok {
				return fmt.Errorf("criterion %q: unknown kind %q", name, d.Kind)
			}
			c, err = f(d.Params)
			if err == nil && c == nil {
				err = fmt.Errorf("criterion %q: kind %q built nothing", name, d.Kind)
			}
		}
		if err != nil {
			return err
		}
		built[name] = c
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for n, c := range built {
		r.named[n] = c
	}
	return nil
}

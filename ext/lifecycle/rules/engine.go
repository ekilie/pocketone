package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Rule leads from a criterion to an action: subjects that match When, and no veto in Unless, are decided Action. A
// budget caps how many subjects one run decides (0: no cap); the rest are reported as over budget.
type Rule[S any] struct {
	Name   string
	Action string
	When   Criterion[S]
	Unless []Veto[S]
	Budget int
}

// Veto blocks a rule for the subjects it matches, whatever When says. Named so reports say which veto held.
type Veto[S any] struct {
	Name string
	If   Criterion[S]
}

// Engine evaluates rules over subjects. Every rule sees every subject: one subject may get several decisions (the
// caller decides whether its rules are exclusive, e.g. by building them from exclusive criteria).
type Engine[S any] struct {
	Rules []Rule[S]
	// Key names a subject in reports (an instance id, a user id).
	Key func(S) string
}

// Decision is one subject a rule decided, and why.
type Decision[S any] struct {
	Rule    string
	Action  string
	Key     string
	Subject S
	Why     string
}

// Vetoed is a subject a rule matched and a veto blocked.
type Vetoed struct {
	Rule, Key, Veto, Why string
}

// Failed is a subject whose criteria returned an error for a rule.
type Failed struct {
	Rule, Key string
	Err       error
}

// Report is what one evaluation decided. Nothing has been done: acting on Decisions (or only logging them, in a dry
// run) is the caller's.
type Report[S any] struct {
	Decisions  []Decision[S]
	OverBudget []Decision[S]
	Vetoed     []Vetoed
	Failed     []Failed
}

// Evaluate checks every rule against every subject, in order.
func (e Engine[S]) Evaluate(ctx context.Context, subjects []S) Report[S] {
	var rep Report[S]
	key := e.Key
	if key == nil {
		key = func(s S) string { return fmt.Sprint(s) }
	}
	for _, rule := range e.Rules {
		n := 0
	subjects:
		for _, s := range subjects {
			if ctx.Err() != nil {
				rep.Failed = append(rep.Failed, Failed{Rule: rule.Name, Key: key(s), Err: ctx.Err()})
				return rep
			}
			r, err := rule.When.Check(ctx, s)
			if err != nil {
				rep.Failed = append(rep.Failed, Failed{Rule: rule.Name, Key: key(s), Err: err})
				continue
			}
			if !r.Match {
				continue
			}
			for _, v := range rule.Unless {
				vr, err := v.If.Check(ctx, s)
				if err != nil {
					rep.Failed = append(rep.Failed, Failed{Rule: rule.Name, Key: key(s), Err: fmt.Errorf("veto %s: %w", v.Name, err)})
					continue subjects
				}
				if vr.Match {
					rep.Vetoed = append(rep.Vetoed, Vetoed{Rule: rule.Name, Key: key(s), Veto: v.Name, Why: vr.Why})
					continue subjects
				}
			}
			d := Decision[S]{Rule: rule.Name, Action: rule.Action, Key: key(s), Subject: s, Why: r.Why}
			if rule.Budget > 0 && n >= rule.Budget {
				rep.OverBudget = append(rep.OverBudget, d)
				continue
			}
			n++
			rep.Decisions = append(rep.Decisions, d)
		}
	}
	return rep
}

// Counts is how many subjects a rule decided, held back by its budget, vetoed and failed on.
type Counts struct {
	Decided, OverBudget, Vetoed, Failed int
}

// Counts per rule.
func (r Report[S]) Counts() map[string]Counts {
	out := map[string]Counts{}
	for _, d := range r.Decisions {
		c := out[d.Rule]
		c.Decided++
		out[d.Rule] = c
	}
	for _, d := range r.OverBudget {
		c := out[d.Rule]
		c.OverBudget++
		out[d.Rule] = c
	}
	for _, v := range r.Vetoed {
		c := out[v.Rule]
		c.Vetoed++
		out[v.Rule] = c
	}
	for _, f := range r.Failed {
		c := out[f.Rule]
		c.Failed++
		out[f.Rule] = c
	}
	return out
}

// Summary is one line per rule with anything to report, in rule order: "warn: 3 decided, 1 vetoed".
func (r Report[S]) Summary(rules []Rule[S]) string {
	counts := r.Counts()
	var lines []string
	seen := map[string]bool{}
	line := func(name string) {
		c, ok := counts[name]
		if !ok || seen[name] {
			return
		}
		seen[name] = true
		parts := []string{fmt.Sprintf("%d decided", c.Decided)}
		if c.OverBudget > 0 {
			parts = append(parts, fmt.Sprintf("%d over budget", c.OverBudget))
		}
		if c.Vetoed > 0 {
			parts = append(parts, fmt.Sprintf("%d vetoed", c.Vetoed))
		}
		if c.Failed > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", c.Failed))
		}
		lines = append(lines, name+": "+strings.Join(parts, ", "))
	}
	for _, rule := range rules {
		line(rule.Name)
	}
	rest := make([]string, 0, len(counts))
	for n := range counts {
		rest = append(rest, n)
	}
	sort.Strings(rest)
	for _, n := range rest {
		line(n)
	}
	if len(lines) == 0 {
		return "nothing to do"
	}
	return strings.Join(lines, "\n")
}

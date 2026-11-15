package rules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type inst struct {
	ID       string
	IdleDays int
	Free     bool
	Pinned   bool
}

func idleKind(params map[string]any) (Criterion[inst], error) {
	days, ok := params["days"].(int)
	if !ok {
		return nil, fmt.Errorf("idle: days must be an int")
	}
	return Pred(func(i inst) (bool, string) {
		return i.IdleDays >= days, fmt.Sprintf("idle %dd (limit %dd)", i.IdleDays, days)
	}), nil
}

func registry() *Registry[inst] {
	r := NewRegistry[inst]()
	r.RegisterKind("idle", idleKind)
	r.Register("free", Pred(func(i inst) (bool, string) {
		return i.Free, map[bool]string{true: "free plan", false: "paid plan"}[i.Free]
	}))
	r.Register("pinned", Pred(func(i inst) (bool, string) { return i.Pinned, "pinned by the owner" }))
	return r
}

var subjects = []inst{
	{ID: "a", IdleDays: 400, Free: true},
	{ID: "b", IdleDays: 400, Free: false},
	{ID: "c", IdleDays: 10, Free: true},
	{ID: "d", IdleDays: 500, Free: true, Pinned: true},
	{ID: "e", IdleDays: 900, Free: true},
}

func TestRulesFromConfig(t *testing.T) {
	r := registry()
	if err := r.Load(map[string]Def{
		"stale":      {Kind: "idle", Params: map[string]any{"days": 365}},
		"archivable": {Kind: "all", Of: []string{"stale", "free"}},
	}); err != nil {
		t.Fatal(err)
	}
	e := Engine[inst]{Key: func(i inst) string { return i.ID }, Rules: []Rule[inst]{{
		Name: "archive-stale", Action: "archive", When: r.Ref("archivable"),
		Unless: []Veto[inst]{{Name: "pinned", If: r.Ref("pinned")}}, Budget: 1,
	}}}
	rep := e.Evaluate(context.Background(), subjects)
	if len(rep.Decisions) != 1 || rep.Decisions[0].Key != "a" || rep.Decisions[0].Why != "idle 400d (limit 365d); free plan" {
		t.Fatalf("decisions: %+v", rep.Decisions)
	}
	if len(rep.OverBudget) != 1 || rep.OverBudget[0].Key != "e" {
		t.Fatalf("over budget: %+v", rep.OverBudget)
	}
	if len(rep.Vetoed) != 1 || rep.Vetoed[0].Key != "d" || rep.Vetoed[0].Veto != "pinned" {
		t.Fatalf("vetoed: %+v", rep.Vetoed)
	}
	if got := rep.Summary(e.Rules); got != "archive-stale: 1 decided, 1 over budget, 1 vetoed" {
		t.Fatalf("summary %q", got)
	}

	// An app redefines "stale" after the rules were built: every rule that refers to it follows.
	r.Register("stale", Pred(func(i inst) (bool, string) { return i.IdleDays >= 30, "idle 30d+" }))
	e.Rules[0].Budget = 0
	rep = e.Evaluate(context.Background(), subjects)
	if len(rep.Decisions) != 2 { // a, e (c is idle 10d; b paid; d pinned)
		t.Fatalf("after replacing stale: %+v", rep.Decisions)
	}
	// ... and so does a config reload.
	if err := r.Load(map[string]Def{"stale": {Kind: "idle", Params: map[string]any{"days": 5}}}); err != nil {
		t.Fatal(err)
	}
	if rep = e.Evaluate(context.Background(), subjects); len(rep.Decisions) != 3 {
		t.Fatalf("after reload: %+v", rep.Decisions)
	}
}

func TestLoadErrorsRegisterNothing(t *testing.T) {
	for name, defs := range map[string]map[string]Def{
		"unknown kind": {"x": {Kind: "nope"}},
		"unknown ref":  {"x": {Kind: "all", Of: []string{"missing"}}},
		"bad params":   {"x": {Kind: "idle", Params: map[string]any{"days": "a year"}}},
		"not takes 1":  {"x": {Kind: "not", Of: []string{"free", "pinned"}}},
		"empty all":    {"x": {Kind: "any"}},
	} {
		r := registry()
		defs["ok"] = Def{Kind: "not", Of: []string{"free"}}
		if err := r.Load(defs); err == nil {
			t.Errorf("%s: no error", name)
		}
		if _, ok := r.Get("ok"); ok {
			t.Errorf("%s: registered the good definition anyway", name)
		}
	}
}

func TestCombinatorsAndFailures(t *testing.T) {
	r := registry()
	boom := errors.New("boom")
	r.Register("broken", Func[inst](func(context.Context, inst) (Result, error) { return Result{}, boom }))
	ctx := context.Background()
	check := func(c Criterion[inst], i inst) Result {
		t.Helper()
		res, err := c.Check(ctx, i)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := check(Any(r.Ref("pinned"), r.Ref("free")), subjects[0]); !res.Match || res.Why != "free plan" {
		t.Fatalf("any: %+v", res)
	}
	if res := check(Not(r.Ref("free")), subjects[1]); !res.Match || res.Why != "paid plan" {
		t.Fatalf("not: %+v", res)
	}
	if res := check(All[inst](), subjects[0]); !res.Match {
		t.Fatal("empty all should match")
	}
	if _, err := r.Ref("missing").Check(ctx, subjects[0]); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing ref: %v", err)
	}

	e := Engine[inst]{Key: func(i inst) string { return i.ID }, Rules: []Rule[inst]{
		{Name: "r1", Action: "x", When: r.Ref("broken")},
		{Name: "r2", Action: "y", When: r.Ref("free"), Unless: []Veto[inst]{{Name: "broken", If: r.Ref("broken")}}},
	}}
	rep := e.Evaluate(ctx, subjects[:2])
	// r1 fails on both; r2 matches only a (free) and its veto fails there.
	if len(rep.Failed) != 3 || len(rep.Decisions) != 0 || !errors.Is(rep.Failed[2].Err, boom) {
		t.Fatalf("failed: %+v decisions: %+v", rep.Failed, rep.Decisions)
	}
	if got := rep.Summary(e.Rules); got != "r1: 0 decided, 2 failed\nr2: 0 decided, 1 failed" {
		t.Fatalf("summary %q", got)
	}
	if got := (Report[inst]{}).Summary(nil); got != "nothing to do" {
		t.Fatalf("empty summary %q", got)
	}
}

package apitokensv1

import (
	"slices"
	"strings"
	"testing"
)

func TestScopeValues(t *testing.T) {
	if n, v := SplitScope("instances=abc"); n != "instances" || v != "abc" {
		t.Fatal(n, v)
	}
	if n, v := SplitScope("deploy"); n != "deploy" || v != "" {
		t.Fatal(n, v)
	}
	if JoinScope("a", "") != "a" || JoinScope("a", "b") != "a=b" {
		t.Fatal("join")
	}

	limited := []string{"deploy", "instances=a", "instances=b"}
	for _, c := range []struct {
		held  []string
		scope string
		holds bool
		cover bool
	}{
		{nil, "instances", true, true},
		{nil, "instances=a", true, true},
		{limited, "instances", true, false}, // may use it, on some values; can't hand out every value
		{limited, "instances=a", true, true},
		{limited, "instances=c", false, false},
		{limited, "deploy", true, true},
		{limited, "deploy=x", true, true}, // a bare scope holds every value
		{limited, "other", false, false},
		{[]string{"instances"}, "instances=z", true, true},
	} {
		if got := HoldsScope(c.held, c.scope); got != c.holds {
			t.Errorf("HoldsScope(%v, %q) = %v", c.held, c.scope, got)
		}
		if got := CoversScope(c.held, c.scope); got != c.cover {
			t.Errorf("CoversScope(%v, %q) = %v", c.held, c.scope, got)
		}
	}

	g := Grant{Scopes: limited}
	if vals, all := g.ScopeValues("instances"); all || strings.Join(vals, ",") != "a,b" {
		t.Fatal(vals, all)
	}
	if _, all := g.ScopeValues("deploy"); !all {
		t.Fatal("bare scope: every value")
	}
	if vals, all := g.ScopeValues("other"); all || len(vals) != 0 {
		t.Fatal(vals, all)
	}
	if !g.HasScopeFor("instances", "b") || g.HasScopeFor("instances", "c") {
		t.Fatal("HasScopeFor")
	}
	if _, all := (&Grant{}).ScopeValues("x"); !all {
		t.Fatal("unscoped: every value")
	}

	for _, c := range []struct{ a, b, want []string }{
		{[]string{"instances"}, []string{"instances=a"}, []string{"instances=a"}},
		{[]string{"instances=a"}, []string{"instances"}, []string{"instances=a"}},
		{[]string{"instances=a", "instances=b"}, []string{"instances=b", "instances=c"}, []string{"instances=b"}},
		{[]string{"deploy", "notes"}, []string{"notes", "mcp"}, []string{"notes"}},
		{[]string{"instances=a"}, []string{"instances=c"}, []string{}},
		{[]string{}, []string{"notes"}, []string{}},
	} {
		if got := IntersectScopes(c.a, c.b); !slices.Equal(got, c.want) {
			t.Errorf("IntersectScopes(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

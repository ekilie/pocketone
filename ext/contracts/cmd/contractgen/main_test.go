package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every contract's generated file is current: change an interface, run go generate.
func TestGeneratedFilesAreCurrent(t *testing.T) {
	dirs, err := filepath.Glob("../../*/v*")
	if err != nil {
		t.Fatal(err)
	}
	more, _ := filepath.Glob("../../*/*/v*")
	dirs = append(dirs, more...)
	if len(dirs) == 0 {
		t.Fatal("no contract packages found")
	}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, "zz_contract.go")); err != nil {
			continue // a routed contract (deploy/v1): no generated code
		}
		c, err := parse(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		want, err := render(c)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		got, _ := os.ReadFile(filepath.Join(dir, "zz_contract.go"))
		if string(got) != string(want) {
			t.Errorf("%s/zz_contract.go is stale: run go generate ./... in ext/contracts", dir)
		}
		wantTS, err := renderTS(dir, c)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		gotTS, _ := os.ReadFile(filepath.Join(dir, "zz_contract.d.ts"))
		if string(gotTS) != wantTS {
			t.Errorf("%s/zz_contract.d.ts is stale: run go generate ./... in ext/contracts", dir)
		}
	}
}

func TestBadContracts(t *testing.T) {
	cases := map[string]string{
		"no id": `type X interface{ A(ctx context.Context) error }`,
		"<name>/v<N>": `//contract:id deploy
type X interface{ A(ctx context.Context) error }`,
		"wire contracts name the token scope": `//contract:id x/v1
//contract:kind wire
type X interface{
	// A does a.
	A(ctx context.Context, a extkit.Auth) error
}`,
		"parameter 2 must be extkit.Auth": `//contract:id x/v1
//contract:kind wire
//contract:scope x
type X interface{
	// A does a.
	A(ctx context.Context, in string) error
}`,
		"the comment is the MCP tool's description": `//contract:id x/v1
//contract:kind wire
//contract:scope x
type X interface{ A(ctx context.Context, a extkit.Auth) error }`,
		"wire contracts are one": `//contract:id x/v1
//contract:kind wire
//contract:scope x
//contract:cardinality many
type X interface{
	// A does a.
	A(ctx context.Context, a extkit.Auth) error
}`,
		"only wire contracts can be host-provided": `//contract:id x/v1
//contract:host
type X interface{ A(ctx context.Context) error }`,
		"returns error or (value, error)": `//contract:id x/v1
type X interface{ A(ctx context.Context) (int, string) }`,
		"keyed contracts name their key method": `//contract:id x/v1
//contract:cardinality keyed
type X interface{ A(ctx context.Context) error }`,
	}
	for want, src := range cases {
		dir := t.TempDir()
		file := "package x\n\nimport \"context\"\n\n" + src + "\n"
		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := parse(dir)
		if want == "no id" {
			want = "no interface with a //contract:id"
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v", want, err)
		}
	}
}

func TestKeyedContractGenerates(t *testing.T) {
	dir := t.TempDir()
	src := `package toolsv1

import "context"

// Tool is one MCP tool.
//
//contract:id mcp.tools/v1
//contract:cardinality keyed
//contract:key Name
//contract:js open
type Tool interface {
	Name() string
	Call(ctx context.Context, args map[string]any) (any, error)
}
`
	if err := os.WriteFile(filepath.Join(dir, "tools.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := render(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`extkit.NewKeyedContract[Tool]("mcp.tools/v1", func(v Tool) string { return v.Name() })`,
		`func (j jsTool) Call(ctx context.Context, in map[string]any) (any, error)`,
		`func (j jsTool) Name() string {`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

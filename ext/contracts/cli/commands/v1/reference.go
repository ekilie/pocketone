package clicmdv1

import (
	"encoding/json"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"pocketone.dev/ext/extkit"
)

// annSource is the cobra annotation [Mount] puts on the commands it attaches: who answers the command.
const annSource = "pocketone.source"

// Reference is the binary's command tree and the HTTP APIs and MCP tools of every contract linked into it, as
// `pocketone reference --json` prints it. pocketone.dev's CLI, REST and MCP references and each contract's and
// extension's "Commands and APIs" section are generated from it (site/sync).
type Reference struct {
	Global    []FlagRef     `json:"global"` // the root's persistent flags, which every command takes
	Commands  []CommandRef  `json:"commands"`
	Contracts []ContractRef `json:"contracts"`
	JS        []JSRef       `json:"js,omitempty"`
}

// JSRef is what one extension adds to an app's JS API (pb_hooks and page code): its TypeScript declarations, as they
// go into the app's types.d.ts. Extension "contracts" is extkit's: the contracts' action hooks and types open to JS.
type JSRef struct {
	Extension string `json:"extension"`
	Types     string `json:"types"`
}

// FlagRef is one flag.
type FlagRef struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage,omitempty"`
}

// CommandRef is one command, every subcommand listed by its full path.
type CommandRef struct {
	Path     string    `json:"path"` // the words after the binary's name: "remote restore to"
	Use      string    `json:"use"`  // the arguments as help shows them
	Short    string    `json:"short,omitempty"`
	Long     string    `json:"long,omitempty"`
	Aliases  []string  `json:"aliases,omitempty"`
	Flags    []FlagRef `json:"flags,omitempty"`
	Runnable bool      `json:"runnable"` // false: a group of subcommands
	// Source says who answers it: "pocketbase" (PocketBase's own commands), "ext/<name>" (an extension),
	// "contract:<id>" (generated from a contract's operation), "contracts/<id>" (a contract package's shared client),
	// "main" (the binary itself); "" when it can't be told (a group).
	Source string `json:"source,omitempty"`
}

// ContractRef is a contract with an HTTP API.
type ContractRef struct {
	ID       string  `json:"id"`
	Prefix   string  `json:"prefix"` // /api/_c/<id>
	Routed   bool    `json:"routed"` // its own API shape: no generated operations
	Provider string  `json:"provider,omitempty"`
	Provided bool    `json:"provided"`
	Ops      []OpRef `json:"ops,omitempty"`
}

// OpRef is one operation and the ways to call it.
type OpRef struct {
	Name     string     `json:"name"`
	Desc     string     `json:"desc,omitempty"`
	Method   string     `json:"method"`
	REST     string     `json:"rest"` // METHOD's path: /api/_c/<id>/<path>
	Scope    string     `json:"scope"`
	Out      string     `json:"out"` // json, stream or raw
	RawIn    bool       `json:"rawIn,omitempty"`
	Command  string     `json:"command,omitempty"` // the pocketone command; "" for raw public operations
	Tool     string     `json:"tool,omitempty"`    // the MCP tool; "" for uploads and downloads
	ReadOnly bool       `json:"readOnly"`
	Input    []FieldRef `json:"input,omitempty"`
}

// FieldRef is one field of an operation's input.
type FieldRef struct {
	Name string `json:"name"` // in the JSON body / MCP arguments
	Flag string `json:"flag"` // the command's flag, without "--"
	Type string `json:"type"` // string, bool, int, float, time, strings, json
}

// BuildReference describes root's command tree and the contract APIs linked into the binary, with their providers on
// app.
func BuildReference(app core.App, root *cobra.Command) Reference {
	ref := Reference{Global: flagRefs(root.PersistentFlags())}
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			p := append(append([]string(nil), path...), sub.Name())
			use := strings.TrimSpace(strings.TrimPrefix(sub.Use, sub.Name()))
			ref.Commands = append(ref.Commands, CommandRef{
				Path: strings.Join(p, " "), Use: use, Short: sub.Short, Long: sub.Long, Aliases: sub.Aliases,
				Flags: flagRefs(sub.LocalFlags()), Runnable: sub.Runnable(), Source: commandSource(sub),
			})
			walk(sub, p)
		}
	}
	walk(root, nil)
	sort.Slice(ref.Commands, func(i, j int) bool { return ref.Commands[i].Path < ref.Commands[j].Path })

	for _, api := range extkit.APIs(app) {
		cr := ContractRef{ID: string(api.ID), Prefix: api.Prefix, Routed: api.Routed, Provider: api.Provider, Provided: api.Provided}
		for _, op := range api.Ops {
			or := OpRef{Name: op.Name, Desc: op.Desc, Method: op.Method, REST: api.Prefix, Scope: op.Scope,
				Out: outName(op.Out), RawIn: op.RawIn, ReadOnly: op.Method == "GET"}
			if op.Path != "" {
				or.REST += "/" + op.Path
			}
			if !(op.Scope == extkit.ScopePublic && (op.RawIn || op.Out == extkit.OutRaw)) {
				or.Command = PathOf(api.ID, op.Name)
			}
			if op.HasTool() {
				or.Tool = extkit.ToolName(api.ID, op.Name)
			}
			for _, f := range inputFields(op.In) {
				or.Input = append(or.Input, FieldRef{Name: f.json, Flag: f.flag, Type: f.kind.String()})
			}
			cr.Ops = append(cr.Ops, or)
		}
		ref.Contracts = append(ref.Contracts, cr)
	}
	type typer interface{ TypeScript() string }
	for _, e := range extkit.Running(app) {
		if t, ok := e.(typer); ok {
			if d := strings.TrimSpace(t.TypeScript()); d != "" {
				ref.JS = append(ref.JS, JSRef{Extension: e.Name(), Types: d})
			}
		}
	}
	if t, ok := extkit.JSContracts(app).(typer); ok {
		if d := strings.TrimSpace(t.TypeScript()); d != "" {
			ref.JS = append(ref.JS, JSRef{Extension: "contracts", Types: d})
		}
	}
	return ref
}

func referenceCommand(app core.App, root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "reference",
		Short:  "Print the command tree and every contract's HTTP API and MCP tools as JSON (for generated docs)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(BuildReference(app, root))
		},
	}
}

func flagRefs(fs *pflag.FlagSet) []FlagRef {
	var out []FlagRef
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		def := f.DefValue
		if def == "[]" || (f.Value.Type() == "bool" && def == "false") {
			def = ""
		}
		out = append(out, FlagRef{Name: f.Name, Shorthand: f.Shorthand, Type: f.Value.Type(), Default: def, Usage: f.Usage})
	})
	return out
}

// commandSource says who answers c: the annotation Mount left, else the package of its Run function.
func commandSource(c *cobra.Command) string {
	if s := c.Annotations[annSource]; s != "" {
		return s
	}
	switch {
	case c.RunE != nil:
		return sourceOf(c.RunE)
	case c.Run != nil:
		return sourceOf(c.Run)
	}
	return ""
}

// sourceOf names the code a function belongs to, from its package path.
func sourceOf(fn any) string {
	v := reflect.ValueOf(fn)
	if !v.IsValid() || v.Kind() != reflect.Func || v.IsNil() {
		return ""
	}
	f := runtime.FuncForPC(v.Pointer())
	if f == nil {
		return ""
	}
	return sourceOfPackage(packageOf(f.Name()))
}

// packageOf is the package path of a function's full name ("github.com/x/y.(*T).Run.func1" → "github.com/x/y").
func packageOf(name string) string {
	slash := strings.LastIndex(name, "/")
	if dot := strings.Index(name[slash+1:], "."); dot >= 0 {
		return name[:slash+1+dot]
	}
	return name
}

func sourceOfPackage(pkg string) string {
	const ext = "pocketone.dev/ext/"
	switch {
	case strings.HasPrefix(pkg, "ganigeorgiev.com/pocketbase/pocketbase"):
		return "pocketbase"
	case strings.HasPrefix(pkg, ext+"contracts/"):
		// ext/contracts/remote/v1/remoteclient → contracts/remote/v1
		parts := strings.Split(strings.TrimPrefix(pkg, ext+"contracts/"), "/")
		for i, p := range parts {
			if len(p) > 1 && p[0] == 'v' && strings.Trim(p[1:], "0123456789") == "" {
				return "contracts/" + strings.Join(parts[:i], ".") + "/" + p
			}
		}
		return "contracts"
	case strings.HasPrefix(pkg, ext):
		name, _, _ := strings.Cut(strings.TrimPrefix(pkg, ext), "/")
		return "ext/" + name
	case pkg == "main":
		return "main"
	}
	return pkg
}

func outName(k extkit.OutKind) string {
	switch k {
	case extkit.OutStream:
		return "stream"
	case extkit.OutRaw:
		return "raw"
	}
	return "json"
}

func (k fieldKind) String() string {
	return [...]string{"string", "bool", "int", "float", "time", "strings", "json"}[k]
}

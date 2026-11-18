// Package clicmdv1 is the cli.commands/v1 contract: the `pocketone` command tree. Each command path ("remote restore
// to") has at most one provider. Every operation of every wire contract linked into the binary is a command by
// default, under `remote` ([Generated]); an extension replaces one by providing the same path, or adds its own:
//
//	clicmdv1.Contract.Provide(app, clicmdv1.Command{Path: "remote rehash", Short: "...", Run: ...})
//
// [Mount] builds the tree once the app's contracts are sealed. Two extensions providing one path are a startup error
// until pocketone.yaml picks one (`providers: {cli.commands/v1: {"remote rehash": rehasher}}`).
package clicmdv1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"sort"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/contracts/remote/v1/remoteclient"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Command is one `pocketone` command.
type Command struct {
	Path  string // the words after the binary's name: "remote restore to"
	Use   string // its arguments as help shows them ("<name> <url>"); "" for none
	Short string
	Long  string
	Args  cobra.PositionalArgs // nil: any
	Flags func(fs *pflag.FlagSet)
	Run   func(c *Ctx, args []string) error

	contract extkit.ContractID // a generated command: the contract whose operation it calls
}

// Contract is cli.commands/v1, keyed by command path.
var Contract = extkit.NewKeyedContract[Command]("cli.commands/v1", func(c Command) string { return c.Path })

// Ctx is a running command.
type Ctx struct {
	context.Context
	Cmd *cobra.Command
	Out io.Writer
}

// Flags are the command's flags.
func (c *Ctx) Flags() *pflag.FlagSet { return c.Cmd.Flags() }

// Remote opens the remote the command talks to: --remote (a name or URL), else the project's default remote, with
// its saved login (or the CI token). With no login it still returns the client, unauthenticated, so public
// operations work and the rest fail with the remote's own "sign in first".
func (c *Ctx) Remote() (*remoteclient.Client, error) {
	name := ""
	if f := c.Cmd.Flags().Lookup("remote"); f != nil {
		name = f.Value.String()
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rc, err := remoteclient.Open(dir, name)
	if err != nil && (rc == nil || !errors.Is(err, remoteclient.ErrNotLoggedIn)) {
		return nil, err
	}
	return rc, nil
}

// Caller calls contract id on the command's remote.
func (c *Ctx) Caller(id extkit.ContractID) (*extkit.Caller, error) {
	rc, err := c.Remote()
	if err != nil {
		return nil, err
	}
	return extkit.NewCaller(id, rc.Remote.URL, rc.Token, rc.HTTP), nil
}

// --- generated commands ----------------------------------------------------------------------------------------

// PathOf is the command path of a contract operation: `remote`, the contract id's words (without a leading "remote"),
// then the operation, with the last id word dropped from its front: restore/v1's restore_to is "remote restore to",
// remote/v1's status is "remote status", mail.suppression/v1's check is "remote mail suppression check".
func PathOf(id extkit.ContractID, op string) string {
	name, _, _ := strings.Cut(string(id), "/")
	words := strings.Split(name, ".")
	if words[0] == "remote" {
		words = words[1:]
	}
	if len(words) > 0 {
		last := words[len(words)-1]
		if op != last {
			op = strings.TrimPrefix(op, last+"_")
		}
		if op == last {
			op = ""
		}
	}
	parts := append([]string{"remote"}, words...)
	if op != "" {
		parts = append(parts, strings.ReplaceAll(op, "_", "-"))
	}
	return strings.Join(parts, " ")
}

// Generated is a command for every operation people call of every wire contract linked into the binary: flags from
// its input's fields (and --json for the whole input), the answer printed as JSON. Public operations that take or
// return a raw body (an unsubscribe page, a mail provider's webhook) are for browsers and services, not people, and
// get none.
func Generated() []Command {
	var out []Command
	for _, wi := range extkit.Wires() {
		for _, op := range wi.Ops {
			if op.Scope == extkit.ScopePublic && (op.RawIn || op.Out == extkit.OutRaw) {
				continue
			}
			out = append(out, generated(wi.ID, op))
		}
	}
	return out
}

func generated(id extkit.ContractID, op extkit.OpInfo) Command {
	fields := inputFields(op.In)
	return Command{
		contract: id,
		Path:     PathOf(id, op.Name),
		Short:    firstSentence(op.Desc) + " (" + string(id) + ")",
		Long:     op.Desc + "\n\nCalls " + op.Method + " /api/_c/" + string(id) + "/" + op.Path + " on the remote (token scope: " + op.Scope + ").",
		Args:     cobra.NoArgs,
		Flags: func(fs *pflag.FlagSet) {
			for _, f := range fields {
				f.define(fs)
			}
			if op.In != nil {
				fs.String("json", "", "the whole input as JSON (@file reads a file, - reads stdin); flags override its fields")
			}
			if op.RawIn {
				fs.String("file", "-", "the body to send (- for stdin)")
				fs.String("content-type", "application/octet-stream", "the body's content type")
			}
		},
		Run: func(c *Ctx, _ []string) error {
			caller, err := c.Caller(id)
			if err != nil {
				return err
			}
			return explain(id, callOp(c, caller, op, fields))
		},
	}
}

func callOp(c *Ctx, caller *extkit.Caller, op extkit.OpInfo, fields []field) error {
	var in any
	if op.In != nil {
		v, err := buildInput(c.Flags(), op.In, fields)
		if err != nil {
			return err
		}
		in = v
	}
	switch {
	case op.RawIn:
		path, _ := c.Flags().GetString("file")
		ct, _ := c.Flags().GetString("content-type")
		var r io.Reader = os.Stdin
		if path != "-" {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			r = f
		}
		var out json.RawMessage
		if err := caller.CallRawIn(c, op.Method, op.Path, extkit.RawIn{Body: r, ContentType: ct}, &out); err != nil {
			return err
		}
		return printJSON(c.Out, out)
	case op.Out == extkit.OutRaw:
		ro, err := caller.CallRawOut(c, op.Method, op.Path, in)
		if err != nil {
			return err
		}
		defer ro.Body.Close()
		_, err = io.Copy(c.Out, ro.Body)
		return err
	case op.Out == extkit.OutStream:
		ctx, stop := signal.NotifyContext(c, os.Interrupt)
		defer stop()
		s, err := caller.CallStreamMethod(ctx, op.Method, op.Path, in)
		if err != nil {
			return err
		}
		defer s.Close()
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			for _, e := range s.Poll() {
				b, _ := json.Marshal(e)
				fmt.Fprintln(c.Out, string(b))
			}
			select {
			case <-ctx.Done():
				return nil
			case <-s.Done:
				for _, e := range s.Poll() {
					b, _ := json.Marshal(e)
					fmt.Fprintln(c.Out, string(b))
				}
				if s.Err != nil {
					return s.Err()
				}
				return nil
			case <-tick.C:
			}
		}
	}
	var out json.RawMessage
	if err := caller.Call(c, op.Method, op.Path, in, &out); err != nil {
		return err
	}
	return printJSON(c.Out, out)
}

// explain turns a contract error into a message for people.
func explain(id extkit.ContractID, err error) error {
	var ce *extkit.Error
	if !errors.As(err, &ce) {
		return err
	}
	if ce.Code == "http" && ce.Status == 404 {
		return fmt.Errorf("this remote doesn't offer %s (`pocketone remote whoami` lists what it does)", id)
	}
	return fmt.Errorf("%s (%s)", ce.Message, ce.Code)
}

func printJSON(w io.Writer, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		_, err := w.Write(append(raw, '\n'))
		return err
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_, err := fmt.Fprintln(w, string(b))
	return err
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i]
	}
	return strings.TrimSuffix(s, ".")
}

// --- input flags -----------------------------------------------------------------------------------------------

type fieldKind int

const (
	kindString fieldKind = iota
	kindBool
	kindInt
	kindFloat
	kindTime
	kindStrings
	kindJSON // anything else: a JSON value
)

type field struct {
	json string // its name in the input
	flag string
	kind fieldKind
}

func inputFields(t reflect.Type) []field {
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	var out []field
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name := strings.Split(sf.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		f := field{json: name, flag: strings.ReplaceAll(strings.ToLower(name), "_", "-"), kind: kindJSON}
		switch {
		case sf.Type == reflect.TypeOf(time.Time{}):
			f.kind = kindTime
		case sf.Type == reflect.TypeOf(json.RawMessage{}):
			f.kind = kindJSON
		case sf.Type.Kind() == reflect.String:
			f.kind = kindString
		case sf.Type.Kind() == reflect.Bool:
			f.kind = kindBool
		case sf.Type.Kind() >= reflect.Int && sf.Type.Kind() <= reflect.Uint64:
			f.kind = kindInt
		case sf.Type.Kind() == reflect.Float32 || sf.Type.Kind() == reflect.Float64:
			f.kind = kindFloat
		case sf.Type.Kind() == reflect.Slice && sf.Type.Elem().Kind() == reflect.String:
			f.kind = kindStrings
		}
		out = append(out, f)
	}
	return out
}

func (f field) define(fs *pflag.FlagSet) {
	switch f.kind {
	case kindBool:
		fs.Bool(f.flag, false, "")
	case kindInt:
		fs.Int64(f.flag, 0, "")
	case kindFloat:
		fs.Float64(f.flag, 0, "")
	case kindStrings:
		fs.StringSlice(f.flag, nil, "")
	case kindTime:
		fs.String(f.flag, "", `a time: RFC 3339 ("2026-11-10T15:00:00Z"), local ("2026-11-10 15:00"), or relative to now ("-2h")`)
	case kindJSON:
		fs.String(f.flag, "", "a JSON value (@file reads a file)")
	default:
		fs.String(f.flag, "", "")
	}
}

// buildInput makes the operation's input from --json and the flags given.
func buildInput(fs *pflag.FlagSet, t reflect.Type, fields []field) (any, error) {
	m := map[string]any{}
	if raw, _ := fs.GetString("json"); raw != "" {
		b, err := readArg(raw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("--json: %w", err)
		}
	}
	for _, f := range fields {
		if !fs.Changed(f.flag) {
			continue
		}
		switch f.kind {
		case kindBool:
			m[f.json], _ = fs.GetBool(f.flag)
		case kindInt:
			m[f.json], _ = fs.GetInt64(f.flag)
		case kindFloat:
			m[f.json], _ = fs.GetFloat64(f.flag)
		case kindStrings:
			m[f.json], _ = fs.GetStringSlice(f.flag)
		case kindTime:
			s, _ := fs.GetString(f.flag)
			at, err := parseTime(s)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", f.flag, err)
			}
			m[f.json] = at.UTC().Format(time.RFC3339Nano)
		case kindJSON:
			s, _ := fs.GetString(f.flag)
			b, err := readArg(s)
			if err != nil {
				return nil, err
			}
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				return nil, fmt.Errorf("--%s: not JSON: %w", f.flag, err)
			}
			m[f.json] = v
		default:
			m[f.json], _ = fs.GetString(f.flag)
		}
	}
	b, _ := json.Marshal(m)
	v := reflect.New(t)
	if err := json.Unmarshal(b, v.Interface()); err != nil {
		return nil, err
	}
	return v.Elem().Interface(), nil
}

func readArg(s string) ([]byte, error) {
	switch {
	case s == "-":
		return io.ReadAll(os.Stdin)
	case strings.HasPrefix(s, "@"):
		return os.ReadFile(s[1:])
	}
	return []byte(s), nil
}

func parseTime(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(d), nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if layout == time.RFC3339Nano {
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("can't read %q as a time", s)
}

// --- the tree --------------------------------------------------------------------------------------------------

// Mount adds app's commands to root: every provided cli.commands/v1 command (replacing a command already at its
// path), then the generated ones nothing provides and root doesn't already have. Call it once app's contracts are
// sealed (after the standard set's Register, or extkit.Setup).
func Mount(app core.App, root *cobra.Command) {
	provided := map[string]Command{}
	for _, c := range Contract.All(app) {
		provided[c.Path] = c
	}
	var paths []string
	for p := range provided {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	// a group made for a contract's commands names the contract
	groups := map[string]string{}
	for _, wi := range extkit.Wires() {
		if p := PathOf(wi.ID, ""); p != "remote" {
			groups[p] = "Calls " + string(wi.ID) + " on a remote"
		}
	}
	for _, p := range paths {
		attach(root, provided[p], true, groups)
	}
	for _, g := range Generated() {
		if _, ok := provided[g.Path]; ok {
			continue
		}
		attach(root, g, false, groups)
	}
	root.AddCommand(referenceCommand(app, root))
}

func attach(root *cobra.Command, c Command, replace bool, groups map[string]string) {
	words := strings.Fields(c.Path)
	if len(words) == 0 || c.Run == nil {
		return
	}
	parent := root
	for i, w := range words[:len(words)-1] {
		next := child(parent, w)
		if next == nil {
			short := groups[strings.Join(words[:i+1], " ")]
			if short == "" {
				short = "Commands under " + strings.Join(words[:i+1], " ")
			}
			next = &cobra.Command{Use: w, Short: short}
			parent.AddCommand(next)
		}
		parent = next
	}
	leaf := words[len(words)-1]
	if old := child(parent, leaf); old != nil {
		if !replace {
			return // a hand-written command already answers here
		}
		parent.RemoveCommand(old)
	}
	use := leaf
	if c.Use != "" {
		use += " " + c.Use
	}
	run := c.Run
	cmd := &cobra.Command{Use: use, Short: c.Short, Long: c.Long, Args: c.Args, SilenceUsage: true,
		Annotations: map[string]string{annSource: sourceOf(c.Run)}}
	if c.contract != "" {
		cmd.Annotations[annSource] = "contract:" + string(c.contract)
	}
	cmd.RunE = func(cc *cobra.Command, args []string) error {
		ctx := cc.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		return run(&Ctx{Context: ctx, Cmd: cc, Out: cc.OutOrStdout()}, args)
	}
	if c.Flags != nil {
		c.Flags(cmd.Flags())
	}
	if cmd.Flags().Lookup("remote") == nil {
		cmd.Flags().StringP("remote", "r", "", "the remote (a name or URL); default: the project's default remote")
	}
	parent.AddCommand(cmd)
}

func child(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}

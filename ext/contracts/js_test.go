package contracts_test

// JS providers: an app's pb_hooks providing and hooking contracts.

import (
	"context"
	"errors"
	"fmt"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	examplev1 "pocketone.dev/ext/contracts/internal/example/v1"
	fallbackv1 "pocketone.dev/ext/contracts/mail/fallback/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/jsvm"
)

// jsSetup registers the Go extensions, loads pb_hooks (files) through jsvm, then seals: the order the standard Set
// will use.
func jsSetup(t *testing.T, files map[string]string, choices extkit.Choices, exts ...extkit.Extension) (core.App, []string, error) {
	t.Helper()
	app := newApp(t)
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := extkit.Prepare(app, exts, nil, choices)
	if err != nil {
		return app, nil, err
	}
	if err := loadHooks(app, dir); err != nil {
		return app, nil, err
	}
	lines, err := extkit.Finish(app, plan, choices)
	return app, lines, err
}

func loadHooks(app core.App, dir string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return jsvm.Register(app, jsvm.Config{
		HooksDir:      dir,
		MigrationsDir: filepath.Join(dir, "no_migrations"),
		TypesDir:      dir,
		Extensions:    []any{extkit.JSContracts(app)},
	})
}

const jsMailer = `
const unused = "file-local"

onContracts((e) => {
  e.provide("mail.fallback/v1", {
  send(msg) {
    if (msg.to[0].Address === "pass@example.com") {
      throw new ContractError("pass") // not mine: the next provider, or PocketBase, sends it
    }
    if (msg.to[0].Address === "fail@example.com") {
      throw new ContractError("upstream", "mail API said 500", 502)
    }
    if (msg.to[0].Address === "capture@example.com") {
      $app.store().set("captured", unused) // methods run on the app's VM: the file's top-level variables are there
      return
    }
    $app.store().set("sent", msg.from.Address + " -> " + msg.to.map((a) => a.Address).join(",") + " (" + msg.text.length + " bytes)")
  },
  })
  e.next()
})
`

// trapExt is a mail.fallback/v1 provider that passes everything on, like mailtrap outside dev mode.
func trapExt() *ext {
	return &ext{name: "trap", decl: extkit.Contracts{Implements: extkit.IDs("mail.fallback/v1")}, register: func(app core.App) error {
		fallbackv1.Contract.Provide(app, passer{})
		return nil
	}}
}

type passer struct{}

func (passer) Send(context.Context, *mailer.Message) error { return fallbackv1.ErrPass }

func needsFallbackExt() *ext {
	return &ext{name: "bulkmail", decl: extkit.Contracts{Requires: extkit.IDs("mail.fallback/v1")}}
}

func to(addr string) *mailer.Message {
	return &mailer.Message{From: mail.Address{Address: "app@example.com"}, To: []mail.Address{{Address: addr}}, Text: "hello"}
}

func TestJSProvidesMailFallback(t *testing.T) {
	app, lines, err := jsSetup(t, map[string]string{"mail.pb.js": jsMailer}, nil, trapExt())
	if err != nil {
		t.Fatal(err)
	}
	if !contains(lines, "mail.fallback/v1 ← trap, pb_hooks") {
		t.Fatalf("lines %q", lines)
	}
	ctx := context.Background()
	chain := fallbackv1.Contract.All(app)
	s := chain[len(chain)-1] // pb_hooks, after trap

	if err := s.Send(ctx, to("a@example.com")); err != nil {
		t.Fatal(err)
	}
	if got := app.Store().Get("sent"); got != "app@example.com -> a@example.com (5 bytes)" {
		t.Fatalf("sent = %v", got)
	}

	// a thrown ContractError is the contract's typed error
	err = s.Send(ctx, to("fail@example.com"))
	var ce *extkit.Error
	if !errors.As(err, &ce) || ce.Code != "upstream" || ce.Status != 502 || ce.Message != "mail API said 500" {
		t.Fatalf("err = %#v", err)
	}

	// one VM per app: provider methods see the file's top-level variables
	if err := s.Send(ctx, to("capture@example.com")); err != nil {
		t.Fatal(err)
	}
	if got := app.Store().Get("captured"); got != "file-local" {
		t.Fatalf("captured = %v", got)
	}

	// a pass is a typed error the chain reads as "not mine"
	err = s.Send(ctx, to("pass@example.com"))
	if !errors.As(err, &ce) || ce.Code != "pass" {
		t.Fatalf("err = %#v", err)
	}
}

func TestJSCantProvideAHooksOnlyContract(t *testing.T) {
	_, _, err := jsSetup(t, map[string]string{"deploy.pb.js": `onContracts((e) => { e.provide("example/v1", { manifest() { return {files: []} } }) })`},
		nil, deployExt())
	if err == nil || !strings.Contains(err.Error(), "example/v1 is closed to pb_hooks (it can only be hooked)") {
		t.Fatalf("err = %v", err)
	}
}

func TestJSHooksDeploy(t *testing.T) {
	hooks := `
onExampleUpload((e) => {
  if (e.input.files.length > 2) {
    throw new ContractError("too_many", "at most 2 files per upload", 413)
  }
  e.next()
})
`
	app, _, err := jsSetup(t, map[string]string{"limits.pb.js": hooks}, nil, deployExt())
	if err != nil {
		t.Fatal(err)
	}
	srv := serve(t, app)
	ctx := context.Background()

	// HTTP through the client
	c := examplev1.NewClient(srv.URL, superuserToken(t, app), nil)
	if r, err := c.Upload(ctx, extkit.Auth{}, examplev1.UploadInput{Files: make([]examplev1.File, 2)}); err != nil || r.Written != 2 {
		t.Fatalf("2 files: %+v %v", r, err)
	}
	_, err = c.Upload(ctx, extkit.Auth{}, examplev1.UploadInput{Files: make([]examplev1.File, 3)})
	if status(err) != 413 || !strings.Contains(err.Error(), "too_many") {
		t.Fatalf("http 3 files: %v", err)
	}
	// Go caller: the same JS hook
	_, err = examplev1.Contract.Must(app).Upload(ctx, superuserAuth(), examplev1.UploadInput{Files: make([]examplev1.File, 3)})
	if status(err) != 413 {
		t.Fatalf("go 3 files: %v", err)
	}
}

func TestJSSatisfiesRequiresWhenNamed(t *testing.T) {
	// bulkmail requires mail.fallback/v1 and only pb_hooks provides it: naming pb_hooks under providers: keeps it on
	_, lines, err := jsSetup(t, map[string]string{"mail.pb.js": jsMailer},
		extkit.Choices{"mail.fallback/v1": []string{"pb_hooks"}}, needsFallbackExt())
	if err != nil {
		t.Fatal(err)
	}
	if contains(lines, "bulkmail off") || !contains(lines, "mail.fallback/v1 ← pb_hooks (providers:)") {
		t.Fatalf("lines %q", lines)
	}

	// without the entry the resolver can't see app code, so bulkmail is off
	_, lines, err = jsSetup(t, map[string]string{"mail.pb.js": jsMailer}, nil, needsFallbackExt())
	if err != nil {
		t.Fatal(err)
	}
	if !contains(lines, "bulkmail off: needs mail.fallback/v1, which nothing provides") {
		t.Fatalf("lines %q", lines)
	}

	// named but not provided: the seal says so
	_, _, err = jsSetup(t, map[string]string{"other.pb.js": `// nothing`}, extkit.Choices{"mail.fallback/v1": []string{"pb_hooks"}}, needsFallbackExt())
	if err == nil || !strings.Contains(err.Error(), "mail.fallback/v1") {
		t.Fatalf("err = %v", err)
	}
}

func TestJSProviderUnderConcurrency(t *testing.T) {
	app, _, err := jsSetup(t, map[string]string{"mail.pb.js": `
onContracts((e) => e.provide("mail.fallback/v1", {
  send(msg) { $app.store().set("last:" + msg.to[0].Address, msg.to[0].Address) },
}))`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := fallbackv1.Contract.All(app)[0]
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		go func(i int) {
			to := fmt.Sprintf("u%d@example.com", i)
			errs <- s.Send(context.Background(), &mailer.Message{To: []mail.Address{{Address: to}}})
		}(i)
	}
	for i := 0; i < 50; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if got := app.Store().Get("last:u49@example.com"); got != "u49@example.com" {
		t.Fatalf("got %v", got)
	}
}

// onContracts: what the app's JS requires is checked after the seal, and the startup lines say what it declared.
func TestJSRequiresAndWorksWith(t *testing.T) {
	decl := `onContracts((e) => { e.require("mail.fallback/v1"); e.worksWith("example/v1"); e.next() })`
	_, _, err := jsSetup(t, map[string]string{"needs.pb.js": decl}, nil)
	if err == nil || !strings.Contains(err.Error(), "pb_hooks requires mail.fallback/v1, which nothing provides") {
		t.Fatalf("missing requirement: %v", err)
	}
	_, lines, err := jsSetup(t, map[string]string{"needs.pb.js": decl}, nil, trapExt())
	if err != nil {
		t.Fatal(err)
	}
	if !contains(lines, "pb_hooks: requires mail.fallback/v1; works with example/v1") {
		t.Fatalf("lines %q", lines)
	}
}

// A JS provider beats a Go default, as any other provider does.
func TestJSBeatsAGoDefault(t *testing.T) {
	def := &ext{name: "trapdefault", decl: extkit.Contracts{Implements: extkit.IDs("mail.fallback/v1"), Defaults: extkit.IDs("mail.fallback/v1")},
		register: func(app core.App) error {
			fallbackv1.Contract.ProvideDefault(app, passer{})
			return nil
		}}
	app, lines, err := jsSetup(t, map[string]string{"mail.pb.js": jsMailer}, nil, def)
	if err != nil {
		t.Fatal(err)
	}
	if chain := fallbackv1.Contract.All(app); len(chain) != 1 {
		t.Fatalf("chain %d long; the default should have stepped aside (lines %q)", len(chain), lines)
	}
	if !contains(lines, "pb_hooks: provides mail.fallback/v1") {
		t.Fatalf("lines %q", lines)
	}
}

// onContracts outside the loader (after the seal) is refused, and a non-function handler is a type error.
func TestJSOnContractsMisuse(t *testing.T) {
	if _, _, err := jsSetup(t, map[string]string{"bad.pb.js": `onContracts("nope")`}, nil); err == nil || !strings.Contains(err.Error(), "must be a function") {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := jsSetup(t, map[string]string{"boom.pb.js": `onContracts((e) => { throw new ContractError("bad", "no") })`}, nil); err == nil || !strings.Contains(err.Error(), "onContracts in pb_hooks") {
		t.Fatalf("err = %v", err)
	}
}

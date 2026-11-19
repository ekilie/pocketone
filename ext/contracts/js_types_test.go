package contracts_test

// Contracts' TypeScript declarations in the app's types.d.ts: typed action hooks and e.provide, checked by tsc.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/pockethost/pocketone/ext/contracts/agent/chat/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/agent/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/bulkmail/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/mail/feedback/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/mail/suppression/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/remote/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/restore/v1"
	_ "github.com/pockethost/pocketone/ext/contracts/settings/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

func contractTypes() string {
	return extkit.JSContracts(nil).(interface{ TypeScript() string }).TypeScript()
}

// A contract with declarations has its hooks typed per operation and its e.provide entry; none falls back to the
// untyped hook.
func TestContractTypesAreTyped(t *testing.T) {
	ts := contractTypes()
	for _, want := range []string{
		"declare function onExampleUpload(handler: (e: contracts.CallEvent<contracts.example.v1.UploadInput>) => void): void;",
		"declare function onExampleManifest(handler: (e: contracts.CallEvent<{}>) => void): void;",
		"declare function onAgentComplete(handler: (e: contracts.CallEvent<contracts.agent.v1.CompleteInput>) => void): void;",
		`"mail.fallback/v1": contracts.mail.fallback.v1.Sender;`,
		"send(m: MailerMessage): void | Promise<void>;",
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("types.d.ts lacks %q", want)
		}
	}
	if strings.Contains(ts, "(e: contracts.CallEvent) => void") {
		t.Error("a generated contract's hook fell back to the untyped CallEvent")
	}
	if strings.Count(ts, "declare function onExampleUpload(") != 1 {
		t.Error("onExampleUpload is declared more than once")
	}
}

const tsGoodHooks = `
onExampleUpload((e) => {
  const n: number = e.input.files.length
  const first: string = e.input.files[0].path
  const size: number = e.input.files[0].size
  if (n > 2 || first === "" || size < 0) {
    throw new ContractError("too_many", "at most 2 files", 413)
  }
  e.next()
})

onExampleManifest((e) => { e.next() })

onAgentComplete((e) => {
  e.input.maxTokens = Math.min(e.input.maxTokens || 512, 512)
  const roles: string[] = e.input.messages.map((m) => m.role)
  if (e.input.temperature !== null && e.input.temperature > 1) e.input.temperature = 1
  console.log(roles, e.auth.kind, e.contract, e.op)
  e.next()
})

onContracts((e) => {
  e.provide("mail.fallback/v1", {
    async send(msg) {
      const to: string = msg.to[0].Address
      if (to.endsWith("@example.com") || msg.subject === "") {
        throw new ContractError("pass", "not mine")
      }
    },
  })
  e.provide("someone.elses/v1", { anything(x: number) { return x + 1 } })
  e.require("restore/v1")
  e.next()
})
`

// tsc checks hooks written against the declarations (strict, without skipLibCheck, so the declarations themselves
// are checked too) and refuses a wrong input field, a wrong type and a provider missing its method. Skipped when tsc
// isn't installed.
func TestContractTypesCheckWithTSC(t *testing.T) {
	if out, err := runContractTSC(t, tsGoodHooks); err != nil {
		t.Fatalf("tsc: %v\n%s", err, out)
	}
	bad := map[string]struct{ src, want string }{
		"unknown input field": {`onExampleUpload((e) => { console.log(e.input.filez); e.next() })`, "filez"},
		"json name in a hook": {`onAgentComplete((e) => { e.input.MaxTokens = 3; e.next() })`, "MaxTokens"},
		"wrong field type":    {`onExampleUpload((e) => { e.input.files[0].size = "big"; e.next() })`, "not assignable"},
		"provider misses its method": {`onContracts((e) => { e.provide("mail.fallback/v1", { sned() {} }); e.next() })`,
			"sned"},
		"provider's argument": {`onContracts((e) => {
  e.provide("mail.fallback/v1", { send(msg) { const n: number = msg.subject } })
  e.next()
})`, "not assignable"},
	}
	for name, c := range bad {
		out, err := runContractTSC(t, c.src)
		if err == nil || !strings.Contains(out, c.want) {
			t.Errorf("%s: tsc should refuse it (mentioning %q):\n%s", name, c.want, out)
		}
	}
}

func runContractTSC(t *testing.T, hooks string) (string, error) {
	t.Helper()
	tsc, err := exec.LookPath("tsc")
	if err != nil {
		t.Skip("tsc isn't installed")
	}
	dir := t.TempDir()
	files := map[string]string{
		// console is the engine's (ext/jsvm/types.d.ts); declared here so this file stands alone
		"contracts.d.ts": contractTypes() + "\ndeclare var console: { log(...args: any[]): void };\n",
		"hooks.pb.ts":    hooks,
		"tsconfig.json": `{"compilerOptions": {"target": "ES2022", "lib": ["ES2022"], "strict": true, "noEmit": true},
			"include": ["contracts.d.ts", "hooks.pb.ts"]}`,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, tsc, "-p", dir).CombinedOutput()
	return string(out), err
}

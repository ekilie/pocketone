package standard_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/standard"
)

// An app with the standard set gets its extensions' JS API in pb_data/types.d.ts, and hooks using them type-check.
func TestTypesFileHasTheSetsJSAPI(t *testing.T) {
	app, _ := newApp(t, standard.New(), nil)
	types, err := os.ReadFile(filepath.Join(app.DataDir(), "types.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"declare var $mcp:", "declare var $apiTokens:", "declare var $bulkmail:",
		"declare function onContracts(", "declare class ContractError", "interface Function {",
	} {
		if !strings.Contains(string(types), want) {
			t.Errorf("pb_data/types.d.ts has no %q", want)
		}
	}

	tsc, err := exec.LookPath("tsc")
	if err != nil {
		t.Skip("tsc isn't installed")
	}
	dir := t.TempDir()
	files := map[string]string{
		"pb_data/types.d.ts": string(types),
		"tsconfig.json": `{"compilerOptions": {"target": "ES2022", "module": "ESNext", "moduleResolution": "Bundler",
			"lib": ["ES2022"], "strict": true, "noEmit": true, "skipLibCheck": true},
			"include": ["pb_data/types.d.ts", "pb_hooks/**/*"]}`,
		"pb_hooks/main.pb.ts": `
			$mcp.addEndpoint({
				name: "notes_archive", description: "Archive a note.", method: "POST", path: "/api/notes/{id}/archive",
				inputSchema: { type: "object", required: ["id"], properties: { id: { type: "string" } } },
				response: ({ status, json }) => (status === 200 ? { archived: json.id } : json),
			})
			const tools: string[] = $mcp.tools()
			$apiTokens.registerScope({ name: "notes:write", description: "Change notes", paths: ["/api/notes"] })
			routerAdd("POST", "/api/notes/{id}/archive", (e) => {
				const ok: boolean = $apiTokens.hasScope(e, "notes:write")
				return e.json(200, { ok, tools })
			}, $apiTokens.requireScope("notes:write"))
			const { rate, daily } = $bulkmail.limits()
			onContracts((e) => {
				e.worksWith("api-tokens/v1")
				e.next()
			})
			throw new ContractError("nope", "not allowed", 403)
		`,
	}
	for name, src := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, tsc, "-p", dir).CombinedOutput(); err != nil {
		t.Fatalf("tsc: %v\n%s", err, out)
	}
}

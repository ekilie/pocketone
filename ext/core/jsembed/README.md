# JS Embed

Ship a plugin's or app's JS hooks and migrations inside the binary, instead of `pb_hooks` and `pb_migrations` on
disk. Nothing is written to disk and nothing on disk can override them. PocketHost's mothership binary uses it for its
own hooks.

## Install

JS Embed is not in the `pocketone` binary. Add it to your own Go build: embed the folder and pass it to jsvm as an
extension.

```go
//go:embed hooks
var files embed.FS

hooks, _ := fs.Sub(files, "hooks")
jsvm.MustRegister(app, jsvm.Config{
	Extensions: []jsvm.Extension{jsembed.Hooks(hooks), jsembed.Migrations(migrations)},
})
```

The module is `github.com/pockethost/pocketone/ext/core/jsembed`.

## Configure

There is no `pocketone.yaml` section, flag or settings screen. You choose what to embed in Go:

- `jsembed.Hooks(fsys)`: the `.pb.js` files at the root of `fsys`. jsvm's `HooksDir` is then ignored and hook
  watching is off.
- `jsembed.Migrations(fsys)`: the migration files at the root of `fsys`. `MigrationsDir` is then ignored.

List only the half you embed; the other stays on disk.

## Use

1. Put your `.pb.js` files at the root of the embedded folder, and shared modules in subfolders (for example
   `lib/`).
2. Load a shared module with `require(__hooks + "/lib/x.js")`. Inside that module, `require("./y.js")` stays in the
   embedded folder, and a relative `require` from a `.pb.js` handler resolves under `__hooks`.
3. Rebuild the binary to change a hook. There's no watching and no restart on edit.

What doesn't work, on purpose:

- `..` escapes, absolute disk paths and `node_modules` lookups are "module not found".
- JS that reads files itself (`$os.readFile(__hooks + ...)`) gets "no such file": `__hooks` is the virtual path
  `/pb_embedded/pb_hooks`, never looked up on disk. A real folder at that path makes no difference.

## Integrations

- **jsvm extensions.** `Hooks` and `Migrations` are `jsvm.Extension`s, so they sit in `jsvm.Config.Extensions` next
  to other extensions such as [JS Errors](../jserrors/README.md).
- **Building on it.** `jsembed.Root` is the value of `__hooks` with embedded hooks. Any `fs.FS` works, not only
  `embed.FS`: a zip, an overlay, or a test's `fstest.MapFS`.

## Example

[`example/main.go`](example/main.go) serves a route from hooks embedded in the binary, with no `pb_hooks` folder on
disk. Run `go run ./example serve` from this folder, then `curl http://127.0.0.1:8090/hello`. The hook is
[`example/hooks/main.pb.js`](example/hooks/main.pb.js); it requires [`example/hooks/lib/greet.js`](example/hooks/lib/greet.js).

```go
// Command example serves its JS hooks from files embedded in the binary: no pb_hooks folder on disk.
//
//	go run ./example serve
//	curl http://127.0.0.1:8090/hello
package main

import (
	"embed"
	"io/fs"
	"log"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pockethost/pocketone/ext/core/devusers"
	"github.com/pockethost/pocketone/ext/core/jsembed"
)

//go:embed hooks
var files embed.FS

func main() {
	hooks, err := fs.Sub(files, "hooks")
	if err != nil {
		log.Fatal(err)
	}
	app := pocketbase.New()
	devusers.Register(app)
	jsvm.MustRegister(app, jsvm.Config{Extensions: []jsvm.Extension{jsembed.Hooks(hooks)}})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## How it works

`Hooks` and `Migrations` implement jsvm's `HookFiles`, `Require` and `MigrationFiles` extension methods, PocketOne's
small seam in jsvm; with no extensions, jsvm behaves like upstream. The `require` registry joins and cleans paths as
text (no symlinks, no disk) and serves only paths under `__hooks` from the FS. It used to live inside jsvm
(`HooksFS`/`MigrationsFS`); it moved here on 2026-11-13.

## License

MIT

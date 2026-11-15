# Sandbox

Confines one PocketOne app's JS hooks and migrations, so apps from people you don't know can share one process.
File access stays inside the app's folder, host primitives are gone, every JS call has a time limit, raw SQL can't
leave the app's own database files, and outbound connections reach public addresses only.

## Install

Sandbox isn't a switch in `pocketone`: [Pocker](../pocker/README.md) applies it to every instance it runs. With
`provider: pocker` in `pocketone.yaml`, every instance is sandboxed and there's nothing to install. A single app
served by `pocketone` without a provider is not sandboxed.

In your own Go build, give it to PocketBase's JS plugin as an extension:

```go
sb := &sandbox.Sandbox{Dir: dir, Env: map[string]string{"GREETING": "hi"}}
jsvm.MustRegister(app, jsvm.Config{
	HooksDir:   filepath.Join(dir, "pb_hooks"),
	Extensions: []jsvm.Extension{sb},
})
```

The module is `github.com/pockethost/pocketone/ext/sandbox`. The SQL guard and the outbound network rules are
separate calls (see Integrations).

## Configure

Sandbox has no `pocketone.yaml` section of its own. Under Pocker its limits are set in the `pocker` section, as
defaults for every instance, and each instance can override them in its own `limits`:

```yaml
pocker:
  max_js_execution: 30s       # each JS call (hook, route, cron, migration) is interrupted after this
  max_js_allocation: 64MiB    # biggest single allocation JS can ask for
  max_call_stack_size: 2000   # JS recursion depth
  max_sleep: 10s              # cap on sleep()
  max_query_duration: 30s     # deadline for SQL statements that have none
  http_send_max_timeout: 60s  # cap on $http.send timeouts
  http_send_max_body: 10MiB   # cap on $http.send response size
  max_concurrent_js: 16       # JS calls one instance runs at once
  egress_allow: []            # private networks apps may still reach, such as an internal SMTP relay
```

The values shown are Pocker's defaults. An instance gets only its own `secrets` as environment.

In a Go build, the same settings are `Sandbox` fields: `Dir` (required), `Env`, `MaxExecution` (default 30 s,
negative turns it off), `MaxSleep`, `MaxAllocation`, `MaxCallStackSize`, `MaxConcurrent`, `HTTPSendMaxTimeout`,
`HTTPSendMaxBody`, and `Console` to send `console.*` output somewhere other than the process's stdout and stderr.

## Use

There's nothing to operate day to day. Hooks that stick to normal app code (`routerAdd`, `onRecord*`, `cronAdd`,
`$app.findRecordById` and the rest) run unchanged. What changes for a hook author:

- `$os.cmd`, `$os.exec`, `$os.exit` and `$filesystem.local` fail with "not available on this host (sandboxed)";
  `$os.args` is empty; process-wide Go methods such as `$app.restart` read as `undefined`.
- Files outside the app's folder can't be read or written, by any API, including through `..` or symlinks.
- `$os.getenv` and `process.env` see only the app's own environment.
- A JS call that runs past its time limit is interrupted with an error; so is one that asks for a huge allocation.
- `$http.send` to a private or loopback address is refused.

Under Pocker, `console.*` output and hook errors go to the instance's own `logs/exec.log`.

## Integrations

### Works with

| Extension | How they fit |
|---|---|
| [Pocker](../pocker/README.md) | Runs every instance through the sandbox, the SQL guard and the outbound network rules, with limits from the `pocker` section and each instance's `limits`. Its attack tests are in [`sandbox_test.go`](../pocker/sandbox_test.go). |
| [Multisite](../multisite/README.md) | Carries the per-instance `limits` from `instances.yaml` or a mothership. |
| [Firewall](../firewall/README.md) | Rate and concurrency limits in front of every request. |
| [Docker](../docker/README.md) | The alternative for untrusted apps: a container per instance instead of checks inside one process. Docker doesn't use the sandbox. |

### Building on Sandbox

- **`Sandbox`** is a `jsvm.Extension` (`OnInit`, `OnCall`, `HTTPClient`); add it to `jsvm.Config.Extensions`.
- **`GuardedDBConnect(maxQuery)`** is a `core.DBConnectFunc`: set it as the app's `DBConnect` to open the databases
  through SQLite's authorizer, with a deadline for statements that have none.
- **`InstallEgressPolicy(sandbox.EgressPolicy{Allow: ...})`** routes Go's default HTTP transport through the
  public-only dialer, for the whole process. `EgressPolicy.Dialer()` gives the same check for your own connections.
- **`BindMail(app)`** sends the app's own SMTP through that dialer.
- **`ErrSandboxed`** is the error disabled bindings return.

## Example

One app whose JS hooks are sandboxed: file access confined to the app folder, host primitives disabled, and a 5 s
limit per JS call. Run it from this folder with `go run ./example serve`.

```go
// Command example runs one PocketOne app whose JS hooks are sandboxed: file access is confined to the app folder,
// host primitives ($os.exec, $os.exit, ...) are disabled, and every JS call has a time limit.
//
//	go run ./example serve
//
// Logins (ext/devusers, local testing only): dashboard admin@example.com / password, app user user@example.com / password.
package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pockethost/pocketone/ext/devusers"
	"github.com/pockethost/pocketone/ext/sandbox"
)

func main() {
	app := pocketbase.New()
	devusers.Register(app) // local testing: superuser admin@example.com and user user@example.com, password "password"

	dir, _ := os.Getwd()
	sb := &sandbox.Sandbox{
		Dir:          dir,                                 // JS may only touch this folder
		Env:          map[string]string{"GREETING": "hi"}, // the only environment JS sees
		MaxExecution: 5 * time.Second,
	}

	jsvm.MustRegister(app, jsvm.Config{
		HooksDir:   filepath.Join(dir, "pb_hooks"),
		Extensions: []jsvm.Extension{sb}, // OnInit, OnCall and HTTPClient
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## How is this safe?

With [pocker](../pocker/README.md), PocketOne can serve hundreds of instances from one Go process, each with its own owner, hooks and data.
This section explains why that's safe enough to run code from people you don't know, and where it isn't.

Configuration and the engine API are in [POCKER.md](../pocker/POCKER.md). The attack tests are in [`sandbox_test.go`](../pocker/sandbox_test.go).

### Why one process

Most hosted PocketBase instances are idle most of the time. A container per instance means paying idle memory and a
cold start of a second or more for each one. In one process an instance's app starts on its first request in tens of
milliseconds, costs a few MB while it runs, and shares the Go runtime with everyone else.

What you give up is the kernel boundary between tenants. Everything below replaces it with checks inside the process.

### The threat

Assume every instance's owner is hostile. They control:
- their `pb_hooks` (JavaScript, run by PocketBase's goja VM);
- their `pb_migrations`;
- their collections, rules and settings;
- any SQL their hooks run;
- requests to their own instance.

They must not be able to:
- read or change another instance's files, database, environment or settings;
- read the host's files, environment, credentials or private network;
- take the process down, or starve everyone else of CPU and memory.

### The layers

#### 1. Each instance is its own app

- Every instance has its own `core.App`, database, settings, JS VMs, migrations list, settings encryption key and
  log.
- Requests are routed by hostname to exactly one app. Nothing in one app holds a reference to another.
- In production only hostnames the instance source maps are served; unknown hosts get 404 and never create a folder.

#### 2. Files: confined by the kernel, not by string checks

Every file API reachable from JS goes through an `os.Root` opened on the instance's folder. That covers:
- `$os.*`
- `$filepath.glob`/`walk`
- `$filesystem.fileFromPath`
- `$template.loadFiles`
- `$apis.static`
- `require()`

The kernel resolves each path beneath that root and refuses `..` and symlinks that lead out. There is no path
string for an attacker to get clever with.

#### 3. Host primitives: removed

The following are gone from JS:
- process control: `$os.cmd`, `$os.exec`, `$os.exit`, `$os.args`;
- `$filesystem.local`;
- PocketBase internals that reach the whole process: `$app.restart`, `bootstrap`, `onServe`, `onTerminate`, raw
  `hook` triggers, `*sql.DB` handles;
- Go's `os/exec`, `net`, `syscall` and `database/sql/driver`.

A field-name mapper hides them, so they read as `undefined`. Everything a normal app uses still works: `routerAdd`,
`onRecord*`, `cronAdd`, `$app.findRecordById` and the rest.

#### 4. Environment: only the instance's own

`$os.getenv` and `process.env` return the instance's own secrets and nothing else. Platform values never go through
the environment:
- the settings encryption key;
- superusers the host keeps in sync;
- private tokens for the host's Go code.

#### 5. SQL: SQLite's own authorizer

Raw SQL is allowed (`$app.db().newQuery(...)`), because PocketBase apps depend on it. Instead of parsing statements,
PocketOne installs SQLite's authorizer callback on every connection, so SQLite itself refuses:
- `ATTACH` and `VACUUM INTO` outside the instance's `pb_data`;
- `load_extension`;
- the pragmas that move SQLite's files elsewhere.

Statements without a deadline get one (`max_query_duration`).

#### 6. Outbound network: public addresses only

PocketOne installs one dialer process-wide, on Go's default HTTP transport and PocketBase's SMTP client. It covers:
- `$http.send`
- `$filesystem.fileFromURL`
- S3 storage and backups
- OAuth2/JWKS fetches
- SMTP

The dialer refuses:
- loopback, private ranges, link-local and cloud metadata;
- CGNAT, multicast, reserved and documentation ranges;
- the IPv6 forms of all of these, including IPv4-in-IPv6.

It checks the address actually connected to, after DNS, so DNS rebinding doesn't help. `$http.send` timeouts and
response sizes are capped.

#### 7. CPU, memory and concurrency

| Resource | What's enforced |
|---|---|
| CPU | Every JS call (hook, route, cron, migration) is interrupted after `max_js_execution`. `sleep()` is capped. |
| Concurrency | At most `max_concurrent_js` JS calls per instance at once; more wait, then fail. Per-IP and per-instance rate and concurrency limits sit in front of every request. |
| One-shot memory | Built-ins that allocate a big result in one step refuse results over `max_js_allocation`: `repeat`, `padStart`, `join`, typed arrays, `ArrayBuffer`, `Buffer`, file reads and others. Recursion depth is capped, so a JS stack overflow can't crash Go. |
| Gradual memory | Not limited: goja can't account memory per VM. Gradual growth counts against the whole process (and its swap). |

Every limit has a process default and a per-instance override.

#### 8. Failures stay with their instance

- A broken hooks file, a panic during start or a failed migration fails that instance alone: it answers 502 and logs
  to its own `logs/exec.log`. Every other instance keeps serving.
- `console.*` output goes to the instance's own log, never to the host's.

### What it doesn't cover

Be clear about these before you host strangers:

- **Gradual memory isn't per instance.** goja can't account memory per VM, so a hook that grows memory slowly uses
  the whole process's memory (and swap). Only one-shot allocations are capped.
- **It's one address space.** A bug in Go, goja, SQLite or PocketOne's own checks would cross tenants in a way a
  container boundary wouldn't. The sandbox tests cover the known paths, but they don't prove nothing else exists.
- **No OS-level layer yet.** Seccomp, Landlock and a second, OS-level egress filter aren't built in.

### How to run it

For untrusted tenants, add the OS layers yourself:
- run PocketOne as a non-root user that owns only the instances folder;
- run it under a cgroup memory limit (with swap), so the worst case is a restart, not a host OOM;
- add an OS firewall rule that lets that user reach public addresses only, an egress layer independent of the
  process;
- keep secrets for host code out of the process environment; use `private:` values instead;
- put a proxy or the built-in firewall in front, with rate limits on.

If you only run your own apps, the defaults are already more than stock PocketBase gives you.

## License

MIT

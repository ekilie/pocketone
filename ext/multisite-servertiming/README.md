# Server Timing

A `Server-Timing` response header on every request to a [Pocker](../multisite-provider-pocker/README.md) instance: how long the
instance's own handler took, and how long it took to wake the instance for the request. Your browser's dev tools show
it in the request's Timing tab.

```
Server-Timing: origin;dur=12.4, wake;dur=0.0
```

## Install

Server Timing is built into `pocketone` and on whenever it serves many instances with `provider: pocker`. There's
nothing to install. It isn't used for a single app or under `provider: docker`.

In your own Pocker build, list it first among the extensions, so its header covers every other hop:

```go
srv := &pocker.Server{Config: cfg, Extensions: []pocker.Extension{servertiming.New() /*, other extensions */}}
```

The module is `github.com/pockethost/pocketone/ext/multisite-servertiming`. It works the same in `pocker.Provider{Extensions: ...}`.

## Configure

There's nothing to configure: no `pocketone.yaml` section, flag or settings screen.

## Use

Open your browser's dev tools, pick a request to an instance and look at its timing, or read the header with curl:

```bash
curl -s -D - -o /dev/null http://alpha.localhost:8090/api/health | grep -i server-timing
```

| Entry | What it measures |
|---|---|
| `origin` | The instance's own handler: from handing it the request until it sent its response headers. |
| `wake` | Starting the instance for this request. `0` when it was already running. |

Each value is in milliseconds, one decimal, up to the response headers (time to first byte). The innermost hop comes
first; values a server in front already sent are kept, and this host's entries are added after them.

## Integrations

- **[Pocker](../multisite-provider-pocker/README.md)** registers `origin` and `wake` for every request it hands to an instance.
- **Add your own entries.** Host code and other extensions call `pocker.Timing(ctx, fn)` during a request; `fn` returns
  `[]pocker.TimingEntry{{Name, Dur, Desc}}` and runs when the headers go out. Only extensions listed after Server Timing
  are covered. Put names in `Desc`, never addresses.
- **Go API.** `FormatTiming`, `AppendServerTiming` and `WithServerTiming` render entries, append them to a header and
  wrap a response writer, for a server that isn't a Pocker extension.

## Example

There's no `example/main.go`. See it used in [`examples/pocketone/multi.go`](../../examples/pocketone/multi.go), which
lists it first in the `pocketone` binary's Pocker extensions.

## How it works

It's a Pocker wrapper (`Wrap(http.Handler)`). For each request it starts collecting timing entries in the request's
context, then stamps them into the `Server-Timing` header just before the response headers go out (not on interim
1xx responses). Flush, Hijack and `http.ResponseController` keep working through its writer.

## License

MIT

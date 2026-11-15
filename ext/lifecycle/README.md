# lifecycle

Looks after the data of the instances a host serves: it measures facts about them and carries out decisions about
them.

**Runs on:** many instances, on an [ext/host](../host/README.md) host (pocker or docker).

> **Status.** Working, first step: facts (disk used, last request served) and the `measure` decision. Archiving,
> restoring, offloading backups and purging come next, as further decisions.

- **Facts** about each instance this host serves itself (not the ones it forwards): `disk_bytes` (its folder's size,
  0 without one) with when it was measured, and `last_active` (the last request this host served for it).
- **A policy provider** decides what to do. Built in: `YAMLPolicy`, rules from the `lifecycle` section, decided on
  this host. A control plane can provide its own (for example one that delegates the decisions to a mothership), and
  providers can wrap each other.
- **A sink** receives the facts that changed (default: a log line). A sink that also implements `Loader` gives the
  facts back at start, so a restart doesn't measure everything again.

```yaml
lifecycle:
  measure_every: 24h   # how old a disk measurement may get
  report_every: 1h     # how often changed last-request times are reported
  concurrency: 2       # instances measured at once
```

```go
lc := &lifecycle.Lifecycle{Directory: h.Directory, Folder: func(i host.Instance) string { return filepath.Join(root, i.ID) }}
h.Gates = append(h.Gates, lc)                // records each local instance's last request
h.Background = append(h.Background, lc.Run) // measures and reports on the policy's schedule
```

It knows nothing about plans, payments or tenants: that belongs to whichever policy provider is configured.

## Rules: criteria an app can replace

`rules` (package `github.com/pockethost/pocketone/ext/lifecycle/rules`, standard library only) is the engine policies
are built from, on an edge or a mothership:

- **Criteria** answer one question about a subject (stale? orphaned? over its plan?) and say why. They live in a
  `Registry` by name: an app registers its own or replaces a built-in one (`Register("stale", ...)`), and every rule
  that refers to the name (`Ref("stale")`) follows, so what "stale" means is the app's to decide.
- **Where they come from** is pluggable too: code, or definitions from config (`Load`) built from kinds the app
  registered (`RegisterKind("idle", ...)`) and combined with `all`, `any` and `not`. A YAML section and a mothership
  are both just sources of definitions.
- **Rules** lead from a criterion to an action, with named vetoes that always win and a budget per run.
- **Reports** list each decision with its reasons, what was vetoed and by what, what the budget held back and what
  failed. Evaluating does nothing by itself: acting on the decisions, or only logging them in a dry run, is the
  caller's.

```go
r := rules.NewRegistry[Facts]()
r.RegisterKind("idle", idleFor)                       // params: { after: 8760h }
_ = r.Load(map[string]rules.Def{
	"stale":      {Kind: "idle", Params: map[string]any{"after": "8760h"}},
	"archivable": {Kind: "all", Of: []string{"stale", "free_plan"}},
})
e := rules.Engine[Facts]{Key: id, Rules: []rules.Rule[Facts]{{
	Name: "archive-stale", Action: "archive", When: r.Ref("archivable"),
	Unless: []rules.Veto[Facts]{{Name: "pinned", If: r.Ref("pinned")}}, Budget: 200,
}}}
report := e.Evaluate(ctx, facts)
```

## License

MIT

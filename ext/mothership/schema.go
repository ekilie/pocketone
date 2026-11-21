// Package mothership is the control plane of a fleet of edges: a PocketOne app that records which edges exist and
// which instances run where, and tells each edge what it needs to know, when it asks.
//
// Two extensions, used together in the stock binary and apart by a control plane with its own collections:
//
//   - [Server] provides instance.watch/v1: the edges' streams, their watches, and the record hooks that send changes.
//   - [Schema] keeps the `edges` and `instances` collections and provides the default instance.resolve/v1 over them.
//
// Edges follow it with the [client] package. One operator, no users: owners, accounts and plans are an extension
// that adds its collections and wraps instance.resolve/v1.
package mothership

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"gopkg.in/yaml.v3"

	resolvev1 "pocketone.dev/ext/contracts/instance/resolve/v1"
	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
	watchv1 "pocketone.dev/ext/contracts/instance/watch/v1"
	"pocketone.dev/ext/extkit"
)

// SchemaName is the schema extension's name.
const SchemaName = "mothership-schema"

// Schema keeps the mothership's collections (EnsureCollections, after bootstrap) and provides the default
// instance.resolve/v1 over them. It turns on with the server (`mothership.serve`).
type Schema struct {
	// Enabled forces it on (true) or off (false); nil follows `mothership.serve`.
	Enabled *bool
	// Apex makes <name>.<apex> a hostname of every instance; "" takes `mothership.apex`.
	Apex string
}

var _ extkit.Extension = (*Schema)(nil)

// NewSchema returns the schema extension.
func NewSchema() *Schema { return &Schema{} }

// Name implements [extkit.Extension].
func (s *Schema) Name() string { return SchemaName }

func (s *Schema) on() bool {
	if s.Enabled != nil {
		return *s.Enabled
	}
	fc, _ := Section.Get()
	return fc.Serve
}

// Contracts implements [extkit.Declarer] (nothing while it's off).
func (s *Schema) Contracts() extkit.Contracts {
	if !s.on() {
		return extkit.Contracts{}
	}
	return extkit.Contracts{Implements: extkit.IDs("instance.resolve/v1"), Defaults: extkit.IDs("instance.resolve/v1")}
}

// Register implements [extkit.Extension].
func (s *Schema) Register(app core.App, _ extkit.Config) error {
	if !s.on() {
		return nil
	}
	fc, _ := Section.Get()
	apex := s.Apex
	if apex == "" {
		apex = fc.Apex
	}
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		return EnsureCollections(e.App)
	})
	resolvev1.Contract.ProvideDefault(app, &Resolver{App: app, Apex: apex})
	return nil
}

// EnsureCollections creates the `edges` and `instances` collections when they are missing (fields an extension
// added are left alone). Only superusers can read or write them; edges log in as superusers.
//
// edges:     edge_id (unique name, e.g. lax-1), internal_url, region, status (online|draining|offline), last_seen
// instances: name (unique slug; <name>.<apex> routes to it), hosts (extra hostnames), edge (placement), power (off =
//
//	not served), suspension (non-empty = blocked with this message), dev, version, secrets, encryption_key,
//	access, limits, plugins, status
func EnsureCollections(app core.App) error {
	edges, err := app.FindCollectionByNameOrId("edges")
	if err != nil {
		edges = core.NewBaseCollection("edges")
		edges.Fields.Add(
			&core.TextField{Name: "edge_id", Required: true},
			&core.TextField{Name: "internal_url"},
			&core.TextField{Name: "region"},
			&core.TextField{Name: "status"},
			&core.DateField{Name: "last_seen"},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		edges.AddIndex("idx_edges_edge_id", true, "edge_id", "")
		if err := app.Save(edges); err != nil {
			return err
		}
	}
	if _, err := app.FindCollectionByNameOrId("instances"); err == nil {
		return nil
	}
	inst := core.NewBaseCollection("instances")
	inst.Fields.Add(
		&core.TextField{Name: "name", Required: true},
		&core.JSONField{Name: "hosts"},
		&core.RelationField{Name: "edge", CollectionId: edges.Id, MaxSelect: 1},
		&core.BoolField{Name: "power"},
		&core.TextField{Name: "suspension"},
		&core.BoolField{Name: "dev"},
		&core.TextField{Name: "version"},
		&core.JSONField{Name: "secrets", Hidden: true},
		&core.TextField{Name: "encryption_key", Hidden: true},
		&core.JSONField{Name: "access"},
		&core.JSONField{Name: "limits"},
		// "plugins" holds per-instance extension settings (multisite.Instance.Extensions); the stored field keeps its
		// old name so existing records and writers keep working.
		&core.JSONField{Name: "plugins"},
		&core.TextField{Name: "status"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	inst.AddIndex("idx_instances_name", true, "name", "")
	inst.AddIndex("idx_instances_edge", false, "edge", "")
	return app.Save(inst)
}

// Resolver is the default instance.resolve/v1, over [EnsureCollections]' schema. A hostname is <name>.<Apex> or one
// of an instance's hosts (the name wins over another instance's hosts entry; among hosts entries, the newest
// record). An instance on another edge answers with Remote "edge:<record id>"; one on the asking edge answers in
// full; one with no edge isn't served.
type Resolver struct {
	App  core.App
	Apex string
}

var _ resolvev1.Resolver = (*Resolver)(nil)

// Resolve implements [resolvev1.Resolver].
func (r *Resolver) Resolve(ctx context.Context, q resolvev1.Query) (resolvev1.Resolution, error) {
	host := strings.ToLower(q.Host)
	res := resolvev1.Resolution{Deps: []string{resolvev1.Host(host), resolvev1.Key("edge:" + q.Edge)}}
	name := ""
	if r.Apex != "" && strings.HasSuffix(host, "."+r.Apex) {
		name = strings.TrimSuffix(host, "."+r.Apex)
	}
	var cands []*core.Record
	err := r.App.RecordQuery("instances").
		AndWhere(dbx.NewExp("name = {:name} OR EXISTS (SELECT 1 FROM json_each(CASE WHEN json_valid([[hosts]]) THEN [[hosts]] ELSE '[]' END) WHERE json_each.value = {:host})",
			dbx.Params{"name": name, "host": host})).
		All(&cands)
	if err != nil {
		return res, err
	}
	sort.SliceStable(cands, func(i, j int) bool {
		ni, nj := name != "" && cands[i].GetString("name") == name, name != "" && cands[j].GetString("name") == name
		if ni != nj {
			return ni
		}
		ui, uj := cands[i].GetDateTime("updated"), cands[j].GetDateTime("updated")
		if !ui.Equal(uj) {
			return ui.After(uj)
		}
		return cands[i].Id < cands[j].Id
	})
	for _, c := range cands {
		res.Deps = append(res.Deps, resolvev1.Record("instances", c.Id))
	}
	if len(cands) == 0 {
		return res, nil
	}
	rec := cands[0]
	owner := rec.GetString("edge")
	if owner == "" {
		return res, nil // not placed: not served anywhere
	}
	self, _ := r.App.FindFirstRecordByData("edges", "edge_id", q.Edge)
	inst := sourcev1.Instance{ID: rec.Id, Hosts: r.hosts(rec)}
	if self == nil || owner != self.Id {
		inst.Remote = "edge:" + owner
		res.Instance = &inst
		return res, nil
	}
	inst.Version = rec.GetString("version")
	inst.Dev = rec.GetBool("dev")
	inst.EncryptionKey = rec.GetString("encryption_key")
	decode(rec, "secrets", &inst.Secrets)
	decode(rec, "access", &inst.Access)
	decode(rec, "limits", &inst.Limits)
	decode(rec, "plugins", &inst.Extensions) // extension settings; the stored field is still named "plugins"
	switch {
	case rec.GetString("suspension") != "":
		inst.Blocked = &sourcev1.Blocked{Status: http.StatusForbidden, Message: rec.GetString("suspension")}
	case !rec.GetBool("power"):
		inst.Blocked = &sourcev1.Blocked{Status: http.StatusConflict, Message: "instance is powered off"}
	}
	res.Instance = &inst
	return res, nil
}

func (r *Resolver) hosts(rec *core.Record) []string {
	var out []string
	if n := rec.GetString("name"); n != "" && r.Apex != "" {
		out = append(out, strings.ToLower(n+"."+r.Apex))
	}
	var extra []string
	decode(rec, "hosts", &extra)
	for _, h := range extra {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// decode converts a JSON field into out through YAML, so types with yaml tags (sourcev1.Limits' durations and sizes)
// decode as they would from a config file. A missing or null field leaves out unchanged.
func decode(rec *core.Record, field string, out any) {
	raw, err := json.Marshal(rec.Get(field))
	if err != nil || string(raw) == "null" || string(raw) == `""` {
		return
	}
	_ = yaml.Unmarshal(raw, out)
}

// Standing implements [resolvev1.Resolver]: the default schema has no work an edge must hold without traffic.
func (r *Resolver) Standing(context.Context, string) (resolvev1.Standing, error) {
	return resolvev1.Standing{}, nil
}

// Edges implements [resolvev1.Resolver].
func (r *Resolver) Edges(context.Context) (resolvev1.EdgeList, error) {
	recs, err := r.App.FindAllRecords("edges")
	if err != nil {
		return resolvev1.EdgeList{}, err
	}
	out := resolvev1.EdgeList{Deps: []string{resolvev1.Collection("edges")}, Edges: make([]watchv1.Edge, 0, len(recs))}
	for _, e := range recs {
		out.Edges = append(out.Edges, watchv1.Edge{ID: e.Id, Name: e.GetString("edge_id"), InternalURL: e.GetString("internal_url"),
			Region: e.GetString("region"), Status: e.GetString("status"), LastSeen: e.GetDateTime("last_seen").Time()})
	}
	sort.Slice(out.Edges, func(i, j int) bool { return out.Edges[i].Name < out.Edges[j].Name })
	return out, nil
}

// Touches implements [resolvev1.Resolver]: an instance claims <name>.<apex> and its hosts (before and after the
// write); an edge's edge_id names its record (only when it changes, not on every heartbeat).
func (r *Resolver) Touches(_ context.Context, rec *core.Record) ([]string, error) {
	orig := rec.Original()
	switch rec.Collection().Name {
	case "instances":
		var out []string
		for _, h := range append(r.hosts(rec), r.hosts(orig)...) {
			out = append(out, resolvev1.Host(h))
		}
		return out, nil
	case "edges":
		now, was := rec.GetString("edge_id"), orig.GetString("edge_id")
		if now == was && orig.Id != "" {
			return nil, nil
		}
		out := []string{resolvev1.Key("edge:" + now)}
		if was != "" && was != now {
			out = append(out, resolvev1.Key("edge:"+was))
		}
		return out, nil
	}
	return nil, nil
}

package modetest

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const readyLimit = 30 * time.Second

// layoutSingle runs each instance as its own `pocketone serve` in its own folder. Its extension settings are
// pocketone.yaml sections, its secrets a .env the app may read (app.env_allow).
func layoutSingle(t *testing.T, w *World, top Topology) {
	for _, in := range top.Instances {
		dir := t.TempDir()
		p := newProc(t, in.ID, dir, top.Env)
		for rel, content := range in.Files {
			p.Write(rel, content)
		}
		cfg := map[string]any{}
		for name, v := range in.Extensions {
			cfg[name] = v
		}
		if len(in.Secrets) > 0 {
			var env strings.Builder
			for _, k := range sortedKeys(in.Secrets) {
				fmt.Fprintf(&env, "%s=%q\n", k, in.Secrets[k])
			}
			p.Write(".env", env.String())
			cfg["app"] = map[string]any{"env_allow": sortedKeys(in.Secrets)}
		}
		if len(cfg) > 0 {
			p.Write("pocketone.yaml", yamlOf(t, cfg))
		}
		p.superuser("pb_data")
		args := []string{"--dir=pb_data", "--http=" + p.Addr}
		if in.Production {
			args = append(args, "--dev=false")
		}
		p.start(args...)
		s := newSite(t, in.ID, dir, p, "")
		w.insts[in.ID] = s
	}
	for _, in := range top.Instances {
		s := w.insts[in.ID]
		s.Proc.waitHTTP(in.ID+" answers /api/health", s.Host, "/api/health", readyLimit)
	}
}

// layoutHosts runs the hosts (one for Host mode, several for Fleet) and places the instances on them. Every host's
// instances file lists every instance; one owned by another host has `remote: edge:@<owner>`.
func layoutHosts(t *testing.T, w *World, top Topology, hosts []HostSpec) {
	fleet := len(hosts) > 1
	procs := map[string]*Proc{}
	for _, h := range hosts {
		if h.ID == "" || procs[h.ID] != nil {
			t.Fatalf("modetest: host ids must be set and unique (%q)", h.ID)
		}
		procs[h.ID] = newProc(t, h.ID, t.TempDir(), append(append([]string(nil), top.Env...), h.Env...))
		w.order = append(w.order, h.ID)
	}
	w.hosts = procs

	owner := map[string]string{}
	for i, in := range top.Instances {
		o := hosts[0].ID
		if fleet {
			o = hosts[i%len(hosts)].ID
			if in.On != "" {
				if procs[in.On] == nil {
					t.Fatalf("modetest: instance %q is on host %q, not in the topology", in.ID, in.On)
				}
				o = in.On
			}
		}
		owner[in.ID] = o
	}

	for _, h := range hosts {
		p := procs[h.ID]
		cfg := map[string]any{
			"server":    map[string]any{"http": p.Addr},
			"multisite": map[string]any{},
			"instances": map[string]any{"file": "./instances.yaml"},
			"pocker":    map[string]any{"root": "./pb_root"},
		}
		if fleet {
			var list []map[string]any
			for _, other := range hosts {
				if other.ID != h.ID {
					list = append(list, map[string]any{"id": other.ID, "internal_url": "http://" + procs[other.ID].Addr})
				}
			}
			cfg["edges"] = map[string]any{"self": h.ID, "list": list}
		}
		maps.Copy(cfg, h.Config)
		p.Write("pocketone.yaml", yamlOf(t, cfg))
		p.Write("instances.yaml", instancesFile(t, top.Instances, owner, h.ID))

		for _, in := range top.Instances {
			if owner[in.ID] != h.ID {
				continue
			}
			rel := filepath.Join("pb_root", in.ID)
			for f, content := range in.Files {
				p.Write(filepath.Join(rel, f), content)
			}
			p.superuser(filepath.Join(rel, "pb_data"))
			w.insts[in.ID] = newSite(t, in.ID, filepath.Join(p.Dir, rel), p, h.ID)
		}
	}
	for _, h := range hosts {
		procs[h.ID].start()
	}
	for _, h := range hosts {
		procs[h.ID].waitHTTP(h.ID+" answers /.well-known/health", "", "/.well-known/health", readyLimit)
	}
	for _, in := range top.Instances {
		s := w.insts[in.ID]
		s.Proc.waitHTTP(in.ID+" answers /api/health on "+s.owner, s.Host, "/api/health", readyLimit)
	}
}

// instancesFile is a host's instances file: every instance, with remote set for those another host owns.
func instancesFile(t *testing.T, insts []Inst, owner map[string]string, self string) string {
	var list []map[string]any
	for _, in := range insts {
		e := map[string]any{"id": in.ID, "hosts": []string{in.ID + ".test"}, "dev": !in.Production}
		if len(in.Secrets) > 0 {
			e["secrets"] = in.Secrets
		}
		if len(in.Extensions) > 0 {
			e["extensions"] = in.Extensions
		}
		if owner[in.ID] != self {
			e["remote"] = "edge:@" + owner[in.ID]
		}
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i]["id"].(string) < list[j]["id"].(string) })
	return yamlOf(t, map[string]any{"instances": list})
}

func newSite(t *testing.T, id, dir string, p *Proc, owner string) *Site {
	host := id + ".test"
	_, port, _ := strings.Cut(p.Addr, ":")
	return &Site{t: t, ID: id, Host: host, URL: "http://" + host + ":" + port, Dir: dir, Proc: p, owner: owner, dial: p.Addr}
}

// yamlOf writes v as JSON, which every YAML parser reads.
func yamlOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

package modetest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const readyLimit = 30 * time.Second

// layoutSingle runs each instance as its own `pocketone serve` in its own folder.
func layoutSingle(t *testing.T, w *World) {
	for _, in := range w.top.Instances {
		w.startSingle(in)
	}
	for _, in := range w.top.Instances {
		w.insts[in.ID].waitUp()
	}
}

// startSingle writes one single app's folder and starts it (without waiting).
func (w *World) startSingle(in Inst) {
	t := w.t
	dir := t.TempDir()
	p := newProc(t, in.ID, dir, w.top.Env)
	writeInstFolder(t, dir, in)
	writeSingleConfig(t, dir, in)
	p.superuser("pb_data")
	args := append([]string{"--dir=pb_data", "--http=" + p.Addr}, in.Args...)
	if in.Production {
		args = append(args, "--dev=false")
	}
	p.start(args...)
	w.insts[in.ID] = newSite(t, in.ID, dir, p, "")
}

// writeSingleConfig writes a single app's pocketone.yaml (extension sections, env_allow) and .env (secrets).
func writeSingleConfig(t *testing.T, dir string, in Inst) {
	cfg := map[string]any{}
	maps.Copy(cfg, in.Extensions)
	if len(in.Secrets) > 0 {
		var env strings.Builder
		for _, k := range sortedKeys(in.Secrets) {
			fmt.Fprintf(&env, "%s=%q\n", k, in.Secrets[k])
		}
		WriteFile(t, filepath.Join(dir, ".env"), env.String())
		cfg["app"] = map[string]any{"env_allow": sortedKeys(in.Secrets)}
	}
	if len(cfg) > 0 {
		WriteFile(t, filepath.Join(dir, "pocketone.yaml"), yamlOf(t, cfg))
	}
}

// writeInstFolder copies in.From, then writes in.Files, into an instance's folder.
func writeInstFolder(t *testing.T, dir string, in Inst) {
	if in.From != "" {
		copyDir(t, in.From, dir)
	}
	for rel, content := range in.Files {
		WriteFile(t, filepath.Join(dir, rel), content)
	}
}

// layoutHosts runs the hosts (one for Host mode, several for Fleet) and places the instances on them. Every host's
// instances file lists every instance; one owned by another host has `remote: edge:@<owner>`.
func layoutHosts(t *testing.T, w *World, hosts []HostSpec) {
	w.fleet = len(hosts) > 1
	for _, h := range hosts {
		if h.ID == "" || w.hosts[h.ID] != nil {
			t.Fatalf("modetest: host ids must be set and unique (%q)", h.ID)
		}
		w.hosts[h.ID] = newProc(t, h.ID, t.TempDir(), append(append([]string(nil), w.top.Env...), h.Env...))
		w.order = append(w.order, h.ID)
	}
	for i, in := range w.top.Instances {
		w.owner[in.ID] = w.pickOwner(in, i)
	}
	for _, h := range hosts {
		p := w.hosts[h.ID]
		cfg := map[string]any{
			"server":    map[string]any{"http": p.Addr},
			"multisite": map[string]any{},
			"instances": map[string]any{"file": "./instances.yaml"},
			"pocker":    map[string]any{"root": "./pb_root"},
		}
		if w.fleet {
			var list []map[string]any
			for _, other := range hosts {
				if other.ID != h.ID {
					list = append(list, map[string]any{"id": other.ID, "internal_url": "http://" + w.hosts[other.ID].Addr})
				}
			}
			cfg["edges"] = map[string]any{"self": h.ID, "list": list}
		}
		maps.Copy(cfg, h.Config)
		p.Write("pocketone.yaml", yamlOf(t, cfg))
	}
	for _, in := range w.top.Instances {
		w.placeOnHost(in)
	}
	w.writeInstanceFiles()
	for _, id := range w.order {
		w.hosts[id].start()
	}
	for _, id := range w.order {
		w.hosts[id].waitHTTP(id+" answers /.well-known/health", "", "/.well-known/health", readyLimit)
	}
	for _, in := range w.top.Instances {
		w.insts[in.ID].waitUp()
	}
}

func (w *World) pickOwner(in Inst, i int) string {
	if !w.fleet {
		return w.order[0]
	}
	if in.On != "" {
		if w.hosts[in.On] == nil {
			w.t.Fatalf("modetest: instance %q is on host %q, not in the topology", in.ID, in.On)
		}
		return in.On
	}
	return w.order[i%len(w.order)]
}

// placeOnHost writes an instance's folder on its owner and creates its superuser.
func (w *World) placeOnHost(in Inst) {
	p := w.hosts[w.owner[in.ID]]
	rel := filepath.Join("pb_root", in.ID)
	writeInstFolder(w.t, filepath.Join(p.Dir, rel), in)
	p.superuser(filepath.Join(rel, "pb_data"))
	w.insts[in.ID] = newSite(w.t, in.ID, filepath.Join(p.Dir, rel), p, p.ID)
}

// writeInstanceFiles writes every host's instances file from the topology.
func (w *World) writeInstanceFiles() {
	for _, id := range w.order {
		w.hosts[id].Write("instances.yaml", instancesFile(w.t, w.top.Instances, w.owner, id))
	}
}

// instancesFile is a host's instances file: every instance, with remote set for those another host owns.
func instancesFile(t *testing.T, insts []Inst, owner map[string]string, self string) string {
	var list []map[string]any
	for _, in := range insts {
		e := map[string]any{"id": in.ID, "hosts": []string{hostname(in.ID)}, "dev": !in.Production}
		if len(in.Secrets) > 0 {
			e["secrets"] = in.Secrets
		}
		if len(in.Extensions) > 0 {
			e["extensions"] = in.Extensions
		}
		maps.Copy(e, in.HostFields)
		if owner[in.ID] != self {
			e["remote"] = "edge:@" + owner[in.ID]
		}
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i]["id"].(string) < list[j]["id"].(string) })
	return yamlOf(t, map[string]any{"instances": list})
}

// hostname is an instance's hostname: <id>.localhost, which resolves to loopback, so a separate process (the CLI)
// reaches it too.
func hostname(id string) string { return id + ".localhost" }

func newSite(t *testing.T, id, dir string, p *Proc, owner string) *Site {
	host := hostname(id)
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

// copyDir copies the files under src into dst.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("modetest: copy %s: %v", src, err)
	}
}

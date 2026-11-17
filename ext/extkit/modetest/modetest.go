// Package modetest runs an extension's tests against the real pocketone binary in every mode the extension runs in.
//
// A test declares a [Topology]: the instances it needs, each configured in full (files, extension settings, secrets),
// and, for a multisite mode, the hosts with their own settings. [Run] places that topology as processes once per mode:
//
//   - [Single]: each instance is its own `pocketone serve`.
//   - [Host]: one multisite host (Pocker) serves every instance, routed by hostname.
//   - [Fleet]: several hosts, each instance on one of them (Inst.On), every host knowing the others through its
//     `edges` section, so a request can enter any host and be forwarded to the owner.
//
// The test body gets a [World] and talks to each instance over HTTP like any client. Every instance has the same URL
// shape in every mode, http://<id>.localhost:<port>; the harness dials the right process, and the hostname resolves to
// loopback, so links an app builds from its URL work from the test and from the CLI.
//
// The binary is examples/pocketone, built once per test process (or P1_MODETEST_BIN, a binary already built). It
// lives under the temp folder, so it runs in dev mode; instances are dev instances unless Inst.Production. -short skips
// every mode test. Every wait is bounded and logged.
package modetest

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
)

// Mode is a way PocketOne runs an app.
type Mode string

const (
	Single Mode = "single" // one app per process
	Host   Mode = "host"   // one multisite host, many instances
	Fleet  Mode = "fleet"  // several multisite hosts that forward to each other
)

// AllApp is every mode an app-level extension runs in: as a single app and as an instance on a host.
var AllApp = []Mode{Single, Host}

// Inst is one instance, configured the same way in every mode.
type Inst struct {
	ID string

	// On is the host the instance lives on in Fleet mode ("" = spread over the hosts in order). Ignored otherwise.
	On string

	// From is a folder copied into the instance's folder before Files (an app made by `pocketone pages create`, ...).
	From string

	// Files are written into the instance's folder before the first serve, by path relative to it
	// (pb_hooks/x.pb.js, pb_pages/index.ejs, pb_migrations/1_x.js, ...).
	Files map[string]string

	// Extensions holds each extension's settings by extension name. Single: the top-level `<name>:` section of the
	// app's pocketone.yaml. Host and Fleet: the instance's `extensions: {<name>: ...}` in the instances file.
	Extensions map[string]any

	// Secrets are environment values only this instance's hooks see. Single: its .env plus app.env_allow. Host and
	// Fleet: the instance's `secrets`.
	Secrets map[string]string

	// Production runs the instance outside dev mode (no dev mail trap).
	Production bool

	// Args are extra `serve` flags for a single app. Ignored on a host.
	Args []string

	// HostFields are extra fields of the instance's entry in a host's instances file (blocked, limits, access, ...).
	// Ignored for a single app.
	HostFields map[string]any
}

// HostSpec is one multisite host.
type HostSpec struct {
	ID string

	// Config holds extra top-level sections of the host's pocketone.yaml (merged over what the harness writes).
	Config map[string]any

	// Env is extra environment for the host process.
	Env []string
}

// Topology is what a test needs; each mode lays it out as processes.
type Topology struct {
	Instances []Inst

	// Hosts for Host and Fleet mode. Default: one host "host" for Host, two ("edge-1", "edge-2") for Fleet. Host mode
	// uses the first.
	Hosts []HostSpec

	// Env is extra environment for every process.
	Env []string
}

// Run runs body once per mode, each as a subtest named after the mode, on a fresh layout of top.
func Run(t *testing.T, top Topology, modes []Mode, body func(t *testing.T, w *World)) {
	t.Helper()
	if len(top.Instances) == 0 {
		t.Fatal("modetest: a topology needs at least one instance")
	}
	seen := map[string]bool{}
	for _, in := range top.Instances {
		if in.ID == "" || seen[in.ID] {
			t.Fatalf("modetest: instance ids must be set and unique (%q)", in.ID)
		}
		seen[in.ID] = true
	}
	for _, m := range modes {
		t.Run(string(m), func(t *testing.T) {
			Bin(t) // skips under -short, builds once
			w := &World{t: t, Mode: m, insts: map[string]*Site{}, hosts: map[string]*Proc{}, owner: map[string]string{}}
			w.top = top
			w.top.Instances = slices.Clone(top.Instances)
			for i := range w.top.Instances { // each mode gets its own maps, so World.Update in one can't reach the next
				in := &w.top.Instances[i]
				in.Files, in.Extensions, in.Secrets, in.HostFields = maps.Clone(in.Files), maps.Clone(in.Extensions), maps.Clone(in.Secrets), maps.Clone(in.HostFields)
			}
			switch m {
			case Single:
				layoutSingle(t, w)
			case Host:
				layoutHosts(t, w, hostsFor(top, 1))
			case Fleet:
				layoutHosts(t, w, hostsFor(top, 2))
			default:
				t.Fatalf("modetest: unknown mode %q", m)
			}
			body(t, w)
		})
	}
}

// World is one mode's running layout.
type World struct {
	t     *testing.T
	Mode  Mode
	top   Topology
	insts map[string]*Site
	hosts map[string]*Proc
	order []string          // host ids in order
	owner map[string]string // instance id → host id
	fleet bool
}

// Instance is the instance with this id, reached through the process that owns it.
func (w *World) Instance(id string) *Site {
	w.t.Helper()
	s, ok := w.insts[id]
	if !ok {
		w.t.Fatalf("modetest: no instance %q in the topology", id)
	}
	return s
}

// Host is the host process with this id (Host and Fleet mode).
func (w *World) Host(id string) *Proc {
	w.t.Helper()
	p, ok := w.hosts[id]
	if !ok {
		w.t.Fatalf("modetest: no host %q in %s mode (hosts: %v)", id, w.Mode, w.order)
	}
	return p
}

// Hosts lists the host ids in order (none in Single mode).
func (w *World) Hosts() []string { return append([]string(nil), w.order...) }

// Via is the instance reached through the given host's address instead of its owner's: the request enters that host
// and, in Fleet mode, is forwarded to the owner.
func (w *World) Via(hostID, instID string) *Site {
	w.t.Helper()
	s := *w.Instance(instID)
	s.dial = w.Host(hostID).Addr
	s.jwt = ""
	return &s
}

// OwnerOf is the id of the host that runs the instance ("" in Single mode).
func (w *World) OwnerOf(id string) string { return w.Instance(id).owner }

// Inst is the instance's current configuration.
func (w *World) Inst(id string) Inst {
	w.t.Helper()
	for _, in := range w.top.Instances {
		if in.ID == id {
			return in
		}
	}
	w.t.Fatalf("modetest: no instance %q in the topology", id)
	return Inst{}
}

// Update changes an instance's configuration the way its mode applies it: on a host, the instances file is rewritten
// and the host picks it up live (Files are written into the folder); a single app gets its new pocketone.yaml, .env
// and files, then its process restarts.
func (w *World) Update(id string, change func(in *Inst)) {
	w.t.Helper()
	i := slices.IndexFunc(w.top.Instances, func(in Inst) bool { return in.ID == id })
	if i < 0 {
		w.t.Fatalf("modetest: no instance %q in the topology", id)
	}
	change(&w.top.Instances[i])
	in := w.top.Instances[i]
	s := w.insts[id]
	Step(w.t, "update "+id)
	for rel, content := range in.Files { // only what changed: a rewrite of an unchanged hook would still restart the app
		path := filepath.Join(s.Dir, rel)
		if b, err := os.ReadFile(path); err != nil || string(b) != content {
			WriteFile(w.t, path, content)
		}
	}
	if w.Mode == Single {
		writeSingleConfig(w.t, s.Dir, in)
		w.Restart(id)
		return
	}
	w.writeInstanceFiles()
}

// Add adds an instance while the world runs: a new single app, or a new entry in every host's instances file (no host
// restart). It waits until the instance answers.
func (w *World) Add(in Inst) *Site {
	w.t.Helper()
	if _, ok := w.insts[in.ID]; ok {
		w.t.Fatalf("modetest: instance %q exists", in.ID)
	}
	Step(w.t, "add "+in.ID)
	w.top.Instances = append(w.top.Instances, in)
	if w.Mode == Single {
		w.startSingle(in)
	} else {
		w.owner[in.ID] = w.pickOwner(in, len(w.top.Instances)-1)
		w.placeOnHost(in)
		w.writeInstanceFiles()
	}
	s := w.insts[in.ID]
	s.waitUp()
	return s
}

// Restart stops and starts the process that runs the instance (on a host, the whole host) and waits until its
// instances answer again.
func (w *World) Restart(id string) {
	w.t.Helper()
	w.Stop(id)
	w.Start(id)
}

// Stop stops the process that runs the instance (on a host, the whole host).
func (w *World) Stop(id string) { w.t.Helper(); w.Instance(id).Proc.Stop() }

// Start starts a stopped process again with the same flags and waits until its instances answer.
func (w *World) Start(id string) {
	w.t.Helper()
	p := w.Instance(id).Proc
	p.restart()
	if w.Mode != Single {
		p.waitHTTP(p.ID+" answers /.well-known/health", "", "/.well-known/health", readyLimit)
	}
	for _, s := range w.insts {
		if s.Proc == p {
			s.waitUp()
		}
	}
}

func hostsFor(top Topology, min int) []HostSpec {
	hosts := append([]HostSpec(nil), top.Hosts...)
	if len(hosts) == 0 {
		if min == 1 {
			hosts = []HostSpec{{ID: "host"}}
		} else {
			hosts = []HostSpec{{ID: "edge-1"}, {ID: "edge-2"}}
		}
	}
	if min == 1 {
		return hosts[:1]
	}
	for len(hosts) < min {
		hosts = append(hosts, HostSpec{ID: fmt.Sprintf("edge-%d", len(hosts)+1)})
	}
	return hosts
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

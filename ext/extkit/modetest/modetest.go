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
// shape in every mode (http://<id>.test:<port>); the harness dials the right process, so links an app builds from its
// URL work from the test.
//
// The binary is examples/pocketone, built once per test process (or P1_MODETEST_BIN, a binary already built). It
// lives under the temp folder, so it runs in dev mode; instances are dev instances unless Inst.Production. -short skips
// every mode test. Every wait is bounded and logged.
package modetest

import (
	"fmt"
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

// Inst is one instance, configured the same way in every mode.
type Inst struct {
	ID string

	// On is the host the instance lives on in Fleet mode ("" = spread over the hosts in order). Ignored otherwise.
	On string

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
			w := &World{t: t, Mode: m, insts: map[string]*Site{}, hosts: map[string]*Proc{}}
			switch m {
			case Single:
				layoutSingle(t, w, top)
			case Host:
				layoutHosts(t, w, top, hostsFor(top, 1))
			case Fleet:
				layoutHosts(t, w, top, hostsFor(top, 2))
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
	insts map[string]*Site
	hosts map[string]*Proc
	order []string // host ids in order
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pocketone.dev/ext/extkit/modetest"
)

// A fleet of stock binaries: a mothership (`mothership: { serve: true }`) and two edges following it lazily. Each
// edge learns a hostname only when a request names it, follows its changes on one stream, forwards requests for the
// other edge's instances, and starts over when the mothership restarts.
func TestMothershipFleet(t *testing.T) {
	bin := modetest.Bin(t)
	env := modetest.Env(t)

	// the mothership
	msDir, msAddr := t.TempDir(), modetest.FreeAddr(t)
	modetest.WriteFile(t, filepath.Join(msDir, "pocketone.yaml"), "mothership:\n  serve: true\n  apex: localhost\n")
	run(t, bin, msDir, env, "superuser", "upsert", modetest.SuperuserEmail, modetest.SuperuserPassword, "--dir=pb_data")
	ms := startProc(t, bin, msDir, env, "mothership", "serve", "--dir=pb_data", "--http="+msAddr, "--dev=false")
	waitGet(t, ms, msAddr, "", "/api/health", 200)
	if !strings.Contains(ms.log.String(), "instance.watch/v1") {
		t.Logf("mothership log:\n%s", modetest.Tail(ms.log.String(), 40))
	}
	token := login(t, msAddr)

	// two edges; each registers itself on first contact
	type edgeProc struct {
		id, dir, addr string
		p             *proc
	}
	edges := map[string]*edgeProc{}
	for _, id := range []string{"e1", "e2"} {
		e := &edgeProc{id: id, dir: t.TempDir(), addr: modetest.FreeAddr(t)}
		modetest.WriteFile(t, filepath.Join(e.dir, "pocketone.yaml"), fmt.Sprintf(`server:
  http: %s
multisite: {}
pocker:
  root: ./pb_root
mothership:
  url: http://%s
  email: %s
  password: %s
  self: %s
  internal_url: http://%s
  region: local
  heartbeat: 1s
`, e.addr, msAddr, modetest.SuperuserEmail, modetest.SuperuserPassword, id, e.addr))
		edges[id] = e
	}
	edgeRec := map[string]string{}
	for _, id := range []string{"e1", "e2"} {
		var rec map[string]any
		mustAPI(t, msAddr, token, "POST", "/api/collections/edges/records", map[string]any{"edge_id": id, "internal_url": "http://" + edges[id].addr, "status": "online"}, &rec)
		edgeRec[id] = rec["id"].(string)
	}
	var a, b map[string]any
	mustAPI(t, msAddr, token, "POST", "/api/collections/instances/records", map[string]any{"name": "alpha", "edge": edgeRec["e1"], "power": true}, &a)
	mustAPI(t, msAddr, token, "POST", "/api/collections/instances/records", map[string]any{"name": "beta", "edge": edgeRec["e2"], "power": true, "hosts": []string{"beta.custom.localhost"}}, &b)
	for _, inst := range []struct{ id, edge string }{{a["id"].(string), "e1"}, {b["id"].(string), "e2"}} {
		run(t, bin, edges[inst.edge].dir, env, "superuser", "upsert", modetest.SuperuserEmail, modetest.SuperuserPassword, "--dir="+filepath.Join("pb_root", inst.id, "pb_data"))
	}
	for _, id := range []string{"e1", "e2"} {
		e := edges[id]
		e.p = startProc(t, bin, e.dir, env, id, "serve")
		waitGet(t, e.p, e.addr, "", "/.well-known/health", 200)
	}

	// each edge serves its own instance, and forwards the other's
	for _, id := range []string{"e1", "e2"} {
		e := edges[id]
		waitGet(t, e.p, e.addr, "alpha.localhost", "/api/health", 200)
		waitGet(t, e.p, e.addr, "beta.localhost", "/api/health", 200)
		waitGet(t, e.p, e.addr, "beta.custom.localhost", "/api/health", 200)
		if code, body := get(e.addr, "nope.localhost", "/api/health"); code != 404 {
			t.Fatalf("%s: unknown host: %d %s", id, code, body)
		}
	}
	if !strings.Contains(edges["e1"].p.log.String(), "mothership: stream open") {
		t.Fatalf("e1 never opened a stream:\n%s", modetest.Tail(edges["e1"].p.log.String(), 40))
	}

	// a name that didn't exist reaches both edges once an instance takes it (watched as "no such host")
	var g map[string]any
	mustAPI(t, msAddr, token, "POST", "/api/collections/instances/records", map[string]any{"name": "nope", "edge": edgeRec["e2"], "power": true}, &g)
	run(t, bin, edges["e2"].dir, env, "superuser", "upsert", modetest.SuperuserEmail, modetest.SuperuserPassword, "--dir="+filepath.Join("pb_root", g["id"].(string), "pb_data"))
	waitGet(t, edges["e1"].p, edges["e1"].addr, "nope.localhost", "/api/health", 200)

	// a change to a watched instance comes down the stream: powered off answers 409 on both edges
	mustAPI(t, msAddr, token, "PATCH", "/api/collections/instances/records/"+a["id"].(string), map[string]any{"power": false}, nil)
	for _, id := range []string{"e1", "e2"} {
		waitGet(t, edges[id].p, edges[id].addr, "alpha.localhost", "/api/health", 409)
	}

	// the mothership restarts: the edges keep serving from their caches meanwhile, then start over on a new stream
	ms.stop(t)
	if code, _ := get(edges["e2"].addr, "beta.localhost", "/api/health"); code != 200 {
		t.Fatalf("e2 stopped serving a cached host while the mothership was down: %d", code)
	}
	ms = startProc(t, bin, msDir, env, "mothership", "serve", "--dir=pb_data", "--http="+msAddr, "--dev=false")
	waitGet(t, ms, msAddr, "", "/api/health", 200)
	modetest.WaitFor(t, "e1 reconnects", 30*time.Second, func() (bool, string) {
		n := strings.Count(edges["e1"].p.log.String(), "mothership: stream open")
		return n >= 2, fmt.Sprintf("%d streams opened", n)
	})
	token = login(t, msAddr)
	mustAPI(t, msAddr, token, "PATCH", "/api/collections/instances/records/"+a["id"].(string), map[string]any{"power": true}, nil)
	for _, id := range []string{"e1", "e2"} {
		waitGet(t, edges[id].p, edges[id].addr, "alpha.localhost", "/api/health", 200)
	}
}

// ---------------------------------------------------------------------------------------------------------------

type proc struct {
	cmd    *exec.Cmd
	log    *modetest.Buffer
	exited chan error
	id     string
}

func startProc(t *testing.T, bin, dir string, env []string, id string, args ...string) *proc {
	t.Helper()
	p := &proc{log: &modetest.Buffer{}, exited: make(chan error, 1), id: id}
	p.cmd = exec.Command(bin, args...)
	p.cmd.Dir, p.cmd.Env, p.cmd.Stdout, p.cmd.Stderr = dir, env, p.log, p.log
	p.cmd.WaitDelay = 5 * time.Second
	modetest.Step(t, "start "+id+": "+strings.Join(args, " "))
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { p.exited <- p.cmd.Wait() }()
	t.Cleanup(func() {
		_ = p.cmd.Process.Kill()
		select {
		case <-p.exited:
		case <-time.After(5 * time.Second):
		}
		if t.Failed() {
			t.Logf("%s log:\n%s", id, modetest.Tail(p.log.String(), 80))
		}
	})
	return p
}

func (p *proc) stop(t *testing.T) {
	t.Helper()
	modetest.Step(t, "stop "+p.id)
	_ = p.cmd.Process.Signal(os.Interrupt)
	select {
	case <-p.exited:
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.exited
	}
	p.exited <- nil // later checks see it as exited, Cleanup doesn't wait
}

func run(t *testing.T, bin, dir string, env []string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	modetest.Step(t, "run "+strings.Join(args, " "))
	if out, err := modetest.Command(ctx, t, dir, env, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// get sends GET path with Host host to addr.
func get(addr, host, path string) (int, string) {
	if host == "" {
		host = addr
	}
	c := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", addr)
		},
	}}
	res, err := c.Get("http://" + host + path)
	if err != nil {
		return 0, err.Error()
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	return res.StatusCode, string(b)
}

func waitGet(t *testing.T, p *proc, addr, host, path string, want int) {
	t.Helper()
	modetest.WaitFor(t, fmt.Sprintf("%s %s%s answers %d", p.id, host, path, want), 30*time.Second, func() (bool, string) {
		select {
		case err := <-p.exited:
			p.exited <- err
			t.Fatalf("%s exited (%v):\n%s", p.id, err, modetest.Tail(p.log.String(), 60))
		default:
		}
		code, body := get(addr, host, path)
		return code == want, fmt.Sprintf("%d %.100s", code, body)
	})
}

func login(t *testing.T, addr string) string {
	t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	mustAPI(t, addr, "", "POST", "/api/collections/_superusers/auth-with-password",
		map[string]string{"identity": modetest.SuperuserEmail, "password": modetest.SuperuserPassword}, &out)
	return out.Token
}

func mustAPI(t *testing.T, addr, token, method, path string, body, out any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, "http://"+addr+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	rb, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %s", method, path, res.StatusCode, rb)
	}
	if out != nil {
		if err := json.Unmarshal(rb, out); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
	}
}

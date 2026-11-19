package docker

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/multisite"
)

// A pb_hooks change restarts an instance twice in a row (PocketBase in place, then the runtime's new container), so a
// request can be dropped without an answer more than once. Without Docker: an instance that drops /v twice, answering
// the runtime's probes in between, still answers the request.
func TestProxyRepeatsThroughRestarts(t *testing.T) {
	var drops atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v" && drops.Add(1) <= 2 {
			conn, _, err := w.(http.Hijacker).Hijack() // closed without an answer, as a restarting instance does
			if err == nil {
				conn.Close()
			}
			return
		}
		w.Header().Set("Connection", "close") // a fresh connection each time, so the transport's own retry stays out
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	_, ps, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(ps)

	rt, err := New(Config{Image: "alpine:3.20", DataRoot: t.TempDir(), Binaries: &Binaries{Root: t.TempDir()},
		Versions: staticVersions(nil), StartTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	c := rt.get("repeat")
	c.container, c.port = "fake", port // running, as far as the runtime knows
	c.portNow.Store(int64(port))
	c.running.Store(true)

	w := httptest.NewRecorder()
	rt.ServeInstance(w, httptest.NewRequest(http.MethodGet, "http://repeat.test/v", nil), multisite.Instance{ID: "repeat"})
	if w.Code != 200 || w.Body.String() != "ok" || drops.Load() != 3 {
		t.Fatalf("%d %q after %d tries (requests should wait, not fail)", w.Code, w.Body.String(), drops.Load())
	}

	// a request that isn't safe to repeat isn't sent again
	drops.Store(0)
	w = httptest.NewRecorder()
	rt.ServeInstance(w, httptest.NewRequest(http.MethodPost, "http://repeat.test/v", nil), multisite.Instance{ID: "repeat"})
	if w.Code != http.StatusBadGateway || drops.Load() != 1 {
		t.Fatalf("POST: %d after %d tries", w.Code, drops.Load())
	}
}

func TestDroppedUnanswered(t *testing.T) {
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF, &net.OpError{Op: "read", Err: syscall.ECONNRESET},
		&net.OpError{Op: "write", Err: syscall.EPIPE}, errors.New("http: server closed idle connection")} {
		if !droppedUnanswered(err) {
			t.Errorf("%v: not counted as dropped", err)
		}
	}
	if droppedUnanswered(errors.New("malformed HTTP response")) {
		t.Error("a bad answer counted as dropped")
	}
}

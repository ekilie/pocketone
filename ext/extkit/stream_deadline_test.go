package extkit

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A stream outlives the server's WriteTimeout: an entry sent after the timeout still reaches the client (PocketBase's
// 5-minute WriteTimeout cut instance.watch/v1's stream every 5 minutes, 2026-11-21).
func TestStreamOutlivesWriteTimeout(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = writeStream(w, r, "tick", Stream{
			Initial: []any{"first"},
			Run: func(ctx context.Context, send func(any) error) error {
				time.Sleep(300 * time.Millisecond) // past the server's WriteTimeout
				return send("late")
			},
		})
	}))
	srv.Config.WriteTimeout = 100 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var got []string
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if d, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			got = append(got, d)
		}
	}
	if len(got) != 2 || got[1] != `"late"` {
		t.Fatalf("got %v (err %v), want the late entry too", got, sc.Err())
	}
}

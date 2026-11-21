package remoteclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A 429 is retried after its Retry-After, with the request body sent again; too long a wait is an error that says so.
func TestRateLimitedRequestsAreRetried(t *testing.T) {
	var calls atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if calls.Add(1) <= 2 {
			w.Header().Set("Retry-After", "3")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	var waits []time.Duration
	prev := RetrySleep
	RetrySleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	defer func() { RetrySleep = prev }()

	c := &Client{Remote: &Remote{URL: srv.URL}, HTTP: NewHTTPClient()}
	req, _ := c.NewRequest(context.Background(), http.MethodPost, "/x", strings.NewReader("payload"))
	res, err := c.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("after retries: %v %v", res, err)
	}
	res.Body.Close()
	if calls.Load() != 3 || len(waits) != 2 || waits[0] != 3*time.Second {
		t.Fatalf("calls %d, waits %v", calls.Load(), waits)
	}
	for _, b := range bodies {
		if b != "payload" {
			t.Fatalf("bodies %q", bodies)
		}
	}

	// a server that wants more than a minute: no waiting, an error naming the wait
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "600")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer slow.Close()
	c = &Client{Remote: &Remote{URL: slow.URL}, HTTP: NewHTTPClient()}
	req, _ = c.NewRequest(context.Background(), http.MethodGet, "/x", nil)
	if _, err := c.Do(req); !IsStatus(err, http.StatusTooManyRequests) || !strings.Contains(err.Error(), "10m0s") {
		t.Fatalf("long wait: %v", err)
	}
}

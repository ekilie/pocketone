package edge_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/extkit/modetest"
)

// whoami answers GET /whoami with the instance's own secret and the edge that forwarded the request ("-" when none).
const whoami = `routerAdd("GET", "/whoami", (e) => {
	return e.string(200, $os.getenv("NAME") + " via " + (e.request.header.get("X-Edge-Forwarded-By") || "-"))
})
`

// Two real hosts, an instance on each: any edge answers for any instance (the entry edge forwards to the owner, which
// sees who forwarded it), data written through one edge is the owner's, an unknown hostname is a 404 on every edge,
// and when an owner goes down the other edge answers 503 for its instance at once while still serving its own.
func TestModes(t *testing.T) {
	top := modetest.Topology{Instances: []modetest.Inst{
		{ID: "a", On: "edge-1", Secrets: map[string]string{"NAME": "a"}, Files: map[string]string{"pb_hooks/whoami.pb.js": whoami}},
		{ID: "b", On: "edge-2", Secrets: map[string]string{"NAME": "b"}, Files: map[string]string{"pb_hooks/whoami.pb.js": whoami}},
	}}
	modetest.Run(t, top, []modetest.Mode{modetest.Fleet}, func(t *testing.T, w *modetest.World) {
		modetest.Step(t, "each instance from its owner, then through the other edge")
		for _, c := range []struct{ inst, via, want string }{
			{"a", "edge-1", "a via -"},
			{"b", "edge-2", "b via -"},
			{"a", "edge-2", "a via edge-2"},
			{"b", "edge-1", "b via edge-1"},
		} {
			if got := w.Via(c.via, c.inst).Get("/whoami"); got != c.want {
				t.Fatalf("%s through %s: %q, want %q", c.inst, c.via, got, c.want)
			}
		}

		modetest.Step(t, "a write through the other edge lands on the owner")
		viaB := w.Via("edge-2", "a")
		su := viaB.JWT()
		viaB.MustJSON(http.MethodPost, "/api/collections", su, map[string]any{
			"name": "notes", "type": "base", "fields": []map[string]any{{"name": "text", "type": "text"}},
			"listRule": "", "createRule": ""}, 200, nil)
		viaB.MustJSON(http.MethodPost, "/api/collections/notes/records", "", map[string]string{"text": "from edge-2"}, 200, nil)
		var list struct {
			Items []struct {
				Text string `json:"text"`
			} `json:"items"`
		}
		w.Instance("a").MustJSON(http.MethodGet, "/api/collections/notes/records", "", nil, 200, &list)
		if len(list.Items) != 1 || list.Items[0].Text != "from edge-2" {
			t.Fatalf("a's notes on its owner: %+v", list.Items)
		}
		if code, b := w.Instance("b").Raw(http.MethodGet, "/api/collections/notes/records", "", nil); code != 404 {
			t.Fatalf("b has a's collection: %d %s", code, b)
		}

		modetest.Step(t, "an unknown hostname is a 404 on every edge")
		for _, h := range w.Hosts() {
			s := w.Via(h, "a")
			if code, b := s.Raw(http.MethodGet, "http://nobody.test/api/health", "", nil); code != 404 {
				t.Fatalf("unknown host through %s: %d %s", h, code, b)
			}
		}

		modetest.Step(t, "the owner goes down: 503 for its instance, the other edge keeps serving its own")
		w.Host("edge-1").Stop()
		modetest.WaitFor(t, "edge-2 answers 503 for a", 15*time.Second, func() (bool, string) {
			start := time.Now()
			code, b := w.Via("edge-2", "a").Raw(http.MethodGet, "/whoami", "", nil)
			return code == 503 && time.Since(start) < 3*time.Second, fmt.Sprintf("%d in %s: %.80s", code, time.Since(start).Round(time.Millisecond), b)
		})
		if got := w.Instance("b").Get("/whoami"); got != "b via -" {
			t.Fatalf("b with edge-1 down: %q", got)
		}
	})
}

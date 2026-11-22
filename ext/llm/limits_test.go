package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	llmc "pocketone.dev/ext/contracts/llm"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	settingsv1 "pocketone.dev/ext/contracts/settings/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
	"pocketone.dev/ext/jsvm"
	"pocketone.dev/ext/llm"
)

// withLimits saves the fake provider again with these limits.
func withLimits(t *testing.T, app core.App, limits ...llm.Limit) {
	t.Helper()
	s, err := llm.LoadSettings(app)
	if err != nil {
		t.Fatal(err)
	}
	s.Limits = limits
	if err := llm.SaveSettings(app, s); err != nil {
		t.Fatal(err)
	}
}

func ledger(t *testing.T, app core.App) []*core.Record {
	t.Helper()
	rows, err := app.FindRecordsByFilter(llm.LedgerCollection, "", "created", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func complete(app core.App, subject string) error {
	ctx := context.Background()
	if subject != "" {
		ctx = llmc.WithSubject(ctx, subject)
	}
	_, err := textv1.Contract.Must(app).Complete(ctx, extkit.Internal, textv1.CompleteInput{Model: "fake/chat-1", Prompt: "hi"})
	return err
}

func isLimit(err error) bool {
	var xe *extkit.Error
	return errors.As(err, &xe) && xe.Code == llmc.ErrLimit.Code && xe.Status == 429
}

func TestLedger(t *testing.T) {
	app, _ := setup(t)
	if err := complete(app, "user:users/u1"); err != nil {
		t.Fatal(err)
	}
	rows := ledger(t, app)
	if len(rows) != 1 {
		t.Fatalf("ledger rows: %d", len(rows))
	}
	r := rows[0]
	if r.GetString("kind") != "text" || r.GetString("model") != "fake/chat-1" || r.GetString("subject") != "user:users/u1" ||
		r.GetInt("input") != 3 || r.GetInt("output") != 2 || r.GetString("refused") != "" {
		t.Fatalf("row: %v", r.PublicExport())
	}
}

func TestAppLimit(t *testing.T) {
	app, _ := setup(t)
	withLimits(t, app, llm.Limit{Scope: llm.ScopeApp, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 1})
	if err := complete(app, ""); err != nil {
		t.Fatal(err)
	}
	err := complete(app, "")
	if !isLimit(err) || !strings.Contains(err.Error(), "the whole app, 1 calls a day") || !strings.Contains(err.Error(), "00:00 UTC") {
		t.Fatalf("second call: %v", err)
	}
	rows := ledger(t, app)
	if len(rows) != 2 || rows[1].GetString("refused") == "" {
		t.Fatalf("ledger: %d rows, last refused %q", len(rows), rows[len(rows)-1].GetString("refused"))
	}
	// a rule narrowed to another kind leaves text alone
	withLimits(t, app, llm.Limit{Scope: llm.ScopeApp, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 1, Kind: "image"})
	if err := complete(app, ""); err != nil {
		t.Fatalf("image-only rule refused text: %v", err)
	}
}

func TestGuestLimits(t *testing.T) {
	app, _ := setup(t)
	// no all-guests rule: guests are refused, users and background work aren't
	if err := complete(app, "guest:aaa"); !isLimit(err) || !strings.Contains(err.Error(), "all guests") {
		t.Fatalf("uncapped guest: %v", err)
	}
	if err := complete(app, "user:users/u1"); err != nil {
		t.Fatalf("user: %v", err)
	}
	withLimits(t, app,
		llm.Limit{Scope: llm.ScopeGuests, Window: llm.WindowMonth, Metric: llm.MetricCalls, Amount: 3},
		llm.Limit{Scope: llm.ScopeGuest, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 1},
	)
	if err := complete(app, "guest:aaa"); err != nil {
		t.Fatalf("first guest call: %v", err)
	}
	if err := complete(app, "guest:aaa"); !isLimit(err) || !strings.Contains(err.Error(), "each guest") {
		t.Fatalf("guest over their own limit: %v", err)
	}
	if err := complete(app, "guest:bbb"); err != nil {
		t.Fatalf("another guest: %v", err)
	}
	if err := complete(app, "guest:ip:10.0.0.1"); err != nil {
		t.Fatalf("a guest by IP: %v", err)
	}
	// three guest calls reached the provider: the shared bucket is full
	if err := complete(app, "guest:ccc"); !isLimit(err) || !strings.Contains(err.Error(), "all guests together") || !strings.Contains(err.Error(), "(UTC)") {
		t.Fatalf("all guests full: %v", err)
	}
	if err := complete(app, "user:users/u1"); err != nil {
		t.Fatalf("users aren't guests: %v", err)
	}
}

func TestCostLimit(t *testing.T) {
	app, _ := setup(t)
	// the fake cloud lists no prices, so every call costs 0: a $0 rule refuses at once, any positive one never fills
	withLimits(t, app, llm.Limit{Scope: llm.ScopeUser, Window: llm.WindowDay, Metric: llm.MetricCost, Amount: 0})
	if err := complete(app, "user:users/u1"); !isLimit(err) || !strings.Contains(err.Error(), "$0.00") {
		t.Fatalf("$0 rule: %v", err)
	}
	if err := complete(app, ""); err != nil {
		t.Fatalf("background work isn't a user: %v", err)
	}
	withLimits(t, app, llm.Limit{Scope: llm.ScopeUser, Window: llm.WindowDay, Metric: llm.MetricOutput, Amount: 3})
	if err := complete(app, "user:users/u1"); err != nil { // 2 output tokens
		t.Fatal(err)
	}
	if err := complete(app, "user:users/u1"); err != nil { // 4 now: over, but checked before
		t.Fatal(err)
	}
	if err := complete(app, "user:users/u1"); !isLimit(err) || !strings.Contains(err.Error(), "used 4 output tokens") {
		t.Fatalf("output tokens: %v", err)
	}
}

func TestLimitSettings(t *testing.T) {
	app, _ := setup(t)
	found, ok := settingsv1.SectionOf(app, "llm")
	if !ok {
		t.Fatal("no llm settings section")
	}
	sec := found.(settingsv1.Saver)
	base := `{"providers":[{"name":"fake","type":"openai","endpoint":"http://x/v1","key":"k1"}],"limits":[%s]}`
	for _, bad := range []string{
		`{"scope":"everyone","window":"day","metric":"calls","amount":1}`,
		`{"scope":"app","window":"week","metric":"calls","amount":1}`,
		`{"scope":"app","window":"day","metric":"calls","amount":1.5}`,
		`{"scope":"app","window":"day","metric":"cost","amount":-1}`,
		`{"scope":"app","window":"day","metric":"cost","amount":1,"kind":"poetry"}`,
		`{"scope":"app","window":"day","metric":"cost","amount":1,"model":"noslash"}`,
	} {
		if _, err := sec.Save(context.Background(), json.RawMessage(strings.Replace(base, "%s", bad, 1))); err == nil {
			t.Errorf("saved a bad limit: %s", bad)
		}
	}
	out, err := sec.Save(context.Background(), json.RawMessage(strings.Replace(base, "%s",
		`{"scope":"guests","window":"day","metric":"cost","amount":0.5}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Limits []llm.Limit
		Kinds  []string
		Spend  struct{ TodayCalls int }
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Limits) != 1 || v.Limits[0].Amount != 0.5 || len(v.Kinds) != 8 {
		t.Fatalf("view: %s", out)
	}
}

const forHooks = `
routerAdd("GET", "/llm-for", async (e) => {
  const r = await $llm.for(e).text.completeAsync({ model: "fake/chat-1", prompt: "hi" });
  return e.json(200, { text: r.text });
});
routerAdd("GET", "/llm-plain", async (e) => {
  const r = await $llm.text.completeAsync({ model: "fake/chat-1", prompt: "hi" });
  return e.json(200, { text: r.text });
});
`

// TestJSFor: $llm.for(e) counts a request's calls for its guest (by IP until the cookie comes back, then by cookie);
// plain $llm counts for nobody in particular.
func TestJSFor(t *testing.T) {
	app, _ := setup(t)
	withLimits(t, app, llm.Limit{Scope: llm.ScopeGuests, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 100})
	dir := filepath.Join(t.TempDir(), "pb_hooks")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "main.pb.js"), []byte(forHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := jsvm.Register(app, jsvm.Config{HooksDir: dir, OnInit: func(rt js.Runtime) { llm.New().BindJS(app, rt) }}); err != nil {
		t.Fatal(err)
	}
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(*core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	ta := app.(*tests.TestApp)
	var cookie string
	(&tests.ApiScenario{
		Method: http.MethodGet, URL: "/llm-for", DisableTestAppCleanup: true,
		TestAppFactory: func(testing.TB) *tests.TestApp { return ta },
		ExpectedStatus: 200, ExpectedContent: []string{`"text":"Hello"`},
		AfterTestFunc: func(t testing.TB, _ *tests.TestApp, res *http.Response) {
			for _, c := range res.Cookies() {
				if c.Name == llm.GuestCookie {
					cookie = c.Name + "=" + c.Value
				}
			}
		},
	}).Test(t)
	if cookie == "" {
		t.Fatal("no guest cookie set")
	}
	(&tests.ApiScenario{
		Method: http.MethodGet, URL: "/llm-for", DisableTestAppCleanup: true, Headers: map[string]string{"Cookie": cookie},
		TestAppFactory: func(testing.TB) *tests.TestApp { return ta },
		ExpectedStatus: 200, ExpectedContent: []string{`"text":"Hello"`},
	}).Test(t)
	(&tests.ApiScenario{
		Method: http.MethodGet, URL: "/llm-plain", DisableTestAppCleanup: true,
		TestAppFactory: func(testing.TB) *tests.TestApp { return ta },
		ExpectedStatus: 200, ExpectedContent: []string{`"text":"Hello"`},
	}).Test(t)
	rows := ledger(t, app)
	if len(rows) != 3 {
		t.Fatalf("ledger rows: %d", len(rows))
	}
	id := strings.SplitN(strings.TrimPrefix(cookie, llm.GuestCookie+"="), ".", 2)[0]
	if s := rows[0].GetString("subject"); !strings.HasPrefix(s, "guest:ip:") {
		t.Errorf("first call's subject %q: want guest:ip:...", s)
	}
	if s := rows[1].GetString("subject"); s != "guest:"+id {
		t.Errorf("second call's subject %q: want guest:%s", s, id)
	}
	if s := rows[2].GetString("subject"); s != "" {
		t.Errorf("plain $llm's subject %q: want none", s)
	}
	// a forged cookie (someone else's id, no valid signature) counts by IP, with a fresh cookie
	(&tests.ApiScenario{
		Method: http.MethodGet, URL: "/llm-for", DisableTestAppCleanup: true,
		Headers:        map[string]string{"Cookie": llm.GuestCookie + "=" + id + ".00000000000000000000000000000000"},
		TestAppFactory: func(testing.TB) *tests.TestApp { return ta },
		ExpectedStatus: 200, ExpectedContent: []string{`"text":"Hello"`},
	}).Test(t)
	if s := ledger(t, app)[3].GetString("subject"); !strings.HasPrefix(s, "guest:ip:") {
		t.Errorf("forged cookie's subject %q: want guest:ip:...", s)
	}
}

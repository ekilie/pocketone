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
	ledgerv1 "pocketone.dev/ext/contracts/llm/ledger/v1"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/js"
	"pocketone.dev/ext/jsvm"
	"pocketone.dev/ext/llm"
)

// A handler changes the model and the subject; the provider gets the new model, the ledger counts the new subject,
// and OnCallDone sees what the call used.
func TestOnCallChanges(t *testing.T) {
	app, fc := setup(t)
	llmc.OnCall(app).BindFunc(func(e *llmc.CallEvent) error {
		if e.Kind != "text" || e.Op != "complete" || e.Subject != "user:users/u1" {
			t.Errorf("event: %s %s %q", e.Kind, e.Op, e.Subject)
		}
		e.Model = "fake/chat-2"
		e.Subject = "team:acme"
		e.Input.(*textv1.CompleteInput).MaxTokens = 7
		return e.Next()
	})
	var done *llmc.CallDoneEvent
	llmc.OnCallDone(app).BindFunc(func(e *llmc.CallDoneEvent) error { done = e; return e.Next() })
	if err := complete(app, "user:users/u1"); err != nil {
		t.Fatal(err)
	}
	if b := fc.body("/v1/chat/completions"); b["model"] != "chat-2" || b["max_tokens"] != float64(7) {
		t.Fatalf("provider got %v", b)
	}
	if r := ledger(t, app)[0]; r.GetString("model") != "fake/chat-2" || r.GetString("subject") != "team:acme" {
		t.Fatalf("ledger: %v", r.PublicExport())
	}
	if done == nil || done.Subject != "team:acme" || done.Usage.OutputTokens != 2 || done.Model != "fake/chat-2" {
		t.Fatalf("done: %+v", done)
	}
}

func TestOnCallRefuses(t *testing.T) {
	app, _ := setup(t)
	llmc.OnCall(app).BindFunc(func(e *llmc.CallEvent) error {
		if e.Subject == "user:users/free" {
			return e.Refuse("Free accounts get 10 questions a day.")
		}
		if e.Subject == "user:users/lazy" {
			return nil // never calls Next
		}
		return e.Next()
	})
	err := complete(app, "user:users/free")
	if !isLimit(err) || !strings.Contains(err.Error(), "Free accounts") {
		t.Fatalf("refused: %v", err)
	}
	if err := complete(app, "user:users/lazy"); !isLimit(err) || !strings.Contains(err.Error(), "e.next()") {
		t.Fatalf("no next: %v", err)
	}
	rows := ledger(t, app)
	if len(rows) != 2 || !strings.Contains(rows[0].GetString("refused"), "Free accounts") {
		t.Fatalf("ledger: %d rows", len(rows))
	}
	if err := complete(app, "user:users/pro"); err != nil {
		t.Fatal(err)
	}
}

// The built-in limits run after custom handlers: they count the model a handler settled on, and a handler that caps
// guests lets guest calls through without an all-guests limit.
func TestBuiltInAfterCustom(t *testing.T) {
	app, _ := setup(t)
	withLimits(t, app, llm.Limit{Scope: llm.ScopeApp, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 1, Model: "fake/chat-2"})
	llmc.OnCall(app).BindFunc(func(e *llmc.CallEvent) error {
		e.Model = "fake/chat-2"
		e.GuestsCapped = true
		return e.Next()
	})
	if err := complete(app, "guest:aaa"); err != nil {
		t.Fatalf("capped guest: %v", err)
	}
	if err := complete(app, ""); !isLimit(err) || !strings.Contains(err.Error(), "fake/chat-2") {
		t.Fatalf("redirected call over the chat-2 limit: %v", err)
	}
}

func TestLedgerAPI(t *testing.T) {
	app, _ := setup(t)
	withLimits(t, app,
		llm.Limit{Scope: llm.ScopeUser, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 5},
		llm.Limit{Scope: llm.ScopeUser, Window: llm.WindowDay, Metric: llm.MetricOutput, Amount: 3},
		llm.Limit{Scope: llm.ScopeApp, Window: llm.WindowMonth, Metric: llm.MetricCalls, Amount: 100},
		llm.Limit{Scope: llm.ScopeGuests, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 100},
	)
	for _, s := range []string{"user:users/u1", "user:users/u1", "user:users/u2", ""} {
		if err := complete(app, s); err != nil {
			t.Fatal(err)
		}
	}
	l := ledgerv1.Contract.Must(app)
	ctx := context.Background()
	in := extkit.Internal

	sp, err := l.Spent(ctx, in, ledgerv1.SpentInput{Subject: "user:users/u1", Window: "day"})
	if err != nil || sp.Calls != 2 || sp.Output != 4 || sp.Input != 6 {
		t.Fatalf("spent u1: %+v %v", sp, err)
	}
	if all, _ := l.Spent(ctx, in, ledgerv1.SpentInput{Window: "month"}); all.Calls != 4 {
		t.Fatalf("spent, whole app: %+v", all)
	}
	// the request's own subject, as $llm.for(e) passes it
	if own, _ := l.Spent(llmc.WithSubject(ctx, "user:users/u2"), in, ledgerv1.SpentInput{}); own.Calls != 1 {
		t.Fatalf("spent, own subject: %+v", own)
	}

	rem, err := l.Remaining(ctx, in, ledgerv1.RemainingInput{Subject: "user:users/u1"})
	if err != nil || len(rem.Limits) != 3 {
		t.Fatalf("remaining: %+v %v", rem, err)
	}
	if first := rem.Limits[0]; first.Metric != "output" || first.Used != 4 || first.Left != 0 || !strings.Contains(first.Limit, "each user") {
		t.Fatalf("tightest first: %+v", first)
	}

	sum, err := l.Summary(ctx, in, ledgerv1.SummaryInput{By: "subject", Window: "day"})
	if err != nil || len(sum.Rows) != 3 {
		t.Fatalf("summary: %+v %v", sum, err)
	}
	if byDay, _ := l.Summary(ctx, in, ledgerv1.SummaryInput{By: "day"}); len(byDay.Rows) != 1 || byDay.Rows[0].Calls != 4 {
		t.Fatalf("summary by day: %+v", byDay)
	}
	if _, err := l.Summary(ctx, in, ledgerv1.SummaryInput{By: "colour"}); err == nil {
		t.Fatal("summary by colour")
	}

	page, err := l.Calls(ctx, in, ledgerv1.CallsInput{Subject: "user:users/u1", Limit: 1})
	if err != nil || len(page.Calls) != 1 || !page.More || page.Calls[0].Model != "fake/chat-1" {
		t.Fatalf("calls: %+v %v", page, err)
	}

	// a signed-in user is answered about themselves only; anonymous callers not at all
	user := extkit.Auth{Kind: "record", Collection: "users", RecordID: "u1", Via: "http"}
	if mine, err := l.Spent(ctx, user, ledgerv1.SpentInput{Window: "day"}); err != nil || mine.Calls != 2 {
		t.Fatalf("user's own spend: %+v %v", mine, err)
	}
	if _, err := l.Spent(ctx, user, ledgerv1.SpentInput{Subject: "user:users/u2"}); !errorCode(err, "forbidden") {
		t.Fatalf("another user's spend: %v", err)
	}
	if _, err := l.Remaining(ctx, extkit.Auth{Via: "http"}, ledgerv1.RemainingInput{}); err == nil {
		t.Fatal("anonymous remaining")
	}
}

func errorCode(err error, code string) bool {
	var xe *extkit.Error
	return errors.As(err, &xe) && xe.Code == code
}

const middlewareHooks = `
$llm.onCall((e) => {
  if (e.input.prompt === "forbidden") throw e.refuse("Not that.");
  if (e.guest) e.guestsCapped = true;
  e.input.maxTokens = 9;
  e.model = "fake/chat-2";
  e.next();
});
$llm.onCallDone((e) => {
  const r = new Record($app.findCollectionByNameOrId("done"));
  r.set("result", JSON.stringify({ subject: e.subject, guest: e.guest, model: e.model, output: e.usage.outputTokens }));
  $app.save(r);
  e.next();
});
routerAdd("POST", "/ask", async (e) => {
  const llm = $llm.for(e);
  try {
    const r = await llm.text.completeAsync({ model: "fake/chat-1", prompt: e.requestInfo().body.prompt });
    return e.json(200, { text: r.text, spent: llm.ledger.spent("day"), left: llm.ledger.remaining() });
  } catch (err) {
    return e.json(429, { error: String(err.message || err) });
  }
});
`

// TestJSMiddleware: $llm.onCall changes and refuses calls from page code, $llm.onCallDone sees them, and
// $llm.for(e).ledger answers about the request's guest.
func TestJSMiddleware(t *testing.T) {
	app, fc := setup(t)
	withLimits(t, app, llm.Limit{Scope: llm.ScopeGuest, Window: llm.WindowDay, Metric: llm.MetricCalls, Amount: 10})
	c := core.NewBaseCollection("done")
	c.Fields.Add(&core.TextField{Name: "result", Max: 10000})
	if err := app.Save(c); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "pb_hooks")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "main.pb.js"), []byte(middlewareHooks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := jsvm.Register(app, jsvm.Config{HooksDir: dir, OnInit: func(rt js.Runtime) { llm.New().BindJS(app, rt) }}); err != nil {
		t.Fatal(err)
	}
	if err := app.OnBootstrap().Trigger(&core.BootstrapEvent{App: app}, func(*core.BootstrapEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	ta := app.(*tests.TestApp)
	var body string
	(&tests.ApiScenario{
		Method: http.MethodPost, URL: "/ask", Body: strings.NewReader(`{"prompt":"hi"}`), DisableTestAppCleanup: true,
		Headers:        map[string]string{"Content-Type": "application/json"},
		TestAppFactory: func(testing.TB) *tests.TestApp { return ta },
		ExpectedStatus: 200, ExpectedContent: []string{`"text":"Hello"`},
		AfterTestFunc: func(t testing.TB, _ *tests.TestApp, res *http.Response) {
			b := make([]byte, 4096)
			n, _ := res.Body.Read(b)
			body = string(b[:n])
		},
	}).Test(t)
	if b := fc.body("/v1/chat/completions"); b["model"] != "chat-2" || b["max_tokens"] != float64(9) {
		t.Fatalf("provider got %v", b)
	}
	var out struct {
		Spent struct{ Calls int }
		Left  []struct {
			Limit string
			Left  float64
		}
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	if out.Spent.Calls != 1 || len(out.Left) != 1 || out.Left[0].Left != 9 {
		t.Fatalf("ledger from JS: %s", body)
	}
	done, err := app.FindAllRecords("done")
	if err != nil || len(done) != 1 || !strings.Contains(done[0].GetString("result"), `"guest":true`) ||
		!strings.Contains(done[0].GetString("result"), `"model":"fake/chat-2"`) {
		t.Fatalf("onCallDone: %v %v", done, err)
	}
	(&tests.ApiScenario{
		Method: http.MethodPost, URL: "/ask", Body: strings.NewReader(`{"prompt":"forbidden"}`), DisableTestAppCleanup: true,
		Headers:        map[string]string{"Content-Type": "application/json"},
		TestAppFactory: func(testing.TB) *tests.TestApp { return ta },
		ExpectedStatus: 429, ExpectedContent: []string{`Not that.`},
	}).Test(t)
}

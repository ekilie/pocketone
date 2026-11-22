package llm

import (
	"context"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"

	llmc "pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

func ledgerApp(t *testing.T) (core.App, *router) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	if _, err := extkit.Setup(app, []extkit.Extension{New()}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return app, &router{app: app}
}

func TestPruneLedger(t *testing.T) {
	app, r := ledgerApp(t)
	for i := 0; i < 2; i++ {
		r.record(context.Background(), &call{kind: "text", model: "p/m"}, llmc.Usage{CostMicroUSD: 10}, "")
	}
	rows, _ := app.FindAllRecords(LedgerCollection)
	old := time.Now().Add(-ledgerKeep - 24*time.Hour).UTC().Format("2006-01-02 15:04:05.000Z")
	if _, err := app.DB().NewQuery("UPDATE {{_llm_calls}} SET [[created]] = {:old} WHERE id = {:id}").
		Bind(dbx.Params{"old": old, "id": rows[0].Id}).Execute(); err != nil {
		t.Fatal(err)
	}
	if err := pruneLedger(app); err != nil {
		t.Fatal(err)
	}
	left, _ := app.FindAllRecords(LedgerCollection)
	if len(left) != 1 || left[0].Id != rows[1].Id {
		t.Fatalf("after prune: %d rows", len(left))
	}
}

// TestSettleJob: a job's row, written at create with no cost, gets the cost once get sees the job done; a row with a
// cost already, or no row at all, is left alone.
func TestSettleJob(t *testing.T) {
	app, r := ledgerApp(t)
	ctx := context.Background()
	r.record(ctx, &call{kind: "video", model: "vultr/vid-1", job: "vultr:abc"}, llmc.Usage{}, "")

	running := llmc.Job{ID: "vultr:abc", Status: llmc.JobRunning, Usage: llmc.Usage{CostMicroUSD: 500000}}
	r.settleJob(ctx, target{}, "vultr:abc", &running)
	rec, err := app.FindFirstRecordByFilter(LedgerCollection, "job = 'vultr:abc'")
	if err != nil || rec.GetInt("cost") != 0 {
		t.Fatalf("a running job settled: %v %v", rec, err)
	}

	done := llmc.Job{ID: "vultr:abc", Status: llmc.JobDone, Usage: llmc.Usage{CostMicroUSD: 500000, Seconds: 8}}
	r.settleJob(ctx, target{}, "vultr:abc", &done)
	rec, _ = app.FindFirstRecordByFilter(LedgerCollection, "job = 'vultr:abc'")
	if rec.GetInt("cost") != 500000 || rec.GetFloat("seconds") != 8 {
		t.Fatalf("done job's row: cost %d, seconds %v", rec.GetInt("cost"), rec.GetFloat("seconds"))
	}

	again := llmc.Job{ID: "vultr:abc", Status: llmc.JobDone, Usage: llmc.Usage{CostMicroUSD: 999}}
	r.settleJob(ctx, target{}, "vultr:abc", &again)
	rec, _ = app.FindFirstRecordByFilter(LedgerCollection, "job = 'vultr:abc'")
	if rec.GetInt("cost") != 500000 {
		t.Fatalf("a settled row changed: %d", rec.GetInt("cost"))
	}

	none := llmc.Job{ID: "vultr:zzz", Status: llmc.JobDone, Usage: llmc.Usage{CostMicroUSD: 1}}
	r.settleJob(ctx, target{}, "vultr:zzz", &none) // no row: nothing happens, nothing panics
	if rows, _ := app.FindAllRecords(LedgerCollection); len(rows) != 1 {
		t.Fatalf("rows: %d", len(rows))
	}
}

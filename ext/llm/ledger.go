package llm

import (
	"context"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	llmc "pocketone.dev/ext/contracts/llm"
	"pocketone.dev/ext/extkit"
)

// The ledger: one row per call that reached a provider (and per call a limit refused), in a system collection. Limits
// count from it, Settings → LLM shows spend from it, and since rows are ordinary records an app can hook
// onRecordAfterCreateSuccess on it (to debit its own per-user credits, say).
const (
	LedgerCollection   = "_llm_calls"
	LedgerCollectionID = "pbc_llm_calls"
)

// Migrations implements extkit.Migrator: 0001 creates the ledger.
func (*Extension) Migrations() []extkit.Migration {
	return []extkit.Migration{{Name: "0001_ledger", Up: createLedger, Down: dropLedger}}
}

func createLedger(tx core.App) error {
	c := core.NewBaseCollection(LedgerCollection, LedgerCollectionID)
	c.System = true
	c.Fields.Add(
		&core.TextField{Id: "llc_kind", Name: "kind", Max: 20, Help: "text, image, speech, transcribe, align, music, video or embed."},
		&core.TextField{Id: "llc_model", Name: "model", Max: 300, Help: "provider/model."},
		&core.TextField{Id: "llc_subject", Name: "subject", Max: 300, Help: "Whom the call was for: user:<collection>/<id>, guest:<id>, guest:ip:<address>, or empty."},
		&core.TextField{Id: "llc_via", Name: "via", Max: 20, Help: "How it arrived: go (page code), http, mcp."},
		&core.NumberField{Id: "llc_input", Name: "input", OnlyInt: true, Help: "Input tokens."},
		&core.NumberField{Id: "llc_cached", Name: "cached", OnlyInt: true, Help: "Input tokens billed at the cached rate."},
		&core.NumberField{Id: "llc_output", Name: "output", OnlyInt: true, Help: "Output tokens."},
		&core.NumberField{Id: "llc_seconds", Name: "seconds", Help: "Seconds of audio or video."},
		&core.NumberField{Id: "llc_cost", Name: "cost", OnlyInt: true, Help: "What it cost, in millionths of a US dollar."},
		&core.BoolField{Id: "llc_estimated", Name: "estimated", Help: "The cost was estimated from listed prices, not reported by the provider."},
		&core.TextField{Id: "llc_refused", Name: "refused", Max: 500, Help: "Why a limit refused the call; empty when it reached the provider."},
		&core.AutodateField{Id: "llc_created", Name: "created", OnCreate: true, System: true},
	)
	c.AddIndex("idx_llm_calls_created", false, "created", "")
	c.AddIndex("idx_llm_calls_subject_created", false, "subject, created", "")
	return tx.Save(c)
}

func dropLedger(tx core.App) error {
	c, err := tx.FindCollectionByNameOrId(LedgerCollection)
	if err != nil {
		return nil
	}
	c.System = false // a system collection can't be deleted
	if err := tx.SaveNoValidate(c); err != nil {
		return err
	}
	return tx.Delete(c)
}

// call is one billable call on its way through: what the ledger row will say.
type call struct {
	kind    string
	model   string
	subject string
	via     string
}

// record writes c's ledger row with what the call used. refused is the limit's reason when one refused it. A failed
// write is logged, never the call's error: the provider already answered.
func (r *router) record(ctx context.Context, c *call, u llmc.Usage, refused string) {
	if c == nil {
		return
	}
	col, err := r.app.FindCachedCollectionByNameOrId(LedgerCollection)
	if err != nil {
		r.app.Logger().Warn("llm: ledger: no collection", "error", err)
		return
	}
	rec := core.NewRecord(col)
	rec.Set("kind", c.kind)
	rec.Set("model", c.model)
	rec.Set("subject", c.subject)
	rec.Set("via", c.via)
	rec.Set("input", u.InputTokens)
	rec.Set("cached", u.CachedTokens)
	rec.Set("output", u.OutputTokens)
	rec.Set("seconds", u.Seconds)
	rec.Set("cost", u.CostMicroUSD)
	rec.Set("estimated", u.CostEstimated)
	rec.Set("refused", refused)
	if err := r.app.SaveWithContext(ctx, rec); err != nil {
		r.app.Logger().Warn("llm: ledger: write failed", "model", c.model, "error", err)
	}
}

// windowStart is when the UTC day or month holding t began; next is when the next one begins.
func windowStart(t time.Time, window string) (start, next time.Time) {
	t = t.UTC()
	if window == WindowMonth {
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	}
	start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 0, 1)
}

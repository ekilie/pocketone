package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	llmc "pocketone.dev/ext/contracts/llm"
	ledgerv1 "pocketone.dev/ext/contracts/llm/ledger/v1"
	"pocketone.dev/ext/extkit"
)

// ledgerService is llm.ledger/v1 over the _llm_calls collection.
type ledgerService struct{ r *router }

var _ ledgerv1.Ledger = ledgerService{}

const ledgerTime = "2006-01-02 15:04:05.000Z"

var errForbiddenSubject = &extkit.Error{Code: "forbidden", Status: 403, Message: "a signed-in user may ask only about themselves"}

// subjectFor is whom Remaining and Spent answer about. Page code and superusers (and tokens a superuser owns) may
// name any subject; without one they get the call's own (from $llm.for). A signed-in user, or a token they own, gets
// themselves. Anyone else is refused.
func subjectFor(ctx context.Context, a extkit.Auth, asked string) (subject string, err error) {
	if a.Superuser() {
		if asked != "" {
			return asked, nil
		}
		return llmc.SubjectOf(ctx), nil
	}
	if (a.Kind == "record" || a.Kind == "token") && a.Collection != "" && a.RecordID != "" {
		own := llmc.UserSubject(a.Collection, a.RecordID)
		if asked != "" && asked != own {
			return "", errForbiddenSubject
		}
		return own, nil
	}
	return "", extkit.ErrUnauthorized
}

// span turns a window ("day", "month") or since/until into a time range; zero times are open ends.
func span(window, since, until string) (from, to time.Time, err error) {
	switch window {
	case "":
	case WindowDay, WindowMonth:
		from, to = windowStart(time.Now(), window)
		return from, to, nil
	default:
		return from, to, badInput(`window must be "day" or "month"`)
	}
	parse := func(name, v string) (time.Time, error) {
		if v == "" {
			return time.Time{}, nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.UTC(), nil
			}
		}
		return time.Time{}, badInput(fmt.Sprintf("%s: give a time as RFC 3339 or YYYY-MM-DD", name))
	}
	if from, err = parse("since", since); err != nil {
		return
	}
	to, err = parse("until", until)
	return
}

// where is the filter for a range, a subject ("" for all), a kind and a model.
func where(from, to time.Time, subject, kind, model string) dbx.Expression {
	exps := []dbx.Expression{}
	if !from.IsZero() {
		exps = append(exps, dbx.NewExp("[[created]] >= {:from}", dbx.Params{"from": from.Format(ledgerTime)}))
	}
	if !to.IsZero() {
		exps = append(exps, dbx.NewExp("[[created]] < {:to}", dbx.Params{"to": to.Format(ledgerTime)}))
	}
	if subject != "" {
		exps = append(exps, dbx.HashExp{"subject": subject})
	}
	if kind != "" {
		exps = append(exps, dbx.HashExp{"kind": kind})
	}
	if model != "" {
		exps = append(exps, dbx.HashExp{"model": model})
	}
	if len(exps) == 0 {
		return nil
	}
	return dbx.And(exps...)
}

// totalsColumns sums a group's rows; refused rows count only as refused.
const totalsColumns = `
	COALESCE(SUM(CASE WHEN [[refused]] = '' THEN 1 ELSE 0 END), 0) AS calls,
	COALESCE(SUM(CASE WHEN [[refused]] != '' THEN 1 ELSE 0 END), 0) AS refused,
	COALESCE(SUM([[input]]), 0) AS input, COALESCE(SUM([[cached]]), 0) AS cached, COALESCE(SUM([[output]]), 0) AS output,
	COALESCE(SUM([[seconds]]), 0) AS seconds, COALESCE(SUM([[cost]]), 0) AS cost`

type totalsRow struct {
	Key     string  `db:"key"`
	Calls   int64   `db:"calls"`
	Refused int64   `db:"refused"`
	Input   int64   `db:"input"`
	Cached  int64   `db:"cached"`
	Output  int64   `db:"output"`
	Seconds float64 `db:"seconds"`
	Cost    int64   `db:"cost"`
}

func (t totalsRow) totals() ledgerv1.Totals {
	return ledgerv1.Totals{Calls: t.Calls, Refused: t.Refused, Input: t.Input, Cached: t.Cached, Output: t.Output,
		Seconds: t.Seconds, CostMicroUSD: t.Cost}
}

func (s ledgerService) Spent(ctx context.Context, a extkit.Auth, in ledgerv1.SpentInput) (ledgerv1.Totals, error) {
	subject, err := subjectFor(ctx, a, in.Subject)
	if err != nil {
		return ledgerv1.Totals{}, err
	}
	from, to, err := span(in.Window, in.Since, in.Until)
	if err != nil {
		return ledgerv1.Totals{}, err
	}
	q := s.r.app.DB().Select(totalsColumns).From(LedgerCollection).WithContext(ctx)
	if w := where(from, to, subject, in.Kind, in.Model); w != nil {
		q = q.Where(w)
	}
	var row totalsRow
	if err := q.One(&row); err != nil {
		return ledgerv1.Totals{}, err
	}
	return row.totals(), nil
}

func (s ledgerService) Remaining(ctx context.Context, a extkit.Auth, in ledgerv1.RemainingInput) (ledgerv1.Remaining, error) {
	subject, err := subjectFor(ctx, a, in.Subject)
	if err != nil {
		return ledgerv1.Remaining{}, err
	}
	st, err := s.r.settings()
	if err != nil {
		return ledgerv1.Remaining{}, err
	}
	out := ledgerv1.Remaining{Limits: []ledgerv1.LimitLeft{}}
	now := time.Now()
	for _, l := range st.Limits {
		// the limits that count this subject's calls, whatever their kind or model
		c := &call{kind: l.Kind, model: l.Model, subject: subject}
		if !l.applies(c) {
			continue
		}
		since, next := windowStart(now, l.Window)
		used, err := s.r.used(ctx, l, c, since)
		if err != nil {
			return out, err
		}
		out.Limits = append(out.Limits, ledgerv1.LimitLeft{
			Limit: l.describe(), Scope: l.Scope, Window: l.Window, Metric: l.Metric, Kind: l.Kind, Model: l.Model,
			Amount: l.Amount, Used: used, Left: max(l.Amount-used, 0), Resets: next.Format(time.RFC3339),
		})
	}
	// tightest first: the smallest share left
	share := func(x ledgerv1.LimitLeft) float64 {
		if x.Amount <= 0 {
			return 0
		}
		return x.Left / x.Amount
	}
	sort.SliceStable(out.Limits, func(i, j int) bool { return share(out.Limits[i]) < share(out.Limits[j]) })
	return out, nil
}

func (s ledgerService) Summary(ctx context.Context, _ extkit.Auth, in ledgerv1.SummaryInput) (ledgerv1.Summary, error) {
	key := map[string]string{"": "[[model]]", "model": "[[model]]", "kind": "[[kind]]", "subject": "[[subject]]",
		"day": "substr([[created]], 1, 10)"}[in.By]
	if key == "" {
		return ledgerv1.Summary{}, badInput(`by must be "model", "kind", "subject" or "day"`)
	}
	from, to, err := span(in.Window, in.Since, in.Until)
	if err != nil {
		return ledgerv1.Summary{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, 1000)
	order := []string{"cost DESC", "calls DESC"}
	if in.By == "day" {
		order = []string{"key DESC"}
	}
	q := s.r.app.DB().Select(key+" AS key", totalsColumns).From(LedgerCollection).GroupBy("key").
		OrderBy(order...).Limit(int64(limit)).WithContext(ctx)
	if w := where(from, to, in.Subject, in.Kind, in.Model); w != nil {
		q = q.Where(w)
	}
	var rows []totalsRow
	if err := q.All(&rows); err != nil {
		return ledgerv1.Summary{}, err
	}
	out := ledgerv1.Summary{Rows: make([]ledgerv1.SummaryRow, 0, len(rows))}
	for _, r := range rows {
		out.Rows = append(out.Rows, ledgerv1.SummaryRow{Key: r.Key, Totals: r.totals()})
	}
	return out, nil
}

func (s ledgerService) Calls(ctx context.Context, _ extkit.Auth, in ledgerv1.CallsInput) (ledgerv1.CallList, error) {
	from, to, err := span(in.Window, in.Since, in.Until)
	if err != nil {
		return ledgerv1.CallList{}, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 500)
	q := s.r.app.RecordQuery(LedgerCollection).OrderBy("created DESC", "id DESC").
		Limit(int64(limit + 1)).Offset(int64(max(in.Offset, 0))).WithContext(ctx)
	if w := where(from, to, in.Subject, in.Kind, in.Model); w != nil {
		q = q.AndWhere(w)
	}
	switch strings.TrimSpace(in.Status) {
	case "":
	case "ok":
		q = q.AndWhere(dbx.HashExp{"refused": ""})
	case "refused":
		q = q.AndWhere(dbx.NewExp("[[refused]] != ''"))
	default:
		return ledgerv1.CallList{}, badInput(`status must be "ok" or "refused"`)
	}
	var recs []*core.Record
	if err := q.All(&recs); err != nil {
		return ledgerv1.CallList{}, err
	}
	out := ledgerv1.CallList{Calls: []ledgerv1.Call{}}
	if len(recs) > limit {
		out.More, recs = true, recs[:limit]
	}
	for _, r := range recs {
		out.Calls = append(out.Calls, ledgerv1.Call{
			ID: r.Id, Created: r.GetDateTime("created").Time().UTC().Format(time.RFC3339), Kind: r.GetString("kind"),
			Model: r.GetString("model"), Subject: r.GetString("subject"), Via: r.GetString("via"),
			Input: int64(r.GetInt("input")), Cached: int64(r.GetInt("cached")), Output: int64(r.GetInt("output")),
			Seconds: r.GetFloat("seconds"), CostMicroUSD: int64(r.GetInt("cost")), Estimated: r.GetBool("estimated"),
			Refused: r.GetString("refused"), Job: r.GetString("job"),
		})
	}
	return out, nil
}

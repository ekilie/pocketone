// Package ledgerv1 is the llm.ledger/v1 contract: what an app's model calls have used and cost, read from the ledger
// every llm.* call leaves (the provider's own record of each call), and how much of each limit is left.
//
// A wire contract at /api/_c/llm.ledger/v1. In pb_hooks it's $llm.ledger.
package ledgerv1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/extkit"
)

// Ledger reads the ledger.
//
// Remaining and Spent answer about one subject (whom calls were for: see llm.WithSubject). A superuser, an API token
// with the llm:read scope, and page code may name any subject; a signed-in user is answered about themselves only.
// Page code calling through $llm.for(e) gets the request's own subject when it names none.
//
//contract:id llm.ledger/v1
//contract:kind wire
//contract:scope llm:read
//contract:js hooks
type Ledger interface {
	// Remaining is, for each limit in Settings → LLM that counts the subject's calls, how much is used and left in
	// its current window, the tightest first.
	//
	//op: GET remaining scope=public
	Remaining(ctx context.Context, a extkit.Auth, in RemainingInput) (Remaining, error)
	// Spent is what the subject's calls used in a window.
	//
	//op: GET spent scope=public
	Spent(ctx context.Context, a extkit.Auth, in SpentInput) (Totals, error)
	// Summary is what calls used, grouped by model, kind, subject or day.
	//
	//op: GET summary
	Summary(ctx context.Context, a extkit.Auth, in SummaryInput) (Summary, error)
	// Calls lists the ledger's rows, newest first.
	//
	//op: GET calls
	Calls(ctx context.Context, a extkit.Auth, in CallsInput) (CallList, error)
}

// RemainingInput names the subject.
type RemainingInput struct {
	// Subject is whom to answer about ("user:users/abc", "guest:<id>", "team:acme"); empty means the caller's own
	// subject, or none, which only limits on the whole app count.
	Subject string `json:"subject,omitempty"`
}

// Remaining is the limits that count a subject's calls.
type Remaining struct {
	Limits []LimitLeft `json:"limits"`
}

// LimitLeft is one limit and how much of it is left.
type LimitLeft struct {
	// Limit is the limit in words: "each guest, $0.10 a day".
	Limit  string `json:"limit"`
	Scope  string `json:"scope"`  // app, user, guest or guests
	Window string `json:"window"` // day or month
	Metric string `json:"metric"` // cost, input, output or calls
	Kind   string `json:"kind,omitempty"`
	Model  string `json:"model,omitempty"`
	// Amount, Used and Left are in the metric's unit: US dollars for cost, else tokens or calls.
	Amount float64 `json:"amount"`
	Used   float64 `json:"used"`
	Left   float64 `json:"left"`
	// Resets is when the window ends (RFC 3339, UTC).
	Resets string `json:"resets"`
}

// SpentInput is a subject and a window, optionally narrowed to a kind or a model.
type SpentInput struct {
	// Subject: as for Remaining, except that empty with no caller's subject means every call (the whole app).
	Subject string `json:"subject,omitempty"`
	// Window is "day" or "month": the UTC calendar day or month holding now. Or give Since and Until (RFC 3339 or
	// YYYY-MM-DD, UTC; Until is exclusive). With none of them, all of the ledger (it keeps 13 months).
	Window string `json:"window,omitempty"`
	Since  string `json:"since,omitempty"`
	Until  string `json:"until,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Totals is what calls used.
type Totals struct {
	Calls        int64   `json:"calls"`   // calls that reached a provider
	Refused      int64   `json:"refused"` // calls a limit refused
	Input        int64   `json:"input"`
	Cached       int64   `json:"cached"`
	Output       int64   `json:"output"`
	Seconds      float64 `json:"seconds"`
	CostMicroUSD int64   `json:"costMicroUSD"` // millionths of a US dollar
}

// SummaryInput groups calls in a window.
type SummaryInput struct {
	// By is what to group by: "model" (the default), "kind", "subject" or "day".
	By string `json:"by,omitempty"`
	// Window is "day" or "month": the UTC calendar day or month holding now. Or give Since and Until (RFC 3339 or
	// YYYY-MM-DD, UTC; Until is exclusive). With none of them, all of the ledger (it keeps 13 months).
	Window  string `json:"window,omitempty"`
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
	Subject string `json:"subject,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Model   string `json:"model,omitempty"`
	// Limit is how many groups, the costliest first (by day: the latest first); default 100, at most 1000.
	Limit int `json:"limit,omitempty"`
}

// Summary is the groups.
type Summary struct {
	Rows []SummaryRow `json:"rows"`
}

// SummaryRow is one group's totals.
type SummaryRow struct {
	Key string `json:"key"` // the model, kind, subject or day (YYYY-MM-DD)
	Totals
}

// CallsInput filters and pages the rows.
type CallsInput struct {
	// Window is "day" or "month": the UTC calendar day or month holding now. Or give Since and Until (RFC 3339 or
	// YYYY-MM-DD, UTC; Until is exclusive). With none of them, all of the ledger (it keeps 13 months).
	Window  string `json:"window,omitempty"`
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
	Subject string `json:"subject,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Model   string `json:"model,omitempty"`
	// Status is "ok" for calls that reached a provider, "refused" for calls a limit refused, empty for both.
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"` // default 50, at most 500
	Offset int    `json:"offset,omitempty"`
}

// CallList is a page of rows.
type CallList struct {
	Calls []Call `json:"calls"`
	More  bool   `json:"more"` // there are more after this page
}

// Call is one row of the ledger.
type Call struct {
	ID           string  `json:"id"`
	Created      string  `json:"created"`
	Kind         string  `json:"kind"`
	Model        string  `json:"model"`
	Subject      string  `json:"subject"`
	Via          string  `json:"via"`
	Input        int64   `json:"input"`
	Cached       int64   `json:"cached"`
	Output       int64   `json:"output"`
	Seconds      float64 `json:"seconds"`
	CostMicroUSD int64   `json:"costMicroUSD"`
	Estimated    bool    `json:"estimated"`
	Refused      string  `json:"refused,omitempty"` // why a limit refused it
	Job          string  `json:"job,omitempty"`
}

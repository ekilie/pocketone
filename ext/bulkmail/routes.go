package bulkmail

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	bulkmailv1 "pocketone.dev/ext/contracts/bulkmail/v1"
	feedbackv1 "pocketone.dev/ext/contracts/mail/feedback/v1"
	suppressionv1 "pocketone.dev/ext/contracts/mail/suppression/v1"
	settingsv1 "pocketone.dev/ext/contracts/settings/v1"
	"pocketone.dev/ext/extkit"
)

// Bulk mail has no routes of its own: its API is the bulkmail/v1, mail.suppression/v1 and mail.feedback/v1
// contracts, and its settings are a settings/v1 section.
const (
	unsubscribePath = "/api/_c/mail.suppression/v1/unsubscribe"
	sesPath         = "/api/_c/mail.feedback/v1/ses"
	webhookPath     = "/api/_c/mail.feedback/v1/webhook"
)

// provide registers bulk mail's contract providers and its settings section on app.
func (bm *BulkMail) provide(app core.App) {
	bulkmailv1.Contract.Provide(app, campaigns{bm})
	// defaults: another extension can keep the suppression list or receive the reports instead
	suppressionv1.Contract.ProvideDefault(app, suppressions{bm})
	feedbackv1.Contract.ProvideDefault(app, receiver{bm})
	settingsv1.Add(app, settingsSection{bm})
	// feedback from any receiver suppresses the address
	feedbackv1.OnEvent(app).BindFunc(func(e *feedbackv1.Event) error {
		var err error
		switch e.Kind {
		case feedbackv1.Bounce:
			err = bm.Feedback(e.Email, ReasonBounced, e.Source, e.Campaign)
		case feedbackv1.Complaint:
			err = bm.Feedback(e.Email, ReasonComplained, e.Source, e.Campaign)
		case feedbackv1.Unsubscribe:
			err = bm.Unsubscribe(e.Email, e.Campaign, e.Source)
		}
		if err != nil {
			return err
		}
		return e.Next()
	})
}

func bad(err error) error {
	var ce *extkit.Error
	if errors.As(err, &ce) {
		return err
	}
	return &extkit.Error{Code: "bad_request", Message: err.Error(), Status: http.StatusBadRequest}
}

// --- bulkmail/v1 -----------------------------------------------------------------------------------------------

type campaigns struct{ bm *BulkMail }

func campaignOut(rec *core.Record, err error) (bulkmailv1.Campaign, error) {
	if err != nil {
		return bulkmailv1.Campaign{}, bad(err)
	}
	raw, err := json.Marshal(rec)
	return bulkmailv1.Campaign{Campaign: raw}, err
}

func (c campaigns) Count(_ context.Context, _ extkit.Auth, in bulkmailv1.CountInput) (bulkmailv1.CountResult, error) {
	a, err := ParseAudience([]byte(in.Audience))
	if err != nil {
		return bulkmailv1.CountResult{}, bad(err)
	}
	n, sup, sample, err := c.bm.CountAudience(a, 5)
	if err != nil {
		return bulkmailv1.CountResult{}, bad(err)
	}
	if sample == nil {
		sample = []string{}
	}
	return bulkmailv1.CountResult{Count: n, Suppressed: sup, Sample: sample}, nil
}

func (c campaigns) Start(_ context.Context, _ extkit.Auth, in bulkmailv1.StartInput) (bulkmailv1.Campaign, error) {
	return campaignOut(c.bm.StartLimit(in.Campaign, max(in.Limit, 0)))
}

func (c campaigns) Pause(_ context.Context, _ extkit.Auth, in bulkmailv1.Ref) (bulkmailv1.Campaign, error) {
	return campaignOut(c.bm.Pause(in.Campaign))
}

func (c campaigns) Resume(_ context.Context, _ extkit.Auth, in bulkmailv1.Ref) (bulkmailv1.Campaign, error) {
	return campaignOut(c.bm.Resume(in.Campaign))
}

func (c campaigns) Cancel(_ context.Context, _ extkit.Auth, in bulkmailv1.Ref) (bulkmailv1.Campaign, error) {
	return campaignOut(c.bm.Cancel(in.Campaign))
}

func (c campaigns) Preview(_ context.Context, _ extkit.Auth, in bulkmailv1.PreviewInput) (bulkmailv1.Preview, error) {
	msg, err := c.bm.Preview(in.Campaign, in.Email)
	if err != nil {
		return bulkmailv1.Preview{}, bad(err)
	}
	p := bulkmailv1.Preview{From: msg.From.String(), Subject: msg.Subject, HTML: msg.HTML, Text: msg.Text, Headers: msg.Headers}
	if len(msg.To) > 0 {
		p.To = msg.To[0].Address
	}
	return p, nil
}

func (c campaigns) Test(_ context.Context, _ extkit.Auth, in bulkmailv1.TestInput) (bulkmailv1.Sent, error) {
	if err := c.bm.Test(in.Campaign, in.To); err != nil {
		return bulkmailv1.Sent{}, bad(err)
	}
	return bulkmailv1.Sent{Sent: true}, nil
}

// --- mail.suppression/v1 ---------------------------------------------------------------------------------------

type suppressions struct{ bm *BulkMail }

func (s suppressions) status(email string) (suppressionv1.Status, error) {
	st := suppressionv1.Status{Email: NormalizeEmail(email)}
	if st.Email == "" {
		return st, bad(errors.New("invalid email"))
	}
	rec, err := s.bm.app.FindFirstRecordByData(SuppressionsId, "email", st.Email)
	if err != nil {
		return st, nil
	}
	since := rec.GetDateTime("created").Time()
	st.Suppressed, st.Reason, st.Since = true, rec.GetString("reason"), &since
	return st, nil
}

func (s suppressions) Check(_ context.Context, _ extkit.Auth, in suppressionv1.Address) (suppressionv1.Status, error) {
	return s.status(in.Email)
}

func (s suppressions) Suppress(_ context.Context, _ extkit.Auth, in suppressionv1.SuppressInput) (suppressionv1.Status, error) {
	reason := in.Reason
	if reason == "" {
		reason = ReasonManual
	}
	source := in.Source
	if source == "" {
		source = "api"
	}
	if _, err := s.bm.Suppress(in.Email, reason, source, in.Campaign); err != nil {
		return suppressionv1.Status{}, bad(err)
	}
	return s.status(in.Email)
}

func (s suppressions) Unsuppress(_ context.Context, _ extkit.Auth, in suppressionv1.Address) (suppressionv1.Status, error) {
	if err := s.bm.Unsuppress(in.Email); err != nil {
		return suppressionv1.Status{}, err
	}
	return s.status(in.Email)
}

func htmlOut(status int, title, body string) extkit.RawOut {
	return extkit.RawOut{Body: io.NopCloser(strings.NewReader(page(title, body))), ContentType: "text/html; charset=utf-8", Status: status}
}

func (s suppressions) UnsubscribePage(_ context.Context, _ extkit.Auth, in suppressionv1.Link) (extkit.RawOut, error) {
	email, _, err := s.bm.ParseUnsubscribeToken(in.T)
	if err != nil {
		return htmlOut(http.StatusBadRequest, "Invalid link", "<p>This unsubscribe link isn't valid.</p>"), nil
	}
	if sup, _ := s.bm.IsSuppressed(email); sup {
		return htmlOut(http.StatusOK, "Unsubscribed", "<p>"+html.EscapeString(email)+" won't receive these emails anymore.</p>"), nil
	}
	// a GET never unsubscribes: link scanners open every URL in a message
	return htmlOut(http.StatusOK, "Unsubscribe",
		`<p>Stop sending these emails to <b>`+html.EscapeString(email)+`</b>?</p>`+
			`<form method="post"><input type="hidden" name="t" value="`+html.EscapeString(in.T)+`">`+
			`<button type="submit">Unsubscribe</button></form>`), nil
}

// Unsubscribe serves both the confirm form and RFC 8058 one-click POSTs from mail providers.
func (s suppressions) Unsubscribe(_ context.Context, _ extkit.Auth, in extkit.RawIn) (extkit.RawOut, error) {
	token := in.Query.Get("t")
	oneClick := false
	if in.Body != nil && strings.HasPrefix(in.ContentType, "application/x-www-form-urlencoded") {
		raw, _ := io.ReadAll(io.LimitReader(in.Body, 1<<14))
		if form, err := url.ParseQuery(string(raw)); err == nil {
			if t := form.Get("t"); t != "" {
				token = t
			}
			oneClick = form.Get("List-Unsubscribe") == "One-Click"
		}
	}
	email, campaignId, err := s.bm.ParseUnsubscribeToken(token)
	if err != nil {
		return htmlOut(http.StatusBadRequest, "Invalid link", "<p>This unsubscribe link isn't valid.</p>"), nil
	}
	source := "link"
	if oneClick {
		source = "one-click"
	}
	if err := s.bm.Unsubscribe(email, campaignId, source); err != nil {
		return extkit.RawOut{}, err
	}
	return htmlOut(http.StatusOK, "Unsubscribed", "<p>"+html.EscapeString(email)+" won't receive these emails anymore.</p>"), nil
}

// --- mail.feedback/v1 ------------------------------------------------------------------------------------------

type receiver struct{ bm *BulkMail }

func (r receiver) SES(_ context.Context, _ extkit.Auth, in extkit.RawIn) (feedbackv1.Ack, error) {
	var m SNSMessage
	if in.Body == nil || json.NewDecoder(io.LimitReader(in.Body, 256<<10)).Decode(&m) != nil {
		return feedbackv1.Ack{}, &extkit.Error{Code: "bad_request", Message: "invalid SNS message", Status: 400}
	}
	n, err := r.bm.handleSNS(&m)
	if err != nil {
		r.bm.app.Logger().Warn("bulkmail: SES feedback rejected", "error", err)
		return feedbackv1.Ack{}, &extkit.Error{Code: "rejected", Message: "rejected", Status: 400}
	}
	return feedbackv1.Ack{Accepted: n}, nil
}

func (r receiver) Webhook(_ context.Context, _ extkit.Auth, in extkit.RawIn) (feedbackv1.Ack, error) {
	s, err := r.bm.Settings()
	if err != nil {
		return feedbackv1.Ack{}, err
	}
	given := ""
	if in.Header != nil {
		given = in.Header.Get("X-Feedback-Secret")
	}
	if s.WebhookSecret == "" || subtle.ConstantTimeCompare([]byte(given), []byte(s.WebhookSecret)) != 1 {
		return feedbackv1.Ack{}, feedbackv1.ErrBadSecret
	}
	var body struct {
		Type     string `json:"type"`
		Email    string `json:"email"`
		Campaign string `json:"campaign"`
	}
	if in.Body == nil || json.NewDecoder(io.LimitReader(in.Body, 1<<14)).Decode(&body) != nil {
		return feedbackv1.Ack{}, &extkit.Error{Code: "bad_request", Message: "invalid body", Status: 400}
	}
	kind := strings.ToLower(body.Type)
	switch kind {
	case feedbackv1.Bounce, feedbackv1.Complaint, feedbackv1.Unsubscribe:
	default:
		return feedbackv1.Ack{}, &extkit.Error{Code: "bad_request", Message: "type must be bounce, complaint or unsubscribe", Status: 400}
	}
	if err := feedbackv1.Raise(r.bm.app, &feedbackv1.Event{Kind: kind, Email: body.Email, Campaign: body.Campaign, Source: "webhook"}); err != nil {
		return feedbackv1.Ack{}, bad(err)
	}
	return feedbackv1.Ack{Accepted: 1}, nil
}

// --- settings/v1 -----------------------------------------------------------------------------------------------

type settingsSection struct{ bm *BulkMail }

type settingsResponse struct {
	Settings
	HasWebhookSecret bool    `json:"hasWebhookSecret"`
	Rate             float64 `json:"rate"`
	Daily            int     `json:"daily"`
	SentToday        int     `json:"sentToday"`
	Allowed          bool    `json:"allowed"`
	SESEndpoint      string  `json:"sesEndpoint"`
	WebhookEndpoint  string  `json:"webhookEndpoint"`
}

func (s settingsSection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "Bulk mail",
		Description: "Sender, pace and where mail providers report bounces and complaints."}
}

func (s settingsSection) Get(context.Context) (json.RawMessage, error) {
	bm := s.bm
	cur, err := bm.Settings()
	if err != nil {
		return nil, err
	}
	rate, daily, sent, err := bm.Limits()
	if err != nil {
		return nil, err
	}
	res := settingsResponse{
		Settings:         cur,
		HasWebhookSecret: cur.WebhookSecret != "",
		Rate:             rate,
		Daily:            daily,
		SentToday:        sent,
		Allowed:          daily != NoBulkMail,
		SESEndpoint:      bm.publicURL() + sesPath,
		WebhookEndpoint:  bm.publicURL() + webhookPath,
	}
	res.Secret, res.WebhookSecret = "", ""
	return json.Marshal(res)
}

func (s settingsSection) Save(ctx context.Context, values json.RawMessage) (json.RawMessage, error) {
	cur, err := s.bm.Settings()
	if err != nil {
		return nil, err
	}
	cur.Secret, cur.WebhookSecret = "", "" // empty = keep
	if err := json.Unmarshal(values, &cur); err != nil {
		return nil, &extkit.Error{Code: "bad_request", Message: "invalid settings", Status: 400}
	}
	if err := s.bm.SaveSettings(cur); err != nil {
		return nil, err
	}
	return s.Get(ctx)
}

func page(title, body string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>` + html.EscapeString(title) + `</title><style>body{font:16px/1.5 system-ui,sans-serif;max-width:32rem;margin:15vh auto;padding:0 1rem;color:#222}` +
		`button{font:inherit;padding:.5rem 1.25rem;border:0;border-radius:6px;background:#222;color:#fff;cursor:pointer}</style></head><body>` +
		`<h1>` + html.EscapeString(title) + `</h1>` + body + `</body></html>`
}

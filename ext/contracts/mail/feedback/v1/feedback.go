// Package feedbackv1 is the mail.feedback/v1 contract: what mail providers report back about mail an app sent
// (bounces, complaints, unsubscribes), received at one place and turned into [Event]s that any extension can act on.
// The provider receives the reports (Amazon SES through SNS, or a generic webhook); handlers bind [OnEvent]: a
// suppression list adds the address, a CRM marks the contact.
//
// A wire contract at /api/_c/mail.feedback/v1. Its operations are public: SES messages are checked by their SNS
// signature, webhook calls by a shared secret.
package feedbackv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Receiver takes mail providers' reports.
//
//contract:id mail.feedback/v1
//contract:kind wire
//contract:scope public
//contract:js hooks
type Receiver interface {
	// SES takes an Amazon SNS delivery carrying SES bounce and complaint notifications (and confirms the topic
	// subscription). Point an SNS HTTPS subscription here.
	//
	//op: POST ses scope=public
	SES(ctx context.Context, a extkit.Auth, in extkit.RawIn) (Ack, error)

	// Webhook takes {"type": "bounce|complaint|unsubscribe", "email": ..., "campaign": ...} from any provider, with
	// the shared secret in the X-Feedback-Secret header.
	//
	//op: POST webhook scope=public
	Webhook(ctx context.Context, a extkit.Auth, in extkit.RawIn) (Ack, error)
}

// Ack is a receipt.
type Ack struct {
	Accepted int `json:"accepted"` // events raised
}

// Kinds of feedback.
const (
	Bounce      = "bounce"      // permanent: the address doesn't exist
	Complaint   = "complaint"   // the recipient marked it as spam
	Unsubscribe = "unsubscribe" // the recipient asked to stop
)

// Event is one report about one address.
type Event struct {
	hook.Event
	App      core.App
	Kind     string // Bounce, Complaint or Unsubscribe
	Email    string
	Campaign string // the bulk mail campaign it was about, when known (X-Bulkmail-Campaign)
	Source   string // ses, webhook, ...
}

// ErrBadSecret: the webhook's secret is missing or wrong.
var ErrBadSecret = &extkit.Error{Code: "unauthorized", Message: "missing or wrong X-Feedback-Secret", Status: 401}

const hookKey = "mail.feedback/v1.hook"

// OnEvent is the hook every feedback event goes through, for any number of handlers.
func OnEvent(app core.App) *hook.Hook[*Event] {
	return app.Store().GetOrSet(hookKey, func() any { return &hook.Hook[*Event]{} }).(*hook.Hook[*Event])
}

// Raise sends ev through [OnEvent] (providers call it for each report).
func Raise(app core.App, ev *Event) error {
	ev.App = app
	return OnEvent(app).Trigger(ev)
}

// Package suppressionv1 is the mail.suppression/v1 contract: addresses an app must not send bulk mail to
// (unsubscribed, bounced, complained), and the unsubscribe link every bulk message carries. Anything that sends to
// many people checks it; whatever provides it keeps the list.
//
// A wire contract at /api/_c/mail.suppression/v1. The unsubscribe operations are public: the link in a message is
// signed by the provider, and mail providers POST to it for one-click unsubscribes (RFC 8058).
package suppressionv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"
	"time"

	"github.com/pockethost/pocketone/ext/extkit"
)

// List is an app's suppression list.
//
//contract:id mail.suppression/v1
//contract:kind wire
//contract:scope suppression
//contract:js hooks
type List interface {
	// Check says whether an address is suppressed, and why.
	//
	//op: GET check scope=suppression:read
	Check(ctx context.Context, a extkit.Auth, in Address) (Status, error)

	// Suppress adds an address (or changes its reason).
	//
	//op: POST suppress
	Suppress(ctx context.Context, a extkit.Auth, in SuppressInput) (Status, error)

	// Unsuppress removes an address, so it receives bulk mail again.
	//
	//op: POST unsuppress
	Unsuppress(ctx context.Context, a extkit.Auth, in Address) (Status, error)

	// UnsubscribePage is the page an unsubscribe link opens (?t=<signed token>). It never unsubscribes by itself: link
	// scanners open every URL in a message.
	//
	//op: GET unsubscribe scope=public
	UnsubscribePage(ctx context.Context, a extkit.Auth, in Link) (extkit.RawOut, error)

	// Unsubscribe confirms the page's form, or a mail provider's one-click POST (List-Unsubscribe=One-Click).
	//
	//op: POST unsubscribe scope=public
	Unsubscribe(ctx context.Context, a extkit.Auth, in extkit.RawIn) (extkit.RawOut, error)
}

// Reasons an address is suppressed.
const (
	Unsubscribed = "unsubscribed"
	Bounced      = "bounced"
	Complained   = "complained"
	Manual       = "manual"
)

// Link is an unsubscribe link's signed token (?t=).
type Link struct {
	T string `json:"t"`
}

// Address is one email address.
type Address struct {
	Email string `json:"email"`
}

// SuppressInput suppresses an address.
type SuppressInput struct {
	Email    string `json:"email"`
	Reason   string `json:"reason"`             // Unsubscribed, Bounced, Complained or Manual
	Source   string `json:"source,omitempty"`   // where it came from: link, one-click, ses, webhook, ...
	Campaign string `json:"campaign,omitempty"` // the campaign that caused it, if any
}

// Status is an address's place on the list.
type Status struct {
	Email      string     `json:"email"`
	Suppressed bool       `json:"suppressed"`
	Reason     string     `json:"reason,omitempty"`
	Since      *time.Time `json:"since,omitempty"`
}

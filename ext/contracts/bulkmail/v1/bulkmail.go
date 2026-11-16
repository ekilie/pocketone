// Package bulkmailv1 is the bulkmail/v1 contract: campaigns to many recipients, sent at a safe pace. Campaigns are
// records in the provider's collection (created and edited with PocketBase's records API); this contract runs them:
// count an audience, start, pause, resume, cancel, preview and test a campaign. Unsubscribes and bounces are
// mail.suppression/v1 and mail.feedback/v1.
//
// A wire contract at /api/_c/bulkmail/v1.
package bulkmailv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"
	"encoding/json"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Campaigns runs an app's bulk mail campaigns.
//
//contract:id bulkmail/v1
//contract:kind wire
//contract:scope bulkmail
//contract:js hooks
type Campaigns interface {
	// Count says how many recipients an audience reaches, how many of them are suppressed, and a few sample addresses.
	//
	//op: POST count scope=bulkmail:read
	Count(ctx context.Context, a extkit.Auth, in CountInput) (CountResult, error)

	// Start queues a campaign's recipients and starts sending (Limit > 0: at most that many new recipients).
	//
	//op: POST start
	Start(ctx context.Context, a extkit.Auth, in StartInput) (Campaign, error)

	// Pause stops sending a campaign; Resume carries on.
	//
	//op: POST pause
	Pause(ctx context.Context, a extkit.Auth, in Ref) (Campaign, error)

	// Resume carries on sending a paused campaign.
	//
	//op: POST resume
	Resume(ctx context.Context, a extkit.Auth, in Ref) (Campaign, error)

	// Cancel stops a campaign for good; recipients not yet sent to are dropped.
	//
	//op: POST cancel
	Cancel(ctx context.Context, a extkit.Auth, in Ref) (Campaign, error)

	// Preview renders a campaign's message for one recipient (the first in its audience when Email is empty).
	//
	//op: POST preview scope=bulkmail:read
	Preview(ctx context.Context, a extkit.Auth, in PreviewInput) (Preview, error)

	// Test sends a campaign's message to one address, marked as a test.
	//
	//op: POST test
	Test(ctx context.Context, a extkit.Auth, in TestInput) (Sent, error)
}

// CountInput is an audience, in the provider's audience shape (see its README).
type CountInput struct {
	Audience json.RawMessage `json:"audience"`
}

// CountResult is an audience's size.
type CountResult struct {
	Count      int      `json:"count"`
	Suppressed int      `json:"suppressed"`
	Sample     []string `json:"sample"`
}

// Ref names a campaign (its record id).
type Ref struct {
	Campaign string `json:"campaign"`
}

// StartInput starts a campaign.
type StartInput struct {
	Campaign string `json:"campaign"`
	Limit    int    `json:"limit,omitempty"`
}

// Campaign is the campaign's record after the action, as PocketBase's records API returns it.
type Campaign struct {
	Campaign json.RawMessage `json:"campaign"`
}

// PreviewInput picks the recipient to render for.
type PreviewInput struct {
	Campaign string `json:"campaign"`
	Email    string `json:"email,omitempty"`
}

// Preview is a rendered message.
type Preview struct {
	To      string            `json:"to"`
	From    string            `json:"from"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html"`
	Text    string            `json:"text"`
	Headers map[string]string `json:"headers"`
}

// TestInput is where a test message goes.
type TestInput struct {
	Campaign string `json:"campaign"`
	To       string `json:"to"`
}

// Sent says a message went out.
type Sent struct {
	Sent bool `json:"sent"`
}

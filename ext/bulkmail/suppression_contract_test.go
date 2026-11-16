package bulkmail

import (
	"context"
	"testing"

	suppressionv1 "github.com/pockethost/pocketone/ext/contracts/mail/suppression/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// otherList is another extension's suppression list: it blocks one address, and knows nothing of bulk mail's table.
type otherList struct{ blocked string }

func (o otherList) Check(_ context.Context, _ extkit.Auth, in suppressionv1.Address) (suppressionv1.Status, error) {
	return suppressionv1.Status{Email: in.Email, Suppressed: in.Email == o.blocked}, nil
}
func (otherList) Suppress(context.Context, extkit.Auth, suppressionv1.SuppressInput) (suppressionv1.Status, error) {
	return suppressionv1.Status{}, nil
}
func (otherList) Unsuppress(context.Context, extkit.Auth, suppressionv1.Address) (suppressionv1.Status, error) {
	return suppressionv1.Status{}, nil
}
func (otherList) UnsubscribePage(context.Context, extkit.Auth, suppressionv1.Link) (extkit.RawOut, error) {
	return extkit.RawOut{}, nil
}
func (otherList) Unsubscribe(context.Context, extkit.Auth, extkit.RawIn) (extkit.RawOut, error) {
	return extkit.RawOut{}, nil
}

// When another extension provides mail.suppression/v1, bulk mail asks it, not its own table.
func TestAnotherSuppressionListDecides(t *testing.T) {
	app, bm := newApp(t, Config{})
	users := verifiedUsers(t, app)
	blocked := NormalizeEmail(users[0].Email())
	if _, err := bm.Suppress(users[1].Email(), ReasonManual, "test", ""); err != nil { // only in bulk mail's own table
		t.Fatal(err)
	}
	suppressionv1.Contract.Provide(app, otherList{blocked: blocked}) // bulk mail's own list is the default: this wins

	if p, ok := suppressionv1.Contract.Provider(app); !ok || p == Name {
		t.Fatalf("provider: %q %v", p, ok)
	}
	n, sup, _, err := bm.CountAudience(Audience{Collection: "users", Filter: "verified = true", EmailField: "email"}, 0)
	if err != nil || sup != 1 || n != len(users)-1 {
		t.Fatalf("count %d, suppressed %d (%v); want %d and 1", n, sup, err, len(users)-1)
	}
	if !bm.suppressedNow(blocked) || bm.suppressedNow(NormalizeEmail(users[1].Email())) {
		t.Fatal("suppressedNow should follow the other list")
	}
}

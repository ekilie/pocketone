// Package fallbackv1 is the mail.fallback/v1 contract: where an app's mail goes when its SMTP is off. Providers form
// an ordered chain (`providers: { mail.fallback/v1: [mailtrap, mailrelay] }` sets the order): each sends a message or
// passes it on with [ErrPass]; when every provider passes, PocketBase sends it its own way (sendmail).
//
// Providers: mailtrap (dev mode: a local inbox), mailrelay (a multisite host's relay), or an app's own pb_hooks. Each
// provider calls [Bind] when it registers; Bind owns the app's mail hook, once.
package fallbackv1

//go:generate go run github.com/pockethost/pocketone/ext/contracts/cmd/contractgen

import (
	"context"
	"errors"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/hook"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Sender delivers an app's mail when its SMTP is off.
//
//contract:id mail.fallback/v1
//contract:cardinality many
//contract:js open
type Sender interface {
	// Send delivers m, or returns ErrPass to leave it to the next provider.
	Send(ctx context.Context, m *mailer.Message) error
}

// ErrPass is a provider's "not mine": the next provider (or PocketBase) sends the message.
var ErrPass = errors.New("mail.fallback: passed")

const boundKey = "mail.fallback/v1.bound"

// Bind makes app's mail go through mail.fallback/v1's providers whenever its SMTP is off. Providers call it in
// Register; it binds the hook once per app, last (the app's own onMailerSend hooks run, and may change the message,
// first).
func Bind(app core.App) {
	if app.Store().Has(boundKey) {
		return
	}
	app.Store().Set(boundKey, true)
	app.OnMailerSend().Bind(&hook.Handler[*core.MailerEvent]{
		Id:       "__mail.fallback/v1__",
		Priority: 1 << 30,
		Func: func(e *core.MailerEvent) error {
			if e.App.Settings().SMTP.Enabled {
				return e.Next() // the app's own SMTP wins
			}
			for _, s := range Contract.All(e.App) {
				err := s.Send(context.Background(), e.Message)
				var ce *extkit.Error
				if errors.Is(err, ErrPass) || (errors.As(err, &ce) && ce.Code == "pass") { // pb_hooks: throw new ContractError("pass")
					continue
				}
				return err // never e.Next() once a provider took it: that would run sendmail too
			}
			return e.Next()
		},
	})
}

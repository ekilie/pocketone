// Package mailtrap catches an app's mail in development: in dev mode, an app whose SMTP is off sends to a built-in
// mail catcher (like MailDev or Mailpit) instead of the host's sendmail, and you read it at http://127.0.0.1:8025.
//
// It provides mail.fallback/v1 (ext/contracts): with SMTP off, the app's mail goes to the contract's providers in
// order; in dev mode mailtrap takes it, so sendmail never runs. Outside dev mode it passes the message on.
//
// A single app:
//
//	mailtrap.MustRegister(app)
//
// Under pocker, add [New] to Server.Extensions; every dev instance's mail lands in the one inbox, labeled by instance.
//
// The inbox listens on 127.0.0.1:8025 (SMTP 127.0.0.1:1025); P1_MAILTRAP_HTTP and P1_MAILTRAP_SMTP move it.
package mailtrap

import (
	"context"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/mailer"
	fallbackv1 "github.com/pockethost/pocketone/ext/contracts/mail/fallback/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's name (extkit / pocker config key).
const Name = "mailtrap"

const reason = "No SMTP configured"

// Extension is the extkit form, for pocker's Server.Extensions and extkit.MustRegister.
type Extension struct{}

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements extkit.Extension.
func (p *Extension) Name() string { return Name }

// Register implements extkit.Extension.
func (p *Extension) Register(app core.App, _ extkit.Config) error {
	Register(app)
	return nil
}

// MustRegister routes app's mail to the dev mail trap while the app is in dev mode with SMTP off.
func MustRegister(app core.App) { Register(app) }

// Contracts implements [extkit.Declarer].
func (p *Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("mail.fallback/v1")}
}

// trap is mail.fallback/v1 in dev mode: the mail lands in the local inbox. Outside dev mode it passes.
type trap struct{ app core.App }

func (t trap) Send(_ context.Context, m *mailer.Message) error {
	if !t.app.IsDev() {
		return fallbackv1.ErrPass
	}
	tr, _ := Default(reason) // if its ports are taken it still keeps mail in memory and prints each one
	_, err := tr.AddMessage(m, t.app.Settings().Meta.AppName)
	return err
}

// Register is MustRegister; it can't fail.
func Register(app core.App) {
	// at startup, so the inbox address shows before the first mail
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		if active(e.App) {
			_, _ = Default(reason)
		}
		return nil
	})
	// mail.fallback/v1: in dev mode, mail lands here instead of sendmail
	fallbackv1.Contract.Provide(app, trap{app})
	fallbackv1.Bind(app)
}

func active(app core.App) bool {
	return app.IsDev() && !app.Settings().SMTP.Enabled
}

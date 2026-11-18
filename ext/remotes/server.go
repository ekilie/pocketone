package remotes

import (
	remoteclient "github.com/pockethost/pocketone/ext/contracts/remote/v1/remoteclient"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	remotev1 "github.com/pockethost/pocketone/ext/contracts/remote/v1"
)

// WhoamiPath is the discovery route: remote/v1's root.
const WhoamiPath = remoteclient.WhoamiPath

// Config for the server half.
type Config struct {
	// Restart restarts the app for remote/v1's restart (default app.Restart(); for an isolate, as under Pocker,
	// that restarts only this app). Tests replace it.
	Restart func(app core.App) error
}

// MustRegister is [Register] that panics on error.
func MustRegister(app core.App, cfg Config) {
	if err := Register(app, cfg); err != nil {
		panic(err)
	}
}

// Register provides remote/v1 on app (whoami at /api/_c/remote/v1, status, restart, logs) and, on a
// *pocketbase.PocketBase, adds the remote, login, logout and link commands.
func Register(app core.App, cfg Config) error {
	// remote/v1 (whoami at /api/_c/remote/v1, status, restart, logs); the contracts' seal mounts its routes
	remotev1.Contract.ProvideDefault(app, &remote{app: app, restart: cfg.Restart})
	if pb, ok := app.(*pocketbase.PocketBase); ok {
		for _, c := range Commands() {
			pb.RootCmd.AddCommand(c)
		}
	}
	return nil
}

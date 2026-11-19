package mailrelay

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	settingsv1 "pocketone.dev/ext/contracts/settings/v1"
)

//go:embed ui
var uiFiles embed.FS

// Status is the relay as one app sees it: its read-only settings/v1 section (`?extension=mailrelay`), which Settings
// → Mail shows.
type Status struct {
	// Active is true when this instance's mail goes through the relay (SMTP off and sending allowed for it).
	// Send is false when the instance's `send: false` refuses its mail.
	Active    bool   `json:"active"`
	Send      bool   `json:"send"`
	Address   string `json:"address"`
	PerMinute int    `json:"perMinute"`
	Daily     int    `json:"daily"`
	SentToday int    `json:"sentToday"`
}

func (r *appRelay) status(app core.App) Status {
	l := r.limits
	return Status{
		Active:    l.send && !app.Settings().SMTP.Enabled,
		Send:      l.send,
		Address:   r.Address(r.id),
		PerMinute: l.perMinute,
		Daily:     l.daily,
		SentToday: r.sentToday(r.id),
	}
}

// bindRoutes adds the relay's read-only settings/v1 section and its Settings → Mail note; it has no routes of its own.
func (r *appRelay) bindRoutes(app core.App) {
	settingsv1.Add(app, relaySection{r})
	ui, _ := fs.Sub(uiFiles, "ui")
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.UIExtensions = append(se.UIExtensions, core.UIExtension{Name: Name, FS: ui})
		return se.Next()
	})
}

// relaySection is the relay's settings/v1 section: read-only, the host sets it.
type relaySection struct{ r *appRelay }

func (s relaySection) Info() settingsv1.Info {
	return settingsv1.Info{Extension: Name, Title: "Mail relay", Description: "Where mail goes while SMTP is off; the host sets it."}
}

func (s relaySection) Get(context.Context) (json.RawMessage, error) {
	return json.Marshal(s.r.status(s.r.app))
}

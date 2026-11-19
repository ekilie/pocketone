package mirror

import (
	"fmt"
	"log/slog"
	"pocketone.dev/ext/extkit"
	"time"

	"pocketone.dev/ext/edge"
)

// FileConfig is pocketone.yaml's `mothership` section (`$VAR` values come from the private .env through ext/config).
// self falls back to the `edges` section's self.
//
//	mothership:
//	  url: http://10.88.0.10:8090
//	  email: $MOTHERSHIP_EMAIL
//	  password: $MOTHERSHIP_PASSWORD
//	  self: lax-1
//	  internal_url: http://10.88.0.5:8080   # registers this edge when the mothership has no record for it
//	  region: lax
//	  apex: example.com                     # <name>.example.com routes to each instance
//	  heartbeat: 15s
//	  resync: 10m
type FileConfig struct {
	URL         string        `yaml:"url" doc:"The mothership's URL. Required."`
	Email       string        `yaml:"email" doc:"Superuser email this edge logs in with; use a $VAR from the .env."`
	Password    string        `yaml:"password" doc:"Superuser password; use a $VAR from the .env."`
	Self        string        `yaml:"self" doc:"This edge's edge_id, such as lax-1. Default: the edges section's self. Required."`
	InternalURL string        `yaml:"internal_url" doc:"Where other edges reach this one. Set: the edge registers itself when the mothership has no record for it."`
	Region      string        `yaml:"region" doc:"Region recorded when the edge registers itself."`
	Apex        string        `yaml:"apex" doc:"Makes <name>.<apex> a hostname of every instance. Empty: hosts only."`
	Heartbeat   time.Duration `yaml:"heartbeat" doc:"How often this edge marks itself online. Default 15s; negative: never."`
	Resync      time.Duration `yaml:"resync" doc:"How often everything is reloaded, in case a change was missed. Default 10m."`
}

// Section is the key of the `mothership` section (pocketone.yaml): a community mothership this host mirrors its
// instances and the other edges from.
var Section = extkit.NewSection("mothership", FileConfig{}, "A community mothership to mirror instances and edges from: url, login, this edge's id, internal URL, region, apex.")

// FromSection returns a mirror for the `mothership` section, or nil when the config has none. Extensions are added to
// the mirror's.
func FromSection(log *slog.Logger, exts ...Extension) (*Mirror, error) {
	fc, found := Section.Get()
	if !found {
		return nil, nil
	}
	if fc.Self == "" {
		e, _ := edge.Section.Get()
		fc.Self = e.Self
	}
	cfg := Config{
		URL: fc.URL, Email: fc.Email, Password: fc.Password, Self: fc.Self, InternalURL: fc.InternalURL,
		Region: fc.Region, Apex: fc.Apex, Heartbeat: fc.Heartbeat, Resync: fc.Resync, Extensions: exts, Logger: log,
	}
	if cfg.URL == "" || cfg.Self == "" {
		return nil, fmt.Errorf("mothership: url and self are required")
	}
	return New(cfg), nil
}

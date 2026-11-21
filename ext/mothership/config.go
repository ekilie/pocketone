package mothership

import (
	"time"

	"pocketone.dev/ext/extkit"
)

// FileConfig is pocketone.yaml's `mothership` section. On the mothership, `serve: true` turns on the watch server
// (and, in the stock binary, the default schema). On an edge, url and self name the mothership to follow (`$VAR`
// values come from the private .env through ext/config).
//
//	# the mothership (a single app)
//	mothership:
//	  serve: true
//	  apex: example.com                    # <name>.example.com routes to each instance
//
//	# an edge (a multisite host)
//	mothership:
//	  url: http://10.88.0.10:8090
//	  email: $MOTHERSHIP_EMAIL
//	  password: $MOTHERSHIP_PASSWORD
//	  self: lax-1
//	  internal_url: http://10.88.0.5:8080   # registers this edge when the mothership has no record for it
//	  region: lax
type FileConfig struct {
	Serve bool   `yaml:"serve" doc:"This app is the mothership: it serves instance.watch/v1 to its edges (and keeps the edges and instances collections)."`
	Apex  string `yaml:"apex" doc:"On the mothership: makes <name>.<apex> a hostname of every instance. Empty: hosts only."`

	URL         string        `yaml:"url" doc:"On an edge: the mothership's URL. Required there."`
	Email       string        `yaml:"email" doc:"Superuser email this edge logs in with; use a $VAR from the .env."`
	Password    string        `yaml:"password" doc:"Superuser password; use a $VAR from the .env."`
	Self        string        `yaml:"self" doc:"This edge's edge_id, such as lax-1. Default: the edges section's self. Required on an edge."`
	InternalURL string        `yaml:"internal_url" doc:"Where other edges reach this one. Set: the edge registers itself when the mothership has no record for it."`
	Region      string        `yaml:"region" doc:"Region recorded when the edge registers itself."`
	Heartbeat   time.Duration `yaml:"heartbeat" doc:"How often this edge marks itself online. Default 15s; negative: never."`
	MaxHosts    int           `yaml:"max_hosts" doc:"Most hostnames an edge keeps answers for (its cache bucket mothership.hosts). Default 50000."`
}

// Section is the key of the `mothership` section (pocketone.yaml).
var Section = extkit.NewSection("mothership", FileConfig{}, "A community mothership: serve: true makes this app one; url, login and self make a host follow one.")

package standard

import "pocketone.dev/ext/extkit"

// Switch is the pocketone.yaml section of a standard member that has no section of its own: `api-tokens: {enabled: false}`
// turns api-tokens off on a single app, and extensions that require it turn off too.
type Switch struct {
	Enabled *bool `yaml:"enabled" doc:"On unless false. Extensions that require it turn off too, and the startup lines say why."`
}

// switchMembers are the standard members without a section of their own. Members with one (cache, storage, s3redirect, pitr)
// read `enabled` there themselves.
var switchMembers = []string{"settings", "templates", "api-tokens", "remotes", "mcp", "pages", "backups", "bulkmail", "deploy", "wasm", "uicache", "agent-chat"}

// switches are their sections, registered at init so the config file's check knows them before the set is built.
var switches = map[string]*extkit.Section[Switch]{}

func init() {
	for _, name := range switchMembers {
		if extkit.IsSection(name) { // the member got a section of its own
			continue
		}
		switches[name] = extkit.NewSection(name, Switch{}, "Turns the "+name+" extension off on a single app (`enabled: false`). On a multisite host it doesn't reach the instances: use pocker's isolate_config, or an instance's `extensions."+name+"`.")
	}
}

// ConfiguredSwitches is the settings layer a single app reads from pocketone.yaml: each switch section's `enabled`.
// RegisterWith uses it; Pocker passes its isolate_config section, then each instance's own `extensions.<name>`.
func ConfiguredSwitches(name string) extkit.Config {
	sec, ok := switches[name]
	if !ok {
		return nil
	}
	v, found := sec.Get()
	if !found || v.Enabled == nil {
		return nil
	}
	return extkit.Config{{"enabled": *v.Enabled}}
}

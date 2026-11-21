package standard_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/standard"
)

// yamlSource is a pocketone.yaml with the given sections.
type yamlSource map[string]string

func (s yamlSource) DecodeSection(name string, out any) (bool, error) {
	src, ok := s[name]
	if !ok {
		return false, nil
	}
	return true, yaml.Unmarshal([]byte(src), out)
}

// Every member can be turned off from pocketone.yaml: its own section, or the set's switch section.
func TestEveryMemberHasASection(t *testing.T) {
	for _, e := range standard.New().Extensions() {
		if !extkit.IsSection(e.Name()) {
			t.Errorf("standard member %s has no pocketone.yaml section", e.Name())
		}
	}
}

// `remotes: {enabled: false}` turns remotes off on a single app, and deploy (which requires remote/v1) with it.
func TestYAMLSwitchTurnsAMemberOff(t *testing.T) {
	extkit.SetSectionSource(yamlSource{"remotes": "enabled: false"})
	defer extkit.SetSectionSource(nil)

	app, _ := newApp(t, standard.New(), nil)
	plan, _ := app.Store().Get("standard.plan").(*extkit.Plan)
	if plan == nil {
		t.Fatal("no plan")
	}
	lines := strings.Join(plan.Lines, "\n")
	for _, want := range []string{"remotes off: enabled: false", "deploy off"} {
		if !strings.Contains(lines, want) {
			t.Errorf("startup lines lack %q:\n%s", want, lines)
		}
	}
	for _, e := range plan.On {
		if e.Name() == "remotes" || e.Name() == "deploy" {
			t.Errorf("%s is on", e.Name())
		}
	}
}

func TestSwitchesInCode(t *testing.T) {
	extkit.SetSectionSource(yamlSource{"remotes": "enabled: false"})
	defer extkit.SetSectionSource(nil)

	set := standard.New()
	set.Switches = func(name string) extkit.Config {
		if name == "bulkmail" {
			return extkit.Config{{"enabled": false}}
		}
		return standard.ConfiguredSwitches(name) // pocketone.yaml still decides the rest
	}
	app, _ := newApp(t, set, nil)
	plan, _ := app.Store().Get("standard.plan").(*extkit.Plan)
	if plan == nil {
		t.Fatal("no plan")
	}
	lines := strings.Join(plan.Lines, "\n")
	for _, want := range []string{"bulkmail off: enabled: false", "remotes off: enabled: false"} {
		if !strings.Contains(lines, want) {
			t.Errorf("startup lines lack %q:\n%s", want, lines)
		}
	}
	for _, e := range plan.On {
		if e.Name() == "bulkmail" || e.Name() == "remotes" {
			t.Errorf("%s is on", e.Name())
		}
	}
}

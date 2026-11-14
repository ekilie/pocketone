package config

import (
	"testing"
	"time"

	"github.com/pockethost/pocketone/ext/pluginkit"
)

type describeTestCfg struct {
	Enabled *bool         `yaml:"enabled" doc:"on unless false"`
	Every   time.Duration `yaml:"every"`
	Hosts   []string      `yaml:"hosts"`
	DNS     struct {
		Provider string `yaml:"provider"`
	} `yaml:"dns"`
	Secret string `yaml:"-"`
}

var _ = pluginkit.NewSection("describe_test", describeTestCfg{Every: time.Hour}, "a test section")

func TestDescribe(t *testing.T) {
	var got *SectionDoc
	for _, d := range Describe() {
		if d.Name == "describe_test" {
			got = &d
		}
	}
	if got == nil {
		t.Fatal("section not described")
	}
	if got.Owner != "github.com/pockethost/pocketone/ext/config" || got.Plugin != "config" {
		t.Errorf("owner %q plugin %q", got.Owner, got.Plugin)
	}
	want := []KeyDoc{
		{Key: "enabled", Type: "bool", Doc: "on unless false"},
		{Key: "every", Type: "duration", Default: "1h0m0s"},
		{Key: "hosts", Type: "list of string"},
		{Key: "dns.provider", Type: "string"},
	}
	if len(got.Keys) != len(want) {
		t.Fatalf("keys = %+v", got.Keys)
	}
	for i, k := range want {
		if got.Keys[i] != k {
			t.Errorf("key %d = %+v, want %+v", i, got.Keys[i], k)
		}
	}
}

func TestPluginOf(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/pockethost/pocketone/ext/mothership/mirror": "mothership",
		"github.com/pockethost/pocketone/ext/pitr":              "pitr",
		"main": "",
	} {
		if got := pluginOf(in); got != want {
			t.Errorf("pluginOf(%q) = %q, want %q", in, got, want)
		}
	}
}

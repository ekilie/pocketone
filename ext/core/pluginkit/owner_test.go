package pluginkit_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/core/pluginkit"
)

var ownedSection = pluginkit.NewSection("owner_test_section", struct{}{}, "a section registered outside pluginkit")

var ownedFuncSection = pluginkit.NewSectionFunc("owner_test_section_func", func() struct{} { return struct{}{} }, "")

func TestSectionOwner(t *testing.T) {
	want := "github.com/pockethost/pocketone/ext/core/pluginkit_test"
	for _, s := range pluginkit.Sections() {
		if (s.Name == ownedSection.Name() || s.Name == ownedFuncSection.Name()) && s.Owner != want {
			t.Errorf("%s: Owner = %q, want %q", s.Name, s.Owner, want)
		}
	}
}

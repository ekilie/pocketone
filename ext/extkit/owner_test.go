package extkit_test

import (
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
)

var ownedSection = extkit.NewSection("owner_test_section", struct{}{}, "a section registered outside extkit")

var ownedFuncSection = extkit.NewSectionFunc("owner_test_section_func", func() struct{} { return struct{}{} }, "")

func TestSectionOwner(t *testing.T) {
	want := "github.com/pockethost/pocketone/ext/extkit_test"
	for _, s := range extkit.Sections() {
		if (s.Name == ownedSection.Name() || s.Name == ownedFuncSection.Name()) && s.Owner != want {
			t.Errorf("%s: Owner = %q, want %q", s.Name, s.Owner, want)
		}
	}
}

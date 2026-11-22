package standard

import (
	"io/fs"
	"strings"
	"testing"
)

func TestBrandingUI(t *testing.T) {
	ui := brandingUI("0.0.3")
	if ui.Name != "pocketone" {
		t.Fatalf("name %q", ui.Name)
	}
	js, err := fs.ReadFile(ui.FS, "main.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`const version = "v0.0.3";`, "https://pocketone.dev/docs", "pocketonehq/pocketone/releases"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("main.js lacks %q", want)
		}
	}
	if _, err := fs.Stat(ui.FS, "logo.svg"); err != nil {
		t.Fatal(err)
	}
	dev, _ := fs.ReadFile(brandingUI("dev+9be9a202").FS, "main.js")
	if !strings.Contains(string(dev), `const version = "dev+9be9a202";`) {
		t.Error("a dev version gets no v prefix")
	}
}

package llm_test

import (
	"testing"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	kvv1 "pocketone.dev/ext/contracts/kv/v1"
	"pocketone.dev/ext/llm"
)

// Settings saved before kv/v1 (the _params row "llm") are moved to the private kv name on the first load.
func TestSettingsMoveFromParams(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	old := &core.Param{Value: []byte(`{"providers":[{"name":"vultr","type":"vultr","key":"k1"}]}`)}
	old.Id = "llm"
	old.MarkAsNew()
	if err := app.Save(old); err != nil {
		t.Fatal(err)
	}
	s, err := llm.LoadSettings(app)
	if err != nil || len(s.Providers) != 1 || s.Providers[0].Key != "k1" {
		t.Fatalf("load: %+v %v", s, err)
	}
	if _, found, _ := kvv1.Lookup(app, "_llm/settings"); !found {
		t.Fatal("not moved to _llm/settings")
	}
	if err := app.ModelQuery(&core.Param{}).Model("llm", &core.Param{}); err == nil {
		t.Fatal("the old row is still there")
	}
}

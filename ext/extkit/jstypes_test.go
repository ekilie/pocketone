package extkit_test

import (
	"strings"
	"testing"

	"pocketone.dev/ext/extkit"
)

type jsTypesProbe interface{ Ping() error }

// A contract's declarations reach types.d.ts only while pb_hooks may touch it (JSHooks or JSOpen).
func TestJSTypesFollowThePolicy(t *testing.T) {
	extkit.NewContract[jsTypesProbe]("jstypes.open/v1", extkit.One).JS(extkit.JSOpen, nil).
		JSTypes("declare namespace contracts.jstypes.open.v1 { interface Probe {} }\n")
	extkit.NewContract[jsTypesProbe]("jstypes.closed/v1", extkit.One).JS(extkit.JSClosed, nil).
		JSTypes("declare namespace contracts.jstypes.closed.v1 { interface Probe {} }\n")
	ts := extkit.JSContracts(nil).(interface{ TypeScript() string }).TypeScript()
	if !strings.Contains(ts, "contracts.jstypes.open.v1") {
		t.Error("an open contract's declarations are missing")
	}
	if strings.Contains(ts, "contracts.jstypes.closed.v1") {
		t.Error("a closed contract's declarations leaked into types.d.ts")
	}
	if !strings.Contains(ts, "interface CallEvent<In = any>") || !strings.Contains(ts, "interface Providers {}") {
		t.Error("the contracts namespace lost its generic CallEvent or Providers")
	}
}

func TestJSTypesNeedsJS(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("JSTypes before JS should panic")
		}
	}()
	extkit.NewContract[jsTypesProbe]("jstypes.nojs/v1", extkit.One).JSTypes("x")
}

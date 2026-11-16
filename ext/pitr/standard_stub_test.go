package pitr

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// testStandard stands in for ext/standard's set: Pocker requires one, and this test is about this extension, not
// the standard members.
var testStandard extkit.AppSet = stubStandard{}

type stubStandard struct{}

func (stubStandard) Prepare(core.App, func(string) extkit.Config) (*extkit.Plan, error) {
	return &extkit.Plan{}, nil
}
func (stubStandard) JSExtensions(core.App) []any                     { return nil }
func (stubStandard) BindJS(core.App, js.Runtime)                     {}
func (stubStandard) Finish(core.App, *extkit.Plan) ([]string, error) { return nil, nil }
func (stubStandard) Extensions() []extkit.Extension                  { return nil }

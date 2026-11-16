package pocker

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/js"
)

// testStandard stands in for ext/standard's set in pocker's own tests (pocker can't import it): Pocker requires a
// standard set, and these tests are about Pocker, not its members. ext/standard's tests run Pocker with the real one.
var testStandard extkit.AppSet = stubSet{}

type stubSet struct{}

func (stubSet) Prepare(core.App, func(string) extkit.Config) (*extkit.Plan, error) {
	return &extkit.Plan{}, nil
}
func (stubSet) JSExtensions(core.App) []any                     { return nil }
func (stubSet) BindJS(core.App, js.Runtime)                     {}
func (stubSet) Finish(core.App, *extkit.Plan) ([]string, error) { return nil, nil }
func (stubSet) Extensions() []extkit.Extension                  { return nil }

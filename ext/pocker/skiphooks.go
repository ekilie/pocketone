package pocker

import (
	"fmt"

	"github.com/pockethost/pocketone/ext/extkit/js"
)

// hookExtensions are the jsvm extensions every app runs with: the sandbox (OnInit, Modules, HTTPClient, hidden
// members). Errors already name pb_hooks/<file>:<line> (the engine runs each file under its own name). In dev mode,
// last, a hook file that fails to load is skipped (logged to the instance log) and the rest of the app runs, so the
// owner can fix it live; otherwise the app doesn't start.
// A running app whose pb_hooks change is stopped, then started again (Manager.hooksChanged), with the same rule.
func hookExtensions(sb any, dev bool, log *instanceLog) []any {
	x := []any{sb}
	if dev {
		x = append(x, skipBrokenHooks{log})
	}
	return x
}

// skipBrokenHooks turns a hook file's load error into a log line. Like PocketBase with its watcher on, handlers the
// file registered before it failed stay registered.
type skipBrokenHooks struct{ log *instanceLog }

func (s skipBrokenHooks) OnLoad(_ js.Runtime, _, file string, _ []byte) (func(), func(error) error) {
	return nil, func(err error) error {
		s.log.Write("stderr", fmt.Sprintf("pb_hooks/%s failed to load; skipped it (dev mode): %v", file, err))
		return nil
	}
}

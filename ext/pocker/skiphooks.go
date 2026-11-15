package pocker

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/dop251/goja"
	"github.com/pockethost/pocketone/ext/jserrors"
)

// hookExtensions are the jsvm extensions every app runs with: the sandbox (OnInit, OnCall, HTTPClient), then hook
// errors naming pb_hooks/<file>:<line>:<col>. In dev mode, last, a hook file that fails to load is skipped (logged
// to the instance log) and the rest of the app runs, so the owner can fix it live; otherwise the app doesn't start.
// A running app whose pb_hooks change keeps serving until the new code starts (Manager.hooksChanged), so in
// production a bad edit doesn't take a working instance down either.
func hookExtensions(sb jsvm.Extension, dev bool, log *instanceLog) []jsvm.Extension {
	x := []jsvm.Extension{sb, jserrors.New()}
	if dev {
		x = append(x, skipBrokenHooks{log})
	}
	return x
}

// skipBrokenHooks turns a hook file's load error into a log line. It's last in the list, so the error it logs has
// already been mapped to the file's own positions. Like PocketBase with its watcher on, handlers the file registered
// before it failed stay registered.
type skipBrokenHooks struct{ log *instanceLog }

func (s skipBrokenHooks) OnLoad(_ *goja.Runtime, _, file string, _ []byte) (func(), func(error) error) {
	return nil, func(err error) error {
		s.log.Write("stderr", fmt.Sprintf("pb_hooks/%s failed to load; skipped it (dev mode): %v", file, err))
		return nil
	}
}

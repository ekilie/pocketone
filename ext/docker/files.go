package docker

import (
	"context"

	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"pocketone.dev/ext/multisite"
)

// Files is instance.fs/v1 for the host's instances (dir: the host's Directory), which the Docker host provides on
// its scope: each instance's folder on the host (<DataRoot>/<id>, mounted into its container), and a restart that
// stops its container and holds new requests; the next request starts a fresh one.
func (rt *Runtime) Files(dir *multisite.Directory) fsv1.Files {
	return &fsv1.Instances{
		Known: func(id string) bool { _, ok := dir.Get(id); return ok },
		New: func(id string) *fsv1.Local {
			return &fsv1.Local{
				Folder:    func() string { return rt.InstanceDir(id) },
				IsRunning: func() bool { return rt.Running(id) },
				Stopped: func(ctx context.Context, fn func(context.Context) error) error {
					return rt.WithInstanceStopped(ctx, id, func(string) error { return fn(ctx) })
				},
			}
		},
	}
}

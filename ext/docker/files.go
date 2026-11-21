package docker

import (
	"context"
	"path/filepath"

	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/multisite"
)

// Files is instance.fs/v1 for the host's instances (dir: the host's Directory; host: its scope), which the Docker
// host provides on its scope: each instance's folder on the host (<DataRoot>/<id>, mounted into its container), its
// cache outside that folder and the container (the host's pb_cache/instances/<id>, extkit.InstanceCacheRoot), and a
// restart that stops its container and holds new requests; the next request starts a fresh one.
func (rt *Runtime) Files(dir *multisite.Directory, host *extkit.Scope) fsv1.Files {
	return &fsv1.Instances{
		Known: func(id string) bool { _, ok := dir.Get(id); return ok },
		New: func(id string) *fsv1.Local {
			return &fsv1.Local{
				Folder: func() string { return rt.InstanceDir(id) },
				Cache: func(name string) string {
					if root := extkit.InstanceCacheRoot(host, id); root != "" {
						return filepath.Join(root, name)
					}
					return filepath.Join(rt.InstanceDir(id), fsv1.CacheFolder, name)
				},
				IsRunning: func() bool { return rt.Running(id) },
				Stopped: func(ctx context.Context, fn func(context.Context) error) error {
					return rt.WithInstanceStopped(ctx, id, func(string) error { return fn(ctx) })
				},
			}
		},
	}
}

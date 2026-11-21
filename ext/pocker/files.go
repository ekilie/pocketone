package pocker

import (
	"context"
	"path/filepath"

	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
	"pocketone.dev/ext/extkit"
)

// Files is instance.fs/v1 for the Manager's instances, which the Pocker host provides on its scope: each instance's
// folder, its cache outside that folder (the host's pb_cache/instances/<id>, extkit.InstanceCacheRoot), and a restart
// that stops its app (holding new requests) and starts it again, reporting a broken start.
func (m *Manager) Files() fsv1.Files {
	return &fsv1.Instances{
		Known: func(id string) bool { _, ok := m.InstanceDir(id); return ok },
		New: func(id string) *fsv1.Local {
			return &fsv1.Local{
				Folder:    func() string { d, _ := m.InstanceDir(id); return d },
				Cache:     func(name string) string { return hostedCacheDir(m.opts.Host, id, m.InstanceDir, name) },
				IsRunning: func() bool { return m.Running(id) },
				Stopped: func(ctx context.Context, fn func(context.Context) error) error {
					return m.WithInstanceStopped(ctx, id, func(string) error { return fn(ctx) })
				},
				Start: func(ctx context.Context) error { return m.StartInstance(ctx, id) },
			}
		},
	}
}

// hostedCacheDir is an instance's cache folder name on a host: in the host's pb_cache when it set one, else the
// instance folder's own pb_cache.
func hostedCacheDir(host *extkit.Scope, id string, instanceDir func(string) (string, bool), name string) string {
	if root := extkit.InstanceCacheRoot(host, id); root != "" {
		return filepath.Join(root, name)
	}
	d, _ := instanceDir(id)
	return filepath.Join(d, fsv1.CacheFolder, name)
}

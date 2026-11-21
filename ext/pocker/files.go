package pocker

import (
	"context"

	fsv1 "pocketone.dev/ext/contracts/instance/fs/v1"
)

// Files is instance.fs/v1 for the Manager's instances, which the Pocker host provides on its scope: each instance's
// folder, a restart that stops its app (holding new requests) and starts it again, reporting a broken start.
func (m *Manager) Files() fsv1.Files {
	return &fsv1.Instances{
		Known: func(id string) bool { _, ok := m.InstanceDir(id); return ok },
		New: func(id string) *fsv1.Local {
			return &fsv1.Local{
				Folder:    func() string { d, _ := m.InstanceDir(id); return d },
				IsRunning: func() bool { return m.Running(id) },
				Stopped: func(ctx context.Context, fn func(context.Context) error) error {
					return m.WithInstanceStopped(ctx, id, func(string) error { return fn(ctx) })
				},
				Start: func(ctx context.Context) error { return m.StartInstance(ctx, id) },
			}
		},
	}
}

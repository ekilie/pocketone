// Package backups is a PocketOne extension: restore/v1 over PocketBase's own backups. Asked to restore an app to a
// moment, it restores the latest backup made at or before it (local or S3, wherever the app's backups go). It's the
// default provider: it steps aside when pitr (any moment, not just backup times) is on for the app.
package backups

import (
	"context"
	"sort"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
	restorev1 "github.com/pockethost/pocketone/ext/contracts/restore/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

// Name is the extension's name.
const Name = "backups"

// Extension provides restore/v1 from the app's backups.
type Extension struct{}

var _ extkit.Extension = (*Extension)(nil)

// New returns the extension.
func New() *Extension { return &Extension{} }

// Name implements [extkit.Extension].
func (*Extension) Name() string { return Name }

// Contracts implements [extkit.Declarer]: the default restore/v1.
func (*Extension) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("restore/v1"), Defaults: extkit.IDs("restore/v1")}
}

// Register implements [extkit.Extension].
func (*Extension) Register(app core.App, _ extkit.Config) error {
	restorev1.Contract.ProvideDefault(app, &restorer{app: app})
	return nil
}

type restorer struct {
	app core.App
	// restart runs after a restore is accepted (tests replace it; the default restores and restarts the app)
	restore func(name string)
}

// snapshots lists the app's backups, newest first.
func (r *restorer) snapshots() ([]restorev1.Snapshot, error) {
	fsys, err := r.app.NewBackupsFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()
	objs, err := fsys.List("")
	if err != nil {
		return nil, err
	}
	out := make([]restorev1.Snapshot, 0, len(objs))
	for _, o := range objs {
		out = append(out, restorev1.Snapshot{At: o.ModTime.UTC(), Name: o.Key, Size: o.Size})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

func (r *restorer) Points(context.Context, extkit.Auth) (restorev1.Points, error) {
	snaps, err := r.snapshots()
	if err != nil {
		return restorev1.Points{}, err
	}
	return restorev1.Points{Provider: Name, Kind: restorev1.Snapshots, Snapshots: snaps,
		Restoring: r.app.Store().Has(core.StoreKeyActiveBackup)}, nil
}

func (r *restorer) RestoreTo(_ context.Context, _ extkit.Auth, in restorev1.RestoreInput) (restorev1.RestoreResult, error) {
	if in.At.IsZero() {
		return restorev1.RestoreResult{}, &extkit.Error{Code: "bad_request", Message: "a moment to restore to is required", Status: 400}
	}
	if r.app.Store().Has(core.StoreKeyActiveBackup) {
		return restorev1.RestoreResult{}, restorev1.ErrBusy
	}
	snaps, err := r.snapshots()
	if err != nil {
		return restorev1.RestoreResult{}, err
	}
	var pick *restorev1.Snapshot
	for i := range snaps { // newest first: the first at or before the moment
		if !snaps[i].At.After(in.At) {
			pick = &snaps[i]
			break
		}
	}
	if pick == nil {
		return restorev1.RestoreResult{}, restorev1.ErrNoPoint
	}
	restore := r.restore
	if restore == nil {
		restore = func(name string) {
			time.Sleep(300 * time.Millisecond) // let the answer go out first
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			if err := r.app.RestoreBackup(ctx, name); err != nil { // restarts the app on success
				r.app.Logger().Error("backups: restore failed", "backup", name, "error", err)
			}
		}
	}
	go restore(pick.Name)
	return restorev1.RestoreResult{Restoring: true, Point: pick.At, Name: pick.Name,
		Message: "restoring the backup " + pick.Name + "; the app restarts to apply it"}, nil
}

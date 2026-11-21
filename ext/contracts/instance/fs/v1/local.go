package fsv1

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Local is an [FS] over a folder on local disk, for providers to build on. Zero values are fine except where noted.
type Local struct {
	// Folder returns the instance folder ("" while there is none yet). Required.
	Folder func() string
	// State returns an extension's state folder. Default: <instance folder>/.pb_<name> (a dot-folder: never code).
	State func(name string) string
	// IsRunning reports whether the instance runs. Default: always.
	IsRunning func() bool
	// Stopped calls fn with the instance stopped; the instance runs again after (now or on its next request).
	// Default: fn as it is (an app can't stop itself).
	Stopped func(ctx context.Context, fn func(context.Context) error) error
	// Start has the instance run again after Stopped, when that doesn't (a single app: its restart). Optional.
	Start func(ctx context.Context) error

	writers sync.Mutex
	stop    sync.Mutex
}

var _ FS = (*Local)(nil)

// ErrNoFolder: the instance has no folder yet.
var ErrNoFolder = errors.New("instance.fs/v1: the instance has no folder yet")

func (l *Local) Dir(context.Context) (string, error) {
	d := l.Folder()
	if d == "" {
		return "", ErrNoFolder
	}
	return d, nil
}

func (l *Local) StateDir(ctx context.Context, name string) (string, error) {
	if !IsSafeRelativePath(name) || filepath.Base(name) != name {
		return "", errors.New("instance.fs/v1: bad state folder name " + name)
	}
	var d string
	if l.State != nil {
		d = l.State(name)
	} else {
		dir, err := l.Dir(ctx)
		if err != nil {
			return "", err
		}
		d = filepath.Join(dir, ".pb_"+name)
	}
	return d, os.MkdirAll(d, 0o755)
}

func (l *Local) Lock(context.Context) (func(), error) {
	l.writers.Lock()
	return l.writers.Unlock, nil
}

func (l *Local) Running(context.Context) (bool, error) {
	if l.IsRunning == nil {
		return true, nil
	}
	return l.IsRunning(), nil
}

func (l *Local) Restart(ctx context.Context, apply func(context.Context) error) (startErr, err error) {
	if apply == nil {
		apply = func(context.Context) error { return nil }
	}
	l.stop.Lock()
	defer l.stop.Unlock()
	if running, _ := l.Running(ctx); !running {
		return nil, apply(ctx)
	}
	if l.Stopped != nil {
		err = l.Stopped(ctx, apply)
	} else {
		err = apply(ctx)
	}
	if l.Start != nil {
		startErr = l.Start(ctx)
	}
	return startErr, err
}

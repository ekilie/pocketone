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
	// Cache returns an extension's cache folder. Default: <instance folder>/pb_cache/<name>.
	Cache func(name string) string
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

func (l *Local) CacheDir(ctx context.Context, name string) (string, error) {
	if !IsSafeRelativePath(name) || filepath.Base(name) != name {
		return "", errors.New("instance.fs/v1: bad cache folder name " + name)
	}
	var d string
	if l.Cache != nil {
		d = l.Cache(name)
	} else {
		dir, err := l.Dir(ctx)
		if err != nil {
			return "", err
		}
		d = filepath.Join(dir, CacheFolder, name)
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

// Instances is a [Files] for a host's instances, for runtimes to provide: it builds each instance's [Local] once and
// keeps it, so every caller shares its lock.
type Instances struct {
	// Known reports whether the host has the instance (else [ErrNoInstance]). Required.
	Known func(id string) bool
	// New builds the instance's Local. Required.
	New func(id string) *Local

	mu sync.Mutex
	fs map[string]*Local
}

var _ Files = (*Instances)(nil)

func (i *Instances) Instance(_ context.Context, id string) (FS, error) {
	if id == Self || !i.Known(id) {
		return nil, ErrNoInstance
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if l, ok := i.fs[id]; ok {
		return l, nil
	}
	if i.fs == nil {
		i.fs = map[string]*Local{}
	}
	l := i.New(id)
	i.fs[id] = l
	return l, nil
}

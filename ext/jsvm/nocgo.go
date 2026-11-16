//go:build !cgo

package jsvm

import (
	"errors"

	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// ErrNoCgo is what Register returns in a build without cgo: the engine is QuickJS, a C library.
var ErrNoCgo = errors.New("ext/jsvm needs a build with cgo (CGO_ENABLED=1); without it, use PocketBase's plugins/jsvm")

// Register fails without cgo.
func Register(app core.App, config Config) error { return ErrNoCgo }

// MustRegister panics without cgo.
func MustRegister(app core.App, config Config) { panic(ErrNoCgo) }

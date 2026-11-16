//go:build !cgo

package jsvm

import (
	"ganigeorgiev.com/pocketbase/pocketbase/core"
)

// Available reports whether this build has the engine (it needs cgo).
const Available = false

// Register fails without cgo.
func Register(app core.App, config Config) error { return ErrNoCgo }

// MustRegister panics without cgo.
func MustRegister(app core.App, config Config) { panic(ErrNoCgo) }

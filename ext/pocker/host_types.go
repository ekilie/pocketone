package pocker

import (
	"context"

	"pocketone.dev/ext/multisite"
)

// The instance model and the host live in ext/multisite (shared by every runtime provider); these aliases keep pocker's
// API unchanged.
type (
	Instance       = multisite.Instance
	Access         = multisite.Access
	Superuser      = multisite.Superuser
	Blocked        = multisite.Blocked
	ChangeKind     = multisite.ChangeKind
	Change         = multisite.Change
	InstanceSource = multisite.InstanceSource
	Limits         = multisite.Limits
	ByteSize       = multisite.ByteSize
	Gate           = multisite.Gate
	TLSProvider    = multisite.TLSProvider
	Wrapper        = multisite.Wrapper
	TimingEntry    = multisite.TimingEntry
	TLSConfig      = multisite.TLSConfig
	YAMLSource     = multisite.YAMLSource
)

const (
	Upsert = multisite.Upsert
	Delete = multisite.Delete
)

// ParseByteSize parses a size like "512MB" (see [multisite.ParseByteSize]).
func ParseByteSize(s string) (ByteSize, error) { return multisite.ParseByteSize(s) }

// WithTimings collects a request's timing entries (see [multisite.WithTimings]).
func WithTimings(ctx context.Context) (context.Context, func() []TimingEntry) {
	return multisite.WithTimings(ctx)
}

// Timing registers timing entries for the current request (see [multisite.Timing]).
func Timing(ctx context.Context, fn func() []TimingEntry) { multisite.Timing(ctx, fn) }

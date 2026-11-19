package multisite

import (
	gatev1 "pocketone.dev/ext/contracts/gate/v1"
	wrapv1 "pocketone.dev/ext/contracts/http/wrap/v1"
	sourcev1 "pocketone.dev/ext/contracts/instance/source/v1"
	runtimev1 "pocketone.dev/ext/contracts/runtime/v1"
	tlsv1 "pocketone.dev/ext/contracts/tls/v1"
)

// The instance record and the host's extension points are contracts (ext/contracts); these are their names here.
type (
	// Instance is one app a host serves: instance.source/v1's record.
	Instance   = sourcev1.Instance
	Access     = sourcev1.Access
	Superuser  = sourcev1.Superuser
	Blocked    = sourcev1.Blocked
	ChangeKind = sourcev1.ChangeKind
	Change     = sourcev1.Change
	Limits     = sourcev1.Limits
	ByteSize   = sourcev1.ByteSize

	// InstanceSource provides the instances: instance.source/v1.
	InstanceSource = sourcev1.Source

	// Gate admits requests: gate/v1.
	Gate             = gatev1.Gate
	ClientIPResolver = gatev1.ClientIPResolver

	// TLSProvider supplies the main listener's TLS: tls/v1.
	TLSProvider = tlsv1.Provider

	// Wrapper wraps the host's handler: http.wrap/v1.
	Wrapper = wrapv1.Wrapper

	// Runtime runs instances: runtime/v1, and its optional hooks.
	Runtime        = runtimev1.Runtime
	Prechecker     = runtimev1.Prechecker
	Syncer         = runtimev1.Syncer
	RuntimeStarter = runtimev1.Starter
	Shutdowner     = runtimev1.Shutdowner
	Statser        = runtimev1.Statser
)

// Kinds of instance changes.
const (
	Upsert = sourcev1.Upsert
	Delete = sourcev1.Delete
)

// ParseByteSize reads a size like "512MB".
var ParseByteSize = sourcev1.ParseByteSize

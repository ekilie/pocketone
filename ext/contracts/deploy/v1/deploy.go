// Package deployv1 is the deploy/v1 contract: a project's code files on an app (check, upload, commit, list, read,
// write, delete, tree). Restart and logs are remote/v1's, which it requires.
//
// A routed contract: its provider is an http.Handler serving the file API under /api/_c/deploy/v1/ (ext/deploy on a
// standalone app; a multisite host provides it for its instances, because Docker runs stock PocketBase images with
// nothing of ours inside). The API's operations:
//
//	POST   /api/_c/deploy/v1/check       which files the remote lacks (JSON)
//	POST   /api/_c/deploy/v1/blobs       upload them (gzip bytes)
//	POST   /api/_c/deploy/v1/commit      swap the new code set in (JSON)
//	GET    /api/_c/deploy/v1/files       list deployed files
//	DELETE /api/_c/deploy/v1/files       delete files
//	GET    /api/_c/deploy/v1/files/raw   read one file (raw bytes)
//	PUT    /api/_c/deploy/v1/files/raw   write one file (raw bytes)
//	GET    /api/_c/deploy/v1/tree        the remote's code tree, for two-way sync
//
// Token scopes: deploy (everything) and deploy:read (list, read, tree).
package deployv1

import (
	"net/http"

	"github.com/pockethost/pocketone/ext/extkit"
)

// Contract is deploy/v1.
var Contract = extkit.NewContract[http.Handler]("deploy/v1", extkit.One).HostProvidable()

// Prefix is where deploy/v1 is served: /api/_c/deploy/v1.
var Prefix = extkit.Routed(Contract)

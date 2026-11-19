// Package examplev1 is a test-only wire contract (example/v1) that exercises the generated machinery: routes, action
// hooks in Go and JS, MCP tools, the client, host-providability. It's shaped like a small deploy API.
package examplev1

//go:generate go run pocketone.dev/ext/contracts/cmd/contractgen

import (
	"context"

	"pocketone.dev/ext/extkit"
)

// Deployer sends a project's code to an app.
//
//contract:id example/v1
//contract:kind wire
//contract:scope example
//contract:host
//contract:js hooks
type Deployer interface {
	// Manifest lists the files currently deployed to the app, with sizes and hashes.
	Manifest(ctx context.Context, a extkit.Auth) (Manifest, error)

	// Upload writes changed files to the app.
	Upload(ctx context.Context, a extkit.Auth, in UploadInput) (UploadResult, error)
}

// File is one deployed file.
type File struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
}

// Manifest is what's deployed.
type Manifest struct {
	Files []File `json:"files"`
}

// UploadInput is an upload's files.
type UploadInput struct {
	Files []File `json:"files"`
}

// UploadResult says what an upload wrote.
type UploadResult struct {
	Written int    `json:"written"`
	By      string `json:"by,omitempty"` // the provider that wrote them
}

// ErrQuota is the contract's typed error for an upload over the caller's quota.
var ErrQuota = &extkit.Error{Code: "quota", Message: "over your deploy quota", Status: 413}

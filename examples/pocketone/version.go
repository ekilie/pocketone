package main

import (
	"fmt"

	"pocketone.dev/ext/pocker"
)

// upstreamVersion is the PocketBase release PocketOne is built on. Bump it when merging a PocketBase release
// (version_test.go checks it against ui/.env's PB_VERSION, which upstream sets on every release).
const upstreamVersion = "0.42.3"

// versionString is what `pocketone --version` prints: "0.0.2 (PocketBase 0.41.1)". PocketOne's version is the release
// a release build stamps in, else the commit ("dev+4700768d"); see pocker.Version.
func versionString() string {
	return fmt.Sprintf("%s (PocketBase %s)", pocker.Version(), upstreamVersion)
}

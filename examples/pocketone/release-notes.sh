#!/usr/bin/env bash
# Prints one release's notes, for goreleaser's --release-notes: its section of CHANGELOG.md (without its heading), with
# relative links (each extension's new version, in its own changelog) pointing at the release on GitHub. Stamp the
# release first with `go run ./.github/extensions release vX.Y.Z`.
# Usage: examples/pocketone/release-notes.sh v0.0.1-alpha.3 > /tmp/notes.md
set -euo pipefail

version="${1:?usage: $0 vX.Y.Z}"
cd "$(dirname "$0")/../.."
exec go run ./.github/extensions release-notes "$version"

#!/usr/bin/env bash
# Prints one version's section of docs/CHANGELOG.md (without its heading), for goreleaser's --release-notes.
# Usage, from the repo root: examples/pocketone/release-notes.sh v0.0.1-alpha.3 > /tmp/notes.md
set -euo pipefail

version="${1:?usage: $0 vX.Y.Z}"
changelog="$(dirname "$0")/../../docs/CHANGELOG.md"

notes="$(awk -v v="$version" '
  /^## / { if (found) exit; if ($2 == v) { found = 1; next } }
  found { print }
' "$changelog")"

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
  echo "No notes for $version in docs/CHANGELOG.md: rename '## Unreleased' to '## $version (date)' first." >&2
  exit 1
fi

# trim leading and trailing blank lines
printf '%s\n' "$notes" | awk 'NF { started = 1 } started' | awk '{ lines[NR] = $0 } END { n = NR; while (n > 0 && lines[n] ~ /^[[:space:]]*$/) n--; for (i = 1; i <= n; i++) print lines[i] }'

#!/usr/bin/env bash
# During an upstream merge: CHANGELOG.md is PocketOne's, and upstream PocketBase's own CHANGELOG.md lives here as
# CHANGELOG.pocketbase.md. Upstream edits its CHANGELOG.md with every release, which conflicts with ours, so after
# `git merge --no-commit upstream/master` (AGENTS.md, "Syncing with upstream") run:
#
#   .github/upstream-changelog.sh [upstream ref, default upstream/master]
#
# It keeps our CHANGELOG.md as it was before the merge, writes upstream's into CHANGELOG.pocketbase.md, and stages both.
set -euo pipefail
cd "$(dirname "$0")/.."
ref=${1:-upstream/master}

git rev-parse -q --verify MERGE_HEAD >/dev/null || { echo "not in a merge; run after git merge --no-commit $ref" >&2; exit 1; }
git show "HEAD:CHANGELOG.md" > CHANGELOG.md
git show "$ref:CHANGELOG.md" > CHANGELOG.pocketbase.md
git add CHANGELOG.md CHANGELOG.pocketbase.md
echo "CHANGELOG.md kept as PocketOne's; CHANGELOG.pocketbase.md = $ref:CHANGELOG.md ($(git rev-parse --short "$ref"))"

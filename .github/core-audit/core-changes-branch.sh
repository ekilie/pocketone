#!/usr/bin/env bash
# Rebuilds the `core-changes` branch: upstream PocketBase `develop` plus one commit holding every change PocketOne makes
# to PocketBase core, and nothing else (no ext/, site/, docs or other PocketOne-owned paths; see core-diff.sh).
# It lets anyone audit our core changes against upstream in one diff:
#   https://github.com/pockethost/pocketone/compare/<upstream sha>...core-changes
#
# Run it after every pull from upstream, then force-push:
#   git fetch upstream && git fetch origin
#   .github/core-audit/core-changes-branch.sh            # defaults: upstream/develop, origin/main
#   git push --force origin core-changes
#
# The core patch is taken from the fork point (merge-base of main and upstream) to main, then applied three-way onto
# upstream develop. Conflicts resolved on an earlier run are replayed (git rerere); a new one stops the script and
# leaves the worktree, and `--continue` finishes after you resolve it.
set -euo pipefail

branch=core-changes
root=$(git rev-parse --show-toplevel)

finish() { # $1 worktree, $2 upstream sha, $3 source sha
  local wt=$1 up_sha=$2 src_sha=$3
  git -C "$wt" -c rerere.enabled=true commit --quiet -F - <<MSG
PocketOne core changes on upstream develop ${up_sha:0:10}

Every change PocketOne makes to PocketBase core, as of PocketOne ${src_sha:0:10}. Extensions (ext/) and other
PocketOne-owned paths are left out. Rebuilt and force-pushed after each upstream pull; never merge from it.

Upstream: ${up_sha}
PocketOne: ${src_sha}
MSG
  git -C "$root" branch -f "$branch" "$(git -C "$wt" rev-parse HEAD)"
  git -C "$root" worktree remove --force "$wt"
  echo "$branch = $(git -C "$root" rev-parse --short "$branch") on upstream ${up_sha:0:10}"
  echo "review: https://github.com/pockethost/pocketone/compare/${up_sha}...$branch"
  echo "push:   git push --force origin $branch"
}

# After resolving conflicts by hand: core-changes-branch.sh --continue <worktree> <upstream sha> <source sha>
if [ "${1:-}" = "--continue" ]; then
  wt=${2:?worktree}
  if grep -rlq '^<<<<<<< ' -- $(git -C "$wt" diff --name-only HEAD | sed "s|^|$wt/|") 2>/dev/null; then
    echo "conflict markers still in $wt"; exit 2
  fi
  git -C "$wt" add -A
  finish "$wt" "${3:?upstream sha}" "${4:?source sha}"
  exit 0
fi

upstream=${1:-upstream/develop}
source=${2:-origin/main}

# Core per core-diff.sh, minus files that only list PocketOne paths or are contributor prose, not code.
pathspec=(. ':!ext' ':!site' ':!examples/pocketone' ':!examples/ce' ':!docs' ':!README.md' ':!AGENTS.md' ':!CLAUDE.md'
  ':!.github' ':!.claude' ':!.gitignore' ':!CONTRIBUTING.md')

up_sha=$(git rev-parse "$upstream^{commit}")
src_sha=$(git rev-parse "$source^{commit}")
fork=$(git merge-base "$src_sha" "$up_sha")
echo "upstream $upstream = ${up_sha:0:10}; source $source = ${src_sha:0:10}; fork point ${fork:0:10}"

patch=$(mktemp)
wt=$(mktemp -d)/core-changes
trap 'rm -f "$patch"' EXIT
git -C "$root" diff --binary "$fork" "$src_sha" -- "${pathspec[@]}" >"$patch"
if [ ! -s "$patch" ]; then echo "no core changes between ${fork:0:10} and ${src_sha:0:10}"; exit 1; fi
echo "core patch: $(git -C "$root" diff --shortstat "$fork" "$src_sha" -- "${pathspec[@]}")"

git -C "$root" worktree add --detach --quiet "$wt" "$up_sha"
if ! git -C "$wt" -c rerere.enabled=true apply --3way --index "$patch"; then
  # rerere replays a resolution recorded on an earlier run; anything still marked is new.
  git -C "$wt" -c rerere.enabled=true rerere >/dev/null 2>&1 || true
  left=$(cd "$wt" && git diff --name-only --diff-filter=U | while read -r f; do grep -lq '^<<<<<<< ' "$f" && echo "$f"; done || true)
  if [ -n "$left" ]; then
    echo "conflicts applying the core patch onto ${up_sha:0:10}:"; echo "$left" | sed 's/^/  /'
    echo "resolve them in $wt (keep upstream's change and ours), then:"
    echo "  $0 --continue $wt $up_sha $src_sha"
    exit 2
  fi
  echo "conflicts resolved from an earlier run (rerere)"
  git -C "$wt" add -A
fi
finish "$wt" "$up_sha" "$src_sha"

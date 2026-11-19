#!/usr/bin/env bash
# Lands HEAD on main through CI: pushes it to a land/<name> branch, where .github/workflows/land.yml runs the checks
# main needs (extension rules, affected modules' tidy/build/vet/tests, core audit trailers, PocketBase's suite when
# core changed, generated files current) and fast-forwards main to it only if they all pass. Your machine is free as
# soon as the push returns; main only ever moves to a commit that passed.
#
#   .github/land.sh [name]     name defaults to the current branch, or main-<short sha> on main
#
# Typical flow: merge your branch into local main as usual (the pre-commit hook regenerates generated files), then run
# this from main. Land again on top before the first finishes if you like: a later land contains the earlier one, so
# whichever passes moves main forward. If main moved under you (another session landed), merge origin/main and land
# again; CI never merges for you.
#
# Watch it with the `gh run watch` line this prints. Needs the hooks on (git config core.hooksPath .github/hooks):
# the pre-push hook runs the quick checks first.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# bounded waits where coreutils' timeout exists (macOS has it only with coreutils)
t() { if command -v timeout >/dev/null; then timeout "$@"; else shift; "$@"; fi; }

if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "land: uncommitted changes to tracked files; commit or stash them first" >&2
  exit 1
fi

echo "$(date +%T) land: fetching origin/main"
t 30 git fetch -q origin main

head=$(git rev-parse HEAD)
main=$(git rev-parse origin/main)
if [ "$head" = "$main" ]; then
  echo "land: HEAD is already origin/main; nothing to land"
  exit 0
fi
if ! git merge-base --is-ancestor "$main" "$head"; then
  echo "land: origin/main has commits HEAD doesn't; run  git merge origin/main  and land again" >&2
  exit 1
fi

branch=$(git symbolic-ref --short -q HEAD || echo detached)
name=${1:-}
if [ -z "$name" ]; then
  if [ "$branch" = main ] || [ "$branch" = detached ]; then
    name="main-$(git rev-parse --short HEAD)"
  else
    name="$branch"
  fi
fi
ref="land/$name"

echo "$(date +%T) land: $(git rev-list --count "$main..$head") commit(s) on top of origin/main -> $ref"
git log --oneline -20 "$main..$head" | sed 's/^/  /'
t 900 git push --force origin "HEAD:refs/heads/$ref"
echo "$(date +%T) land: pushed; CI lands it on main if the checks pass"

# Point at the run, if gh is here (the run shows up a few seconds after the push).
if command -v gh >/dev/null; then
  for i in 1 2 3 4 5; do
    id=$(t 15 gh run list --branch "$ref" --workflow land.yml --limit 1 --json databaseId,headSha \
      --jq ".[] | select(.headSha == \"$head\") | .databaseId" 2>/dev/null || true)
    if [ -n "$id" ]; then
      echo "  watch:  gh run watch $id --exit-status"
      exit 0
    fi
    sleep 3
  done
  echo "  watch:  gh run list --branch $ref --workflow land.yml"
fi

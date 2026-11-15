#!/usr/bin/env bash
# Rebuilds the branches that show PocketOne's changes to PocketBase core against upstream `develop`:
#
#   core-changes        upstream develop + one commit holding every core change on main (no ext/, site/, docs or other
#                       PocketOne-owned paths; see core-diff.sh). Audit everything in one diff.
#   core-hook/<name>    one hook each, stacked in HOOK_STACK order (each on the one before), so a branch's compare link
#                       against its parent shows just that hook.
#   core-bugfix/<name>  one upstream bug fix each, straight on upstream develop, written to be taken upstream as is.
#
# Run it after every pull from upstream (and after a core change lands on main), then force-push:
#   git fetch upstream && git fetch origin
#   .github/core-audit/core-changes-branch.sh            # defaults: upstream/develop, origin/main
#   git push --force origin core-changes 'refs/heads/core-hook/*' 'refs/heads/core-bugfix/*'
#   (and commit README.md if the script repinned its compare links)
#
# core-changes is rebuilt from main's core diff (fork point..main) applied three-way onto upstream develop. The hook and
# bugfix branches are rebased onto it. Conflicts resolved on an earlier run are replayed (git rerere); a new one stops
# the script and leaves the worktree. Finally the hook stack plus the bugfix branches is checked against core-changes:
# the code must match (comments and tests may differ, since the bugfix branches are written for upstream).
set -euo pipefail

HOOK_STACK=(filesystem-driver on-filesystem-create app-migrations on-log jsvm-extensions jsvm-run)

branch=core-changes
root=$(git rev-parse --show-toplevel)
rr=(-c rerere.enabled=true -c rerere.autoupdate=true)

finish() { # $1 worktree, $2 upstream sha, $3 source sha
  local wt=$1 up_sha=$2 src_sha=$3
  git -C "$wt" "${rr[@]}" commit --quiet -F - <<MSG
PocketOne core changes on upstream develop ${up_sha:0:10}

Every change PocketOne makes to PocketBase core, as of PocketOne ${src_sha:0:10}. Extensions (ext/) and other
PocketOne-owned paths are left out. Rebuilt and force-pushed after each upstream pull; never merge from it.

Upstream: ${up_sha}
PocketOne: ${src_sha}
MSG
  git -C "$root" branch -f "$branch" "$(git -C "$wt" rev-parse HEAD)"
  git -C "$root" worktree remove --force "$wt"
  echo "$branch = $(git -C "$root" rev-parse --short "$branch") on upstream ${up_sha:0:10}"
  rebase_branches "$up_sha"
  check_sum
  # The README's compare links pin the upstream sha; point them at the new base.
  if grep -qE "compare/[0-9a-f]{40}\.\.\.core-" "$root/README.md"; then
    sed -E -i.bak "s#compare/[0-9a-f]{40}\.\.\.core-#compare/${up_sha}...core-#g" "$root/README.md" && rm -f "$root/README.md.bak"
    if ! git -C "$root" diff --quiet -- README.md; then echo "README.md: compare links now pin ${up_sha:0:10}; commit it on main"; fi
  fi
  echo "review: https://github.com/pockethost/pocketone/compare/${up_sha}...$branch"
  echo "push:   git push --force origin $branch 'refs/heads/core-hook/*' 'refs/heads/core-bugfix/*'"
}

rebase_one() { # $1 new base, $2 old base, $3 branch [, more args to git rebase]
  local onto=$1 from=$2 br=$3; shift 3
  [ "$(git -C "$root" merge-base "$br" "$onto")" = "$(git -C "$root" rev-parse "$onto^{commit}")" ] &&
    [ "$from" = "$(git -C "$root" rev-parse "$onto^{commit}")" ] && return 0 # already on it
  local wt; wt=$(mktemp -d)/rebase
  git -C "$root" worktree add --detach --quiet "$wt" "$br"
  if ! git -C "$wt" "${rr[@]}" rebase --quiet --onto "$onto" "$from" "$br" "$@"; then
    if [ -n "$(git -C "$wt" diff --name-only --diff-filter=U)" ]; then
      echo "conflicts rebasing $br onto ${onto:0:10}; resolve in $wt and run: git -C $wt rebase --continue"
      echo "then run this script again"
      exit 2
    fi
    # rerere resolved everything: keep going
    while ! GIT_EDITOR=true git -C "$wt" "${rr[@]}" rebase --continue >/dev/null 2>&1; do
      [ -n "$(git -C "$wt" diff --name-only --diff-filter=U)" ] && { echo "conflicts in $br; resolve in $wt"; exit 2; }
    done
  fi
  git -C "$root" worktree remove --force "$wt"
  echo "rebased $br onto ${onto:0:10}"
}

rebase_branches() { # $1 upstream sha
  local up=$1 b
  local first="core-hook/${HOOK_STACK[0]}" top="core-hook/${HOOK_STACK[${#HOOK_STACK[@]}-1]}"
  if git -C "$root" rev-parse -q --verify "$top" >/dev/null; then
    # the whole stack in one rebase; --update-refs moves every core-hook branch in it
    rebase_one "$up" "$(git -C "$root" merge-base "$first" "$up")" "$top" --update-refs
  fi
  for b in $(git -C "$root" for-each-ref --format='%(refname:short)' 'refs/heads/core-bugfix/'); do
    rebase_one "$up" "$(git -C "$root" merge-base "$b" "$up")" "$b"
  done
}

# The hook stack plus every bugfix branch must equal core-changes, ignoring comments and tests.
check_sum() {
  local top="core-hook/${HOOK_STACK[${#HOOK_STACK[@]}-1]}" wt b residual
  git -C "$root" rev-parse -q --verify "$top" >/dev/null || { echo "no $top: skipping the sum check"; return 0; }
  wt=$(mktemp -d)/sum
  git -C "$root" worktree add --detach --quiet "$wt" "$top"
  for b in $(git -C "$root" for-each-ref --format='%(refname:short)' 'refs/heads/core-bugfix/'); do
    if ! git -C "$wt" "${rr[@]}" merge --quiet --no-edit "$b" >/dev/null 2>&1; then
      if [ -n "$(git -C "$wt" diff --name-only --diff-filter=U)" ]; then
        echo "sum check: merging $b conflicts (resolve in $wt, commit, rerun; rerere keeps it)"; exit 2
      fi
      git -C "$wt" "${rr[@]}" commit --quiet --no-edit
    fi
  done
  residual=$(git -C "$wt" diff HEAD "$branch" -- . ':!*_test.go' | grep -E '^[-+]' | grep -vE '^(\+\+\+|---) ' |
    grep -vE '^[-+][[:space:]]*(//.*)?$' || true)
  git -C "$root" worktree remove --force "$wt"
  if [ -n "$residual" ]; then
    echo "sum check FAILED: core-changes has code no core-hook/core-bugfix branch carries (or a branch is stale):"
    echo "$residual" | head -40
    echo "add it to the right branch (or a new core-hook/<name> in HOOK_STACK), then rerun"
    exit 3
  fi
  echo "sum check: hook stack + bugfix branches = core-changes (code)"
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
if ! git -C "$wt" "${rr[@]}" apply --3way --index "$patch" 2>/dev/null; then
  # rerere replays a resolution recorded on an earlier run; anything still marked is new.
  git -C "$wt" "${rr[@]}" rerere >/dev/null 2>&1 || true
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

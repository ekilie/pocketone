#!/usr/bin/env bash
# Prints the PocketBase core changes in a commit range: every path outside what PocketOne owns.
#
#   .github/core-audit/core-diff.sh <base> <head>     # e.g. origin/main HEAD, or <sha>^ <sha>
#   .github/core-audit/core-diff.sh --files <base> <head>
#
# PocketOne-owned (not core): ext/, site/, examples/pocketone/ (examples/ce/ before 2026-11-14), docs/, README.md, CONTRIBUTING.md, AGENTS.md, CLAUDE.md, CHANGELOG.md (PocketOne's; upstream's is CHANGELOG.pocketbase.md), CHANGELOG.pocketbase.md, .github/, .claude/, .claude-plugin/, agent/.
# Everything else is PocketBase's own code and needs a core audit (see AGENTS.md).
set -euo pipefail
files_only=0
if [ "${1:-}" = "--files" ]; then files_only=1; shift; fi
base=${1:?base commit}
head=${2:?head commit}
pathspec=(. ':!ext' ':!site' ':!examples/pocketone' ':!examples/ce' ':!docs' ':!README.md' ':!CONTRIBUTING.md' ':!AGENTS.md' ':!CLAUDE.md' ':!CHANGELOG.md' ':!CHANGELOG.pocketbase.md' ':!.github' ':!.claude' ':!.claude-plugin' ':!agent')
if [ "$files_only" = 1 ]; then
  git diff --name-only "$base" "$head" -- "${pathspec[@]}"
else
  git diff "$base" "$head" -- "${pathspec[@]}"
fi

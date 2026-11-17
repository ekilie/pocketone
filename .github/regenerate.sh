#!/usr/bin/env bash
# Regenerates every file built from other files: each contract README's
# Extensions index and the contract catalogue (from every extension's Contracts()), then pocketone.dev's generated
# pages, nav, search index, llms.txt / llms-full.txt and the Agent Skills' reference folders (site/sync).
#
# Local commits include its output: the .github/hooks/pre-commit hook (git config core.hooksPath .github/hooks) runs it
# on the staged files and refuses a commit that would leave a generated file stale. Outside pull requests never commit
# generated files; CI runs it on main after the merge and commits what changed (.github/workflows/site.yml). Run it
# yourself any time:
#
#   .github/regenerate.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "$(date +%T) contract indexes (go run ./.github/extensions contracts)"
go run ./.github/extensions contracts

echo "$(date +%T) site (cd site/sync && go run .)"
(cd site/sync && go run .)

echo "$(date +%T) done"

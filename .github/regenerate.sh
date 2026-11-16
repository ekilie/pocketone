#!/usr/bin/env bash
# Regenerates every file built from other files, so nobody has to commit them by hand: each contract README's
# Extensions index and the contract catalogue (from every extension's Contracts()), then pocketone.dev's generated
# pages, nav, search index, llms.txt / llms-full.txt and the Agent Skills' reference folders (site/sync).
#
# CI runs it on main after every push and commits what changed (.github/workflows/site.yml), so a pull request
# (an extension author's included) never has to touch a file outside the folders it changes. Run it yourself any time:
#
#   .github/regenerate.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "$(date +%T) contract indexes (go run ./.github/extensions contracts)"
go run ./.github/extensions contracts

echo "$(date +%T) site (cd site/sync && go run .)"
(cd site/sync && go run .)

echo "$(date +%T) done"

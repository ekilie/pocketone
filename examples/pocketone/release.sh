#!/usr/bin/env bash
# Cuts a PocketOne release as a draft on GitHub, but only from a commit that passed the release check
# (.github/workflows/release-check.yml: every module, PocketBase's suite on our core, ext/jsvm on 32-bit ARM, every
# release target). Run from a clean checkout of main, in sync with origin:
#
#   go run ./.github/extensions release vX.Y.Z      # stamp the changelogs; commit and push to main
#   examples/pocketone/release.sh --check            # start the release check on origin/main (~15-25 min)
#   examples/pocketone/release.sh --status           # how that check is doing
#   examples/pocketone/release.sh vX.Y.Z             # once it passed: tag, push the tag, goreleaser (draft release)
#
# Pushing the tag also publishes the desktop build image, ghcr.io/pocketonehq/pocketone-build:vX.Y.Z
# (.github/workflows/build-image.yml); it can be published again from that workflow's Run workflow button.
#
# Needs gh (signed in) and goreleaser.
set -euo pipefail

repo=pocketonehq/pocketone
workflow=release-check.yml
cd "$(dirname "$0")/../.."

step() { echo "$(date +%T) $*" >&2; }
die() { echo "release: $*" >&2; exit 1; }
# macOS has no timeout unless coreutils is installed; then the limits are skipped.
command -v timeout >/dev/null || timeout() { shift; "$@"; }

# The commit to release: HEAD, which must be origin/main with nothing uncommitted.
in_sync() {
  [ "$(git symbolic-ref --short -q HEAD || true)" = main ] || die "not on main"
  [ -z "$(git status --porcelain)" ] || die "the working tree has changes; commit them first"
  step "fetching origin"
  timeout 30 git fetch -q origin main || die "git fetch origin main failed"
  [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "HEAD is not origin/main; push or pull first"
  git rev-parse HEAD
}

# The newest release-check run on a commit: "<status> <conclusion> <url>", or nothing.
latest_run() {
  timeout 30 gh run list -R "$repo" --workflow "$workflow" --commit "$1" --limit 1 \
    --json status,conclusion,url --jq '.[0] | select(.) | "\(.status) \(.conclusion // "-") \(.url)"'
}

case "${1:-}" in
--check)
  sha=$(in_sync)
  if run=$(latest_run "$sha") && [ -n "$run" ]; then
    read -r status conclusion url <<<"$run"
    if [ "$status" != completed ] || [ "$conclusion" = success ]; then
      step "a release check on ${sha:0:9} is already there ($status $conclusion): $url"
      exit 0
    fi
  fi
  step "starting the release check on main (${sha:0:9})"
  timeout 30 gh workflow run "$workflow" -R "$repo" --ref main
  step "started; examples/pocketone/release.sh --status shows how it's doing"
  exit 0
  ;;
--status)
  sha=$(in_sync)
  run=$(latest_run "$sha") || die "gh run list failed"
  [ -n "$run" ] || { step "no release check on ${sha:0:9} yet; start one with --check"; exit 1; }
  read -r status conclusion url <<<"$run"
  step "release check on ${sha:0:9}: $status $conclusion $url"
  [ "$status" = completed ] && [ "$conclusion" = success ]
  exit
  ;;
v*) ;;
*) die "usage: $0 --check | --status | vX.Y.Z" ;;
esac

version=$1
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || die "not a version: $version"
sha=$(in_sync)

step "checking the release check on ${sha:0:9}"
run=$(latest_run "$sha") || die "gh run list failed"
[ -n "$run" ] || die "no release check on ${sha:0:9}; start one with: examples/pocketone/release.sh --check"
read -r status conclusion url <<<"$run"
[ "$status" = completed ] || die "the release check on ${sha:0:9} is still $status: $url"
[ "$conclusion" = success ] || die "the release check on ${sha:0:9} ended $conclusion: $url"
step "release check passed: $url"

# A tag the Go proxy has seen can never point at another commit, so a version it knows is burned.
step "checking the version is unused"
if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
  [ "$(git rev-parse "$version^{commit}")" = "$sha" ] || die "tag $version already exists on another commit"
fi
code=$(curl -s -m 10 -o /dev/null -w '%{http_code}' "https://proxy.golang.org/pocketone.dev/@v/$version.info" || true)
[ "$code" != 200 ] || [ "$(git rev-parse -q --verify "refs/tags/$version^{commit}" || true)" = "$sha" ] ||
  die "the Go proxy already knows $version; pick the next version"

notes=$(mktemp)
step "release notes for $version"
examples/pocketone/release-notes.sh "$version" >"$notes" || die "no release notes; stamp the changelogs first (go run ./.github/extensions release $version)"
[ -s "$notes" ] || die "the release notes for $version are empty"

git rev-parse -q --verify "refs/tags/$version" >/dev/null || git tag -a "$version" -m "PocketOne $version"
step "pushing tag $version"
timeout 60 git push origin "refs/tags/$version"

step "goreleaser"
export GITHUB_TOKEN="${GITHUB_TOKEN:-$(gh auth token)}"
goreleaser release --config examples/pocketone/.goreleaser.yaml --clean --release-notes "$notes"
step "draft release made for $version, and Formula/pocketone.rb pushed to pockethost/homebrew-tap: publish the draft now (brew install fails until then)"

#!/usr/bin/env bash
# Runs PocketBase's own test suite, exactly as upstream wrote it, against PocketOne's core.
#
#   .github/upstream-tests/run.sh [<upstream base>]
#   .github/upstream-tests/run.sh --as-is      # the core module's tests as they are on this checkout, ours included
#
# The base defaults to the merge base of HEAD and upstream/master (the PocketBase commit main last merged). Every
# *_test.go in the core module is put back to that commit's version: test files we changed are restored, test files we
# added are removed. Then `go test ./...` runs in the root module (core only: ext/, site/sync and examples/pocketone are
# their own modules). Tests listed in known-deviations.txt are skipped; each line there says why the test expects
# upstream's behavior where PocketOne deliberately differs. Everything else must pass.
#
# Leaves the worktree's test files changed; CI runs it on a throwaway checkout. Locally, `git checkout -- . && git clean -fd -- '*_test.go'`
# afterwards. Writes a Markdown summary to $GITHUB_STEP_SUMMARY when set, otherwise to stdout.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
here=.github/upstream-tests

asis=0
if [ "${1:-}" = "--as-is" ]; then asis=1; shift; fi
base=${1:-$(git merge-base HEAD upstream/master)}
echo "$(date +%T) upstream base $(git log -1 --format='%h %s' "$base")"

# Test files in the root module only (skip nested modules and PocketOne-owned folders).
core=(':!ext' ':!site' ':!examples/pocketone' ':!.github' ':!agent' ':!docs')
restored=0; removed=0; skip=""
[ "$asis" = 1 ] || while IFS=$'\t' read -r status file; do
  case "$status" in
    M) git checkout -q "$base" -- "$file"; restored=$((restored+1)); echo "  restored $file" ;;
    A) rm -f "$file"; removed=$((removed+1)); echo "  removed  $file (ours)" ;;
    D) git checkout -q "$base" -- "$file"; restored=$((restored+1)); echo "  restored $file (deleted on main)" ;;
  esac
done < <(git diff --name-status --no-renames "$base" HEAD -- '*_test.go' "${core[@]}")
[ "$asis" = 1 ] && echo "$(date +%T) --as-is: test files as they are on this checkout, nothing skipped" \
  || echo "$(date +%T) $restored upstream test files restored, $removed of ours removed"

[ "$asis" = 1 ] || skip=$(sed 's/#.*//; s/[[:space:]]//g; /^$/d' "$here/known-deviations.txt" | paste -sd'|' -)
skipargs=()
[ -n "$skip" ] && skipargs=(-skip "^($skip)\$")

out=$(mktemp)
echo "$(date +%T) go test ./... (skipping: ${skip:-none})"
set +e
go test -json -count=1 -timeout 20m ${skipargs[@]+"${skipargs[@]}"} ./... >"$out"
code=$?
set -e
echo "$(date +%T) go test exited $code"

# A failed test gets two more tries on its own: a few upstream tests wait a fixed time for events and can miss it when
# the whole suite runs in parallel on a busy machine. One that passes on a retry is listed as flaky and doesn't fail
# the run; one that fails all three times does. A package that fails with no failed test (a build error) always does.
flaky=(); failed=()
if [ "$code" != 0 ]; then
  echo "--- output of failed tests"
  jq -rs 'map(select(.Test)) as $all
    | ($all | map(select(.Action == "fail")) | map(.Package + "\u0000" + .Test)) as $f
    | $all[] | select(.Action == "output" and ((.Package + "\u0000" + .Test) as $k | $f | index($k))) | .Output' "$out" | tr -d '\r'
  broken=$(jq -rs '(map(select(.Test and .Action == "fail")) | map(.Package) | unique) as $t
    | map(select(.Test == null and .Action == "fail") | .Package) | unique | map(select(. as $p | $t | index($p) | not)) | .[]' "$out")
  for p in $broken; do failed+=("$p (no test failed: build or setup error)"); done
  while read -r pkg test; do
    [ -z "$pkg" ] && continue
    ok=0
    for try in 2 3; do
      echo "$(date +%T) retry $try: $pkg $test"
      if go test -count=1 -timeout 5m -run "^${test}\$" "$pkg" >/dev/null 2>&1; then ok=1; break; fi
    done
    if [ "$ok" = 1 ]; then flaky+=("$pkg $test"); echo "  passed on retry"; else failed+=("$pkg $test"); echo "  failed every try"; fi
  done < <(jq -rs 'map(select(.Test and .Action == "fail" and (.Test | contains("/") | not))) | map(.Package + " " + .Test) | unique | .[]' "$out")
  [ ${#failed[@]} = 0 ] && code=0
fi

summary=${GITHUB_STEP_SUMMARY:-/dev/stdout}
{
  jq -rs --arg base "$(git log -1 --format='%h' "$base")" --arg head "$(git rev-parse --short HEAD)" --arg code "$code" --arg asis "$asis" '
    def tests(a): map(select(.Test and .Action == a)) | map(.Package + " " + .Test) | unique;
    (map(select(.Test == null and (.Action == "pass" or .Action == "fail"))) | map(.Package) | unique | length) as $pkgs
    | (if $asis == "1" then "## Core tests, ours included\n", "PocketOne `\($head)`, on PocketBase `\($base)`.\n"
       else "## PocketBase test suite on PocketOne core\n", "PocketOne `\($head)`, upstream tests from PocketBase `\($base)`.\n" end),
      "| Result | Packages | Passed | Failed | Skipped |",
      "|---|---|---|---|---|",
      "| \(if $code == "0" then "pass" else "fail" end) | \($pkgs) | \(tests("pass") | length) | \(tests("fail") | length) | \(tests("skip") | length) |\n",
      "Counts are from the first run, subtests included."
  ' "$out"
  if [ ${#flaky[@]} -gt 0 ]; then echo; echo "Failed, then passed on retry (flaky):"; echo; printf -- '- `%s`\n' "${flaky[@]}"; fi
  if [ ${#failed[@]} -gt 0 ]; then echo; echo "Failed:"; echo; printf -- '- `%s`\n' "${failed[@]}"; fi
} >>"$summary"
exit "$code"

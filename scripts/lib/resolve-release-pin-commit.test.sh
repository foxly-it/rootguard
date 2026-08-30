#!/usr/bin/env bash
# Regression tests for resolve-release-pin-commit.sh, exercised against
# synthetic git histories built in a scratch repo - found in review,
# round 7: the retry-detection logic this replaced was never actually
# reachable (an earlier, still-strict guard rejected every retry before
# it got a chance to run) and, on its own terms, could mistake an
# unrelated commit for the release's own pin commit. Every scenario
# named in that review is covered below.

set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./resolve-release-pin-commit.sh
source "${script_dir}/resolve-release-pin-commit.sh"

failures=0
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

repo="${work_dir}/repo"
mkdir -p "$repo"
cd "$repo"
# -b main: deterministic regardless of the host's init.defaultBranch -
# every scenario below assumes "main" is the checked-out branch from the
# very first commit.
git init -q -b main
git config user.name "test"
git config user.email "test@example.com"

commit() {
  local message="$1"
  shift
  local file
  for file in "$@"; do
    mkdir -p "$(dirname "$file")"
    printf '%s\n' "$RANDOM" >"$file"
    git add "$file"
  done
  git commit -q -m "$message"
  git rev-parse HEAD
}

# assert_resolves source_ref version main_ref expected_sha description
assert_resolves() {
  local source_ref="$1" version="$2" main_ref="$3" expected="$4" description="$5"
  local actual
  if ! actual="$(resolve_release_pin_commit "$source_ref" "$version" "$main_ref" 2>/tmp/resolve-stderr)"; then
    echo "FAIL (${description}): expected success resolving to ${expected}, got failure: $(cat /tmp/resolve-stderr)" >&2
    failures=$((failures + 1))
    return
  fi
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL (${description}): expected ${expected}, got ${actual:-<empty>}" >&2
    failures=$((failures + 1))
  fi
}

# assert_rejects source_ref version main_ref description
assert_rejects() {
  local source_ref="$1" version="$2" main_ref="$3" description="$4"
  if resolve_release_pin_commit "$source_ref" "$version" "$main_ref" >/tmp/resolve-stdout 2>/dev/null; then
    echo "FAIL (${description}): expected failure, got success resolving to $(cat /tmp/resolve-stdout)" >&2
    failures=$((failures + 1))
  fi
}

version="1.2.3"
pin_message="ci: pin compose.release.yaml, .env.release.example, and site/*.html to ${version} [skip ci]"

# --- Scenario 1: first release attempt - main is exactly SOURCE_REF ---
source_ref="$(commit "feat: something releasable" src/main.go)"
assert_resolves "$source_ref" "$version" main "" "first attempt"

# --- Scenario 2/3: retry directly after the pin commit (git-state
# identical whether promotion partially succeeded before the retry or
# not - promotion never touches git history) ---
pin_commit="$(commit "$pin_message" compose.release.yaml .env.release.example site/index.html)"
assert_resolves "$source_ref" "$version" main "$pin_commit" "retry right after the pin commit / after partial promotion"

# --- Scenario 4: retry after main has moved further still (an unrelated
# commit landed after the pin commit) - must find the pin commit, not
# main's new tip ---
commit "docs: unrelated change" README.md >/dev/null
assert_resolves "$source_ref" "$version" main "$pin_commit" "retry after main moved further past the pin commit"

# --- Scenario 5: a foreign commit that happens to touch the same three
# paths, but isn't this release's own pin commit (different message) -
# must be rejected, not silently accepted by content alone ---
git checkout -q -b foreign-same-paths "$source_ref"
commit "chore: unrelated pin-file edit" compose.release.yaml .env.release.example >/dev/null
git branch -f main foreign-same-paths
git checkout -q main
git branch -D foreign-same-paths >/dev/null
assert_rejects "$source_ref" "$version" main "foreign commit touching the same paths, different message"

# A revert of a real pin commit auto-generates a message that *contains*
# the original as a quoted substring - the exact-subject check (not just
# --grep's substring pre-filter) must still reject it.
git checkout -q -b revert-message-substring "$source_ref"
commit "Revert \"${pin_message}\"" compose.release.yaml >/dev/null
git branch -f main revert-message-substring
git checkout -q main
git branch -D revert-message-substring >/dev/null
assert_rejects "$source_ref" "$version" main "commit message containing the pin message as a substring, not equal to it"

# --- Scenario 6a: right message, but a merge commit (two parents) ---
git checkout -q -b merge-candidate "$source_ref"
commit "chore: some other branch" other.txt >/dev/null
git checkout -q -b merge-candidate2 "$source_ref"
git merge -q --no-ff -m "$pin_message" merge-candidate
git branch -f main merge-candidate2
git checkout -q main
git branch -D merge-candidate merge-candidate2 >/dev/null
assert_rejects "$source_ref" "$version" main "right message but a merge commit (two parents)"

# --- Scenario 6b: right message, single parent, but that parent isn't
# SOURCE_REF (built on top of untested content) ---
git checkout -q -b wrong-parent-base "$source_ref"
commit "chore: an extra untested commit" extra.txt >/dev/null
commit "$pin_message" compose.release.yaml >/dev/null
git branch -f main wrong-parent-base
git checkout -q main
git branch -D wrong-parent-base >/dev/null
assert_rejects "$source_ref" "$version" main "right message, single parent, but not a direct child of SOURCE_REF"

# --- Scenario 7: right message, direct child of SOURCE_REF, but it also
# touches a path outside compose.release.yaml/.env.release.example/site/ ---
git checkout -q -b out-of-scope "$source_ref"
commit "$pin_message" compose.release.yaml unrelated/other.go >/dev/null
git branch -f main out-of-scope
git checkout -q main
git branch -D out-of-scope >/dev/null
assert_rejects "$source_ref" "$version" main "right message and parentage, but touches an out-of-scope path"

if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} resolve-release-pin-commit.sh test failure(s)" >&2
  exit 1
fi
echo "All resolve-release-pin-commit.sh tests passed."

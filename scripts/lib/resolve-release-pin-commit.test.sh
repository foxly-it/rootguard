#!/usr/bin/env bash
# Regression tests for resolve-release-pin-commit.sh, exercised against
# synthetic git histories built in a scratch repo.
#
# Found in review, round 7: the retry-detection logic this replaced was
# never actually reachable (an earlier, still-strict guard rejected
# every retry before it got a chance to run) and, on its own terms,
# could mistake an unrelated commit for the release's own pin commit.
#
# Found in review, round 8: round 7's own identity-based fix (message,
# parentage, path scope) proved a candidate commit *was* genuinely this
# release's pin commit, but never checked whether main's current tip
# still reflected it - a `git revert` of that exact commit, or any
# later commit touching the same paths again, left the original
# commit's identity untouched while silently invalidating it as a safe
# retry point.
#
# Every scenario named in both reviews is covered below.

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
  if ! actual="$(resolve_release_pin_commit "$source_ref" "$version" "$main_ref" 2>"${work_dir}/resolve-stderr")"; then
    echo "FAIL (${description}): expected success resolving to ${expected}, got failure: $(cat "${work_dir}/resolve-stderr")" >&2
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
  if resolve_release_pin_commit "$source_ref" "$version" "$main_ref" >"${work_dir}/resolve-stdout" 2>/dev/null; then
    echo "FAIL (${description}): expected failure, got success resolving to $(cat "${work_dir}/resolve-stdout")" >&2
    failures=$((failures + 1))
  fi
}

version="1.2.3"
pin_message="$(release_pin_commit_message "$version")"

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

# --- Scenario 8 (round 8): a real pin commit, then `git revert`ed - the
# exact scenario found live: a maintainer reverting a pin commit after a
# failed promotion, then retrying, must not resurrect the reverted pins ---
git checkout -q -b revert-real-pin "$source_ref"
real_pin="$(commit "$pin_message" compose.release.yaml .env.release.example site/index.html)"
git revert --no-edit "$real_pin" >/dev/null
git branch -f main revert-real-pin
git checkout -q main
git branch -D revert-real-pin >/dev/null
assert_rejects "$source_ref" "$version" main "a real pin commit that was later git revert-ed"

# --- Scenario 9 (round 8): a same-message --allow-empty commit - passes
# message, parentage, and scope (there's nothing to be "out of scope"),
# but never actually pins anything ---
git checkout -q -b empty-pin "$source_ref"
git commit -q --allow-empty -m "$pin_message"
git branch -f main empty-pin
git checkout -q main
git branch -D empty-pin >/dev/null
assert_rejects "$source_ref" "$version" main "an empty --allow-empty commit with the right message"

# --- Scenario 10 (round 8): a valid pin commit, then a *later*, normally
# authored commit changes the pin files again (not a revert, just drift)
# - same underlying risk as scenario 8, different cause ---
git checkout -q -b pin-then-drift "$source_ref"
commit "$pin_message" compose.release.yaml .env.release.example site/index.html >/dev/null
commit "chore: hand-edit the release pins" compose.release.yaml >/dev/null
git branch -f main pin-then-drift
git checkout -q main
git branch -D pin-then-drift >/dev/null
assert_rejects "$source_ref" "$version" main "a valid pin commit whose paths were changed again afterward"

# --- Scenario 11 (round 8): the pin commit exists, but isn't reachable
# from main at all (main was force-pushed to a history that never
# included it) - must be rejected the same as "not found", not crash ---
git checkout -q -b force-pushed-away "$source_ref"
commit "$pin_message" compose.release.yaml .env.release.example site/index.html >/dev/null
git checkout -q -b force-push-replacement "$source_ref"
commit "chore: a completely different history" other.txt >/dev/null
git branch -f main force-push-replacement
git checkout -q main
git branch -D force-pushed-away force-push-replacement >/dev/null
assert_rejects "$source_ref" "$version" main "the pin commit exists in the repo but was force-pushed out of main's history"

if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} resolve-release-pin-commit.sh test failure(s)" >&2
  exit 1
fi
echo "All resolve-release-pin-commit.sh tests passed."

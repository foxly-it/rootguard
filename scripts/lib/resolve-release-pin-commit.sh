#!/usr/bin/env bash
# Shared by release-alpha.yml's update-alpha-pins job (both the early
# main-race guard and "Commit updated pins" itself) - finds the exact
# prior pin commit for a release, if one already exists, so a retried
# workflow run can resume after origin/main has moved past the commit
# this release was tested against (SOURCE_REF), without ever mistaking
# an unrelated commit for its own.
#
# Found in review, round 6: the fix that shipped that round compared the
# *file content* of compose.release.yaml/.env.release.example/site/ at
# origin/main's current tip against what the run had just regenerated -
# true for the release's own earlier pin commit, but equally true for
# any later, unrelated commit that simply never touched those paths.
# That silently resolved to an untested commit as the release's pin/tag
# target - and the early guard (a separate, still-strict SOURCE_REF
# equality check) ran *before* this logic ever got a chance to run at
# all, so the documented retry path was unreachable in the first place.
# Both bugs found live: reproducible by pushing a same-message pin
# commit, then a second, unrelated commit, to a scratch main.
#
# Found in review, round 7's own fix (round 8's review): identifying the
# commit by its unique message, direct single-parent-of-SOURCE_REF
# shape, and path scope proves *that commit* is genuinely this release's
# own pin commit - it does not prove main's current tip still reflects
# it. Git history is immutable: a `git revert` of that exact commit, or
# any later commit touching these paths again, leaves the original
# commit's message/parentage/scope untouched while silently invalidating
# it as a safe retry point. Confirmed live: reverting a real pin commit
# still let it resolve here and get promoted/tagged/released - main had
# explicitly taken the release pins back, and this function said "safe
# to reuse" anyway. Fixed with one more check below, purely against git
# refs (no working-tree regeneration needed, so this still runs safely
# from the early guard, before the caller has generated anything).
#
# Exercised against synthetic git histories in
# resolve-release-pin-commit.test.sh, run by ci.yml.
#
# Usage: source this file, then call
#   resolve_release_pin_commit "$source_ref" "$version" "$main_ref"
# from inside a git worktree with $main_ref already fetched. Echoes the
# resolved pin commit sha on success - empty if $main_ref already equals
# $source_ref exactly (the ordinary first-attempt case, nothing to
# resolve). Exits non-zero with a message on stderr if $main_ref has
# moved away from $source_ref without a uniquely identifiable, validly
# shaped, still-current pin commit for $version in between - the caller
# should treat that as "abort this release", the same as it always has.

# release_pin_commit_message version -> echoes the exact commit message
# "Commit updated pins" always uses, and this resolver searches for -
# found in review, round 7: this string used to be duplicated by hand
# across this file, release-alpha.yml, and this file's own test, with a
# comment saying "keep in sync by hand" - a later text change to any one
# of them would have silently broken retry detection instead of failing
# loudly. One function now, called from all three.
release_pin_commit_message() {
  echo "ci: pin compose.release.yaml, .env.release.example, and site/*.html to $1 [skip ci]"
}

resolve_release_pin_commit() {
  local source_ref version main_ref
  # Found in review: source_ref used to be compared as a raw string
  # throughout this function - fine for the normal case (release-alpha.yml
  # always passes the full 40-character SHA via needs.version.outputs -
  # see the "version" job's own source_ref output), but a manual
  # workflow_dispatch typing an abbreviated SHA into its source_sha input
  # (the one documented by-hand escape hatch) would still point at the
  # exact same commit while failing every string-equality check here.
  # Normalizing once, up front, means the rest of this function never has
  # to think about it again.
  source_ref="$(git rev-parse "${1}^{commit}")"
  version="$2"
  main_ref="$3"
  local main_sha
  main_sha="$(git rev-parse "$main_ref")"

  if [[ "$main_sha" == "$source_ref" ]]; then
    echo ""
    return 0
  fi

  local message
  message="$(release_pin_commit_message "$version")"

  # --grep is a fast pre-filter (it matches a commit whose message merely
  # *contains* $message, e.g. a `git revert`'s auto-generated message
  # quoting the original) - the per-candidate exact-subject comparison
  # below is what actually decides.
  local candidates sha subject
  local -a exact=()
  candidates="$(git log "$main_ref" --fixed-strings --grep="$message" --format=%H)"
  for sha in $candidates; do
    subject="$(git log -1 --format=%s "$sha")"
    if [[ "$subject" == "$message" ]]; then
      exact+=("$sha")
    fi
  done

  if [[ ${#exact[@]} -ne 1 ]]; then
    echo "${main_ref} has moved to ${main_sha}, which wasn't tested (expected ${source_ref}) - found ${#exact[@]} commit(s) with this exact release's own pin-commit message on ${main_ref}, need exactly 1 to treat this as a safe retry" >&2
    return 1
  fi

  local candidate="${exact[0]}"

  # Must be a direct, single-parent child of source_ref - not a merge
  # commit, and not merely *descended from* it through other commits
  # (that would mean this "pin commit" itself was built on top of
  # content this release never tested).
  local parents parent_count candidate_parent
  parents="$(git rev-list --parents -n1 "$candidate")"
  parent_count=$(($(wc -w <<<"$parents") - 1))
  candidate_parent="$(awk '{print $2}' <<<"$parents")"
  if [[ "$parent_count" -ne 1 || "$candidate_parent" != "$source_ref" ]]; then
    echo "${candidate} matches this release's own pin-commit message, but isn't a direct single-parent child of ${source_ref} (parents:${parents#"$candidate"}) - refusing to treat it as a safe retry point" >&2
    return 1
  fi

  # Must touch only the paths the real pin-commit step ever writes -
  # defense in depth against a forged or corrupted commit that happens
  # to match both the message and the parentage checks above.
  local out_of_scope
  out_of_scope="$(git diff --name-only "${source_ref}" "${candidate}" | grep -Ev '^(compose\.release\.yaml|\.env\.release\.example|site/)' || true)"
  if [[ -n "$out_of_scope" ]]; then
    echo "${candidate} touches paths outside compose.release.yaml/.env.release.example/site/ - refusing to treat it as this release's own pin commit: ${out_of_scope}" >&2
    return 1
  fi

  # Must actually pin something - a genuine pin commit always changes
  # compose.release.yaml and .env.release.example, since VERSION (part
  # of every image reference they pin) differs from whatever the
  # previously published release used. Rejects e.g. a same-message
  # `git commit --allow-empty` that would otherwise sail through every
  # check above with nothing to show for it.
  local pinned_paths
  pinned_paths="$(git diff --name-only "${source_ref}" "${candidate}" -- compose.release.yaml .env.release.example)"
  if [[ -z "$pinned_paths" ]]; then
    echo "${candidate} matches this release's own pin-commit message and shape, but changes neither compose.release.yaml nor .env.release.example relative to ${source_ref} - refusing to treat an empty commit as a real pin commit" >&2
    return 1
  fi

  # Must still be current: message, parentage, and scope all describe
  # the commit *as it was made* - none of them notice a later commit
  # that changed these same paths again (a `git revert` included, since
  # reverting doesn't rewrite the original commit, it adds a new one on
  # top of it). Compare the candidate's own tree for these paths against
  # main_ref's *current* tip - a pure git-ref comparison, no working-tree
  # regeneration needed, so this is safe to call before "Update digest
  # pins" has even run.
  if ! git diff --quiet "${candidate}" "${main_ref}" -- compose.release.yaml .env.release.example site/; then
    echo "${candidate} matches this release's own pin-commit message, shape, and scope, but ${main_ref}'s current tip (${main_sha}) no longer matches its content for compose.release.yaml/.env.release.example/site/ - something (a revert, a manual edit, or another process) changed these paths again since. Refusing to reuse a superseded pin commit" >&2
    return 1
  fi

  echo "$candidate"
}

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
# Fixed by identifying the release's own commit precisely instead of by
# content: its commit message is unique to this exact version (nothing
# else in the system ever produces it), and its shape is independently
# verified - direct single-parent child of SOURCE_REF, touching only the
# three paths the real pin-commit step ever writes - rather than trusting
# either the message or the content alone. Exercised against synthetic
# git histories in resolve-release-pin-commit.test.sh, run by ci.yml.
#
# Usage: source this file, then call
#   resolve_release_pin_commit "$source_ref" "$version" "$main_ref"
# from inside a git worktree with $main_ref already fetched. Echoes the
# resolved pin commit sha on success - empty if $main_ref already equals
# $source_ref exactly (the ordinary first-attempt case, nothing to
# resolve). Exits non-zero with a message on stderr if $main_ref has
# moved away from $source_ref without a uniquely identifiable, validly
# shaped pin commit for $version in between - the caller should treat
# that as "abort this release", the same as it always has.

resolve_release_pin_commit() {
  local source_ref="$1" version="$2" main_ref="$3"
  local main_sha
  main_sha="$(git rev-parse "$main_ref")"

  if [[ "$main_sha" == "$source_ref" ]]; then
    echo ""
    return 0
  fi

  # This exact string is also what "Commit updated pins" itself commits
  # with - keep the two in sync by hand if that message ever changes.
  local message="ci: pin compose.release.yaml, .env.release.example, and site/*.html to ${version} [skip ci]"

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

  echo "$candidate"
}

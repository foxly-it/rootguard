#!/usr/bin/env bash
# Decides whether a vulnerability scan gates the current run (#719).
# Upstream vulnerability data changes daily on its own; between 2026-09-21
# and 2026-10-16 nearly every red PR was red for that reason alone. So a
# scan gates on push, schedule, dispatch and release runs, but on a pull
# request only when the PR touches what the scan covers: the given globs,
# .trivyignore.yaml, scripts/ci/*, the trivy DB cache action, or the
# calling workflow file. Otherwise the caller reports findings without
# failing; main's push and nightly scans still gate on them.
#
# Usage: pr-gate.sh <glob>...   (globs relative to the repository root)
# Exit codes: 0 = gate, 1 = report only. Any doubt (e.g. the changed-file
# list can't be fetched) means gate.
set -Eeuo pipefail

[[ "${GITHUB_EVENT_NAME:-}" == "pull_request" ]] || exit 0

pr="${GITHUB_REF#refs/pull/}"
pr="${pr%%/*}"
# owner/repo/.github/workflows/<file>@<ref> -> .github/workflows/<file>
workflow="${GITHUB_WORKFLOW_REF#*/*/}"
workflow="${workflow%%@*}"

if ! changed="$(gh pr diff "${pr}" --name-only --repo "${GITHUB_REPOSITORY}" 2>/dev/null)"; then
  echo "pr-gate: could not list the files changed by PR #${pr} - gating to be safe" >&2
  exit 0
fi

patterns=("$@" ".trivyignore.yaml" "scripts/ci/*" ".github/actions/trivy-db-cache/*" "${workflow}")
while read -r file; do
  for pattern in "${patterns[@]}"; do
    # shellcheck disable=SC2053 # unquoted on purpose: glob match
    if [[ -n "${file}" && -n "${pattern}" && "${file}" == ${pattern} ]]; then
      echo "pr-gate: ${file} matches ${pattern} - gating" >&2
      exit 0
    fi
  done
done <<<"${changed}"

echo "pr-gate: PR #${pr} touches none of: ${patterns[*]} - reporting only" >&2
exit 1

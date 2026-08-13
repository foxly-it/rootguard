#!/usr/bin/env bash
# Checks the checkable facts in site/*.html against reality: every mentioned
# RootGuard alpha version string matches the latest real release tag, and
# every local (non-http) href/src actually resolves to a file in the repo.
# Deliberately does not attempt to judge prose accuracy - see ROADMAP.md 0.6
# ("Website status and Wiki updated as a required CI/release check"), scoped
# to hard, checkable facts rather than content review.

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_dir}"

latest_tag="$(git tag --list 'v0.1.0-alpha.*' | sort -t. -k4 -n | tail -1)"
if [[ -z "${latest_tag}" ]]; then
  echo "No v0.1.0-alpha.* tag found - cannot determine the current version" >&2
  exit 1
fi
latest_version="${latest_tag#v}"
echo "Current release: ${latest_version}"

failures=0

echo
echo "== Version references =="
# Skips whole lines matching "Ab/ab/Starting with/required from 0.1.0-alpha.N"
# - phrasing that deliberately names the version a feature shipped in, not a
# claim that it's the current one (e.g. "Starting with 0.1.0-alpha.2, ...").
# Line-level, not lookbehind, to stay portable across BSD and GNU grep - the
# bilingual data-de/data-en pair for a historical reference always carries
# the marker phrase on the same physical (minified, one-element-per-line)
# line as the version number itself.
historical_reference_pattern='([Aa]b |Starting with |required from )0\.1\.0-alpha\.[0-9]+'
version_matches=""
for file in site/*.html; do
  matches="$(grep -vE "${historical_reference_pattern}" "${file}" | grep -Eo '0\.1\.0-alpha\.[0-9]+' | sed "s#^#${file}:#" || true)"
  if [[ -n "${matches}" ]]; then
    version_matches+="${matches}"$'\n'
  fi
done
while IFS=: read -r file match; do
  [[ -z "${file}" ]] && continue
  if [[ "${match}" != "${latest_version}" ]]; then
    echo "STALE  ${file}: mentions ${match}, latest is ${latest_version}" >&2
    failures=$((failures + 1))
  fi
done <<< "${version_matches}"

echo
echo "== Local link/asset references =="
while IFS= read -r reference; do
  file="${reference%%|*}"
  target="${reference#*|}"
  # Only local, root- or file-relative references - never external URLs,
  # mailto:, or fragment-only anchors (#section), which aren't files here.
  case "${target}" in
    http://*|https://*|mailto:*|tel:*|\#*|"") continue ;;
  esac
  target="${target%%#*}"
  resolved="site/${target#/}"
  if [[ ! -f "${resolved}" ]]; then
    echo "MISSING ${file}: references ${target} (resolved ${resolved})" >&2
    failures=$((failures + 1))
  fi
done < <(grep -rEo '(href|src)="[^"]*"' site/*.html | sed -E 's#^(site/[^:]+):(href|src)="([^"]*)"#\1|\3#')

echo
if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} stale or broken reference(s) found in site/*.html" >&2
  exit 1
fi
echo "All checked site facts are current."

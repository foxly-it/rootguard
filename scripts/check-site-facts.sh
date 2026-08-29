#!/usr/bin/env bash
# Checks the checkable facts in site/*.html and README.md against reality:
# every mentioned RootGuard version string matches the latest real release
# tag, and every local (non-http) href/src actually resolves to a file in
# the repo. Deliberately does not attempt to judge prose accuracy - see
# ROADMAP.md 0.6 ("Website status and Wiki updated as a required CI/release
# check"), scoped to hard, checkable facts rather than content review.
#
# README.md joined the site/*.html set here after a follow-up review found
# it silently stale for a long time (still pointing new users at
# v0.1.0-beta.1's compose.alpha.yaml while site/*.html had long since moved
# on to the current release under the compose.release.yaml name) - nothing
# checked it because it lives outside site/. The local link/asset check
# below is deliberately still site/*.html-only: README.md's relative links
# resolve against the repo root, a different base than site/*.html's, and
# it has none of the kind that check exists for (a broken local
# image/stylesheet reference on the public site).

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_dir}"

# shellcheck source=version-pattern.sh
. "${repository_dir}/scripts/version-pattern.sh"

# Ranked by creation date (not sort -V), so a later-cut stable tag
# correctly outranks an earlier rc for the same release without needing
# install.sh's sort -V precedence workaround.
latest_tag="$(git for-each-ref 'refs/tags/v*' --sort=-creatordate --format='%(refname:short)' \
  | grep -E "^v${rootguard_version_pattern}\$" | head -1)"
if [[ -z "${latest_tag}" ]]; then
  echo "No release tag found - cannot determine the current version" >&2
  exit 1
fi
latest_version="${latest_tag#v}"
echo "Current release: ${latest_version}"

failures=0

echo
echo "== Version references =="
# Skips whole lines matching a version reference that names a boundary,
# not a claim about the current release: "Ab/ab/Starting with/required
# from 0.1.0-alpha.N" (a feature shipped starting there) and "bis
# einschließlich/up to and including 0.1.0-beta.14" (a bug applied up to
# and including there - added for the beta.14 -> 1.0.0-rc.1 transition
# note, the same kind of historical fact in the other direction). Line-
# level, not lookbehind, to stay portable across BSD and GNU grep - the
# bilingual data-de/data-en pair for a historical reference always carries
# the marker phrase on the same physical (minified, one-element-per-line)
# line as the version number itself. Safe to use the bare-capable pattern
# here even though the extraction below needs extra care (see
# rootguard_extract_versions): the required marker phrase immediately
# before it already rules out SVG/IP/date noise matching by accident.
historical_reference_pattern="([Aa]b |Starting with |required from |bis einschließlich |up to and including )${rootguard_version_pattern}"
version_matches=""
for file in site/*.html README.md; do
  matches="$(grep -vE "${historical_reference_pattern}" "${file}" | rootguard_extract_versions | sed "s#^#${file}:#" || true)"
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
  echo "${failures} stale or broken reference(s) found in site/*.html or README.md" >&2
  exit 1
fi
echo "All checked site facts are current."

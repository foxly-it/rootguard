#!/usr/bin/env bash
# Checks the checkable facts in site/*.html against reality: every mentioned
# RootGuard version string matches the latest real release tag, and every
# local (non-http) href/src actually resolves to a file in the repo.
# Deliberately does not attempt to judge prose accuracy - see ROADMAP.md 0.6
# ("Website status and Wiki updated as a required CI/release check"), scoped
# to hard, checkable facts rather than content review.

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_dir}"

# Requires a prerelease suffix starting with a letter (the "-word.N" part)
# rather than matching any bare X.Y.Z - RootGuard is still pre-1.0, every
# real release has one, and without that requirement this also matches
# plain dotted-number sequences with no relation to a RootGuard version at
# all: caught live, this pattern without the leading-letter requirement
# matched fragments of the GitHub icon's own SVG path coordinate data
# ("...1.3.8-1.6-2.7-.3-5.5-1.3-5.5-5.9...") as fake "1.3.8-1.6" version
# references. Generalized past "alpha|beta" specifically (accepts "rc", a
# letter revision, ...) so a prerelease scheme change doesn't silently stop
# this script from finding anything.
#
# Accepted gap once a stable release exists: this pattern still can't
# positively match a bare "1.0.0" prose mention (no suffix to require),
# so a *correctly* updated stable-version mention becomes invisible to
# this check rather than confirmed - it'll still correctly flag a
# leftover stale prerelease mention as stale (that comparison happens
# below, against latest_version, which tag_version_pattern below can be
# bare). Not widened without the false-positive risk above being solved
# first, and there's no real stable-release site copy to design that
# against yet - revisit once one exists.
version_pattern='[0-9]+\.[0-9]+\.[0-9]+-[A-Za-z][0-9A-Za-z]*(\.[0-9A-Za-z]+)*'

# Tag resolution needs a *different, wider* pattern than the prose scan
# below: the site's own release-pipeline (release-alpha.yml) and the Core
# now both support a stable release with no prerelease suffix at all
# ("1.0.0"), so a real stable tag must still be found here - unlike
# version_pattern above, whose mandatory suffix exists to keep the prose
# scan from matching bare-number noise (SVG path coordinates, AdGuard's
# own "v0.107.78", ...) that has nothing to do with a RootGuard version.
# That false-positive risk doesn't apply to real git tags, so it's safe to
# make the suffix optional here. Ranked by creation date (not sort -V), so
# a later-cut stable tag correctly outranks an earlier rc for the same
# release without needing install.sh's sort -V precedence workaround.
tag_version_pattern='[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z][0-9A-Za-z]*(\.[0-9A-Za-z]+)*)?'
latest_tag="$(git for-each-ref 'refs/tags/v*' --sort=-creatordate --format='%(refname:short)' \
  | grep -E "^v${tag_version_pattern}\$" | head -1)"
if [[ -z "${latest_tag}" ]]; then
  echo "No release tag found - cannot determine the current version" >&2
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
historical_reference_pattern="([Aa]b |Starting with |required from )${version_pattern}"
version_matches=""
for file in site/*.html; do
  matches="$(grep -vE "${historical_reference_pattern}" "${file}" | grep -Eo "${version_pattern}" | sed "s#^#${file}:#" || true)"
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

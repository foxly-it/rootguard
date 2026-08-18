#!/usr/bin/env bash
# Mechanically refreshes site/*.html's version references to the latest
# release tag - the exact thing scripts/check-site-facts.sh checks for,
# and the thing that went stale unnoticed for two days after both
# v0.1.0-beta.2 and v0.1.0-beta.3 shipped, because the release pipeline
# updates compose.alpha.yaml/.env.alpha.example automatically but never
# touched the site. Meant to run as an automated step right after a
# release tag is cut (see release-alpha.yml's update-alpha-pins job),
# committed alongside the compose/.env pin update.
#
# Idempotent: running it again with nothing stale is a silent no-op.

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_dir}"

latest_tag="$(git for-each-ref 'refs/tags/v0.1.0-*' --sort=-creatordate --format='%(refname:short)' \
  | grep -E '^v0\.1\.0-(alpha|beta)\.[0-9]+$' | head -1)"
if [[ -z "${latest_tag}" ]]; then
  echo "No v0.1.0-alpha.*/beta.* tag found - cannot determine the current version" >&2
  exit 1
fi
latest_version="${latest_tag#v}"

# Same exclusion as check-site-facts.sh: a line naming the version a
# feature shipped in ("Starting with 0.1.0-beta.1, ...") is a historical
# fact, not a current-version claim, and must never be bumped.
historical_reference_pattern='([Aa]b |Starting with |required from )0\.1\.0-(alpha|beta)\.[0-9]+'
# The two docs.html .env-example lines carry a version *and* a digest -
# a plain version-string substitution would leave the old digest in
# place, so they're excluded here and handled explicitly below instead.
update_image_line_pattern='ROOTGUARD_(CORE|WEBAPP)_UPDATE_IMAGE='

changed_files=()
for file in site/*.html; do
  stale_versions="$(grep -vE "${historical_reference_pattern}" "${file}" \
    | grep -vE "${update_image_line_pattern}" \
    | grep -Eo '0\.1\.0-(alpha|beta)\.[0-9]+' | sort -u || true)"
  file_changed=0
  for stale in ${stale_versions}; do
    [[ "${stale}" == "${latest_version}" ]] && continue
    awk -v old="${stale}" -v new="${latest_version}" \
        -v hist="${historical_reference_pattern}" -v img="${update_image_line_pattern}" '
      $0 !~ hist && $0 !~ img { gsub(old, new) }
      { print }
    ' "${file}" > "${file}.tmp"
    mv "${file}.tmp" "${file}"
    echo "Bumped ${file}: ${stale} -> ${latest_version}"
    file_changed=1
  done
  [[ "${file_changed}" == 1 ]] && changed_files+=("${file}")
done

# docs.html's embedded .env example must mirror the real update-target
# images exactly, digest included - substitute the whole line rather than
# just the version substring.
docs_file="site/docs.html"
if [[ -f "${docs_file}" ]]; then
  for var in ROOTGUARD_CORE_UPDATE_IMAGE ROOTGUARD_WEBAPP_UPDATE_IMAGE; do
    real_line="$(grep "^${var}=" .env.alpha.example || true)"
    [[ -z "${real_line}" ]] && continue
    if ! grep -qF "${real_line}" "${docs_file}"; then
      awk -v var="${var}=" -v replacement="${real_line}" '
        index($0, var) { sub(var "[^\"<]*", replacement) }
        { print }
      ' "${docs_file}" > "${docs_file}.tmp"
      mv "${docs_file}.tmp" "${docs_file}"
      echo "Bumped ${docs_file}: ${var} -> matches .env.alpha.example"
      changed_files+=("${docs_file}")
    fi
  done
fi

if [[ "${#changed_files[@]}" -eq 0 ]]; then
  echo "site/*.html already reflects ${latest_version} - nothing to do"
else
  echo "Updated: $(printf '%s ' "${changed_files[@]}" | tr ' ' '\n' | sort -u | tr '\n' ' ')"
fi

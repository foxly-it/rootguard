#!/usr/bin/env bash
# Checks every local (non-http) markdown link across the whole repository
# actually resolves to a real file, relative to the linking file's own
# directory - the same kind of check check-site-facts.sh already runs for
# site/*.html, but that one is deliberately scoped to the public website
# and never looks at README.md/CONTRIBUTING.md/SECURITY.md or any other
# markdown file anywhere else in the repo.
#
# Found in review: PR #423 deleted the five per-component SECURITY.md
# files (dead monorepo-migration leftovers, each pointing at its own
# archived per-component repo's advisory page) on the stated reasoning
# that nothing linked to them - confirmed with a narrow grep for the
# deleted paths specifically, but never checked markdown links generally.
# At least eight files (each component's own README.md/CONTRIBUTING.md)
# turned out to link to their own now-deleted SECURITY.md via a plain,
# unqualified, unlabeled-as-code reference - correct before the deletion,
# broken after it, and nothing caught it. This check exists so that
# specific class of mistake - and any other broken local markdown link,
# by any cause - fails CI immediately instead of silently shipping.
#
# Strips fenced code blocks and inline code spans before searching for
# links - found live, on this exact script's own first CI run: this very
# audit log's own prose, describing the finding above, quotes what the
# broken reference used to look like inside a backtick code span
# specifically so it renders as literal text, not a real link - a naive
# raw-text search over the whole file misread that quoted example as a
# second real link and reported it as broken (it was never a link at
# all). Markdown code spans/blocks are exactly the place literal
# "](...)"-shaped example text legitimately shows up without being a
# real link.

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_dir}"

failures=0

while IFS= read -r -d '' file; do
  dir="$(dirname "$file")"
  # Matches [text](target) - not ![text](target) (an image, checked the
  # same way regardless, the leading "!" is simply part of what "text"
  # captures here) - deliberately not distinguishing the two since a
  # broken image reference is just as real a problem as a broken link.
  while IFS= read -r target; do
    [[ -z "$target" ]] && continue
    case "$target" in
      http://* | https://* | mailto:* | tel:* | '#'*) continue ;;
    esac
    # Strip a trailing #fragment - same reasoning as
    # check-site-facts.sh's own local-link check: a fragment identifies a
    # heading within the target file, not a separate file to resolve.
    target="${target%%#*}"
    [[ -z "$target" ]] && continue
    resolved="${dir}/${target}"
    if [[ ! -e "$resolved" ]]; then
      echo "MISSING ${file}: links to ${target} (resolved ${resolved})" >&2
      failures=$((failures + 1))
    fi
  done < <(
    awk '/^```/ { infence = !infence; next } infence { next } { print }' "$file" \
      | sed -E 's/`[^`]*`//g' \
      | grep -oE '\]\([^)]+\)' \
      | sed -E 's/^\]\((.*)\)$/\1/'
  )
done < <(git ls-files -z -- '*.md')

if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} broken local markdown link(s) found" >&2
  exit 1
fi
echo "All local markdown links resolve."

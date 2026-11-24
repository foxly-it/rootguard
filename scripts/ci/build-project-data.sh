#!/usr/bin/env bash
# Builds the website's project-data.json from three GitHub API responses and
# prints it.
#
# usage: build-project-data.sh <releases.json> <pulls.json> <commits.json>
#
# The responses come in as files (--slurpfile), never as arguments. Linux caps
# one argument at 128 KiB, and the releases response outgrew that with v1.0.15:
# from 2026-11-19 on, pages.yml's inline `jq --argjson` died with "Argument list
# too long" in every run. The step is continue-on-error, so the runs stayed
# green and the site kept serving the committed copy from 2026-08-01.
set -Eeuo pipefail

if (( $# != 3 )); then
  echo "usage: $0 <releases.json> <pulls.json> <commits.json>" >&2
  exit 2
fi

jq -n \
  --arg generated_at "$(date -u +%FT%TZ)" \
  --slurpfile releases_response "$1" \
  --slurpfile pulls_response "$2" \
  --slurpfile commits_response "$3" \
  '
  # An API error comes back as an object. Fail on it instead of publishing it.
  def list($name): if length == 1 and (.[0] | type) == "array" then .[0]
    else error("\($name): expected one JSON array") end;
  ($releases_response | list("releases")) as $releases
  | ($pulls_response | list("pulls")) as $pulls
  | ($commits_response | list("commits")) as $commits
  | {
      generated_at: $generated_at,
      current_version: (($releases | map(select(.draft == false)) | first | .tag_name) // "unreleased"),
      pull_requests: ($pulls | map({
        number: .number,
        title: .title,
        updated_at: .updated_at,
        html_url: .html_url
      })),
      commits: ($commits | map({
        sha: .sha[0:7],
        message: (.commit.message | split("\n")[0]),
        date: .commit.author.date,
        html_url: .html_url
      })),
      releases: ($releases | map(((.body // "") | split("\n")) as $lines | {
        tag: .tag_name,
        name: (.name // .tag_name),
        published_at: .published_at,
        html_url: .html_url,
        prerelease: .prerelease,
        summary: (([$lines[] | select(
          length > 0 and
          (startswith("#") | not) and
          (startswith("- ") | not) and
          (startswith("```") | not)
        )][0]) // ""),
        highlights: ([$lines[] | select(startswith("- ")) | ltrimstr("- ")][0:4])
      }))
    }'

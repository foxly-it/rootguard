#!/usr/bin/env bash
set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }
build() { "$repository_dir/scripts/ci/build-project-data.sh" "$tmp/releases.json" "$tmp/pulls.json" "$tmp/commits.json"; }
check() { jq -e "$1" "$tmp/out.json" >/dev/null || fail "$2"; }

# 100 releases of 4 KB notes: well past the 128 KiB Linux allows per argument,
# which is what broke the website's data from v1.0.15 on.
jq -n '[range(100) | {
  tag_name: "v1.0.\(100 - .)", name: null, draft: (. == 0), prerelease: false,
  published_at: "2026-11-22T07:36:00Z", html_url: "https://example.invalid/\(.)",
  body: ("## Head\n\nSummary \(.)\n- one\n- two\n- three\n- four\n- five\n" + ("x" * 4000))
}]' >"$tmp/releases.json"
(( $(wc -c <"$tmp/releases.json") > 131072 )) || fail "the fixture no longer exceeds the argument limit"
echo '[{"number":7,"title":"A PR","updated_at":"2026-11-24T00:00:00Z","html_url":"https://example.invalid/pr","body":"ignored"}]' >"$tmp/pulls.json"
echo '[{"sha":"0123456789abcdef","commit":{"message":"subject\n\nbody","author":{"date":"2026-11-24T00:00:00Z"}},"html_url":"https://example.invalid/c"}]' >"$tmp/commits.json"

build >"$tmp/out.json" || fail "a large releases response should build"
check '.current_version == "v1.0.99"' "a draft must not become the current version"
check '.releases | length == 100' "releases are missing"
check '.releases[1].name == "v1.0.99" and .releases[1].summary == "Summary 1"' "name fallback or summary is wrong"
check '.releases[1].highlights == ["one", "two", "three", "four"]' "highlights are wrong"
check '.pull_requests == [{"number":7,"title":"A PR","updated_at":"2026-11-24T00:00:00Z","html_url":"https://example.invalid/pr"}]' "pull requests are wrong"
check '.commits[0].sha == "0123456" and .commits[0].message == "subject"' "commits are wrong"
check '.generated_at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z$")' "generated_at is not a UTC timestamp"

# GitHub sends null for a release without notes.
echo '[{"tag_name":"v2.0.0","name":"Two","draft":false,"prerelease":false,"published_at":null,"html_url":"u","body":null}]' >"$tmp/releases.json"
build >"$tmp/out.json" || fail "a release without notes should build"
check '.current_version == "v2.0.0" and .releases[0].summary == "" and .releases[0].highlights == []' "empty notes are wrong"

echo '[]' >"$tmp/releases.json"
build >"$tmp/out.json" || fail "no releases should build"
check '.current_version == "unreleased" and .releases == []' "the unreleased fallback is wrong"

# An API error is an object; it must not be published as data.
echo '{"message":"API rate limit exceeded"}' >"$tmp/releases.json"
build >"$tmp/out.json" 2>/dev/null && fail "an API error object should not build"
: >"$tmp/releases.json"
build >"$tmp/out.json" 2>/dev/null && fail "an empty response should not build"

echo "All build-project-data.sh tests passed."

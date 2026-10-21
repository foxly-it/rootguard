#!/usr/bin/env bash
set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -f -- "$tmp"/bin/docker "$tmp"/images "$tmp"/repo/scripts/check-alpine-pins.sh "$tmp"/repo/a/Dockerfile "$tmp"/repo/b/Dockerfile; rmdir -- "$tmp"/bin "$tmp"/repo/scripts "$tmp"/repo/a "$tmp"/repo/b "$tmp"/repo "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/repo/scripts" "$tmp/repo/a" "$tmp/repo/b"
cp "$repository_dir/scripts/check-alpine-pins.sh" "$tmp/repo/scripts/"

# Stub docker: records the image it was asked to run, answers per package.
cat >"$tmp/bin/docker" <<'EOF'
#!/usr/bin/env bash
image=""; after=0
for arg in "$@"; do
  if (( after )); then
    case "$arg:$PIN_TEST_MODE" in
      libssl3:current|libssl3:missing) v=3.5.9-r0 ;;
      libssl3:*) v=3.5.10-r0 ;;
      libcrypto3:current|libcrypto3:missing) v=3.5.9-r0 ;;
      libcrypto3:*) v=3.5.10-r0 ;;
      curl:curl-only) v=8.23.0-r0 ;;
      curl:*|libcurl:*) v=8.22.0-r0 ;;
      tiff:missing) v="" ;;
      tiff:*) v=4.7.2-r0 ;;
      *) exit 3 ;;
    esac
    printf '%s=%s\n' "$arg" "$v"
  elif [[ "$arg" == "--" ]]; then after=1
  elif [[ "$arg" == *@sha256:* ]]; then image="$arg"; echo "$image" >>"$PIN_TEST_IMAGES"
  fi
done
EOF
chmod +x "$tmp/bin/docker"
export PATH="$tmp/bin:$PATH" PIN_TEST_IMAGES="$tmp/images"

write_fixtures() {
  cat >"$tmp/repo/a/Dockerfile" <<'EOF'
FROM golang:1-alpine@sha256:builder AS builder
FROM docker:29-cli@sha256:runtime
# libssl3=3.5.0-r0 in a comment must be ignored
RUN apk add --no-cache \
        libssl3=3.5.9-r0 libcrypto3=3.5.9-r0 \
    && true
EOF
  cat >"$tmp/repo/b/Dockerfile" <<'EOF'
FROM nginx:1-alpine@sha256:nginx
RUN apk add --no-cache \
        libcurl=8.22.0-r0 curl=8.22.0-r0 \
        tiff=4.7.2-r0
EOF
  : >"$PIN_TEST_IMAGES"
}
run() { (cd "$tmp/repo" && PIN_TEST_MODE="$1" ./scripts/check-alpine-pins.sh "${@:2}" a/Dockerfile b/Dockerfile); }
fail() { echo "FAIL: $*" >&2; exit 1; }

write_fixtures
out="$(run current)" || fail "current pins should exit 0"
grep -q "All apk pins are current." <<<"$out" || fail "missing all-clear message"
[[ "$(cat "$PIN_TEST_IMAGES")" == $'docker:29-cli@sha256:runtime\nnginx:1-alpine@sha256:nginx' ]] \
  || fail "wrong base images: $(tr '\n' ' ' <"$PIN_TEST_IMAGES")"

write_fixtures
set +e; out="$(run drift)"; rc=$?; set -e
(( rc == 1 )) || fail "drift should exit 1, got $rc"
grep -q "DRIFT  libssl3: pinned=3.5.9-r0 newest=3.5.10-r0" <<<"$out" || fail "drift not reported"
grep -q "libssl3=3.5.9-r0 libcrypto3=3.5.9-r0" "$tmp/repo/a/Dockerfile" || fail "report-only run modified the Dockerfile"

write_fixtures
run drift --fix >/dev/null || fail "--fix should exit 0"
grep -q "libssl3=3.5.10-r0 libcrypto3=3.5.10-r0 \\\\" "$tmp/repo/a/Dockerfile" || fail "multi-pin line not rewritten"
grep -q "libssl3=3.5.0-r0 in a comment" "$tmp/repo/a/Dockerfile" || fail "comment was rewritten"
grep -q "tiff=4.7.2-r0" "$tmp/repo/b/Dockerfile" || fail "current pin changed"

# #767: a pin must only match its own name - "curl=" also occurs inside
# "libcurl=", and only curl moves here.
write_fixtures
run curl-only --fix >/dev/null || fail "curl-only --fix should exit 0"
grep -q "libcurl=8.22.0-r0 curl=8.23.0-r0 \\\\" "$tmp/repo/b/Dockerfile" \
  || fail "curl fix touched libcurl: $(grep curl "$tmp/repo/b/Dockerfile")"

write_fixtures
set +e; run missing --fix >/dev/null 2>&1; rc=$?; set -e
(( rc == 2 )) || fail "missing repository version should exit 2, got $rc"

echo "All check-alpine-pins.sh tests passed."

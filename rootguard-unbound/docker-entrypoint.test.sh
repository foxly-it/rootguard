#!/usr/bin/env bash
# Regression tests for docker-entrypoint.sh - found in a v1.0.0
# correctness review: no set -e (a failed setup step was silently
# ignored and Unbound started anyway), a trust-anchor guard that only
# checked file existence (a crash-truncated root.key was mistaken for an
# already-initialized one and never repaired), and a non-atomic copy
# straight to the final path (a crash mid-copy could itself produce
# exactly that truncated file for the next startup to mistake).
#
# Runs the real script as a subprocess (it isn't function-structured, and
# its own final "exec $@" makes sourcing it unsafe) with
# ROOT_KEY_SOURCE/ROOT_KEY_PATH/UNBOUND_STATE_DIR/UNBOUND_MODULE_DIR
# overridden to an isolated tmp sandbox, "$@" set to a harmless marker
# command instead of the real unbound binary.
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT

failures=0

setup_sandbox() {
  rm -rf -- "${tmp_dir:?}"/*
  mkdir -p "$tmp_dir/source" "$tmp_dir/state" "$tmp_dir/module"
  ROOT_KEY_SOURCE="$tmp_dir/source/root.key"
  ROOT_KEY_PATH="$tmp_dir/state/root.key"
  UNBOUND_STATE_DIR="$tmp_dir/state"
  UNBOUND_MODULE_DIR="$tmp_dir/module"
  export ROOT_KEY_SOURCE ROOT_KEY_PATH UNBOUND_STATE_DIR UNBOUND_MODULE_DIR
  printf 'fresh-reference-trust-anchor\n' > "$ROOT_KEY_SOURCE"
}

marker="$tmp_dir/marker"

run_entrypoint() {
  rm -f -- "$marker"
  sh "$script_dir/docker-entrypoint.sh" sh -c "touch '$marker'"
}

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

# --- a fresh install with no root.key yet: the trust anchor is copied
# in, with the right permissions, and "$@" (the real unbound binary in
# production) still runs afterward ---
setup_sandbox
if ! run_entrypoint; then
  fail "fresh install: entrypoint should succeed"
fi
if [[ ! -f "$marker" ]]; then
  fail "fresh install: exec \"\$@\" should still run after a successful setup"
fi
if [[ "$(cat "$ROOT_KEY_PATH")" != "fresh-reference-trust-anchor" ]]; then
  fail "fresh install: root.key content should match the reference source"
fi
mode="$(stat -f '%Lp' "$ROOT_KEY_PATH" 2>/dev/null || stat -c '%a' "$ROOT_KEY_PATH")"
if [[ "$mode" != "640" ]]; then
  fail "fresh install: expected root.key mode 640, got $mode"
fi
if compgen -G "$tmp_dir/state/root.key.tmp.*" > /dev/null; then
  fail "fresh install: a temp file was left behind instead of being renamed into place"
fi

# --- an already-initialized, non-empty root.key must survive untouched -
# a real RFC5011-rolled trust anchor differs from the reference copy and
# must never be silently overwritten back to it ---
setup_sandbox
printf 'already-rolled-trust-anchor\n' > "$ROOT_KEY_PATH"
if ! run_entrypoint; then
  fail "already initialized: entrypoint should succeed"
fi
if [[ "$(cat "$ROOT_KEY_PATH")" != "already-rolled-trust-anchor" ]]; then
  fail "already initialized: an existing, non-empty root.key must not be overwritten"
fi

# --- a zero-byte root.key (left behind by a prior crash mid-copy, before
# this fix existed) must be *repaired*, not mistaken for
# already-initialized - this is the exact regression the -f -> -s guard
# change fixes ---
setup_sandbox
: > "$ROOT_KEY_PATH"
if ! run_entrypoint; then
  fail "zero-byte root.key: entrypoint should succeed"
fi
if [[ "$(cat "$ROOT_KEY_PATH")" != "fresh-reference-trust-anchor" ]]; then
  fail "zero-byte root.key: an empty (crash-truncated) root.key should have been repaired, not left empty"
fi

# --- a failed setup step (the reference source is missing - a broken
# dns-root-data install) must abort instead of silently starting Unbound
# anyway - the set -e regression ---
setup_sandbox
rm -f -- "$ROOT_KEY_SOURCE"
if run_entrypoint; then
  fail "missing source: entrypoint should fail, not succeed"
fi
if [[ -f "$marker" ]]; then
  fail "missing source: exec \"\$@\" must never run after a failed setup step"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "$failures docker-entrypoint.sh test failure(s)" >&2
  exit 1
fi
echo "All docker-entrypoint.sh tests passed."

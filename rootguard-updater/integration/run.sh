#!/usr/bin/env bash
set -euo pipefail

integration_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
compose=(docker compose -f "$integration_dir/compose.e2e.yaml")

cleanup() {
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

build_fixture() {
  local image="$1"
  local version="$2"
  local fail_health="${3:-false}"
  docker build \
    --build-arg "VERSION=$version" \
    --build-arg "FAIL_HEALTH=$fail_health" \
    --tag "$image" \
    "$integration_dir/fixture" >/dev/null
}

wait_for_updater() {
  for _ in {1..60}; do
    if curl --fail --silent http://127.0.0.1:18082/health >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "updater did not become healthy" >&2
  return 1
}

start_update() {
  curl --fail --silent \
    --request POST \
    --header "Authorization: Bearer integration-token-not-a-real-secret-32" \
    http://127.0.0.1:18082/api/control-plane/update >/dev/null
}

wait_for_outcome() {
  local expected_state="$1"
  for _ in {1..90}; do
    status="$(curl --fail --silent \
      --header "Authorization: Bearer integration-token-not-a-real-secret-32" \
      http://127.0.0.1:18082/api/control-plane/status)"
    state="$(jq -r .state <<<"$status")"
    if [[ "$state" == "$expected_state" ]]; then
      printf '%s' "$status"
      return
    fi
    sleep 1
  done
  echo "updater did not reach $expected_state" >&2
  return 1
}

assert_running_image() {
  local container="$1"
  local expected_image="$2"
  local expected_id
  expected_id="$(docker image inspect --format '{{.Id}}' "$expected_image")"
  actual_id="$(docker inspect --format '{{.Image}}' "$container")"
  test "$actual_id" = "$expected_id"
}

build_fixture rootguard-e2e-core:old core-old
build_fixture rootguard-e2e-core:new core-new
build_fixture rootguard-e2e-webapp:old webapp-old
build_fixture rootguard-e2e-webapp:new webapp-new
build_fixture rootguard-e2e-webapp:bad webapp-bad true

cleanup
"${compose[@]}" up --detach --build
wait_for_updater
start_update
success_status="$(wait_for_outcome idle)"
test "$(jq -r '.history[0].outcome' <<<"$success_status")" = success
assert_running_image rootguard-core rootguard-e2e-core:new
assert_running_image rootguard-webapp rootguard-e2e-webapp:new

cleanup
WEB_TARGET_TAG=bad "${compose[@]}" up --detach --build
wait_for_updater
start_update
rollback_status="$(wait_for_outcome failed)"
test "$(jq -r '.history[0].outcome' <<<"$rollback_status")" = rolled_back
test "$(jq -r '.message | contains("rolled back safely")' <<<"$rollback_status")" = true
assert_running_image rootguard-core rootguard-e2e-core:old
assert_running_image rootguard-webapp rootguard-e2e-webapp:old

echo "real paired update and rollback scenarios passed"

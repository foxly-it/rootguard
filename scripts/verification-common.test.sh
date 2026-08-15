#!/usr/bin/env bash
# Tests for verification-common.sh's resource-ownership state machine,
# using a fake `docker` shell function instead of real containers. Exactly
# the kind of test that would have caught both real bugs found in this
# file this cycle: the original trap-registered-before-guard destructive
# bug, and the owns_managed_resources-set-after-compose-up timing bug.
# Run directly: ./scripts/verification-common.test.sh

set -Eeuo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
failures=0

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1" >&2; failures=$((failures + 1)); }

# Runs body (a snippet of shell code) in one subshell with a fake `docker`
# recording every call to docker_log, then asserts the subshell's exit
# code and, optionally, whether a teardown-style call (rm/volume rm/
# network rm) happened. Everything lives in a single subshell - no nested
# `bash -c` - so the fake function and every variable it closes over are
# naturally in scope without needing export.
run_case() {
  local name="$1" body="$2" expect_exit="$3" expect_teardown="$4" mode="${5:-}"
  local docker_log cookie archive exit_code=0
  docker_log="$(mktemp)"
  cookie="$(mktemp)"
  archive="$(mktemp)"

  # `(...) || exit_code=$?` looks equivalent to a plain subshell followed
  # by a status check, but it isn't: POSIX (and bash) ignore `-e` for any
  # command of an AND-OR list other than the last, and that suppression
  # extends to *everything the subshell runs*, not just the subshell's own
  # exit status - reasserting `set -e` as the subshell's first statement
  # does not override it either. `set +e` around a *plain* subshell
  # statement (not part of `||`) is the actual working idiom.
  set +e
  (
    set -Eeuo pipefail
    compose_file=/dev/null web_port=1 dns_port=1
    cookie_file="${cookie}" archive_file="${archive}"
    FAKE_DOCKER_MODE="${mode}"

    docker() {
      echo "$*" >>"${docker_log}"
      if [[ "$1" == "compose" ]]; then
        for arg in "$@"; do
          [[ "${arg}" == "up" ]] && { [[ "${FAKE_DOCKER_MODE}" == "compose-up-fails" ]] && return 1 || return 0; }
        done
        return 0
      fi
      case "$1 $2" in
        "container inspect")
          [[ "${FAKE_DOCKER_MODE}" == "existing-container" && "$3" == "rootguard-core" ]] && return 0
          return 1 ;;
        "volume inspect") return 1 ;;
        "network inspect") return 1 ;;
      esac
      [[ "$1" == "info" ]] && { [[ "${FAKE_DOCKER_MODE}" == "arch-mismatch" ]] && echo "{{.Architecture}}" || echo "x86_64"; }
      return 0
    }

    # shellcheck source=verification-common.sh
    . "${script_dir}/verification-common.sh"
    eval "${body}"
  )
  exit_code=$?
  set -e

  local ok=true
  if [[ "${exit_code}" != "${expect_exit}" ]]; then
    fail "${name}: expected exit ${expect_exit}, got ${exit_code}"
    ok=false
  fi
  local teardown_happened=false
  grep -qE '^(rm -f rootguard-|compose -f .* down|network rm|volume rm)' "${docker_log}" && teardown_happened=true
  if [[ "${teardown_happened}" != "${expect_teardown}" ]]; then
    fail "${name}: expected teardown=${expect_teardown}, got ${teardown_happened} (log: $(tr '\n' ';' <"${docker_log}"))"
    ok=false
  fi
  [[ "${ok}" == true ]] && pass "${name}"
  rm -f "${docker_log}" "${cookie}" "${archive}"
}

# 1. Existing resource detected -> guard exits 1, no teardown at all.
run_case "guard refuses on existing container, no teardown" \
  "guard_no_existing_resources" 1 false "existing-container"

# 2. No existing resources -> guard passes cleanly.
run_case "guard passes when nothing pre-exists" \
  "guard_no_existing_resources" 0 false ""

# 3. cleanup with owns_managed_resources still false (a failure before
#    install_stack ever marked ownership) must not tear anything down.
run_case "cleanup before ownership is set removes nothing" \
  "owns_managed_resources=false; cleanup" 0 false ""

# 4. cleanup with owns_managed_resources true must tear down - this is
#    exactly the state install_stack now sets *before* `compose up`, so a
#    `compose up` that starts creating resources and then fails still
#    gets torn down instead of orphaned (the timing bug this replaces).
run_case "cleanup after ownership is set tears down" \
  "owns_managed_resources=true; cleanup" 0 true ""

# 5. install_stack itself, not just cleanup in isolation: a failing
#    `compose up` must still leave the run torn down rather than orphaned
#    - the exact scenario the timing bug broke (ownership used to be set
#    only *after* a successful `compose up`, so a failing one left
#    owns_managed_resources false and any partial resources it created
#    behind for the next run's guard check to trip over).
run_case "install_stack tears down after compose up fails" \
  "trap cleanup EXIT; install_stack" 1 true "compose-up-fails"

# 6. A guard refusal must still clean up the temp cookie/archive files
#    even though it never touches Docker resources, now that the trap is
#    registered before the guard check runs in both callers.
docker_log="$(mktemp)"
cookie="$(mktemp)"
archive="$(mktemp)"
set +e
(
  set -Eeuo pipefail
  compose_file=/dev/null web_port=1 dns_port=1
  cookie_file="${cookie}" archive_file="${archive}"
  FAKE_DOCKER_MODE="existing-container"
  docker() {
    echo "$*" >>"${docker_log}"
    [[ "$1 $2" == "container inspect" && "$3" == "rootguard-core" ]] && return 0
    return 1
  }
  # shellcheck source=verification-common.sh
  . "${script_dir}/verification-common.sh"
  trap cleanup EXIT
  guard_no_existing_resources
)
exit_code=$?
set -e
if [[ "${exit_code}" == 1 && ! -e "${cookie}" && ! -e "${archive}" ]]; then
  pass "guard refusal via trap removes temp files"
else
  fail "guard refusal via trap removes temp files: exit=${exit_code} cookie_exists=$([[ -e ${cookie} ]] && echo yes || echo no)"
fi
rm -f "${docker_log}" "${cookie}" "${archive}"

if (( failures > 0 )); then
  echo "${failures} test(s) failed" >&2
  exit 1
fi
echo "All verification-common.sh tests passed"

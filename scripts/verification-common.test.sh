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

# Test bodies are real functions, called by name - not strings passed
# through `eval`. `eval "trap handler EXIT; ..."` turned out to leave the
# trap's own errexit propagation broken on bash 5.x (this repo's CI
# runners) in a way it isn't when the exact same code is literal source:
# a `docker` call inside the trap handler that itself fails silently
# aborted the whole subshell instead of continuing past its `|| true`,
# skipping every step after it (observed: cleanup's own teardown call
# never ran). Reproduced and root-caused against a real bash 5.3 (this
# repo's local dev bash on macOS is the ancient stock 3.2, which never
# showed the bug) before settling on named functions as the fix, since
# that's the one invocation form that behaved identically on both.
case_guard_refuses() { guard_no_existing_resources; }
case_guard_passes() { guard_no_existing_resources; }
case_cleanup_no_ownership() { owns_managed_resources=false; cleanup; }
case_cleanup_with_ownership() { owns_managed_resources=true; cleanup; }
case_install_stack_tears_down() { trap cleanup EXIT; install_stack; }
case_guard_refusal_via_trap() { trap cleanup EXIT; guard_no_existing_resources; }

# Runs case_fn (a function name) in one subshell with a fake `docker`
# recording every call to docker_log, then asserts the subshell's exit
# code and, optionally, whether a teardown-style call (rm/volume rm/
# network rm) happened. Everything lives in a single subshell - no nested
# `bash -c`, no `eval` - so the fake function, every variable it closes
# over, and case_fn itself are naturally in scope without needing export.
run_case() {
  local name="$1" case_fn="$2" expect_exit="$3" expect_teardown="$4" mode="${5:-}"
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
      # Deliberately `if`/`fi`, not `[[ ]] && { ...; }` as a bare loop-body
      # statement: under `set -e`, bash treats a failing (false) `&&`-list
      # used as a plain statement inside a `for` loop as a real command
      # failure and aborts right there on bash 5.x (observed live; bash
      # 3.2 tolerated it). `if`'s own condition is exempt from errexit
      # regardless of its truth value, so this form is safe everywhere.
      if [[ "$1" == "compose" ]]; then
        for arg in "$@"; do
          if [[ "${arg}" == "up" ]]; then
            if [[ "${FAKE_DOCKER_MODE}" == "compose-up-fails" ]]; then
              return 1
            fi
            return 0
          fi
        done
        return 0
      fi
      case "$1 $2" in
        "container inspect")
          if [[ "${FAKE_DOCKER_MODE}" == "existing-container" && "$3" == "rootguard-core" ]]; then
            return 0
          fi
          return 1 ;;
        "volume inspect") return 1 ;;
        "network inspect") return 1 ;;
      esac
      if [[ "$1" == "info" ]]; then
        if [[ "${FAKE_DOCKER_MODE}" == "arch-mismatch" ]]; then
          echo "{{.Architecture}}"
        else
          echo "x86_64"
        fi
      fi
      return 0
    }

    # shellcheck source=verification-common.sh
    . "${script_dir}/verification-common.sh"
    "${case_fn}"
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
  case_guard_refuses 1 false "existing-container"

# 2. No existing resources -> guard passes cleanly.
run_case "guard passes when nothing pre-exists" \
  case_guard_passes 0 false ""

# 3. cleanup with owns_managed_resources still false (a failure before
#    install_stack ever marked ownership) must not tear anything down.
run_case "cleanup before ownership is set removes nothing" \
  case_cleanup_no_ownership 0 false ""

# 4. cleanup with owns_managed_resources true must tear down - this is
#    exactly the state install_stack now sets *before* `compose up`, so a
#    `compose up` that starts creating resources and then fails still
#    gets torn down instead of orphaned (the timing bug this replaces).
run_case "cleanup after ownership is set tears down" \
  case_cleanup_with_ownership 0 true ""

# 5. install_stack itself, not just cleanup in isolation: a failing
#    `compose up` must still leave the run torn down rather than orphaned
#    - the exact scenario the timing bug broke (ownership used to be set
#    only *after* a successful `compose up`, so a failing one left
#    owns_managed_resources false and any partial resources it created
#    behind for the next run's guard check to trip over).
run_case "install_stack tears down after compose up fails" \
  case_install_stack_tears_down 1 true "compose-up-fails"

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
  case_guard_refusal_via_trap
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

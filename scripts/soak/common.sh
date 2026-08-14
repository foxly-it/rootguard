#!/usr/bin/env bash
# Shared helpers for the 0.9 30-day endurance-test scripts in this directory.
# Sourced, not executed directly. Assumes it runs on the dedicated soak host
# next to a compose.alpha.yaml + .env deployment (see README.md in this dir).

: "${ROOTGUARD_SOAK_DIR:=/root/rootguard-soak}"
: "${ROOTGUARD_SOAK_ENV:=${ROOTGUARD_SOAK_DIR}/.env}"
: "${ROOTGUARD_SOAK_WEB_PORT:=8080}"
: "${ROOTGUARD_SOAK_DNS_PORT:=53}"
: "${ROOTGUARD_SOAK_LOG_DIR:=/var/log/rootguard-soak}"
: "${ROOTGUARD_SOAK_COOKIE_JAR:=${ROOTGUARD_SOAK_DIR}/cookies.txt}"

mkdir -p "$ROOTGUARD_SOAK_LOG_DIR"

soak_now() { date -u +%Y-%m-%dT%H:%M:%SZ; }

soak_admin_password() {
  grep -E '^ROOTGUARD_ADMIN_PASSWORD=' "$ROOTGUARD_SOAK_ENV" | head -1 | cut -d= -f2-
}

# Logs in and refreshes the shared cookie jar. Returns non-zero if the
# WebApp never became reachable/healthy within the retry budget.
soak_login() {
  local password code
  password="$(soak_admin_password)"
  for _ in $(seq 1 30); do
    code="$(curl --silent --output /dev/null --write-out '%{http_code}' \
      --cookie-jar "$ROOTGUARD_SOAK_COOKIE_JAR" \
      --header 'Content-Type: application/json' \
      --data "{\"username\":\"admin\",\"password\":\"${password}\"}" \
      "http://127.0.0.1:${ROOTGUARD_SOAK_WEB_PORT}/api/auth/login" 2>/dev/null || true)"
    [ "$code" = "200" ] && return 0
    sleep 2
  done
  return 1
}

# soak_call METHOD PATH [JSON_DATA] - authenticated call, prints the response
# body on stdout, fails loud (non-zero + stderr message) on unreachable host
# or non-2xx status. Never used for the WebApp-restart-mid-swap window during
# an update - callers poll status separately for that.
soak_call() {
  local method="$1" path="$2" data="${3:-}" body code curl_status
  local -a args=(--silent --cookie "$ROOTGUARD_SOAK_COOKIE_JAR" --write-out '\n%{http_code}' --request "$method")
  [ -n "$data" ] && args+=(--header 'Content-Type: application/json' --data "$data")
  set +e
  body="$(curl "${args[@]}" "http://127.0.0.1:${ROOTGUARD_SOAK_WEB_PORT}${path}")"
  curl_status=$?
  set -e
  if [ "$curl_status" -ne 0 ]; then
    echo "curl failed (exit ${curl_status}) for ${method} ${path}" >&2
    return 1
  fi
  code="${body##*$'\n'}"
  body="${body%$'\n'*}"
  if [ "${code:0:1}" != 2 ]; then
    echo "${method} ${path} returned HTTP ${code}: ${body}" >&2
    return 1
  fi
  printf '%s' "$body"
}

# soak_log FILE JSON - append one line to a log file under ROOTGUARD_SOAK_LOG_DIR
soak_log() {
  printf '%s\n' "$2" >> "${ROOTGUARD_SOAK_LOG_DIR}/${1}"
}

soak_resolve_ok() {
  local answer
  # dig +short prints connection/timeout errors ("communications error to
  # ...", ";; connection timed out") to STDOUT, not stderr - a non-empty
  # string alone doesn't mean "got an answer". Require a real IPv4/IPv6
  # literal on the first line, not just any non-empty first line.
  answer="$(dig +short +time=5 +tries=2 @127.0.0.1 -p "$ROOTGUARD_SOAK_DNS_PORT" example.com A 2>/dev/null || true)"
  answer="${answer%%$'\n'*}"
  [[ "$answer" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] && printf '%s' "$answer"
}

soak_dnssec_reject_ok() {
  local status
  status="$(dig +dnssec +time=5 +tries=2 @127.0.0.1 -p "$ROOTGUARD_SOAK_DNS_PORT" dnssec-failed.org A 2>/dev/null \
    | sed -n 's/^;; ->>HEADER<<- opcode: QUERY, status: \([^,]*\).*/\1/p')"
  [ "$status" = "SERVFAIL" ]
}

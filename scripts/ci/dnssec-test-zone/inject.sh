#!/usr/bin/env bash
# Wires a running rootguard-unbound container up to resolve and validate
# the local DNSSEC test zone setup.sh (this directory) already started -
# so container-based Unbound tests can dig good.rgtest-ci.internal /
# bad.rgtest-ci.internal instead of the real internet.
#
# Uses the container's own default-route network gateway to reach
# setup.sh's authority on the runner host - NOT
# --add-host=host.docker.internal:host-gateway, even where that's
# configured (compose.integration.yaml sets it, but only as a no-op
# for a real deployment; see its own comment).
#
# Found live: with host.docker.internal (-> the *default* bridge's
# gateway, 172.17.0.1) as forward-addr, a container that isn't on that
# bridge - true for every compose-created service, each on its own
# custom network - still gets its query there via the host's normal
# inter-bridge routing, but NSD (bound to 0.0.0.0:8053, replying based
# on its route back to the client) answers from *that bridge's own*
# gateway address instead, e.g. 172.29.53.1. Unbound's outbound UDP
# sockets are connect()ed to the address they queried (anti-spoofing:
# only that exact peer's replies are delivered) so a same-instant,
# correctly-formed reply from a *different* source address is silently
# dropped at the kernel - Unbound retries (with backoff) until it gives
# up, 10+ seconds later, and the client sees nothing at all: not a slow
# answer, no answer whatsoever ("communications error ... timed out").
# dig doesn't hit this since it (unlike a security-hardened resolver)
# doesn't filter replies by source address. The container's own network
# gateway is the one address guaranteed to round-trip: it's the address
# the host actually uses to talk to the container, so replies to it
# come from it, by construction.
#
# Found in review: Unbound's `forward-addr:` directive rejects a hostname
# outright ("cannot parse forward ip address") - it needs a literal IP.
#
# Usage: ./inject.sh <container-name>
# Requires setup.sh to have already run (reads $OUT_DIR/trust-anchor).
# Restarts the container so the new config takes effect the same way a
# real settings change would - the caller is responsible for waiting on
# its health check afterward, same as any other restart.

set -Eeuo pipefail

container="${1:?usage: inject.sh <container-name>}"
out_dir="${DNSSEC_TEST_ZONE_DIR:-/tmp/rootguard-ci-dnssec-test}"
nsd_port="8053"

trust_anchor="$(cat "${out_dir}/trust-anchor")"

# DNSSEC_TEST_AUTHORITY_IP overrides the auto-detected gateway - found in
# review: the gateway-IP approach below is specific to how Docker's own
# Linux bridge networking routes container-to-host traffic, verified
# live only on native Linux GitHub runners. On Docker Desktop (macOS/
# Windows, e.g. a developer reproducing a CI failure locally), the VM
# running the Docker daemon sits behind a different, non-routable-from-
# the-container network layer entirely - the resolved "gateway" is not
# actually reachable from inside the container the way it is on Linux.
# This lets a local run set the authority's real, reachable address by
# hand instead (e.g. the host's LAN IP); CI itself never sets this, so
# the auto-detected Linux gateway path is completely unaffected.
gateway_ip="${DNSSEC_TEST_AUTHORITY_IP:-}"
if [[ -z "$gateway_ip" ]]; then
  # docker inspect's own failure (container doesn't exist, daemon
  # unreachable) is caught explicitly here, separately from the pipe
  # below - found in a v1.0.0 correctness review: with the whole
  # "docker inspect | grep | sort | head" chain as one pipeline, `set -e`
  # aborted the script the instant docker inspect failed, before the
  # friendly, actionable "-z" error message two lines down ever had a
  # chance to run for that case.
  networks="$(docker inspect "$container" --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{"\n"}}{{end}}')" || {
    echo "::error::docker inspect ${container} failed - is the container running?" >&2
    exit 1
  }
  # Sorted for determinism - Go template range over a map (here,
  # NetworkSettings.Networks) iterates in random key order, so an
  # unsorted first-match on a multi-network container (every compose
  # service here has one) would pick a different network from run to
  # run. A network with no gateway of its own (e.g. an internal-only
  # network) renders empty and is filtered out below.
  #
  # `|| true` on this second pipe too - found alongside the fix above:
  # grep -v exits 1 when it filters out every line (a container whose
  # only network genuinely has no gateway renders nothing else), which
  # under pipefail poisons this whole assignment's exit status the exact
  # same way docker inspect's own failure did - the "-z" check below was
  # never actually reachable for *either* of the two cases its own error
  # message claims to cover, only discovered once the first one was
  # fixed and this one still aborted the script identically.
  gateway_ip="$(printf '%s\n' "$networks" | grep -v '^$' | sort | head -1)" || true
fi
if [[ -z "$gateway_ip" ]]; then
  echo "::error::${container} has no network gateway IP - can't reach the local DNSSEC test authority from inside it (set DNSSEC_TEST_AUTHORITY_IP to override, e.g. on Docker Desktop)" >&2
  exit 1
fi

conf_path="${out_dir}/99-ci-dnssec-test.conf"
cat >"$conf_path" <<EOF
server:
  trust-anchor: "${trust_anchor}"

forward-zone:
  name: "rgtest-ci.internal."
  forward-addr: ${gateway_ip}@${nsd_port}
EOF

docker cp "$conf_path" "${container}:/etc/unbound/unbound.d/99-ci-dnssec-test.conf"
docker exec "$container" unbound-checkconf
docker restart "$container"

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

# Sorted for determinism - Go template range over a map (here,
# NetworkSettings.Networks) iterates in random key order, so an
# unsorted first-match on a multi-network container (every compose
# service here has one) would pick a different network from run to
# run. A network with no gateway of its own (e.g. an internal-only
# network) renders empty and is filtered out below.
gateway_ip="$(docker inspect "$container" --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{"\n"}}{{end}}' | grep -v '^$' | sort | head -1)"
if [[ -z "$gateway_ip" ]]; then
  echo "::error::${container} has no network gateway IP - can't reach the local DNSSEC test authority from inside it" >&2
  exit 1
fi
# Recorded for callers that need to reach the authority themselves - e.g.
# a guided ForwardZone under test that has to point at the same address
# this script just resolved, for a zone this script deliberately doesn't
# forward itself (setup.sh's unsigned split-DNS zone; see
# scenario_integration_test.go).
printf '%s' "$gateway_ip" >"${out_dir}/gateway-ip"

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

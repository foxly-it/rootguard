#!/usr/bin/env bash
# Wires a running rootguard-unbound container up to resolve and validate
# the local DNSSEC test zone setup.sh (this directory) already started -
# so container-based Unbound tests can dig good.rgtest-ci.internal /
# bad.rgtest-ci.internal instead of the real internet.
#
# If the container was started with
# --add-host=host.docker.internal:host-gateway (docker run) or
# extra_hosts: ["host.docker.internal:host-gateway"] (compose), that's
# used to find setup.sh's own authority, running on the runner host
# itself. Otherwise (release-alpha.yml's smoke-test/upgrade-test: Core's
# own installer.Manager creates this container dynamically via the
# Docker API, so nothing here controls how it's started) falls back to
# the container's own network gateway IP directly - the same address
# host-gateway itself would have resolved to, just found the way any
# container can always reach the host it's running on, without needing
# to have been told to in advance.
#
# Found in review: Unbound's `forward-addr:` directive rejects a hostname
# outright ("cannot parse forward ip address") - it needs a literal IP,
# resolved fresh here rather than assumed, since different container
# network setups (the default bridge vs. a compose-created custom
# network) can give it a different address.
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

# Found live: under `pipefail`, getent's own non-zero exit ("key not
# found", the expected outcome without --add-host) makes the *pipeline's*
# exit status non-zero too - even though awk itself, downstream, exits 0
# - which trips `set -e` and aborts before the fallback below ever runs.
# The `|| true` is load-bearing, not decorative.
gateway_ip="$(docker exec "$container" getent ahostsv4 host.docker.internal 2>/dev/null | awk '{print $1; exit}' || true)"
if [[ -z "$gateway_ip" ]]; then
  gateway_ip="$(docker inspect "$container" --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{"\n"}}{{end}}' | grep -v '^$' | head -1)"
fi
if [[ -z "$gateway_ip" ]]; then
  echo "::error::${container} has no resolvable host.docker.internal and no network gateway IP either - can't reach the local DNSSEC test authority from inside it" >&2
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

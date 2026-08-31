#!/usr/bin/env bash
# Starts a local, throwaway DNSSEC-signed test authority for CI - so PR
# and release gates that need to prove "does this resolver actually
# validate DNSSEC correctly" don't depend on real internet domains
# (example.com, dnssec-failed.org) being reachable and stable from
# whatever GitHub Actions runner happens to execute the job.
#
# Found in review: every one of these checks queried real internet
# domains directly - reasonable for the *product's* own diagnostics
# feature (a real user genuinely wants to know "can my resolver reach
# and validate the real internet"), but wrong for a CI gate: a transient
# DNS/network hiccup on the runner's own connection then fails the build
# for reasons that have nothing to do with the code under test. Confirmed
# live: main's own independent, scheduled Unbound CI run failed this way
# multiple times in a row, at the same time as an unrelated PR's.
#
# Serves two records under one throwaway DNSSEC-signed zone, signed fresh
# on every run (never committed - a checked-in signed zone would
# eventually expire and break CI on its own, and committing private key
# material is bad practice regardless of how throwaway it is):
#   good.rgtest-ci.internal.  - validates cleanly (the example.com role)
#   bad.rgtest-ci.internal.   - a deliberately corrupted RRSIG, so a
#                               validating resolver must SERVFAIL it
#                               (the dnssec-failed.org role)
#
# Also starts a second, throwaway authority for split-DNS scenarios that
# need a forward target distinct from the signed zone above (a guided
# ForwardZone forwarding rgtest-ci.internal itself would be
# indistinguishable from this script's own base wiring, which already
# forwards all of it - see inject.sh): a single unsigned record,
# split.rgtest-split.internal. -> 203.0.113.50.
#
# That second authority runs as its own Docker container, on the
# standard port 53 *inside its own network namespace* - not this
# script's own nsd instance, and not the host's port 53 either. Found
# live: a guided ForwardZone's own Settings.Validate() requires a bare
# canonical IP address in `servers[]` (no "ip@port" syntax; that's
# Unbound raw config's own forward-addr extension, which inject.sh's own
# base wiring uses directly, not something the guided-settings API
# accepts) - so a scenario test driving the real Settings.Render() path
# needs this authority reachable on the port a bare IP implies. The
# *host's* own port 53 turned out already bound on GitHub's runners
# (confirmed live: nsd failed with "can't bind udp socket 0.0.0.0@53:
# Address already in use") - a second container sidesteps that
# entirely, its own port 53 in its own netns, reachable from another
# unnetworked container (as the Go scenario tests start theirs) via
# Docker's default bridge.
#
# Usage: ./setup.sh - installs nsd/ldns-utils if missing, generates and
# signs the DNSSEC zone, starts nsd listening on 0.0.0.0:8053 serving it,
# and writes $OUT_DIR/trust-anchor - the signed zone's DNSKEY as a single
# line, ready to drop straight into an Unbound `trust-anchor:` config
# directive. Also starts the split-DNS container authority and writes
# $OUT_DIR/split-authority-ip - its own reachable IP. inject.sh (this
# directory) wires a given container up to the DNSSEC zone.
#
# Real internet resolution is still exercised, just not as a blocking PR/
# release gate - see ci-real-dns-upstream.yml.
#
# Linux only: this script itself runs `apt-get` and GNU `date -d`
# directly on the host (both true of every caller - ci.yml, ci-unbound.yml,
# clean-install.yml, backup-restore.yml - which all run on GitHub's
# ubuntu-latest/ubuntu-24.04(-arm) runners). Not portable to macOS/Docker
# Desktop as-is; DNSSEC_TEST_AUTHORITY_IP (inject.sh) only fixes the
# container-to-host gateway lookup for a container already running on
# Linux, not this script's own host-side tooling.

set -Eeuo pipefail

zone="rgtest-ci.internal."
split_zone="rgtest-split.internal."
nsd_port="8053"
out_dir="${DNSSEC_TEST_ZONE_DIR:-/tmp/rootguard-ci-dnssec-test}"

mkdir -p "$out_dir"
cd "$out_dir"

if ! command -v nsd >/dev/null 2>&1 || ! command -v ldns-signzone >/dev/null 2>&1; then
  sudo apt-get update -qq
  sudo apt-get install -y -qq nsd ldnsutils
fi

cat >zone.txt <<EOF
\$TTL 300
${zone}      IN SOA  ns.${zone} hostmaster.${zone} ( $(date +%s) 3600 900 604800 300 )
${zone}      IN NS   ns.${zone}
ns.${zone}   IN A    203.0.113.1
good.${zone} IN A    203.0.113.10
bad.${zone}  IN A    203.0.113.20
EOF

ksk="$(ldns-keygen -a RSASHA256 -b 2048 -k "${zone}")"
zsk="$(ldns-keygen -a RSASHA256 -b 1024 "${zone}")"
# An absolute date - ldns-signzone's -e does not accept a relative
# offset (found live: a "+604800" argument silently produced an
# expiration date in 1970, before the signatures' own inception time,
# which would have made every record - "good" included - fail to
# validate). 30 days is far more than any single CI run needs.
expire="$(date -u -d "+30 days" +%Y%m%d%H%M%S)"
ldns-signzone -e "$expire" zone.txt "$ksk" "$zsk"

# Corrupt exactly bad.<zone>'s own A-record RRSIG - flips the first
# base64 character of its signature, which no longer cryptographically
# verifies. Every other record's signature - including good.<zone>'s -
# is untouched and must still validate cleanly.
python3 - "bad.${zone}" <<'PYEOF'
import sys

target = sys.argv[1]
path = "zone.txt.signed"
with open(path) as f:
    lines = f.readlines()

corrupted = False
out = []
for line in lines:
    if line.startswith(target) and "\tRRSIG\tA " in line:
        parts = line.rstrip("\n").split(" ")
        sig = parts[-1]
        parts[-1] = ("B" if sig[0] == "A" else "A") + sig[1:]
        line = " ".join(parts) + "\n"
        corrupted = True
    out.append(line)

if not corrupted:
    sys.exit(f"did not find the RRSIG A record for {target} to corrupt")

with open(path, "w") as f:
    f.writelines(out)
PYEOF

# ${ksk}.key is exactly the KSK's own DNSKEY record (ldns-keygen -k)
# - strip the trailing ";{id = ...}" comment ldns appends, an Unbound
# `trust-anchor:` line takes the bare record.
sed -E 's/[[:space:]]+;\{.*\}$//' "${ksk}.key" >trust-anchor

cat >nsd.conf <<EOF
server:
  ip-address: 0.0.0.0@${nsd_port}
  hide-version: yes
  username: root
  zonesdir: "${out_dir}"
  logfile: "${out_dir}/nsd.log"
  pidfile: "${out_dir}/nsd.pid"
  chroot: ""

# nsd defaults its remote-control TLS interface to on, needing a cert
# this throwaway setup never generates - found live: nsd exited 1 with
# nothing on stderr, the actual "could not setup remote control TLS
# context" error only visible in its own logfile. Nothing here ever uses
# nsd-control, so just turn it off instead of generating certs for it.
remote-control:
  control-enable: no

zone:
  name: "${zone}"
  zonefile: "${out_dir}/zone.txt.signed"
EOF

nsd-checkconf nsd.conf
nsd-checkzone "${zone}" zone.txt.signed

# Only ever stop a leftover nsd this exact script started earlier - a bare
# `pkill nsd` would just as happily kill an unrelated nsd process on a
# shared/self-hosted runner or a developer's own machine running this
# locally. Identified by its own pidfile under this test directory, and
# double-checked against /proc so a recycled PID that now belongs to some
# other process is never touched.
if [[ -f nsd.pid ]]; then
  old_pid="$(cat nsd.pid)"
  if [[ "$old_pid" =~ ^[0-9]+$ ]] && ps -p "$old_pid" -o args= 2>/dev/null | grep -qF "${out_dir}/nsd.conf"; then
    sudo kill "$old_pid"
    for _ in $(seq 1 20); do
      kill -0 "$old_pid" 2>/dev/null || break
      sleep 0.25
    done
  fi
  rm -f nsd.pid
fi
# nsd logs its own startup failures (a bind conflict, e.g.) to its own
# logfile, not stderr - found live already, for the remote-control TLS
# issue above. Dump it on failure so a bind conflict is diagnosable from
# CI output instead of just "exit 1" with nothing else to go on.
if ! sudo nsd -c "${out_dir}/nsd.conf"; then
  echo "::error::nsd failed to start - ${out_dir}/nsd.log follows" >&2
  cat "${out_dir}/nsd.log" >&2 2>/dev/null || true
  exit 1
fi

# Checks actual record content, not just dig's exit code - an exit code
# alone can't tell a real answer apart from an empty NOERROR, REFUSED, or
# NXDOMAIN response, any of which would let this loop declare the
# authority "ready" before it can actually serve what the caller expects.
dnssec_authority_ready=false
for _ in $(seq 1 20); do
  good_answer="$(dig +short +time=1 +tries=1 @127.0.0.1 -p "$nsd_port" "good.${zone}" A 2>/dev/null || true)"
  if [[ "$good_answer" == "203.0.113.10" ]]; then
    dnssec_authority_ready=true
    break
  fi
  sleep 0.5
done
if [[ "$dnssec_authority_ready" != true ]]; then
  echo "::error::Local DNSSEC test authority never came up" >&2
  exit 1
fi
echo "Local DNSSEC test authority is up on 0.0.0.0:${nsd_port}, serving ${zone}"

# Split-DNS authority: a throwaway container, not this script's own nsd
# instance - see this file's own header comment on why. alpine, not the
# rootguard-unbound image other CI steps build: this script runs before
# that image exists in some callers (ci.yml's own "validate" job builds
# the stack *after* calling this script), and alpine's own package
# manager is fast enough that installing nsd fresh here each run is
# still cheap.
cat >split-zone.txt <<EOF
\$TTL 300
${split_zone}       IN SOA  ns.${split_zone} hostmaster.${split_zone} ( $(date +%s) 3600 900 604800 300 )
${split_zone}       IN NS   ns.${split_zone}
ns.${split_zone}    IN A    203.0.113.2
split.${split_zone} IN A    203.0.113.50
EOF
# chroot/username/remote-control mirror the host nsd.conf above, same
# reasons: an unset chroot can default to a path this throwaway zonesdir
# was never placed under, dropping privileges to a package-created "nsd"
# user can fail to read a bind-mounted zone owned by the container's
# root, and the default remote-control TLS interface needs a cert this
# throwaway setup never generates (the exact failure the host instance
# above already hit and fixed the same way).
cat >split-nsd.conf <<EOF
server:
  ip-address: 0.0.0.0@53
  zonesdir: "/zones"
  username: root
  chroot: ""

remote-control:
  control-enable: no

zone:
  name: "${split_zone}"
  zonefile: "/zones/split-zone.txt"
EOF

# Only ever remove a leftover container this exact script created
# earlier - same reasoning as the nsd pidfile check above, applied to
# Docker instead of a bare process: label it ourselves and check that
# label before removing anything by this name, rather than trusting the
# name alone not to collide with something unrelated.
split_authority_label="io.rootguard.ci=dnssec-split-authority"
existing_split_authority="$(docker ps -aq --filter "name=^rgtest-split-authority$" --filter "label=${split_authority_label}")"
if [[ -n "$existing_split_authority" ]]; then
  docker rm -f "$existing_split_authority" >/dev/null
fi
# Pinned by digest, not just the "3.20" tag - alpine:3.20 is itself
# regularly rebuilt in place (security patches), so the tag alone isn't
# reproducible run to run. This is the "3.20" manifest list's digest as
# of this comment being written; bump it by hand when there's a reason
# to (a newer nsd package, e.g.), not silently on whatever happens to be
# tagged "3.20" today.
docker run --rm --detach --name rgtest-split-authority \
  --label "${split_authority_label}" \
  -v "${out_dir}/split-nsd.conf:/etc/nsd.conf:ro" \
  -v "${out_dir}/split-zone.txt:/zones/split-zone.txt:ro" \
  --entrypoint sh \
  alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc \
  -c 'apk add --no-cache nsd bind-tools >/dev/null 2>&1 && exec nsd -d -c /etc/nsd.conf' >/dev/null

# Checked from *inside* the container (docker exec ... dig @127.0.0.1),
# not from the host against the container's own bridge IP - found in
# review: a host-side check works on Linux (this script's own supported
# platform - see the header comment above) but not from a macOS/Windows
# Docker Desktop host, where the Docker daemon runs inside its own VM and
# the container's bridge IP isn't routable from the host at all. Querying
# the container's own loopback from inside it needs no such routability,
# so this check - unlike the rest of this script - actually works
# regardless of host OS. bind-tools (added to the apk install above)
# provides dig.
split_answer=""
for _ in $(seq 1 40); do
  split_answer="$(docker exec rgtest-split-authority dig +short +time=1 +tries=1 @127.0.0.1 "split.${split_zone}" A 2>/dev/null || true)"
  [[ "$split_answer" == "203.0.113.50" ]] && break
  sleep 0.5
done
if [[ "$split_answer" != "203.0.113.50" ]]; then
  echo "::error::Split-DNS test authority container never came up" >&2
  docker logs rgtest-split-authority 2>&1 || true
  exit 1
fi
# The container's own bridge IP, resolved separately from the readiness
# check above - this is genuinely needed by another *container* (the Go
# scenario tests' own), not the host, so it's fine that it isn't
# reachable from a Docker Desktop host itself: container-to-container
# traffic on Docker's default bridge stays inside the Docker daemon's own
# network regardless of which OS that daemon runs on.
split_authority_ip="$(docker inspect --format '{{.NetworkSettings.IPAddress}}' rgtest-split-authority)"
printf '%s' "$split_authority_ip" >"${out_dir}/split-authority-ip"
echo "Split-DNS test authority is up at ${split_authority_ip}:53, serving ${split_zone}"

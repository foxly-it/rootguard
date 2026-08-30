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
# Serves two records under one throwaway zone, signed fresh on every run
# (never committed - a checked-in signed zone would eventually expire and
# break CI on its own, and committing private key material is bad
# practice regardless of how throwaway it is):
#   good.rgtest-ci.internal.  - validates cleanly (the example.com role)
#   bad.rgtest-ci.internal.   - a deliberately corrupted RRSIG, so a
#                               validating resolver must SERVFAIL it
#                               (the dnssec-failed.org role)
#
# Usage: ./setup.sh - installs nsd/ldns-utils if missing, generates and
# signs the zone, starts nsd listening on 0.0.0.0:8053, and writes
# $OUT_DIR/trust-anchor - the zone's DNSKEY as a single line, ready to
# drop straight into an Unbound `trust-anchor:` config directive.
# inject.sh (this directory) wires a given container up to consume it.
#
# Real internet resolution is still exercised, just not as a blocking PR/
# release gate - see ci-real-dns-upstream.yml.

set -Eeuo pipefail

zone="rgtest-ci.internal."
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
sudo pkill nsd 2>/dev/null || true
sudo nsd -c "${out_dir}/nsd.conf"

for _ in $(seq 1 20); do
  if dig +short +time=1 +tries=1 @127.0.0.1 -p "$nsd_port" "good.${zone}" A >/dev/null 2>&1; then
    echo "Local DNSSEC test authority is up on 0.0.0.0:${nsd_port}, serving ${zone}"
    exit 0
  fi
  sleep 0.5
done
echo "::error::Local DNSSEC test authority never came up" >&2
exit 1

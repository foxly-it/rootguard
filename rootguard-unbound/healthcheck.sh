#!/bin/sh
set -eu

# Found in review: dig's own exit status reflects transport, not RCODE -
# it documents 0 as "DNS response received, including NXDOMAIN status",
# and the same holds for SERVFAIL/REFUSED/NOERROR-with-no-answer. Piping
# to /dev/null and checking only the exit code meant this healthcheck
# passed as long as *something* answered on 5335, even a resolver that
# refuses or fails every single query - live-verified against a
# controlled responder returning fixed RCODEs, all reported "healthy".
# That status gates real deploy/update decisions (Docker's own
# service_healthy dependency, installer.Manager.waitForUnbound) - a
# resolver silently unable to actually recurse (outbound UDP/53 blocked,
# an unusable trust anchor) would let a deploy "succeed" while resolving
# nothing. +short already suppresses everything but actual answer data
# on success and prints nothing at all on any non-NOERROR-with-an-answer
# result - checking that instead of the discarded exit code is the fix.
answer="$(dig @127.0.0.1 -p 5335 cloudflare.com A +time=1 +tries=1 +short)"
[ -n "$answer" ]

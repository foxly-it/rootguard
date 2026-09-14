#!/bin/sh
# Found in a v1.0.0 correctness review: this script had no `set -e` at
# all, so a failed mkdir/cp/chmod during setup was silently ignored and
# `exec "$@"` still started Unbound afterward - possibly with no trust
# anchor file at all, producing a confusing runtime failure with no
# actual indication the real cause was a setup step that failed earlier.
set -eu

echo "========================================"
echo "RootGuard Unbound starting..."
echo "========================================"

# Overridable purely for docker-entrypoint.test.sh - a real container
# always uses the defaults.
ROOT_KEY_SOURCE="${ROOT_KEY_SOURCE:-/usr/share/dns/root.key}"
ROOT_KEY_PATH="${ROOT_KEY_PATH:-/var/lib/unbound/root.key}"
UNBOUND_STATE_DIR="${UNBOUND_STATE_DIR:-/var/lib/unbound}"
UNBOUND_MODULE_DIR="${UNBOUND_MODULE_DIR:-/etc/unbound/unbound.d}"

############################################################
# Ensure directories exist (container has no systemd)
#
# Writable trust anchor location:
#   $ROOT_KEY_PATH (/var/lib/unbound/root.key)
#
# Module directory for RootGuard GUI:
#   $UNBOUND_MODULE_DIR (/etc/unbound/unbound.d)
############################################################
mkdir -p "$UNBOUND_STATE_DIR"
mkdir -p "$UNBOUND_MODULE_DIR"

############################################################
# Initialize writable trust anchor (only once)
#
# Debian provides a reference trust anchor via dns-root-data:
#   $ROOT_KEY_SOURCE (/usr/share/dns/root.key)
#
# We copy it once into a writable location so Unbound can
# update it safely (RFC5011 rollover).
#
# `-s` (exists AND non-empty), not `-f` (exists) - found in the same
# review: a prior run that crashed mid-copy (OOM-killed, forcibly
# stopped) could leave a zero-byte or truncated root.key in place, which
# `-f` alone would treat as "already initialized" and never repair,
# silently running Unbound with a corrupt trust anchor forever after.
#
# Copied via a same-directory temp file plus `mv`, not written directly
# to $ROOT_KEY_PATH - `mv` within one filesystem is atomic, so a crash
# mid-copy now leaves either the complete old file or no file at all,
# never a partial one for the next startup's own `-s` check to
# mistake for a real trust anchor.
############################################################
if [ ! -s "$ROOT_KEY_PATH" ]; then
    echo "Initializing writable trust anchor..."
    tmp_root_key="${ROOT_KEY_PATH}.tmp.$$"
    cp "$ROOT_KEY_SOURCE" "$tmp_root_key"
    chmod 640 "$tmp_root_key"
    mv "$tmp_root_key" "$ROOT_KEY_PATH"
fi

############################################################
# Start Unbound in foreground (PID 1)
############################################################
exec "$@"

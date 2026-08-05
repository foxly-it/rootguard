#!/bin/sh

echo "========================================"
echo "RootGuard Unbound starting..."
echo "========================================"

############################################################
# Ensure directories exist (container has no systemd)
#
# Writable trust anchor location:
#   /var/lib/unbound/root.key
#
# Module directory for RootGuard GUI:
#   /etc/unbound/unbound.d
############################################################
mkdir -p /var/lib/unbound
mkdir -p /etc/unbound/unbound.d

############################################################
# Initialize writable trust anchor (only once)
#
# Debian provides a reference trust anchor via dns-root-data:
#   /usr/share/dns/root.key
#
# We copy it once into a writable location so Unbound can
# update it safely (RFC5011 rollover).
############################################################
if [ ! -f /var/lib/unbound/root.key ]; then
    echo "Initializing writable trust anchor..."
    cp /usr/share/dns/root.key /var/lib/unbound/root.key
    chmod 640 /var/lib/unbound/root.key
fi

############################################################
# Start Unbound in foreground (PID 1)
############################################################
exec "$@"

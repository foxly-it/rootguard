############################################################
# RootGuard Unbound - Production Base (Debian Forky)
############################################################

FROM debian:forky-slim

############################################################
# Install packages
#
# - unbound        : resolver
# - dns-root-data  : root.hints + root.key (reference)
# - ca-certificates: TLS trust store
# - dnsutils       : local DNS health checks
############################################################
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        unbound \
        dns-root-data \
        ca-certificates \
        dnsutils && \
    dpkg --compare-versions "$(dpkg-query -W -f='${Version}' unbound)" ge "1.25.2-1" && \
    rm -rf /var/lib/apt/lists/*

############################################################
# Stable runtime identity
#
# Debian system-user allocation changed from 100:101 to
# 996:996 between base-image releases. Persistent Docker
# volumes retain numeric ownership, so an image update would
# otherwise make the writable RFC5011 trust anchor inaccessible.
############################################################
RUN groupmod --gid 101 unbound \
    && usermod --uid 100 --gid 101 unbound \
    && test "$(id -u unbound)" = "100" \
    && test "$(id -g unbound)" = "101"

############################################################
# Create required directories
#
# /var/lib/unbound       -> writable trust anchor location
# /run/unbound           -> runtime directory
# /etc/unbound/unbound.d -> GUI modules directory
############################################################
RUN mkdir -p /var/lib/unbound \
    && mkdir -p /run/unbound \
    && mkdir -p /etc/unbound/unbound.d \
    && chown -R unbound:unbound /var/lib/unbound /run/unbound /etc/unbound/unbound.d

############################################################
# Copy config + entrypoint
############################################################
COPY unbound.conf /etc/unbound/unbound.conf
COPY unbound.d/ /etc/unbound/unbound.d/
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY healthcheck.sh /healthcheck.sh
RUN chmod +x /docker-entrypoint.sh /healthcheck.sh \
    && chown -R unbound:unbound /etc/unbound/unbound.d

############################################################
# Expose DNS port
############################################################
EXPOSE 5335/tcp
EXPOSE 5335/udp

############################################################
# Security: run as non-root (recommended)
#
# Notes:
# - Binding to port 5335 does not require root
# - Our writable paths are under /var/lib/unbound
# - Module directory exists and is writable for later mounts
############################################################
USER unbound

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/bin/sh", "/healthcheck.sh"]

############################################################
# Start command
############################################################
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["unbound", "-d", "-c", "/etc/unbound/unbound.conf"]

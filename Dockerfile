############################################################
# RootGuard Unbound - Production Base (Debian 13 stable-slim)
############################################################

FROM debian:stable-slim

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
    rm -rf /var/lib/apt/lists/*

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

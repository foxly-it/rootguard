############################################################
# RootGuard Unbound - reproducible upstream build on Debian 13
############################################################

ARG DEBIAN_IMAGE="debian:13-slim@sha256:020c0d20b9880058cbe785a9db107156c3c75c2ac944a6aa7ab59f2add76a7bd"
ARG UNBOUND_VERSION="1.25.2"
ARG UNBOUND_SHA256="0d92275c703d5f5f8baba3dab22117dd8c29b495588a5c229768ed6581566600"

FROM ${DEBIAN_IMAGE} AS builder

ARG UNBOUND_VERSION
ARG UNBOUND_SHA256
ARG DEBIAN_FRONTEND=noninteractive

# Direct build inputs are version-pinned. Debian verifies every downloaded
# package against its signed repository metadata; the base image is pinned by
# its multi-architecture OCI digest above.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential=12.12 \
        ca-certificates=20250419 \
        curl=8.14.1-2+deb13u4 \
        libevent-dev=2.1.12-stable-10+b1 \
        libexpat1-dev=2.8.3-1~deb13u1 \
        libssl-dev=3.5.7-1~deb13u2 \
        pkg-config=1.8.1-4 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

RUN curl --fail --show-error --silent --location \
        --output unbound.tar.gz \
        "https://nlnetlabs.nl/downloads/unbound/unbound-${UNBOUND_VERSION}.tar.gz" \
    && printf '%s  %s\n' "${UNBOUND_SHA256}" unbound.tar.gz | sha256sum --check --strict \
    && mkdir source \
    && tar --extract --gzip --file unbound.tar.gz --directory source --strip-components=1

WORKDIR /build/source

RUN ./configure \
        --prefix=/usr \
        --sysconfdir=/etc \
        --localstatedir=/var \
        --runstatedir=/run \
        --with-chroot-dir= \
        --with-libevent \
        --with-pthreads \
        --with-ssl=/usr \
        --with-libexpat=/usr \
        --disable-rpath \
        --disable-static \
    && make -j"$(nproc)" \
    && make DESTDIR=/out install \
    && /out/usr/sbin/unbound -V | grep -F "Version ${UNBOUND_VERSION}"

FROM ${DEBIAN_IMAGE}

ARG UNBOUND_VERSION
ARG UNBOUND_SHA256
ARG DEBIAN_FRONTEND=noninteractive
# COMMIT/CREATED default to a placeholder for local `docker build` (the
# release pipeline always passes real values) rather than staying unset -
# an empty label reads as "field missing" to the Stack Center's metadata
# completeness check the same way an absent one does.
ARG COMMIT=unknown
ARG CREATED=unknown

LABEL org.opencontainers.image.title="RootGuard Unbound" \
      org.opencontainers.image.description="DNSSEC-validating Unbound resolver built reproducibly from verified upstream source" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${UNBOUND_VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}" \
      io.rootguard.unbound.source="https://nlnetlabs.nl/downloads/unbound/unbound-${UNBOUND_VERSION}.tar.gz" \
      io.rootguard.unbound.source.sha256="${UNBOUND_SHA256}" \
      io.rootguard.base.digest="sha256:020c0d20b9880058cbe785a9db107156c3c75c2ac944a6aa7ab59f2add76a7bd"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bind9-dnsutils=1:9.20.26-1~deb13u1 \
        ca-certificates=20250419 \
        dns-root-data=2025080400~deb13u1 \
        libevent-2.1-7t64=2.1.12-stable-10+b1 \
        libexpat1=2.8.3-1~deb13u1 \
        libssl3t64=3.5.7-1~deb13u2 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 101 unbound \
    && useradd --uid 100 --gid 101 --system --home-dir /var/lib/unbound --no-create-home --shell /usr/sbin/nologin unbound \
    && test "$(id -u unbound)" = "100" \
    && test "$(id -g unbound)" = "101"

COPY --from=builder /out/ /

RUN mkdir -p /var/lib/unbound /run/unbound /etc/unbound/unbound.d \
    && chown -R unbound:unbound /var/lib/unbound /run/unbound /etc/unbound/unbound.d

COPY unbound.conf /etc/unbound/unbound.conf
COPY unbound.d/ /etc/unbound/unbound.d/
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY healthcheck.sh /healthcheck.sh

RUN chmod 0755 /docker-entrypoint.sh /healthcheck.sh \
    && chown -R unbound:unbound /etc/unbound/unbound.d \
    && unbound -V | grep -F "Version ${UNBOUND_VERSION}" \
    && cp /usr/share/dns/root.key /var/lib/unbound/root.key \
    && unbound-checkconf /etc/unbound/unbound.conf \
    && rm /var/lib/unbound/root.key

EXPOSE 5335/tcp
EXPOSE 5335/udp

USER unbound

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/bin/sh", "/healthcheck.sh"]

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["unbound", "-d", "-c", "/etc/unbound/unbound.conf"]

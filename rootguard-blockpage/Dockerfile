############################################################
# RootGuard Blockpage - static nginx image, no external assets
############################################################

ARG NGINX_IMAGE="nginx:1.29-alpine@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de"

FROM ${NGINX_IMAGE}

ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=unknown

LABEL org.opencontainers.image.title="RootGuard Blockpage" \
      org.opencontainers.image.description="RootGuard-branded landing page for AdGuard Home's custom IP for blocked hosts" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${CREATED}"

# Found in review, round 14: the stock nginx:1.29-alpine base - already
# the current tag as of this pin, confirmed live - carries 9 CVEs across
# these 8 packages, all already fixed on Alpine 3.23's own `main` repo
# (confirmed live via pkgs.alpinelinux.org) but not yet what this base
# image's own layer has baked in - a plain digest bump doesn't help here,
# since this pin already *is* the newest available "1.29-alpine" build.
# Pinned to each package's actual current version on that repo (not
# always trivy's own reported "fixed version" - expat and nghttp2 have
# both moved past their respective CVE fixes since), same stopgap pattern
# already used for docker:29-cli's OpenSSL pin in Core/Updater's
# Dockerfiles: replace with a plain digest bump once upstream publishes a
# nginx:1.29-alpine build with these baked in.
#
# Found live, round 15: Alpine 3.23's `main` repo moved libexpat past
# this pin's own 2.8.3-r0 to 2.8.4-r0 (the same libexpat CVE fix already
# applied to Core/Updater's docker:29-cli base this round) - the mirror
# no longer carries 2.8.3-r0 at all, and fontconfig's own `libexpat.so.1`
# dependency now resolves only against the newer build, so the pin
# outright broke every build (`apk`'s own "unable to select packages",
# confirmed live in CI). Bumped to match what's actually installable.
#
# Found live, round 16: same story for curl/libcurl - the mirror moved
# both from 8.20.0-r0 to 8.22.0-r0 and no longer carries the old build,
# breaking every build again. Confirmed live against the pinned base
# (`apk list -a`) that every other package below is still current;
# only curl/libcurl needed bumping this round. Also newly caught by
# trivy in the same CI run, not by the apk pin (libuuid wasn't pinned at
# all before): 6 HIGH util-linux/libuuid CVEs already fixed upstream at
# 2.41.6-r0. Added to the pinned set rather than left to trivy's own
# .trivyignore suppression, matching this file's existing policy of
# fixing what's actually fixable over suppressing it.
RUN apk add --no-cache --upgrade \
        c-ares=1.34.8-r0 \
        curl=8.22.0-r0 \
        libcurl=8.22.0-r0 \
        libcrypto3=3.5.8-r0 \
        libssl3=3.5.8-r0 \
        libexpat=2.8.4-r0 \
        libuuid=2.41.6-r0 \
        libxml2=2.13.9-r1 \
        nghttp2-libs=1.69.0-r0

RUN rm -f /etc/nginx/conf.d/default.conf
# Deliberately NOT under /etc/nginx/templates/: the stock image's own
# 20-envsubst-on-templates.sh hook scans that directory and runs its own
# unscoped envsubst (no variable allowlist) over anything ending in
# .template - which would also "substitute" this file's real nginx
# variables ($host, $remote_addr, $binary_remote_addr, ...) as if they were
# unset shell variables, wiping them out. Keeping the template outside that
# directory means only our own entrypoint script (which passes an explicit
# envsubst allowlist) ever touches it.
COPY nginx/blockpage.conf.template /etc/nginx/blockpage.conf.template
COPY nginx/entrypoint.d/19-render-blockpage-conf.sh /docker-entrypoint.d/19-render-blockpage-conf.sh
RUN chmod +x /docker-entrypoint.d/19-render-blockpage-conf.sh
COPY web/ /usr/share/nginx/html/

# Deliberately no USER instruction: the stock image's nginx.conf already
# has "user nginx;", which drops worker processes (the ones that handle
# every client connection) to the non-root nginx user once the master
# starts as root. Forcing the master itself to run non-root defeats that
# and breaks its own privilege drop, and root is still needed to create
# cache/pid files under the read_only + tmpfs mounts this service runs
# with (see the compose service Core renders).
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
    CMD wget -q -O - http://127.0.0.1:8080/clientip.txt >/dev/null || exit 1

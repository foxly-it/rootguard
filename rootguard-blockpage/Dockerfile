############################################################
# RootGuard Blockpage - static nginx image, no external assets
############################################################

ARG NGINX_IMAGE="nginx:1.29-alpine@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de"

FROM ${NGINX_IMAGE}

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="RootGuard Blockpage" \
      org.opencontainers.image.description="RootGuard-branded landing page for AdGuard Home's custom IP for blocked hosts" \
      org.opencontainers.image.source="https://github.com/foxly-it/rootguard" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}"

RUN rm -f /etc/nginx/conf.d/default.conf
COPY nginx/blockpage.conf /etc/nginx/conf.d/blockpage.conf
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

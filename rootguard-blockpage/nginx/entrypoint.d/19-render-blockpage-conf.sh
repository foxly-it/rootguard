#!/bin/sh
# Renders blockpage.conf from its template, substituting the AdGuard
# Basic-Auth token Core publishes to the shared read-only volume.
#
# Deliberately self-contained (reads its own token, does its own envsubst
# with an explicit variable allowlist, writes its own output) rather than
# relying on the stock 20-envsubst-on-templates.sh hook: that hook (a) forks
# executable /docker-entrypoint.d/*.sh scripts, so an export from an
# executable script here would die with that child and never reach it, and
# (b) runs its own unscoped envsubst (no allowlist) over anything under
# /etc/nginx/templates/, which would "substitute" this file's real nginx
# variables ($host, $remote_addr, ...) as if they were unset shell
# variables, wiping them out - hence the template lives outside that
# directory (see Dockerfile) and only this script ever touches it. A
# missing or empty token degrades to an empty Authorization header rather
# than failing the container start - the /api/reason location in the
# template treats that as "unavailable", same as AdGuard being down.
set -eu

token="$(cat /etc/nginx/secrets/basic-auth-token 2>/dev/null || true)"
ADGUARD_AUTH_TOKEN="$token" envsubst '${ADGUARD_AUTH_TOKEN}' \
  < /etc/nginx/blockpage.conf.template \
  > /etc/nginx/conf.d/blockpage.conf

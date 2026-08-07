#!/bin/sh
# Renders blockpage.conf from its template, substituting the AdGuard
# Basic-Auth token Core publishes to the shared read-only volume.
#
# Deliberately self-contained (reads its own token, does its own envsubst,
# writes its own output) rather than exporting ADGUARD_AUTH_TOKEN for the
# stock 20-envsubst-on-templates.sh hook to pick up: nginx's entrypoint
# forks executable /docker-entrypoint.d/*.sh scripts, so an export from an
# executable script here would die with that child and never reach the next
# sibling script. A missing or empty token degrades to an empty Authorization
# header rather than failing the container start - the /api/reason location
# in the template treats that as "unavailable", same as AdGuard being down.
set -eu

token="$(cat /etc/nginx/secrets/basic-auth-token 2>/dev/null || true)"
ADGUARD_AUTH_TOKEN="$token" envsubst '${ADGUARD_AUTH_TOKEN}' \
  < /etc/nginx/templates/blockpage.conf.template \
  > /etc/nginx/conf.d/blockpage.conf

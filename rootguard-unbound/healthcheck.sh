#!/bin/sh
set -eu

dig @127.0.0.1 -p 5335 cloudflare.com A +time=1 +tries=1 +short >/dev/null
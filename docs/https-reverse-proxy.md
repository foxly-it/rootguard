**English** · [Deutsch](https-reverse-proxy.de.md)

# HTTPS via a reverse proxy

RootGuard does not terminate its own TLS and never will - that would
largely reimplement what established reverse proxies like Caddy, Zoraxy,
Nginx Proxy Manager, or HAProxy already provide robustly, actively
maintained, and with automatic certificate renewal. This page describes
what RootGuard expects from a proxy in front of it, with one working
example for each of the four options above.

Only the WebGUI is affected (default port `8080`, `ROOTGUARD_WEB_PORT`).
The DNS port (`53`, TCP+UDP) is not an HTTP service and keeps running
independently of the proxy setup described here.

## Requirements

These two points aren't recommendations, they're requirements - without
them, login won't work, or the session cookie won't be marked secure:

1. **Pass the Host header through unchanged.** RootGuard's same-origin
   protection for write requests
   (`rootguard-webapp/backend/internal/httpapi/origin.go`) compares the
   browser's `Origin`/`Referer` header against the `Host` header arriving
   at the backend. If the proxy passes through an internal name (e.g.
   `localhost:8080` or the Docker service name) instead of the public
   hostname (e.g. `rootguard.example.com`), every login and every write
   action fails with `403 Cross-origin administration request rejected`.
   This is by far the most common failure cause with a newly set up proxy -
   see troubleshooting below.
2. **Set `X-Forwarded-Proto: https`.** RootGuard's session cookie only gets
   the `Secure` flag when the request either arrives directly over TLS or
   `X-Forwarded-Proto` is set to `https`
   (`rootguard-webapp/backend/internal/httpapi/auth.go`,
   `requestIsHTTPS`). Without this header, RootGuard treats the connection
   as plaintext HTTP and sets the cookie accordingly without `Secure` -
   login still works, but without the protection a TLS reverse proxy is
   actually supposed to provide.

All four examples below already satisfy both requirements. For your own,
differing configuration, always explicitly verify that exactly these two
headers arrive correctly.

## Caddy

Automatic certificate acquisition/renewal with no further configuration;
Caddy sets `X-Forwarded-*` and passes the host through correctly by
default.

```caddyfile
rootguard.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

If RootGuard itself runs in Docker and Caddy runs on the same host outside
the RootGuard Compose project, replace `127.0.0.1:8080` with the actually
reachable address (e.g. the port published on the Compose network, or the
service name plus internal port `8080` on a shared Docker network).

## Zoraxy

In the Zoraxy UI under *HTTP Proxy* → *Add Proxy Rule*:

- **Matching Domain:** `rootguard.example.com`
- **Target:** `http://127.0.0.1:8080` (or RootGuard's actual address on the
  relevant network)
- **TLS/SSL:** enable, let it obtain a certificate automatically
- Under advanced options, make sure **"Host Header Override" stays
  disabled** (the default) - this way Zoraxy passes the original Host
  header through unchanged, as required under "Requirements".

The `X-Forwarded-Proto` header is set automatically by Zoraxy when TLS
rewriting is active.

## Nginx Proxy Manager

Under *Proxy Hosts* → *Add Proxy Host*:

- **Domain Names:** `rootguard.example.com`
- **Forward Hostname/IP:** RootGuard's WebApp address
- **Forward Port:** `8080`
- **SSL** tab: request a certificate (Let's Encrypt) or provide your own,
  enable **Force SSL**.

Nginx Proxy Manager generates its host blocks with
`proxy_set_header Host $host;` and `proxy_set_header X-Forwarded-Proto
$scheme;` by default - both requirements are satisfied without manual
intervention. If you extend the generated configuration via *Advanced* →
*Custom Nginx Configuration*, don't remove or override these two lines.

## HAProxy

```haproxy
frontend rootguard_https
    bind *:443 ssl crt /etc/haproxy/certs/rootguard.example.com.pem
    http-request set-header X-Forwarded-Proto https
    default_backend rootguard_webapp

backend rootguard_webapp
    server webapp 127.0.0.1:8080 check
```

HAProxy already passes the `Host` header through unchanged in this basic
configuration (no `http-request set-header Host ...` present); the
explicit `X-Forwarded-Proto` is necessary because, unlike Caddy/Zoraxy/Nginx
Proxy Manager, HAProxy doesn't set it automatically. Certificate
acquisition and renewal aren't built into HAProxy itself - typically
handled by an upstream `certbot`/ACME client or a separate certificate
management setup that provides the PEM file at the path referenced here
and reloads HAProxy after renewal.

## Troubleshooting

**"Cross-origin administration request rejected" on every login/action:**
the proxy isn't passing the public hostname through as the `Host` header.
Check what actually arrives at the backend (e.g. `docker compose logs
webapp` during a login attempt, or a brief `tcpdump`/debug header). For a
custom Nginx-based configuration, add `proxy_set_header Host $host;`; for
Caddy/Zoraxy/NPM this should already be the default case - see the
respective notes above.

**Login works, but the session is lost on every page reload:**
`X-Forwarded-Proto` isn't arriving as `https`, the session cookie is set
without `Secure`, and the browser may discard it under mixed HTTP/HTTPS
conditions. Set the header explicitly as in the HAProxy example above.

## See also

- `docs/architecture.md` - session/cookie behavior in detail.
- `docs/threat-model.md` - the browser/session trust boundary.
- [ROADMAP.md](../ROADMAP.md), section 0.5 - scope decision: documented
  reverse-proxy operation instead of a RootGuard-native TLS implementation.

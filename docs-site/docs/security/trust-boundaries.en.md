# Trust boundaries

- WebApp without Docker socket or arbitrary host commands
- HttpOnly, SameSite Strict session and same-origin checks for writes
- Session inventory with targeted revocation, rate limiting, and an audit log for login and password recovery
- Core and Updater only on internal Docker networks and protected by bearer tokens
- Unbound read-only, non-root, without additional capabilities
- AdGuard administration without a public port
- Documented threat model plus automated dependency, container, secret, and static-analysis scans in CI

!!! note "HTTPS"
    RootGuard deliberately does not terminate its own TLS - a proxy in front of it does. [HTTPS via a reverse proxy](../guides/https-reverse-proxy.md) covers the two requirements (Host-header passthrough, `X-Forwarded-Proto`) and example configurations for Caddy, Zoraxy, Nginx Proxy Manager, and HAProxy.

[Open the full threat model](../security/threat-model.md)

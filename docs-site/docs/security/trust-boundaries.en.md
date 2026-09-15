# Trust boundaries

- WebApp without Docker socket or arbitrary host commands
- HttpOnly, SameSite Strict session and same-origin checks for writes
- Session inventory with targeted revocation, rate limiting, and an audit log for login and password recovery
- Core and Updater only on internal Docker networks and protected by bearer tokens
- Unbound read-only, non-root, without additional capabilities
- AdGuard administration without a public port
- Documented threat model plus automated dependency, container, secret, and static-analysis scans in CI

!!! note "HTTPS"
    RootGuard deliberately does not terminate its own TLS - a proxy in front of it does. The documentation covers the two requirements (Host-header passthrough, `X-Forwarded-Proto`) and example configurations for Caddy, Zoraxy, Nginx Proxy Manager, and HAProxy.

    [Open HTTPS guide ↗](https://github.com/foxly-it/rootguard/blob/main/docs/https-reverse-proxy.md){: target="_blank" rel="noopener" }

[Open full threat model ↗](https://github.com/foxly-it/rootguard/blob/main/docs/threat-model.md){: target="_blank" rel="noopener" }

# Protected native administration

AdGuard Home receives no public administration port. RootGuard runs the official setup internally, stores generated credentials in Core's protected volume, and exposes the interface only through `/adguard-ui/` behind the authenticated WebApp.

!!! note "Upstream protection"
    RootGuard expects Unbound at the fixed `172.29.53.2:5335` endpoint and verifies this chain after startup and updates.

## Blockpage

Instead of AdGuard's generic default response, a dedicated RootGuard-branded page explains why a domain was blocked, including the domain, timestamp, and client IP plus a plain-language rundown of common blocking reasons. Setup enables AdGuard's `blocking_mode: custom_ip` automatically and can be turned off via a Setup toggle; it stays on by default. Because a DNS-level blocking address cannot present a valid TLS certificate for arbitrary blocked domains, the page only appears over HTTP - HTTPS requests instead show the browser's own certificate warning, which is AdGuard Home's own documented behavior.

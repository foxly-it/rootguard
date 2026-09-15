# Deploy the DNS stack with guidance

1. **Sign in** — Use `ROOTGUARD_ADMIN_USER` and `ROOTGUARD_ADMIN_PASSWORD`. The session is protected server-side and expires after twelve hours. Starting with 0.1.0-alpha.2, "Forgot password?" also provides local recovery with a separate recovery key.
2. **Choose host address** — Choose an existing LAN IP. `0.0.0.0` binds all host addresses; a specific LAN IP limits exposure more narrowly.
3. **Run preflight** — RootGuard checks the address, port, Docker Engine, and Compose before changing containers. Occupied DNS ports are detected in two stages: first from already-published Docker ports, then via a real host-level bind attempt that also catches non-Docker processes such as systemd-resolved or dnsmasq. Failures are explained with a cause, next action, and collapsible technical details.
4. **Deploy** — Unbound is started and verified, then AdGuard Home is configured internally with Unbound as its only upstream.

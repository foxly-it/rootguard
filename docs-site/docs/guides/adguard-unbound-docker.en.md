# Setting up AdGuard Home and Unbound with Docker

This guide manually rebuilds exactly what RootGuard automates internally: AdGuard Home filters ads and trackers, Unbound resolves recursively with DNSSEC validation. Both images work independently of RootGuard itself - `adguard/adguardhome` is the official AdGuard image, `ghcr.io/foxly-it/rootguard-unbound` is RootGuard's own hardened Unbound image (non-root, `read_only`, `cap_drop: ALL`, DNSSEC validation baked in).

## Requirements

- Docker Engine with the Compose plugin (`docker compose version`).
- Port `53/tcp` and `53/udp` free on the host - on Debian/Ubuntu, `systemd-resolved` commonly blocks this port by default.

## docker-compose.yaml

```yaml
services:
  unbound:
    image: ghcr.io/foxly-it/rootguard-unbound:latest
    container_name: unbound
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    volumes:
      - unbound-config:/etc/unbound/unbound.d
      - unbound-state:/var/lib/unbound
    networks:
      - dns

  adguardhome:
    image: adguard/adguardhome:latest
    container_name: adguardhome
    restart: unless-stopped
    depends_on:
      - unbound
    ports:
      - "53:53/tcp"
      - "53:53/udp"
      - "3000:3000/tcp"
    volumes:
      - adguard-work:/opt/adguardhome/work
      - adguard-conf:/opt/adguardhome/conf
    networks:
      - dns

networks:
  dns:

volumes:
  unbound-config:
  unbound-state:
  adguard-work:
  adguard-conf:
```

## Setup

1. `docker compose up -d` in the directory containing the file above.
2. Unbound takes a few seconds to start (loading root hints and the DNSSEC trust anchor). Quick check:
   ```shell
   docker compose exec unbound dig @127.0.0.1 -p 5335 example.com A
   ```
3. Open the AdGuard Home setup wizard at `http://<host-ip>:3000` and create an admin account.
4. In the wizard, under **Upstream DNS servers**, enter:
   ```
   [/]unbound:5335
   ```
   The service name `unbound` resolves within the shared `dns` Docker network - no fixed IP needed.
5. Finish setup, then test:
   ```shell
   dig @<host-ip> example.com A
   dig @<host-ip> +dnssec cloudflare.com A
   ```
   An `ad` flag in the second response confirms Unbound is actively validating DNSSEC.
6. Point your router or individual devices at `<host-ip>` as their DNS server.

## Troubleshooting

**AdGuard Home won't start, port 53 is in use:** usually `systemd-resolved`. Check with `ss -tulpn | grep :53`. Either disable `systemd-resolved`'s own stub listener (`DNSStubListener=no` in `/etc/systemd/resolved.conf`, then `systemctl restart systemd-resolved`) or put AdGuard Home on a different host port.

**AdGuard Home isn't resolving anything even though both containers are running:** usually a misconfigured upstream. It must read exactly `unbound:5335` (the service name, not `localhost` or `127.0.0.1` - from inside the AdGuard container, that would be the wrong host).

## See also

- [Unbound examples](unbound-examples.en.md) - local zones, conditional forwarding, and more, once Unbound is managed through RootGuard instead of by hand.
- Maintaining this configuration by hand means no guided interface, no update checking, no automatic backups. For all of that, see [Getting started with RootGuard](../getting-started.en.md) - the same two images, managed centrally through one web interface.

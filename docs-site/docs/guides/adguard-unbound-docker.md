# AdGuard Home und Unbound mit Docker einrichten

Dieser Guide baut per Hand genau das nach, was RootGuard intern automatisiert: AdGuard Home filtert Werbung und Tracker, Unbound löst rekursiv mit DNSSEC-Validierung auf. Beide Images sind unabhängig von RootGuard selbst nutzbar - `adguard/adguardhome` ist das offizielle AdGuard-Image, `ghcr.io/foxly-it/rootguard-unbound` ist RootGuards eigenes, gehärtetes Unbound-Image (nicht-root, `read_only`, `cap_drop: ALL`, DNSSEC-Validierung fest verdrahtet).

## Voraussetzungen

- Docker Engine mit Compose-Plugin (`docker compose version`).
- Port `53/tcp` und `53/udp` frei auf dem Host - unter Debian/Ubuntu blockiert `systemd-resolved` diesen Port häufig standardmäßig.

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

## Einrichtung

1. `docker compose up -d` im Verzeichnis mit der obigen Datei.
2. Unbound braucht ein paar Sekunden zum Start (lädt Root-Hints und den DNSSEC-Trust-Anchor). Kurz prüfen:
   ```shell
   docker compose exec unbound dig @127.0.0.1 -p 5335 example.com A
   ```
3. Den AdGuard-Home-Einrichtungsassistenten unter `http://<Host-IP>:3000` öffnen, Admin-Zugang anlegen.
4. Im Assistenten unter **Upstream-DNS-Server** eintragen:
   ```
   [/]unbound:5335
   ```
   Der Servicename `unbound` ist im gemeinsamen Docker-Netzwerk `dns` auflösbar - keine feste IP nötig.
5. Einrichtung abschließen, dann testen:
   ```shell
   dig @<Host-IP> example.com A
   dig @<Host-IP> +dnssec cloudflare.com A
   ```
   Ein `ad`-Flag in der zweiten Antwort bestätigt aktive DNSSEC-Validierung durch Unbound.
6. Router oder einzelne Geräte auf `<Host-IP>` als DNS-Server umstellen.

## Fehlerbehebung

**AdGuard Home startet nicht, Port 53 belegt:** Meist `systemd-resolved`. Prüfen mit `ss -tulpn | grep :53`. Entweder `systemd-resolved`s eigenen Stub-Listener deaktivieren (`DNSStubListener=no` in `/etc/systemd/resolved.conf`, danach `systemctl restart systemd-resolved`) oder AdGuard Home auf einen anderen Host-Port legen.

**AdGuard Home löst nicht auf, obwohl beide Container laufen:** Meistens ein falsch eingetragener Upstream. Er muss exakt `unbound:5335` lauten (Servicename, nicht `localhost` oder `127.0.0.1` - das wäre aus Sicht des AdGuard-Containers der falsche Host).

## Siehe auch

- [Unbound: Anwendungsbeispiele](unbound-examples.md) - lokale Zonen, Conditional Forwarding und mehr, sobald Unbound über RootGuard statt von Hand verwaltet wird.
- Diese Konfiguration von Hand pflegen bedeutet: keine geführte Oberfläche, keine Update-Prüfung, kein automatisches Backup. Für all das siehe [Erste Schritte mit RootGuard](../getting-started.md) - dieselben zwei Images, zentral über eine Weboberfläche verwaltet.

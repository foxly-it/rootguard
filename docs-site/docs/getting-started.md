# Erste Schritte

## Voraussetzungen

- Docker Engine oder Docker Desktop mit Docker Compose v2
- Empfohlen mindestens 2 vCPU und 2 GB RAM für den Docker-Host
- Eine feste LAN-IP oder DHCP-Reservierung für den RootGuard-Host
- Freier TCP- und UDP-Port 53 auf der gewählten Host-Adresse
- Git für das Repository
- Ein starkes Admin-Passwort und ein zufälliges internes API-Token

!!! success "Stable"
    RootGuard 1.0 ist erschienen: Kernpfade, Updates mit Rollback und unveränderliche, attestierte Releases sind produktionsreif. Wie bei jeder Netzwerkinfrastruktur empfiehlt sich trotzdem, Backups und einen alternativen DNS-Weg bereitzuhalten.

## Plattformen und Prüfung

Jedes Release wird mit demselben geschützten Ende-zu-Ende-Test auf nativen Linux-amd64-/arm64-Runnern und Docker Desktop geprüft. Der Test umfasst Login, AIO-Bereitstellung, rekursive DNS-Auflösung und die Ablehnung einer ungültigen DNSSEC-Kette.

| Plattform | Architektur | Prüfung | Status |
| --- | --- | --- | --- |
| Linux, GitHub-gehosteter Runner (Ubuntu 24.04) | `amd64` | Nativer automatischer Runner | Bestanden 2026-07-28 |
| Linux, GitHub-gehosteter Runner (Ubuntu 24.04) | `arm64` | Nativer automatischer Runner | Bestanden 2026-07-28 |
| Docker Desktop 4.x auf macOS | Apple Silicon / `arm64` | Derselbe portable Prüfer | Bestanden 2026-07-28 |

Upgrade (ein echtes N-1 → N-Upgrade über den Control-Plane-Updater) und Backup/Restore (echter Export → Teardown → Neuinstallation → Restore) laufen auf derselben nativen `amd64`/`arm64`-Matrix, nicht nur die Neuinstallation.

**Unterstützte Plattformen:**

- **Linux** (jede Distribution) mit Docker Engine + Compose v2, `amd64` oder `arm64` - das primäre, vollständig geprüfte Ziel.
- **Docker Desktop auf macOS**, Apple Silicon (`arm64`) - geprüft; Intel-Macs verwenden dieselben veröffentlichten `amd64`-Manifeste, werden aber nicht separat getestet.
- **Docker Desktop auf Windows** (WSL2-Backend) - noch nicht in der Prüfmatrix, aber erwartet zu funktionieren.

Bare-Metal-/systemd-Installationen und Multi-Node-Deployments sind bewusst nicht unterstützt: RootGuard ist eine Single-Node-Docker-Appliance.

!!! warning "Docker-Engine-Version"
    RootGuards Core- und Updater-Container rufen an drei Stellen `docker cp` auf (Backup-Export, Backup-Restore, Update-Rollback). Drei `docker cp`-Schwachstellen (CVE-2026-41567, CVE-2026-41568, CVE-2026-42306) wurden upstream in Docker Engine 29.5.1 behoben. **Verwende Docker Engine 29.5.1 oder neuer**, oder bestätige, dass deine Distribution alle drei Fixes zurückportiert hat.

Der Installer erzwingt kein hartes Minimum, aber komfortabel läuft der volle Stack (Core, WebApp, Updater, AdGuard Home, Unbound) bereits mit 1 vCPU / 2 GB RAM bei leichter Last. Praktische Empfehlung: **2 vCPU, 2 GB RAM** als Grundausstattung für ein reales Heimnetzwerk.

**Bekannte Grenzen:** nur Single-Node ohne Hochverfügbarkeit; Upgrade-Kompatibilität ist nur N-1 → N (Versionen überspringen ist nicht unterstützt); Restore ist eine Clean-Replacement-Operation, kein In-Place-Merge.

## Installation

!!! info "Automatische Installation"
    Für die schnelle Variante mit automatischer Docker-Erkennung/-Installation, automatisch erzeugten Sicherheitsschlüsseln und einer kurzen Abfrage für Benutzername/Passwort siehe den Ein-Befehl-Schnellstart auf der [Startseite](https://rootguard.foxly.de/#quickstart). Die folgenden Schritte zeigen den manuellen Weg zum Nachvollziehen.

```shell
mkdir rootguard && cd rootguard

curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/compose.release.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/.env.release.example

# Zwei unabhängige Sicherheitsschlüssel erzeugen
openssl rand -hex 32
openssl rand -hex 32

# .env ausfüllen, dann RootGuard starten
docker compose -f compose.release.yaml up -d
```

Trage zwei getrennt erzeugte Zufallswerte als `ROOTGUARD_API_TOKEN` und `ROOTGUARD_RECOVERY_TOKEN` sowie ein eigenes starkes `ROOTGUARD_ADMIN_PASSWORD` in `.env` ein. RootGuard lädt versionierte amd64-/arm64-Images aus GHCR; ein Checkout oder lokaler Build der Komponenten ist nicht erforderlich.

```text
http://localhost:8080/login
```

## Erste Einrichtung

1. **Anmelden** - Verwende `ROOTGUARD_ADMIN_USER` und `ROOTGUARD_ADMIN_PASSWORD`. Die Sitzung bleibt serverseitig geschützt und läuft nach zwölf Stunden ab. „Passwort vergessen?" bietet zusätzlich eine lokale Wiederherstellung mit einem separaten Recovery-Schlüssel.
2. **Host-Adresse wählen** - Wähle eine bereits vorhandene LAN-IP. `0.0.0.0` bindet alle Host-Adressen, eine konkrete LAN-IP begrenzt die Erreichbarkeit enger.
3. **Vorprüfung ausführen** - RootGuard prüft Adresse, Port, Docker Engine und Compose, bevor Container verändert werden. Belegte DNS-Ports werden zweistufig erkannt, auch bei lokalen Resolvern wie systemd-resolved oder dnsmasq.
4. **Bereitstellen** - Unbound wird gestartet und geprüft, danach AdGuard Home intern eingerichtet und ausschließlich mit Unbound als Upstream verbunden.

## Router & Clients

Trage die im Setup angezeigte feste Host-IP als DNS-Server im Router ein. Verwende niemals `127.0.0.1` oder die interne Docker-Adresse `172.29.53.2` auf anderen Geräten. Port 53 muss für TCP und UDP erreichbar sein.

```shell title="Prüfung von einem Client"
dig @192.168.178.10 example.com A
dig +dnssec @192.168.178.10 dnssec-failed.org A
```

Die erste Abfrage muss eine Adresse liefern. Die zweite muss mit `SERVFAIL` enden; dadurch wird eine ungültige DNSSEC-Kette korrekt verworfen.

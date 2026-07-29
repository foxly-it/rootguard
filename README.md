# RootGuard — Self-hosted DNS protection

[Deutsch](#deutsch) · [English](#english)

![RootGuard – Self-hosted DNS protection](assets/rootguard-social-preview.png)

<a id="deutsch"></a>

## Deutsch

**RootGuard schützt alle Geräte in deinem Netzwerk zentral vor Werbung und
bekannten Trackern.** Eine übersichtliche Weboberfläche verbindet dafür
[AdGuard Home](https://github.com/AdguardTeam/AdGuardHome) mit einem eigenen
rekursiven [Unbound](https://github.com/NLnetLabs/unbound)-Resolver – als
vollständiger, selbst betriebener Docker-Compose-Stack für netzwerkweite
DNS-Filterung, rekursive DNS-Auflösung und DNSSEC-Validierung.

[![Release](https://img.shields.io/github/v/release/foxly-it/rootguard?include_prereleases&label=release)](https://github.com/foxly-it/rootguard/releases)
[![CI](https://github.com/foxly-it/rootguard/actions/workflows/ci.yml/badge.svg)](https://github.com/foxly-it/rootguard/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/foxly-it/rootguard)](LICENSE)
[![Website](https://img.shields.io/badge/website-rootguard.foxly.de-72c483)](https://rootguard.foxly.de/)

[Website](https://rootguard.foxly.de/) ·
[Handbuch](https://rootguard.foxly.de/docs.html) ·
[Wiki](https://rootguard.foxly.de/wiki.html) ·
[Roadmap](https://rootguard.foxly.de/roadmap.html) ·
[Releases](https://github.com/foxly-it/rootguard/releases)

> [!IMPORTANT]
> RootGuard befindet sich in einer öffentlichen Alpha. Die Version ist zum
> Ausprobieren und für reproduzierbare Rückmeldungen gedacht. Sie ist noch nicht
> für den Einsatz als einziger DNS-Dienst in einer Produktivumgebung vorgesehen.

## Warum RootGuard?

Ein DNS-Filter kann Werbung und Tracker für Fernseher, Smartphones,
Spielekonsolen und Computer blockieren, ohne auf jedem Gerät eine Erweiterung
zu installieren. RootGuard ergänzt diese Filterung um einen eigenen Resolver
und eine gemeinsame Bedienoberfläche.

- **Netzwerkweite Filterung:** AdGuard Home stoppt unerwünschte DNS-Anfragen.
- **Eigene DNS-Auflösung:** Unbound fragt die DNS-Hierarchie rekursiv ab und
  validiert DNSSEC.
- **Zentrale Verwaltung:** Setup, Konfiguration, Updates und Rollbacks laufen
  über die RootGuard-Weboberfläche.
- **Geführtes lokales DNS:** Lokale Einträge, private Domains, Conditional
  Forwarding und sichere RFC1918-Reverse-Zonen kommen ohne rohe
  Unbound-Konfiguration aus.
- **Self-hosted und offen:** Daten und Kontrolle bleiben auf dem eigenen
  Docker-Host; der Quellcode ist unter AGPL-3.0-or-later verfügbar.

```text
Geräte im Netzwerk → AdGuard Home → Unbound → DNS-Hierarchie
                         Filter       DNSSEC
```

## Quick Start

Voraussetzung ist ein Rechner mit Docker Compose v2. Die öffentliche Alpha
verwendet fertige Images für `amd64` und `arm64`; ein lokaler Build ist nicht
notwendig.

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/.env.alpha.example
```

Erzeuge zwei voneinander unabhängige Sicherheitsschlüssel:

```sh
openssl rand -hex 32
openssl rand -hex 32
```

Trage in `.env` ein eigenes starkes `ROOTGUARD_ADMIN_PASSWORD` sowie die beiden
Werte getrennt als `ROOTGUARD_API_TOKEN` und `ROOTGUARD_RECOVERY_TOKEN` ein.
Starte anschließend den Stack:

```sh
docker compose -f compose.alpha.yaml up -d
```

Öffne `http://<IP-des-Docker-Hosts>:8080/login` und folge dem geführten Setup.
Die vollständigen Voraussetzungen, Router-Einrichtung und Fehlerbehebung stehen
im [Handbuch](https://rootguard.foxly.de/docs.html#quickstart).

## Was ist enthalten?

| Komponente | Aufgabe |
| --- | --- |
| **RootGuard WebApp** | Login, Dashboard und geführte Bedienung |
| **RootGuard Core** | Orchestrierung und geprüfte Konfigurationsänderungen |
| **RootGuard Updater** | Gemeinsame Core-/WebApp-Updates mit Rollback |
| **AdGuard Home** | Netzwerkweite DNS-Filterung |
| **Unbound** | Rekursive DNS-Auflösung und DNSSEC-Validierung |

Die [Live-Produktansicht](https://rootguard.foxly.de/) zeigt die aktuelle
Oberfläche. Architektur, Vertrauensgrenzen und Update-Abläufe sind bewusst aus
diesem Einstieg ausgelagert:

- [Installation und Betrieb](https://rootguard.foxly.de/docs.html)
- [Architektur](docs/architecture.md)
- [Aktueller Projektstand](docs/project-state.md)
- [Roadmap bis 1.0](ROADMAP.md)
- [Release Notes v0.1.0-alpha.3](RELEASE_NOTES_0.1.0-alpha.3.md)
- [Image-Digests und Aufbewahrungsregeln](docs/image-retention-policy.md)
- [Geprüfte Installationsplattformen](docs/platform-support.md)

## Entwicklung

Für Änderungen am gesamten Projekt wird das Repository mit seinen
Komponenten-Submodules geklont:

```sh
git clone --recurse-submodules https://github.com/foxly-it/rootguard.git
cd rootguard
cp .env.example .env
docker compose up --build -d
```

Die Komponenten bleiben eigenständige Repositories:

| Repository | Verantwortung |
| --- | --- |
| [`rootguard-core`](https://github.com/foxly-it/rootguard-core) | Control Plane und DNS-Orchestrierung |
| [`rootguard-webapp`](https://github.com/foxly-it/rootguard-webapp) | Weboberfläche und Sitzungsverwaltung |
| [`rootguard-updater`](https://github.com/foxly-it/rootguard-updater) | Kontrollierte Control-Plane-Updates |
| [`rootguard-unbound`](https://github.com/foxly-it/rootguard-unbound) | Gehärtetes Unbound-Image |

## Mitwirken

Beiträge, Tests und verständliche Dokumentation sind willkommen. Der
[Beitragsleitfaden](CONTRIBUTING.md) erklärt Entwicklungssetup, Zuständigkeiten,
Tests und Pull Requests.

Ein guter Einstieg sind Issues mit
[`good first issue`](https://github.com/foxly-it/rootguard/labels/good%20first%20issue)
oder [`help wanted`](https://github.com/foxly-it/rootguard/labels/help%20wanted).
Sicherheitsprobleme bitte nicht öffentlich melden, sondern den Hinweisen in
[SECURITY.md](SECURITY.md) folgen.

## Lizenz und Marke

RootGuard ist freie Software unter
[GNU AGPL-3.0-or-later](LICENSE). Hinweise zur Nutzung der Namen und Logos von
RootGuard und Foxly IT stehen in [TRADEMARKS.md](TRADEMARKS.md).

---

<a id="english"></a>

## English

**RootGuard centrally protects every device on your network from ads and known
trackers.** Its approachable web interface combines
[AdGuard Home](https://github.com/AdguardTeam/AdGuardHome) with a private,
recursive [Unbound](https://github.com/NLnetLabs/unbound) resolver in one
self-hosted Docker Compose stack for network-wide DNS filtering, recursive DNS,
and DNSSEC validation.

[Website](https://rootguard.foxly.de/) ·
[Documentation](https://rootguard.foxly.de/docs.html) ·
[Wiki](https://rootguard.foxly.de/wiki.html) ·
[Roadmap](https://rootguard.foxly.de/roadmap.html) ·
[Releases](https://github.com/foxly-it/rootguard/releases)

> [!IMPORTANT]
> RootGuard is a public alpha intended for evaluation and reproducible
> feedback. It is not yet recommended as the only DNS service for a production
> network.

### Why RootGuard?

A DNS filter can block ads and trackers for TVs, smartphones, game consoles,
computers, and IoT devices without installing an extension on every device.
RootGuard adds a private recursive resolver and one coherent management
interface.

- **Network-wide ad and tracker blocking:** AdGuard Home filters unwanted DNS
  requests before they reach your devices.
- **Private recursive DNS:** Unbound resolves names through the DNS hierarchy
  and validates DNSSEC signatures.
- **Unified management:** Guided setup, configuration, updates, health checks,
  diagnostics, and rollbacks live in the RootGuard WebGUI.
- **Guided local DNS:** Manage local records, private domains, conditional
  forwarding, and RFC1918 reverse zones without editing raw Unbound files.
- **Self-hosted and open source:** Data and control remain on your Docker host;
  the source is available under AGPL-3.0-or-later.

```text
Network devices → AdGuard Home → Unbound → DNS hierarchy
                       filtering      DNSSEC
```

### Quick start

RootGuard requires Docker Compose v2. The public alpha provides ready-made
`amd64` and `arm64` container images, so no local build is required.

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/.env.alpha.example
```

Generate two independent random security tokens:

```sh
openssl rand -hex 32
openssl rand -hex 32
```

Set a strong `ROOTGUARD_ADMIN_PASSWORD` in `.env` and use the generated values
separately for `ROOTGUARD_API_TOKEN` and `ROOTGUARD_RECOVERY_TOKEN`. Then start
the stack:

```sh
docker compose -f compose.alpha.yaml up -d
```

Open `http://<docker-host-ip>:8080/login` and follow the guided setup. See the
[installation guide](https://rootguard.foxly.de/docs.html#quickstart) for
requirements, router configuration, upgrades, and troubleshooting.

### Included components

| Component | Responsibility |
| --- | --- |
| **RootGuard WebApp** | Login, dashboard, and guided management |
| **RootGuard Core** | Orchestration and validated configuration changes |
| **RootGuard Updater** | Coordinated Core/WebApp updates with rollback |
| **AdGuard Home** | Network-wide DNS filtering |
| **Unbound** | Recursive DNS resolution and DNSSEC validation |

Further technical information:

- [Architecture and trust boundaries](docs/architecture.md)
- [Current project state](docs/project-state.md)
- [Roadmap to 1.0](ROADMAP.md)
- [Release notes for v0.1.0-alpha.3](RELEASE_NOTES_0.1.0-alpha.3.md)
- [Image digests and retention policy](docs/image-retention-policy.md)
- [Verified installation platforms](docs/platform-support.md)

### Development and contributions

Clone the main repository with all component submodules:

```sh
git clone --recurse-submodules https://github.com/foxly-it/rootguard.git
cd rootguard
cp .env.example .env
docker compose up --build -d
```

Contributions, testing, and clear documentation are welcome. Read
[CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request. Report
security vulnerabilities privately by following [SECURITY.md](SECURITY.md).

### License and trademark

RootGuard is free and open-source software licensed under
[GNU AGPL-3.0-or-later](LICENSE). See [TRADEMARKS.md](TRADEMARKS.md) for the
RootGuard and Foxly IT name and logo policy.

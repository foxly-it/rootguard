# RootGuard

RootGuard bündelt die gemeinsam entwickelten Komponenten des Projekts in einem
übergeordneten Repository. Die einzelnen Komponenten bleiben eigenständige
Git-Repositories und werden hier als Submodules eingebunden.

## Komponenten

- `rootguard-core` – zentrale RootGuard-Anwendung
- `rootguard-webapp` – Weboberfläche und zugehöriges Backend
- `rootguard-unbound` – Unbound-basierter DNS-Dienst

## Repository klonen

```sh
git clone --recurse-submodules git@github.com:foxly-it/rootguard.git
cd rootguard
```

Bei einem bereits vorhandenen Checkout werden die Submodules so geladen:

```sh
git submodule update --init --recursive
```

## Entwicklungsstack starten

RootGuard benötigt ein internes API-Token und ein Passwort für die
Weboberfläche. Beide Werte werden ausschließlich lokal in `.env` gespeichert:

```sh
cp .env.example .env
openssl rand -hex 32
```

Den ausgegebenen Zufallswert als `ROOTGUARD_API_TOKEN` eintragen, ein starkes
`ROOTGUARD_ADMIN_PASSWORD` setzen und anschließend starten:

```sh
docker compose up --build -d
```

Danach sind erreichbar:

- RootGuard WebApp: `http://localhost:8080`

In der WebApp kann AdGuard Home anschließend unter **AdGuard Home** sicher
initialisiert werden. RootGuard erzeugt die internen Zugangsdaten und trägt
`172.29.53.2:5335` als einzigen Upstream-DNS im internen DNS-Netz ein. Weder
die native AdGuard-Verwaltung noch Core und sein Docker-Zugriff werden nach
außen veröffentlicht; privilegierte Aktionen laufen ausschließlich über die
authentifizierte Webapp.

> Der aktuelle Stack ist ein Entwicklungsstand. Vor dem Einsatz als DNS für
> ein gesamtes Netzwerk müssen insbesondere HTTPS, Wiederherstellungstests und
> die RootGuard-Oberflächen für Filter, Clients und Abfragestatistiken ergänzt
> werden.

## Website und Dokumentation

Die Projektwebsite wird aus `site/` über GitHub Pages veröffentlicht und ist
für `https://rootguard.foxly.de` vorgesehen. Sie beschreibt Architektur,
Sicherheitsgrenzen, Schnellstart und Roadmap bewusst als aktive Entwicklung.

## Komponenten aktualisieren

Änderungen werden zuerst im jeweiligen Komponenten-Repository committed und
gepusht. Anschließend wird der neue Komponenten-Stand im Hauptrepository
festgehalten:

```sh
git submodule update --remote
git add rootguard-core rootguard-webapp rootguard-unbound
git commit -m "Update RootGuard components"
```

Weitere Hinweise zur Struktur stehen in
[`docs/architecture.md`](docs/architecture.md).

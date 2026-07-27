# Zu RootGuard beitragen

Danke für dein Interesse an RootGuard. Beiträge können Code, Tests,
Dokumentation, Übersetzungen oder reproduzierbare Fehlerberichte sein.

## Einstieg

1. Prüfe bestehende [Issues](https://github.com/foxly-it/rootguard/issues) und
   die [Roadmap](ROADMAP.md).
2. Für den ersten Beitrag eignen sich insbesondere
   [`good first issue`](https://github.com/foxly-it/rootguard/labels/good%20first%20issue)
   und [`help wanted`](https://github.com/foxly-it/rootguard/labels/help%20wanted).
3. Beschreibe bei größeren Änderungen vor der Umsetzung kurz den geplanten
   Lösungsweg im Issue.

Sicherheitslücken gehören nicht in öffentliche Issues. Verwende dafür den in
[SECURITY.md](SECURITY.md) beschriebenen vertraulichen Meldeweg.

## Entwicklungsumgebung

```sh
git clone --recurse-submodules https://github.com/foxly-it/rootguard.git
cd rootguard
cp .env.example .env
```

Setze in `.env` ein starkes Admin-Passwort sowie getrennte zufällige API- und
Recovery-Token. Anschließend lässt sich der Entwicklungsstack bauen:

```sh
docker compose up --build -d
```

Bei einem bestehenden Checkout:

```sh
git submodule update --init --recursive
```

## Das richtige Repository wählen

Dieses Hauptrepository koordiniert Compose, Website, Dokumentation, Releases
und die Komponentenstände. Änderungen am Anwendungscode gehören zunächst in
das jeweilige Komponenten-Repository:

- `rootguard-core` – Orchestrierung und interne API
- `rootguard-webapp` – Benutzeroberfläche und Anmeldung
- `rootguard-updater` – kontrollierte Core-/WebApp-Updates
- `rootguard-unbound` – Unbound-Image und Resolver-Basis

Nach einem gemergten Komponenten-PR wird der zugehörige Submodule-Stand im
Hauptrepository aktualisiert.

## Änderungen umsetzen

- Halte einen Pull Request auf ein klar abgegrenztes Problem beschränkt.
- Ergänze oder aktualisiere Tests für geändertes Verhalten.
- Aktualisiere Handbuch, Wiki und Projektstatus, wenn sich sichtbares Verhalten
  oder der dokumentierte Funktionsumfang ändert.
- Veröffentliche keine Zugangsdaten, `.env`-Dateien, Tokens oder privaten
  Netzwerkdetails.
- Verwende für neue Texte die deutsche und englische Variante, wenn die
  betroffene Oberfläche zweisprachig ist.

## Vor dem Pull Request

Für Änderungen im Hauptrepository:

```sh
git diff --check
docker compose config
```

Führe zusätzlich die Tests des betroffenen Komponenten-Repositories aus. Die
CI des Hauptrepositories startet einen vollständigen Stack und prüft Login,
Setup, DNS-Auflösung und DNSSEC.

## Pull Request

Ein Pull Request sollte enthalten:

- eine kurze Erklärung des Problems und der Lösung;
- die ausgeführten Prüfungen;
- Screenshots bei sichtbaren Änderungen;
- Hinweise zu Migration, Konfiguration oder bekannten Einschränkungen;
- eine Verknüpfung zum zugehörigen Issue, sofern vorhanden.

Mit einem Beitrag erklärst du dich damit einverstanden, ihn unter der
Projektlizenz [AGPL-3.0-or-later](LICENSE) zu veröffentlichen.

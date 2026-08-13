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
git clone https://github.com/foxly-it/rootguard.git
cd rootguard
cp .env.example .env
```

Setze in `.env` ein starkes Admin-Passwort sowie getrennte zufällige API- und
Recovery-Token. Anschließend lässt sich der Entwicklungsstack bauen:

```sh
docker compose up --build -d
```

## Das richtige Verzeichnis wählen

RootGuard ist ein Monorepo. Anwendungscode gehört in das jeweilige
Komponentenverzeichnis, jedes mit eigenem Dockerfile und eigenem
pfadgefiltertem CI-Workflow:

- `rootguard-core` – Orchestrierung und interne API
- `rootguard-webapp` – Benutzeroberfläche und Anmeldung
- `rootguard-updater` – kontrollierte Core-/WebApp-Updates
- `rootguard-unbound` – Unbound-Image und Resolver-Basis

Ein Pull Request kann ein oder mehrere dieser Verzeichnisse gemeinsam mit den
zugehörigen Doku-Updates (`ROADMAP.md`, `docs/project-state.md`) in einem
Schritt ändern.

## Änderungen umsetzen

- Halte einen Pull Request auf ein klar abgegrenztes Problem beschränkt.
- Ergänze oder aktualisiere Tests für geändertes Verhalten.
- Aktualisiere Handbuch, Wiki und Projektstatus, wenn sich sichtbares Verhalten
  oder der dokumentierte Funktionsumfang ändert.
- Veröffentliche keine Zugangsdaten, `.env`-Dateien, Tokens oder privaten
  Netzwerkdetails.
- Verwende für neue Texte die deutsche und englische Variante, wenn die
  betroffene Oberfläche zweisprachig ist.

## Commit-Nachrichten

Der Titel jedes Commits (bzw. bei einem gesquashten Pull Request: der
Merge-Commit-Titel) folgt [Conventional Commits](https://www.conventionalcommits.org/):

```
<typ>(<optionaler bereich>): <kurze beschreibung>
```

Gebräuchliche Typen: `feat` (neues Verhalten), `fix` (Bugfix), `docs`
(Dokumentation), `refactor`, `perf`, `test`, `ci`, `chore`. Ein
`!` nach dem Typ/Bereich (z. B. `feat!:`) oder eine `BREAKING CHANGE:`-Zeile
im Commit-Body markiert eine rückwärtsinkompatible Änderung.

Daraus generiert `cliff.toml` bei jedem Release automatisch den Abschnitt in
[CHANGELOG.md](CHANGELOG.md) - ein Commit ohne passenden Typ landet dort
lediglich unter „Other" statt in der passenden Kategorie, bricht aber nichts.

## Vor dem Pull Request

Für Änderungen im Hauptrepository:

```sh
git diff --check
docker compose config
```

Führe zusätzlich die Tests des betroffenen Komponentenverzeichnisses aus. Die
Integrations-CI startet zudem einen vollständigen Stack und prüft Login,
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

## Release-Prozess (nur Maintainer)

Ein neuer Alpha-Release wird über den Workflow „Cut next alpha release"
(`.github/workflows/release-version-bump.yml`, manuell per
`workflow_dispatch` ausgelöst) angestoßen. Er ermittelt die nächste
Versionsnummer selbst, generiert den passenden Abschnitt in
[CHANGELOG.md](CHANGELOG.md) aus der Commit-Historie seit dem letzten Tag,
committet und taggt - der bestehende `release-alpha.yml`-Workflow übernimmt
danach unverändert Build, Signierung und Veröffentlichung wie bisher.

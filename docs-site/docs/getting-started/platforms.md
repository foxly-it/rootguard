# Saubere Installation, gleiche Prüfung

Jedes Release wird mit demselben geschützten Ende-zu-Ende-Test auf nativen Linux-amd64-/arm64-Runnern und Docker Desktop geprüft. Der Test umfasst Login, AIO-Bereitstellung, rekursive DNS-Auflösung und die Ablehnung einer ungültigen DNSSEC-Kette.

## Prüfmatrix

| Plattform | Architektur | Prüfung | Status |
| --- | --- | --- | --- |
| Linux, GitHub-gehosteter Runner (Ubuntu 24.04) | `amd64` | Nativer automatischer Runner | Bestanden 2026-07-28 |
| Linux, GitHub-gehosteter Runner (Ubuntu 24.04) | `arm64` | Nativer automatischer Runner | Bestanden 2026-07-28 |
| Docker Desktop 4.x auf macOS | Apple Silicon / `arm64` | Derselbe portable Prüfer | Bestanden 2026-07-28 |

Upgrade (ein echtes N-1 → N-Upgrade über den Control-Plane-Updater) und Backup/Restore (echter Export → Teardown → Neuinstallation → Restore) laufen auf derselben nativen `amd64`/`arm64`-Matrix, nicht nur die Neuinstallation.

## Test selbst wiederholen

Nur für eine leere, verzichtbare Docker-Umgebung gedacht:

```shell
git clone https://github.com/foxly-it/rootguard.git
cd rootguard
./scripts/verify-clean-install.sh
```

Voraussetzungen: Docker Engine oder Docker Desktop mit Compose v2, `curl`, `dig` und `jq`. Mit `ROOTGUARD_TEST_ARCH=amd64` oder `arm64` lässt sich eine bestimmte Docker-Architektur erzwingen.

Der Prüfer verweigert den Start, falls bereits ein RootGuard-Container, ein benanntes Daten-Volume oder ein DNS-Netzwerk existiert. Auf einem sauberen Host erzeugt er ausschließlich RootGuard-Ressourcen und entfernt diese nach dem Test wieder - niemals über einen globalen Docker-Prune-Befehl, und ohne fremde Container, Images, Netzwerke oder Volumes zu berühren.

## Unterstützte Plattformen

- **Linux** (jede Distribution) mit Docker Engine + Compose v2, `amd64` oder `arm64` - das primäre, vollständig geprüfte Ziel.
- **Docker Desktop auf macOS**, Apple Silicon (`arm64`) - geprüft; Intel-Macs verwenden dieselben veröffentlichten `amd64`-Manifeste, werden aber nicht separat getestet.
- **Docker Desktop auf Windows** (WSL2-Backend) - noch nicht in der Prüfmatrix. Erwartet zu funktionieren (dasselbe Compose-Modell, dieselben veröffentlichten Images), aber ungeprüft - bis zu einem nativen Testlauf als Best-Effort zu behandeln.

Bare-Metal-/systemd-Installationen und Multi-Node-Deployments sind bewusst nicht unterstützt: RootGuard ist eine Single-Node-Docker-Appliance.

## Docker-Engine-Version

RootGuards Core- und Updater-Container rufen an drei Stellen `docker cp` auf (Backup-Export, Backup-Restore und Update-Rollback). Drei `docker cp`-Schwachstellen (CVE-2026-41567, CVE-2026-41568, CVE-2026-42306) wurden upstream in Docker Engine 29.5.1 behoben. **Verwende Docker Engine 29.5.1 oder neuer**, oder bestätige, dass deine Distribution alle drei Fixes zurückportiert hat - manche Distributionen patchen Sicherheitslücken, ohne die angezeigte Versionsnummer zu erhöhen.

Die Vorprüfung der Installation zeigt dies als nicht blockierenden Hinweis (`docker_engine_cp_cve`), sobald sie die Docker-Engine-Version eindeutig als älter als 29.5.1 lesen kann - sie warnt statt zu blockieren, weil zurückportierte Distributions-Pakete (z. B. Debian/Ubuntus `docker.io`) häufig genug sind, dass ein reiner Versionsvergleich echte Fehlalarme erzeugen würde.

## Mindestanforderungen

Der Installer erzwingt kein hartes Minimum, aber die Performance-Baseline zeigt RootGuards vollen Stack (Core, WebApp, Updater, AdGuard Home, Unbound) komfortabel laufend auf einem begrenzten Host mit 1 vCPU / 2 GB RAM bei leichter bis moderater Abfragelast (Steady-State-Speicherverbrauch deutlich unter 100 MB über alle fünf Container). Ein 1-vCPU-Host wird als reales Ziel nicht empfohlen - er wird unter Dauerlast selbst zur Durchsatzgrenze, nicht RootGuard. Praktische Empfehlung: **2 vCPU, 2 GB RAM** als komfortable Grundausstattung für ein reales Heimnetzwerk.

## Bekannte Grenzen

- **Nur Single-Node.** Keine Hochverfügbarkeit, kein Failover zwischen Instanzen.
- **Upgrade-Kompatibilität ist nur N-1 → N.** Versionen beim Upgrade zu überspringen ist weder getestet noch unterstützt - Releases der Reihe nach durchlaufen.
- **Restore ist eine Clean-Replacement-Operation, kein In-Place-Merge.** Es wird auf ein frisches, noch nie installiertes Ziel bereitgestellt - ein Restore gegen eine bereits installierte Instanz erfordert vorher einen vollständigen Abbau.

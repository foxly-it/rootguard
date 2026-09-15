# Die Bereiche der Oberfläche

| Bereich | Beschreibung |
| --- | --- |
| **Dashboard** | Installation, DNS-Endpunkt, Dienste, DNSSEC und Verbindungskette sowie echte CPU-/RAM- und aggregierte AdGuard-Kennzahlen auf einen Blick. |
| **Einrichtung** | Netzwerk-Preflight und persistenter AIO-Bereitstellungsfortschritt. |
| **Stack & Updates** | Dienstzustand, sichere Updates und Verlauf bleiben sofort sichtbar; technische Release-Details sind einklappbar. |
| **Backups** | Wiederherstellungspunkte, verschlüsselter Export und saubere Vollwiederherstellung. |
| **Logs & Diagnose** | Zentrale, durchsuchbare Protokolle für alle verwalteten Dienste mit lokaler Filterung. |
| **Unbound** | Profile, geführte Einstellungen, Zonen, Live-Konfiguration und Direktiven - siehe [eigene Seite](webgui/unbound.md). |
| **AdGuard Home** | RootGuard-Status und geschützter Zugriff auf die native AdGuard-Oberfläche. |

![Dashboard einer vollständig eingerichteten RootGuard-Installation](assets/screenshots/dashboard.png)

## Live-Kennzahlen

Das Dashboard aggregiert ausschließlich die CPU- und RAM-Nutzung der fünf fest freigegebenen RootGuard-Container. Zusätzlich liest Core über die intern authentifizierte AdGuard-Home-API die Gesamtzahl der DNS-Anfragen und blockierten Anfragen aus und berechnet daraus die Filterquote. Query-Namen und Clientdaten verlassen AdGuard dabei nicht. Die Anzeige aktualisiert sich automatisch alle zehn Sekunden.

## AdGuard Home: geschützte native Verwaltung

AdGuard Home erhält keinen öffentlichen Admin-Port. RootGuard führt den offiziellen Einrichtungsablauf intern durch, erzeugt Zugangsdaten im geschützten Core-Volume und stellt die Oberfläche ausschließlich unter `/adguard-ui/` über die angemeldete WebApp bereit.

![AdGuard-Home-Seite mit Live-Filterprüfung](assets/screenshots/adguard.png)

!!! note "Upstream-Schutz"
    RootGuard erwartet Unbound fest unter `172.29.53.2:5335` und prüft diese Kette nach Start und Update.

**Blockseite:** Statt AdGuards generischer Standardantwort zeigt eine eigene, RootGuard-gebrandete Seite an, warum eine Domain blockiert wurde, inklusive Domain, Zeitpunkt und Client-IP. Die Einrichtung aktiviert AdGuards `blocking_mode: custom_ip` automatisch und ist per Schalter im Setup deaktivierbar. Da eine DNS-seitige Blockadresse kein gültiges TLS-Zertifikat für beliebige blockierte Domains vorweisen kann, greift die Seite nur bei HTTP - HTTPS-Anfragen zeigen stattdessen die Zertifikatswarnung des Browsers.

## Updates & Rollback

![Stack & Updates mit digest-verifizierten Images](assets/screenshots/stack-updates.png)

**AdGuard Home, Unbound und Blockseite:** Core zieht nur konfigurierte Ziel-Images, vergleicht echte Image-IDs, sichert persistente Dienstpfade und ersetzt genau einen Dienst. DNS, DNSSEC und Upstream werden danach geprüft. Bei Fehlern folgen Daten- und Image-Rollback.

**Core und WebApp:** Der separate Updater bleibt während des Austauschs aktiv. Er aktualisiert Core und WebApp nur gemeinsam, prüft beide Image-IDs und Health-Endpunkte und pinnt bei einem Fehler beide vorherigen Images.

!!! warning "Update ab 0.1.0-beta.14 oder älter"
    Core-Versionen bis einschließlich 0.1.0-beta.14 erkennen 1.0.0 nicht automatisch als verfügbares Update. Bestehende Installationen auf diesem Stand müssen einmalig manuell auf das neue Release zeigen:

    ```shell
    docker exec rootguard-core wget -qO- \
      --header="Authorization: Bearer $ROOTGUARD_API_TOKEN" \
      --header="Content-Type: application/json" \
      --post-data='{"target_images":{"core":"ghcr.io/foxly-it/rootguard-core:1.0.0","webapp":"ghcr.io/foxly-it/rootguard-webapp:1.0.0"}}' \
      http://updater:8082/api/control-plane/update
    ```

**Verlauf und begrenztes Aufräumen:** Das Stack Center bewahrt bis zu 50 Update-, Fehler-, Rollback- und Cleanup-Ereignisse dauerhaft auf. Interne AdGuard- und Unbound-Update-Backups sind pro Dienst auf konfigurierbare 2-50 Wiederherstellungspunkte begrenzt (Standard 5). Für externe Sicherungen erstellt die Backup-Seite ein passwortverschlüsseltes age-v1-Vollbackup mit versioniertem Manifest und SHA-256-Prüfsummen.

!!! note "Unveränderliche Release-Images"
    Der öffentliche Release-Stack kombiniert lesbare Versions-Tags mit geprüften Multi-Arch-Digests. Docker startet damit exakt das veröffentlichte Artefakt, selbst wenn ein Tag später verändert würde:

    ```ini title=".env"
    ROOTGUARD_CORE_UPDATE_IMAGE=ghcr.io/foxly-it/rootguard-core:1.0.0@sha256:2fd43b4943cb5b26daf71e79eb9d6bec8e42d2b1e07d40ab7ed425b065113c41
    ROOTGUARD_WEBAPP_UPDATE_IMAGE=ghcr.io/foxly-it/rootguard-webapp:1.0.0@sha256:fda7b2b50a56a8a95851b59642a6543e76f8bb4dc4104df1b249916f3da2c45e
    ```

## Backups

Wiederherstellungspunkte für AdGuard Home und Unbound entstehen automatisch vor jedem Update. Für externe Sicherungen erzeugt diese Seite ein passwortverschlüsseltes age-v1-Vollbackup; derselbe Import-Assistent unterscheidet zwischen einem Vollbackup, einem exportierten Unbound-Konfigurationspaket und einer vorhandenen `unbound.conf`.

![Backups-Seite mit den drei Importwegen](assets/screenshots/backups.png)

## Logs & Diagnose

Zentrale, durchsuchbare Protokolle für alle fünf verwalteten Dienste, mit lokaler Filterung, optionaler automatischer Aktualisierung und einem redigierten Diagnosebericht zum Download.

![Logs & Diagnose mit Dienstauswahl](assets/screenshots/logs.png)

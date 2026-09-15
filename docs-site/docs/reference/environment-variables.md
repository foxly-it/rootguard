# Wichtige Umgebungsvariablen

## Geheimnisse und Zugang

| Variable | Beschreibung |
| --- | --- |
| `ROOTGUARD_API_TOKEN` | Internes Geheimnis zwischen WebApp, Core und Updater. |
| `ROOTGUARD_ADMIN_USER` | Benutzername für die WebGUI; Standard `admin`. |
| `ROOTGUARD_ADMIN_PASSWORD` | Erforderliches starkes Passwort für die WebGUI. |
| `ROOTGUARD_RECOVERY_TOKEN` | Ab 0.1.0-alpha.2 erforderlicher separater Zufallsschlüssel für lokale Passwort-Recovery; niemals mit Passwort oder API-Token identisch setzen. |
| `ROOTGUARD_WEB_BIND` | Host-Bindung der WebGUI, z. B. `0.0.0.0`. |
| `ROOTGUARD_WEB_PORT` | Host-Port der WebGUI, z. B. `8080`. |

## Ersteinrichtungs-Images

| Variable | Beschreibung |
| --- | --- |
| `ROOTGUARD_CORE_IMAGE` | Image für den Core-Container. |
| `ROOTGUARD_WEBAPP_IMAGE` | Image für den WebApp-Container. |
| `ROOTGUARD_UPDATER_IMAGE` | Image für den Updater-Container. |
| `ROOTGUARD_UNBOUND_IMAGE` | Freigegebenes Unbound-Image für die Ersteinrichtung. |
| `ROOTGUARD_ADGUARD_IMAGE` | Freigegebenes AdGuard-Image für die Ersteinrichtung. |
| `ROOTGUARD_ADGUARD_BETA_IMAGE` | Alternatives AdGuard-Beta-Image, ebenfalls freigegeben. |
| `ROOTGUARD_BLOCKPAGE_IMAGE` | Image für den Blockpage-Container. |
| `ROOTGUARD_ATTESTATION_PROXY_IMAGE` | Image für den Attestation-Proxy-Container. |

## Update-Ziele

Diese Werte legen fest, worauf ein Update aktualisiert - sie werden nie vom Browser gesetzt, nur serverseitig in `.env`.

| Variable | Beschreibung |
| --- | --- |
| `ROOTGUARD_CORE_UPDATE_IMAGE` | Serverseitiges Ziel für Core-Updates. |
| `ROOTGUARD_WEBAPP_UPDATE_IMAGE` | Serverseitiges Ziel für WebApp-Updates. |
| `ROOTGUARD_UNBOUND_UPDATE_IMAGE` | Serverseitiges Ziel für Unbound-Updates. |
| `ROOTGUARD_ADGUARD_UPDATE_IMAGE` | Serverseitiges Ziel für AdGuard-Updates. |
| `ROOTGUARD_BLOCKPAGE_UPDATE_IMAGE` | Serverseitiges Ziel für Blockpage-Updates. |
| `ROOTGUARD_UPDATER_UPDATE_IMAGE` | Serverseitiges Ziel für Updater-Selbst-Updates. |
| `ROOTGUARD_ATTESTATION_PROXY_UPDATE_IMAGE` | Serverseitiges Ziel für Attestation-Proxy-Updates. |

!!! example "Vollständiges Beispiel (Release-Stack)"
    ```ini title=".env"
    ROOTGUARD_CORE_IMAGE=ghcr.io/foxly-it/rootguard-core:1.0.0@sha256:2fd43b4943cb5b26daf71e79eb9d6bec8e42d2b1e07d40ab7ed425b065113c41
    ROOTGUARD_WEBAPP_IMAGE=ghcr.io/foxly-it/rootguard-webapp:1.0.0@sha256:fda7b2b50a56a8a95851b59642a6543e76f8bb4dc4104df1b249916f3da2c45e
    ROOTGUARD_UPDATER_IMAGE=ghcr.io/foxly-it/rootguard-updater:1.0.0@sha256:99c9edd94d9f6b5fcb1f52ea291a37076a3803c02d8e4b4c8352e7c85908d9d4
    ROOTGUARD_UNBOUND_IMAGE=ghcr.io/foxly-it/rootguard-unbound:1.0.0@sha256:962d247fc4c125f37e3cac4f7f4c8902814f2d204fb64ceb7ccd1fbde65dfb58
    ROOTGUARD_ADGUARD_IMAGE=adguard/adguardhome:v0.107.79@sha256:aba9e3bf0613be3ba3755e1fc311b126e2c24bec25e18b6483894a88283074f0
    ROOTGUARD_BLOCKPAGE_IMAGE=ghcr.io/foxly-it/rootguard-blockpage:1.0.0@sha256:26ff3e2687797c9bf02ca7969d4288f3bbf6517ff134dc30f484854ba367a097
    ROOTGUARD_ATTESTATION_PROXY_IMAGE=ghcr.io/foxly-it/rootguard-attestation-proxy:1.0.0@sha256:1577ec69a34e0cd465b8ad05cdb0c64c75f684b95058858b1783a65efa2fc400

    ROOTGUARD_API_TOKEN=replace-with-a-long-random-token
    ROOTGUARD_ADMIN_USER=admin
    ROOTGUARD_ADMIN_PASSWORD=replace-with-a-strong-password
    ROOTGUARD_RECOVERY_TOKEN=replace-with-a-separate-long-random-recovery-key

    ROOTGUARD_WEB_BIND=0.0.0.0
    ROOTGUARD_WEB_PORT=8080
    ```

    Die vollständige, aktuelle Vorlage mit allen Update-Zielen liegt in [`.env.release.example`](https://github.com/foxly-it/rootguard/blob/main/.env.release.example){: target="_blank" rel="noopener" } im Repository-Root.

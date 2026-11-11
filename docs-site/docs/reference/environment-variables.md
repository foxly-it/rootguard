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
| `ROOTGUARD_DOCKER_PROXY_IMAGE` | Image für den Docker-Proxy-Container. Die WebGUI aktualisiert ihn nicht, siehe [Betrieb](../operations.md#docker-proxy). |

## Docker-Anbindung

| Variable | Beschreibung |
| --- | --- |
| `ROOTGUARD_DOCKER_SOCKET_PATH` | Pfad zum Docker-Socket auf dem Host, Standard `/var/run/docker.sock`. Nur bei Rootless Docker nötig (typisch `/run/user/<uid>/docker.sock`); `install.sh` erkennt das und setzt den Wert selbst. |

!!! danger "`ROOTGUARD_SKIP_ATTESTATION` nur für Tests"
    `compose.release.yaml` reicht diese Variable an Core durch, damit Testumgebungen lokal gebaute Images ohne Signatur verwenden können. Auf `true` gesetzt schaltet sie **jede** Signatur- und Herkunftsprüfung ab, bei Einrichtung, Updates und Self-Update. In einer echten Installation nie setzen.

## Update-Ziele

Diese Werte werden nie vom Browser gesetzt, nur serverseitig in `.env`. Für die RootGuard-Komponenten fragt Core zuerst das neueste veröffentlichte Release bei GitHub ab und nimmt diese Werte nur, wenn das nicht klappt. Bei AdGuard Home gilt die Regel in der Tabelle.

| Variable | Beschreibung |
| --- | --- |
| `ROOTGUARD_CORE_UPDATE_IMAGE` | Serverseitiges Ziel für Core-Updates. |
| `ROOTGUARD_WEBAPP_UPDATE_IMAGE` | Serverseitiges Ziel für WebApp-Updates. |
| `ROOTGUARD_UNBOUND_UPDATE_IMAGE` | Serverseitiges Ziel für Unbound-Updates. |
| `ROOTGUARD_ADGUARD_UPDATE_IMAGE` | Serverseitiges Ziel für AdGuard-Updates. Ein älterer Pin desselben Images weicht der Version, die der laufende Core mitbringt. |
| `ROOTGUARD_BLOCKPAGE_UPDATE_IMAGE` | Serverseitiges Ziel für Blockpage-Updates. |
| `ROOTGUARD_UPDATER_UPDATE_IMAGE` | Serverseitiges Ziel für Updater-Selbst-Updates. |
| `ROOTGUARD_ATTESTATION_PROXY_UPDATE_IMAGE` | Serverseitiges Ziel für Attestation-Proxy-Updates. |

!!! example "Die Werte, die du selbst setzt"
    ```ini title=".env"
    ROOTGUARD_API_TOKEN=replace-with-a-long-random-token
    ROOTGUARD_ADMIN_USER=admin
    ROOTGUARD_ADMIN_PASSWORD=replace-with-a-strong-password
    ROOTGUARD_RECOVERY_TOKEN=replace-with-a-separate-long-random-recovery-key

    ROOTGUARD_WEB_BIND=0.0.0.0
    ROOTGUARD_WEB_PORT=8080
    ```

    Die Image-Zeilen nicht von Hand schreiben: Die Vorlage [`.env.release.example`](https://github.com/foxly-it/rootguard/blob/main/.env.release.example){: target="_blank" rel="noopener" } des jeweiligen Releases enthält alle Images und Update-Ziele, fest auf geprüfte Digests gepinnt. Die [manuelle Installation](../getting-started.md#installation) lädt sie passend zum aktuellen Release.

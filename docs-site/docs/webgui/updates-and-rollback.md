# Zwei getrennte Sicherheitswege

## AdGuard Home, Unbound und Blockseite

Core zieht nur konfigurierte Ziel-Images, vergleicht echte Image-IDs, sichert persistente Dienstpfade und ersetzt genau einen Dienst. DNS, DNSSEC und Upstream werden danach geprüft. Bei Fehlern folgen Daten- und Image-Rollback.

## Core und WebApp

Der separate Updater bleibt während des Austauschs aktiv. Er aktualisiert Core und WebApp nur gemeinsam, prüft beide Image-IDs und Health-Endpunkte und pinnt bei einem Fehler beide vorherigen Images. Browser-Anfragen können weder Image-Namen noch Compose-Argumente festlegen.

!!! warning "Update ab 0.1.0-beta.14 oder älter"
    Core-Versionen bis einschließlich 0.1.0-beta.14 erkennen 1.0.0 nicht automatisch als verfügbares Update - die Versionserkennung war fest auf das alte Schema `0.1.0-(alpha|beta).N` begrenzt. Bestehende Installationen auf diesem Stand müssen einmalig manuell auf das neue Release zeigen; danach erkennt der aktualisierte Core jedes folgende Release wieder normal automatisch.

Einmalig auf dem Host ausführen, auf dem RootGuard läuft (`ROOTGUARD_API_TOKEN` ist der Wert aus deiner `.env`):

```shell
docker exec rootguard-core wget -qO- \
  --header="Authorization: Bearer $ROOTGUARD_API_TOKEN" \
  --header="Content-Type: application/json" \
  --post-data='{"target_images":{"core":"ghcr.io/foxly-it/rootguard-core:1.0.0","webapp":"ghcr.io/foxly-it/rootguard-webapp:1.0.0"}}' \
  http://updater:8082/api/control-plane/update
```

!!! success "Realer Rollback-Test"
    Die Updater-CI ersetzt echte Core- und WebApp-Testcontainer gemeinsam. Ein absichtlich fehlerhafter WebApp-Kandidat liefert HTTP 503; der Test weist danach beide vorherigen laufenden Image-IDs und den gespeicherten Rollback-Verlauf nach.

## Verlauf und begrenztes Aufräumen

Das Stack Center bewahrt bis zu 50 Update-, Fehler-, Rollback- und Cleanup-Ereignisse dauerhaft auf. Erst ein ausdrücklicher Klick lädt die manuelle Bestandsaufnahme; sie zeigt nur ältere, selbst protokollierte Image-IDs und ungenutzte Volumes mit dem Label `io.rootguard.cleanup=true` samt geschätztem freigebbarem Speicher. Vor dem bestätigten Löschen prüft RootGuard die Auswahl erneut; globale Docker-Prune-Befehle werden nie verwendet.

Interne AdGuard- und Unbound-Update-Backups werden getrennt geschützt. Auf der eigenen Backup-Seite sind Anzahl und Speichernutzung sichtbar; pro Dienst bleiben konfigurierbare 2 bis 50 Wiederherstellungspunkte erhalten (Standard 5). RootGuard löscht nur eindeutig per Pfad und Manifest erkannte eigene Backups. Unbekannte Daten und Symlinks werden angezeigt, aber niemals entfernt.

Für externe Sicherungen erstellt die Backup-Seite ein passwortverschlüsseltes age-v1-Vollbackup mit versioniertem Manifest und SHA-256-Prüfsummen. Es enthält RootGuard-Konfiguration sowie persistente AdGuard-/Unbound-Daten, aber keine Browser-Sitzungen oder externen `.env`-Geheimnisse. Das Passwort wird nicht gespeichert.

Dasselbe verschlüsselte Archiv kann auf einer sauberen RootGuard-Installation geprüft und wiederhergestellt werden. Archivgrenzen, Manifest, Prüfsummen, Zieladresse, Port und vorhandene Docker-Ressourcen werden vor jeder Änderung erneut geprüft; fehlgeschlagene Versuche räumen ihre neu angelegten Ressourcen auf.

Die zentrale Seite Logs & Diagnose liest ausschließlich die fünf fest freigegebenen Dienste. Core begrenzt jede Ausgabe auf die letzten 30 Minuten, 100 Zeilen und 64 KiB, entfernt Steuerzeichen und redigiert häufige Zugangsdatenmuster; der Browser kann keine freien Container- oder Pfadnamen übergeben.

Bei unveränderlich referenzierten Core- und WebApp-Releases prüft RootGuard außerdem den signierten SLSA-Herkunftsnachweis, die erwartete GitHub-Workflowidentität und die Sigstore-Transparenzdaten. Fehlende oder ungültige Nachweise und vorübergehende Netzfehler werden getrennt angezeigt.

!!! note "Unveränderliche Release-Images"
    Der öffentliche Release-Stack kombiniert lesbare Versions-Tags mit geprüften Multi-Arch-Digests. Docker startet damit exakt das veröffentlichte Artefakt, selbst wenn ein Tag später verändert würde.

```ini title=".env"
ROOTGUARD_CORE_UPDATE_IMAGE=ghcr.io/foxly-it/rootguard-core:1.0.0@sha256:2fd43b4943cb5b26daf71e79eb9d6bec8e42d2b1e07d40ab7ed425b065113c41
ROOTGUARD_WEBAPP_UPDATE_IMAGE=ghcr.io/foxly-it/rootguard-webapp:1.0.0@sha256:fda7b2b50a56a8a95851b59642a6543e76f8bb4dc4104df1b249916f3da2c45e
```

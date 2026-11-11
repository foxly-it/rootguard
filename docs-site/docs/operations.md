# Betrieb

## Nützliche Befehle

```shell
# Status der Control Plane
docker compose -f compose.release.yaml ps

# Logs
docker compose -f compose.release.yaml logs --tail=200 core webapp updater

# Control Plane neu starten
docker compose -f compose.release.yaml restart core webapp updater

# Gestoppten Stack wieder starten
docker compose -f compose.release.yaml start

# DNS testen
dig @192.168.178.10 example.com A  # durch die eigene Host-IP ersetzen
```

Konfigurations- und Installationszustand liegt in benannten Docker-Volumes. Entferne Volumes nicht mit `docker compose down --volumes`, solange du keine bewusste Neuinstallation durchführen willst.

!!! warning "Saubere Neuinstallation"
    Der Setup-Controller erzeugt zusätzliche benannte DNS-Volumes außerhalb der Control-Plane-Compose. Ein normales Stoppen löscht keine Daten. Entferne Container und Volumes nur bewusst nach einem Backup und anhand der versionsgleichen Release-Hinweise.

## Docker-Proxy

Core und Updater erreichen Docker nicht direkt, sondern über `rootguard-docker-proxy`, den einzigen Container mit Zugriff auf den Docker-Socket. Er lässt nur die Docker-API-Aufrufe durch, die RootGuard tatsächlich braucht, und lehnt alles andere ab.

Der Docker-Proxy hat **keinen eigenen Update-Kanal**: Die WebGUI aktualisiert ihn nicht. Braucht ein Release einen neueren Proxy, nennen die Release-Hinweise den einen Befehl dafür. Unter **Stack & Updates** steht er mit Version und Signaturstatus, unter **Logs & Diagnose** stehen die Aufrufe, die er abgelehnt hat. Auf dem Host geht es auch so:

```shell
docker logs --tail=50 rootguard-docker-proxy
```

**Installationen aus 1.0.0 oder 1.0.1** haben noch keinen Docker-Proxy: Core und Updater greifen dort direkt auf den Docker-Socket zu, und das Update in der WebGUI ändert daran nichts, weil es die Compose-Datei nie austauscht. Stack & Updates weist darauf hin. Der geprüfte Weg zum aktuellen Aufbau: auf der [Backup-Seite](webgui.md#backups) ein Vollbackup exportieren und es auf einer frischen Installation wiederherstellen.

## Häufige Probleme

??? question "Port 53 ist bereits belegt"
    Die Vorprüfung erkennt das automatisch - auch bei lokalen Resolvern wie systemd-resolved oder dnsmasq, die nicht als Docker-Container sichtbar sind - und benennt die Ursache in der Fehlermeldung. Stoppe den blockierenden Dienst, oder wähle für Tests in der Einrichtung einen anderen DNS-Port; Routerbetrieb benötigt üblicherweise Port 53.

??? question "Die WebGUI ist erreichbar, aber APIs schlagen fehl"
    Prüfe `docker compose ps` und die Logs von Core und Updater. Nach Ablauf einer Sitzung meldest du dich erneut an. Ein 401 bedeutet eine fehlende Sitzung; ein 502 weist typischerweise auf einen internen Dienst hin.

??? question "Ich habe das Admin-Passwort vergessen"
    Wähle „Passwort vergessen?" und verwende den unabhängigen `ROOTGUARD_RECOVERY_TOKEN` aus deiner lokalen `.env`-Datei. Das neue Passwort muss mindestens zwölf Zeichen haben; danach werden alle bestehenden Sitzungen beendet. Ist kein Recovery-Schlüssel eingerichtet, setze `ROOTGUARD_ADMIN_PASSWORD` lokal neu und erstelle ausschließlich den WebApp-Container kontrolliert neu.

??? question "DNS funktioniert nur auf dem Host"
    Verwende auf Clients die LAN-IP des Hosts, nicht `localhost`. Prüfe Firewall, TCP/UDP 53 und ob der Router eigene DNS-Vorgaben oder DoH erzwingt.

??? question "Ein Update wurde zurückgerollt"
    Lies die Meldung unter Stack & Updates und prüfe die Helper-/Core-Logs. Ein Rollback ist ein Schutzmechanismus: Die vorherigen Images bleiben aktiv, bis das Ziel-Image oder die Konfiguration korrigiert wurde.

??? question "Eine Meldung enthält „is not on the allowlist“ oder „forbidden“"
    Der [Docker-Proxy](#docker-proxy) hat einen Docker-Aufruf abgelehnt. Das passiert vor jeder Änderung, der laufende Stack bleibt unverändert. Prüfe `docker logs rootguard-docker-proxy` und die Release-Hinweise: Meist braucht das neue Release einen aktualisierten Proxy, oder es ist ein Fehler, den du als Issue melden kannst.

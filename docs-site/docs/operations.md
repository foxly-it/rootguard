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

## Häufige Probleme

??? question "Port 53 ist bereits belegt"
    Die Vorprüfung erkennt das automatisch - auch bei lokalen Resolvern wie systemd-resolved oder dnsmasq, die nicht als Docker-Container sichtbar sind - und benennt die Ursache in der Fehlermeldung. Stoppe den blockierenden Dienst, oder verwende für Tests einen anderen `ROOTGUARD_DNS_PORT`; Routerbetrieb benötigt üblicherweise Port 53.

??? question "Die WebGUI ist erreichbar, aber APIs schlagen fehl"
    Prüfe `docker compose ps` und die Logs von Core und Updater. Nach Ablauf einer Sitzung meldest du dich erneut an. Ein 401 bedeutet eine fehlende Sitzung; ein 502 weist typischerweise auf einen internen Dienst hin.

??? question "Ich habe das Admin-Passwort vergessen"
    Wähle „Passwort vergessen?" und verwende den unabhängigen `ROOTGUARD_RECOVERY_TOKEN` aus deiner lokalen `.env`-Datei. Das neue Passwort muss mindestens zwölf Zeichen haben; danach werden alle bestehenden Sitzungen beendet. Ist kein Recovery-Schlüssel eingerichtet, setze `ROOTGUARD_ADMIN_PASSWORD` lokal neu und erstelle ausschließlich den WebApp-Container kontrolliert neu.

??? question "DNS funktioniert nur auf dem Host"
    Verwende auf Clients die LAN-IP des Hosts, nicht `localhost`. Prüfe Firewall, TCP/UDP 53 und ob der Router eigene DNS-Vorgaben oder DoH erzwingt.

??? question "Ein Update wurde zurückgerollt"
    Lies die Meldung unter Stack & Updates und prüfe die Helper-/Core-Logs. Ein Rollback ist ein Schutzmechanismus: Die vorherigen Images bleiben aktiv, bis das Ziel-Image oder die Konfiguration korrigiert wurde.

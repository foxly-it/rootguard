# Nützliche Befehle

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

# Control Plane starten

!!! info "Automatische Installation"
    Für die schnelle Variante mit automatischer Docker-Erkennung/-Installation, automatisch erzeugten Sicherheitsschlüsseln und einer kurzen Abfrage für Benutzername/Passwort siehe den Ein-Befehl-Schnellstart auf der [Startseite](https://rootguard.foxly.de/#quickstart). Der folgende Weg zeigt jeden Schritt einzeln zum manuellen Nachvollziehen.

```shell
mkdir rootguard && cd rootguard

curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/compose.release.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/.env.release.example

# Zwei unabhängige Sicherheitsschlüssel erzeugen
openssl rand -hex 32
openssl rand -hex 32

# .env ausfüllen, dann RootGuard starten
docker compose -f compose.release.yaml up -d
```

Trage zwei getrennt erzeugte Zufallswerte als `ROOTGUARD_API_TOKEN` und `ROOTGUARD_RECOVERY_TOKEN` sowie ein eigenes starkes `ROOTGUARD_ADMIN_PASSWORD` in `.env` ein. RootGuard lädt versionierte amd64-/arm64-Images aus GHCR; ein Checkout oder lokaler Build der Komponenten ist nicht erforderlich. Die Control Plane startet zuerst, die DNS-Dienste werden anschließend im Setup erzeugt.

```text
http://localhost:8080/login
```

# Häufige Probleme

??? question "Port 53 ist bereits belegt"
    Die Vorprüfung erkennt das automatisch - auch bei lokalen Resolvern wie systemd-resolved oder dnsmasq, die nicht als Docker-Container sichtbar sind - und benennt die Ursache in der Fehlermeldung. Stoppe den blockierenden Dienst, oder verwende für Tests einen anderen `ROOTGUARD_DNS_PORT`; Routerbetrieb benötigt üblicherweise Port 53.

??? question "Die WebGUI ist erreichbar, aber APIs schlagen fehl"
    Prüfe `docker compose ps` und die Logs von Core und Updater. Nach Ablauf einer Sitzung meldest du dich erneut an. Ein 401 bedeutet eine fehlende Sitzung; ein 502 weist typischerweise auf einen internen Dienst hin.

??? question "Ich habe das Admin-Passwort vergessen"
    Wähle ab 0.1.0-alpha.2 „Passwort vergessen?" und verwende den unabhängigen `ROOTGUARD_RECOVERY_TOKEN` aus deiner lokalen `.env`-Datei. Das neue Passwort muss mindestens zwölf Zeichen haben; danach werden alle bestehenden Sitzungen beendet. Ist kein Recovery-Schlüssel eingerichtet, setze `ROOTGUARD_ADMIN_PASSWORD` lokal neu und erstelle ausschließlich den WebApp-Container kontrolliert neu.

??? question "DNS funktioniert nur auf dem Host"
    Verwende auf Clients die LAN-IP des Hosts, nicht `localhost`. Prüfe Firewall, TCP/UDP 53 und ob der Router eigene DNS-Vorgaben oder DoH erzwingt.

??? question "Ein Update wurde zurückgerollt"
    Lies die Meldung unter Stack & Updates und prüfe die Helper-/Core-Logs. Ein Rollback ist ein Schutzmechanismus: Die vorherigen Images bleiben aktiv, bis das Ziel-Image oder die Konfiguration korrigiert wurde.

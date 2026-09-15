# DNS-Stack geführt bereitstellen

1. **Anmelden** — Verwende `ROOTGUARD_ADMIN_USER` und `ROOTGUARD_ADMIN_PASSWORD`. Die Sitzung bleibt serverseitig geschützt und läuft nach zwölf Stunden ab. Ab 0.1.0-alpha.2 bietet „Passwort vergessen?" zusätzlich eine lokale Wiederherstellung mit einem separaten Recovery-Schlüssel.
2. **Host-Adresse wählen** — Wähle eine bereits vorhandene LAN-IP. `0.0.0.0` bindet alle Host-Adressen, eine konkrete LAN-IP begrenzt die Erreichbarkeit enger.
3. **Vorprüfung ausführen** — RootGuard prüft Adresse, Port, Docker Engine und Compose, bevor Container verändert werden. Belegte DNS-Ports werden zweistufig erkannt: zunächst anhand bereits veröffentlichter Docker-Ports, danach über einen echten Bindungsversuch auf Host-Ebene, der auch Nicht-Docker-Prozesse wie systemd-resolved oder dnsmasq erfasst. Fehler werden mit Ursache, nächstem Schritt sowie einklappbaren technischen Details erklärt.
4. **Bereitstellen** — Unbound wird gestartet und geprüft, danach AdGuard Home intern eingerichtet und ausschließlich mit Unbound als Upstream verbunden.

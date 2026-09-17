# Lokales DNS im Heimnetz

Die meisten Router lösen Namen für Geräte im eigenen Netzwerk nur notdürftig auf - oft nur für per DHCP vergebene Namen, ohne Wildcard-Domains, ohne Reverse-DNS für alle Geräte, und ohne Kontrolle darüber, was bei internen gegenüber öffentlichen Namen passiert. Ein eigener rekursiver Resolver mit lokalen Zonen behebt das grundsätzlich, unabhängig davon, welche Software genau läuft.

## Warum ein eigener Resolver

- **Sprechende Namen statt IP-Adressen.** `nas.home.lan` statt `192.168.178.20` - für Backups, SSH-Configs, Bookmarks.
- **Reverse-DNS (PTR), das tatsächlich stimmt.** Log-Dateien, `dig -x`, viele Admin-Oberflächen zeigen Hostnamen an - ohne funktionierendes PTR bleibt das eine nackte IP.
- **Kontrolle über interne vs. öffentliche Auflösung.** Manche internen Domains sollen niemals das Netzwerk verlassen (Split-DNS); andere sollen bewusst öffentlich auflösbar bleiben, obwohl sie in einem privaten Adressbereich liegen.
- **Ein Ort für Conditional Forwarding.** Firmen-VPN, ein zweites internes Netzwerk, ein separater Nameserver für eine bestimmte Domain - all das lässt sich an einer Stelle bündeln statt auf jedem Client einzeln.

## Die Grundbausteine

**Lokale Zonen (A/AAAA/CNAME).** Ein Resolver, der für eine selbst definierte Zone (z. B. `home.lan`) autoritativ antwortet, statt die Anfrage rekursiv aufzulösen. Jedes Gerät bekommt einen Namen, optional IPv4 und IPv6 parallel.

**Reverse DNS (PTR).** Die Umkehrung: aus `192.168.178.20` wird `nas.home.lan`. Muss zur Vorwärtsauflösung passen, sonst widersprechen sich Log-Einträge und tatsächliche Erreichbarkeit.

**Conditional Forwarding.** Eine bestimmte Domain wird nicht rekursiv aufgelöst, sondern gezielt an einen anderen, dafür zuständigen DNS-Server weitergereicht - typisch für Firmennetze, VPN-Domains, oder eine zweite interne Zone.

**Private-Domain-Policy.** Was passiert, wenn jemand außerhalb des Netzwerks versucht, eine `192.168.x.x`-Adresse aufzulösen? Die sichere Standardantwort ist NXDOMAIN; bewusstes Öffnen ist möglich, sollte aber die Ausnahme sein.

## Geräte automatisch übernehmen statt von Hand pflegen

Jedes Gerät einzeln einzutragen skaliert schlecht. Zwei Wege, die das vermeiden:

- **Import aus dem Router**, wenn er Geräteinformationen über eine Schnittstelle bereitstellt (z. B. FRITZ!Box über TR-064) - Hostnamen und Adressen werden übernommen, nicht neu erfunden.
- **Reverse-DNS-Erkennung** über einen IP-Bereich, wenn Geräte bereits über DHCP-Hostnamen erreichbar sind, aber kein zentrales Geräteverzeichnis existiert.

## Wie RootGuard das umsetzt

RootGuard verwaltet genau diese vier Bausteine über eine geführte Oberfläche auf einem gehärteten Unbound-Resolver: Formular statt Unbound-Syntax, Live-Prüfung gegen den laufenden Resolver vor jeder Aktivierung, Schleifenerkennung bei Conditional Forwarding, Versionierung mit Rollback. Konkrete, durchgerechnete Beispiele für alle vier Bausteine: [Unbound: Anwendungsbeispiele](unbound-examples.md).

## Siehe auch

- [Unbound: Anwendungsbeispiele](unbound-examples.md) - dieselben Konzepte, Schritt für Schritt in RootGuards Oberfläche.
- [Erste Schritte mit RootGuard](../getting-started.md) - Installation und Ersteinrichtung.

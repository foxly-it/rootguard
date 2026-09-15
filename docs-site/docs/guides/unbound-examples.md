# Unbound: Anwendungsbeispiele

Vier durchgerechnete Beispiele für die geführten Unbound-Einstellungen - siehe die [Unbound-Seite](../webgui/unbound.md) für die Konzepte dahinter.

## Lokale DNS-Zonen

Der geführte Assistent unter **Unbound → Lokale Zonen** erzeugt A-, AAAA- und CNAME-Einträge für Geräte in deinem Netzwerk, ohne dass du Unbound-Syntax von Hand schreiben musst.

**Ziel:** Eine Zone `home.lan` mit drei Hosts:

| Host | IPv4 | IPv6 | PTR ableiten |
| --- | --- | --- | --- |
| `nas.home.lan` | `192.168.178.20` | – | ja |
| `printer.home.lan` | `192.168.178.30` | – | ja |
| `ap1.home.lan` | `192.168.178.40` | `fd00::40` | ja |

![Ausgefülltes Formular für die home.lan-Zone](../assets/screenshots/unbound-local-zone.png)

1. Öffne **Unbound → Lokale Zonen** und lege die Zone `home.lan` an.
2. Füge pro Host einen Eintrag mit Name und Adresse(n) hinzu.
3. Aktiviere „PTR-Eintrag ableiten" pro Host. RootGuard leitet automatisch die passende Reverse-Zone ab - vorausgesetzt, die Adresse ist über alle geführten Zonen hinweg eindeutig.
4. Prüfe die Änderungsvorschau, dann aktivieren. RootGuard führt `unbound-checkconf` gegen die effektive Konfiguration aus, bevor irgendetwas aktiv wird.

```shell
dig @192.168.178.10 nas.home.lan A
dig @192.168.178.10 -x 192.168.178.20
```

!!! note "Grenzen der geführten Oberfläche"
    Per-Host-TTL und komplexere CNAME-Ketten sind bewusst nicht Teil dieser geführten Oberfläche ([Issue #131](https://github.com/foxly-it/rootguard/issues/131)) - dafür der Expertenmodus.

## Conditional Forwarding

Conditional Forwarding leitet Anfragen für bestimmte interne Zonen an eigene DNS-Server weiter, statt sie rekursiv über die öffentliche Root-Hierarchie aufzulösen.

**Ziel:** Die Zone `corp.example` an zwei geordnete Server weiterleiten: `10.0.0.53` (primär), `10.0.0.54` (sekundär).

1. Öffne **Unbound → Conditional Forwarding** und lege die Zone `corp.example` an, mit beiden Zielen in dieser Reihenfolge.
2. RootGuard prüft beide Ziele **live gegen den laufenden Unbound-Container**: Erst wenn die Zone von einem Ziel mit `NOERROR` und einem gültigen SOA-Eintrag beantwortet wird, lässt sich die Zone aktivieren.
3. Optionale Opt-ins, jeweils einzeln sichtbar: rekursiver Fallback, unsignierte Antworten erlauben (`domain-insecure`), private RFC1918-Antworten erlauben.
4. Aktivieren durchläuft dieselbe Vorschau-, Checkconf-, Versions- und Rollback-Kette wie jede andere Unbound-Änderung.

```shell
dig @192.168.178.10 host.corp.example A
```

!!! warning "Schleifenerkennung"
    RootGuard verweigert Ziele, die auf RootGuard selbst, AdGuard Home oder eine andere bereits konfigurierte Weiterleitungszone zurückzeigen würden.

## FRITZ!Box-Import

Statt jeden Host manuell anzulegen, kann RootGuard bekannte Geräte direkt aus deiner FRITZ!Box übernehmen (TR-064). Zugangsdaten sind nur nötig, wenn deine FRITZ!Box eine Anmeldung verlangt.

1. Öffne **Unbound → Lokale Zonen → Aus FRITZ!Box importieren** und trage die FRITZ!Box-Adresse ein (z. B. `192.168.178.1`).
2. RootGuard fragt die Geräteliste ab und zeigt einen **Entwurf** mit Hostname, erkannten Adressen und Konflikten.
3. Wähle einzeln aus, welche Geräte übernommen werden. Nichts wird automatisch importiert; Hostnamen lassen sich vor der Übernahme anpassen.
4. Übernommene Hosts durchlaufen dieselbe Vorschau-, Checkconf- und Aktivierungskette wie manuell angelegte lokale Zonen.

**Alternative ohne FRITZ!Box:** Reverse-DNS-Erkennung über einen expliziten CIDR-Präfix (z. B. `192.168.178.0/24`) - nur private IPv4/Unicast-IPv6-Präfixe, max. 256 Adressen insgesamt, 16 parallele Abfragen mit gemeinsamem 15-Sekunden-Limit.

!!! note "Datenschutz"
    FRITZ!Box-Zugangsdaten werden ausschließlich serverseitig für die eine Abfrage verwendet - niemals im Browser gespeichert, niemals geloggt.

## Private Domains und Reverse DNS

RootGuard entscheidet für jeden der drei privaten RFC1918-Bereiche (`10/8`, `172.16/12`, `192.168/16`) einzeln, wie Rückwärts-Lookups behandelt werden: **NXDOMAIN** (Standard, sicher) oder **transparente öffentliche Weiterauflösung** (mit sichtbarer Warnung vor der Aktivierung).

**Beispiel:** `192.168.178.0/24` bewusst öffentlich auflösen lassen, `10/8` und `172.16/12` unverändert auf NXDOMAIN lassen:

1. Öffne **Unbound → Private Domains**.
2. Wähle für `192.168/16` explizit „Transparente Weiterauflösung".
3. Bestätige die Warnung und aktiviere - durchläuft dieselbe Vorschau- und Checkconf-Prüfung wie jede andere Unbound-Änderung.

Willst du stattdessen nur, dass `192.168.178.20` als `nas.home.lan` aufgelöst wird, ist das der falsche Ort dafür - siehe [Lokale DNS-Zonen](#lokale-dns-zonen) oben: abgeleitete PTR-Einträge funktionieren unabhängig von dieser bereichsweiten Einstellung.

# Anwendungsbeispiel: Private Domains und Reverse DNS

RootGuard entscheidet für jeden der drei privaten RFC1918-Adressbereiche einzeln, wie Rückwärts-Lookups (Reverse DNS) behandelt werden.

## Die drei Bereiche

| Bereich | Typisches Netz |
| --- | --- |
| `10/8` | Große private Netze, VPNs |
| `172.16/12` | Docker-Standardnetze, manche Router |
| `192.168/16` | Die meisten Heimnetzwerke |

## Zwei Optionen pro Bereich

- **NXDOMAIN (Standard)** - ein Rückwärts-Lookup für eine Adresse aus diesem Bereich, die RootGuard nicht selbst kennt, wird sicher mit „nicht gefunden" beantwortet. Kein interner Adressname verlässt dein Netzwerk in Richtung öffentlicher DNS-Hierarchie.
- **Transparente öffentliche Weiterauflösung** - der Lookup wird stattdessen normal rekursiv aufgelöst. RootGuard zeigt vor der Aktivierung eine **sichtbare Warnung**, dass damit private Rückwärts-Lookups nach außen gelangen können.

## Beispiel: `192.168.178.0/24` bewusst öffentlich auflösen lassen

Falls dein Setup aus einem bestimmten Grund öffentliche Reverse-DNS-Antworten für dein Heimnetz benötigt (z. B. ein Gerät, das öffentliche PTR-Einträge erwartet):

1. Öffne **Unbound → Private Domains**.
2. Wähle für `192.168/16` explizit „Transparente Weiterauflösung" statt der Standardeinstellung NXDOMAIN.
3. Bestätige die eingeblendete Warnung - sie erklärt genau, was sich dadurch ändert.
4. Aktivieren durchläuft dieselbe Vorschau- und Checkconf-Prüfung wie jede andere Unbound-Änderung.

Für `10/8` und `172.16/12` bleibt in diesem Beispiel die sichere NXDOMAIN-Voreinstellung unverändert - die drei Bereiche werden unabhängig voneinander konfiguriert.

## PTR-Einträge aus lokalen Zonen

Wenn du stattdessen einfach willst, dass `192.168.178.20` als `nas.home.lan` aufgelöst wird (ohne Bereich-weite Weiterauflösung), ist das der falsche Ort dafür - siehe stattdessen das [Anwendungsbeispiel „Lokale DNS-Zonen"](local-dns-zones.md): PTR-Einträge, die aus deinen eigenen A-/AAAA-Einträgen abgeleitet werden, funktionieren unabhängig von der hier beschriebenen bereichsweiten NXDOMAIN/Fallback-Einstellung.

!!! note "Warum NXDOMAIN die sichere Voreinstellung ist"
    Ein Rückwärts-Lookup für eine private Adresse, der nach außen gelangt, kann interne Namenskonventionen oder Netzwerkstruktur preisgeben. Neue und migrierte Installationen starten deshalb für alle drei Bereiche mit NXDOMAIN; jede Ausnahme ist eine bewusste, einzeln sichtbare Entscheidung.

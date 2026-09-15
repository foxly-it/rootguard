# Anwendungsbeispiel: Lokale DNS-Zonen

Der geführte Assistent unter **Unbound → Lokale Zonen** erzeugt A-, AAAA- und CNAME-Einträge für Geräte in deinem Netzwerk, ohne dass du Unbound-Syntax von Hand schreiben musst.

## Ziel

Eine Zone `home.lan` mit drei Hosts:

| Host | IPv4 | IPv6 | PTR ableiten |
| --- | --- | --- | --- |
| `nas.home.lan` | `192.168.178.20` | – | ja |
| `printer.home.lan` | `192.168.178.30` | – | ja |
| `ap1.home.lan` | `192.168.178.40` | `fd00::40` | ja |

## Schritte

1. Öffne **Unbound → Lokale Zonen** und lege die Zone `home.lan` an.
2. Füge pro Host einen Eintrag mit Name und Adresse(n) hinzu. Für `nas` und `printer` genügt IPv4; für `ap1` trägst du zusätzlich die IPv6-Adresse ein.
3. Aktiviere „PTR-Eintrag ableiten“ pro Host. RootGuard leitet automatisch die passende Reverse-Zone (`178.168.192.in-addr.arpa`) ab - vorausgesetzt, die Adresse ist über alle geführten Zonen hinweg eindeutig.
4. Prüfe die Änderungsvorschau: RootGuard zeigt die generierten `local-zone`/`local-data`/`local-data-ptr`-Direktiven, bevor irgendetwas aktiv wird.
5. Aktiviere. RootGuard führt `unbound-checkconf` gegen die effektive Konfiguration aus und schreibt erst danach um; bei einem Fehler bleibt die vorherige Version aktiv.

## Nach der Aktivierung prüfen

```shell
dig @192.168.178.10 nas.home.lan A
dig @192.168.178.10 -x 192.168.178.20
```

Die erste Abfrage muss `192.168.178.20` liefern, die zweite (Reverse-Lookup) `nas.home.lan.`

!!! note "Grenzen der geführten Oberfläche"
    CNAME-Einträge lassen sich anlegen, aber **per-Host-TTL und komplexere CNAME-Ketten sind bewusst nicht Teil dieser geführten Oberfläche** ([Issue #131](https://github.com/foxly-it/rootguard/issues/131)). Für diese seltenen Fälle nutze den Expertenmodus - er greift auf dieselbe Vorschau-, Checkconf- und Rollback-Kette zu.

!!! tip "Client-Zugriffsregeln"
    Wer im Netzwerk überhaupt Anfragen stellen darf, wird bewusst weiterhin in AdGuard Home verwaltet, nicht hier - Unbound-Zonen betreffen nur, wie Namen aufgelöst werden, nicht, wer fragen darf.

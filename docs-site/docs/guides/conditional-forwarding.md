# Anwendungsbeispiel: Conditional Forwarding

Conditional Forwarding leitet Anfragen für bestimmte interne Zonen an eigene DNS-Server weiter, statt sie rekursiv über die öffentliche Root-Hierarchie aufzulösen - etwa für eine Firmen- oder VPN-Zone, die nur intern bekannt ist.

## Ziel

Die Zone `corp.example` an zwei geordnete Server weiterleiten:

1. `10.0.0.53` (primär)
2. `10.0.0.54` (sekundär, falls der erste nicht antwortet)

## Schritte

1. Öffne **Unbound → Conditional Forwarding** und lege eine neue Zone `corp.example` an.
2. Trage `10.0.0.53` als ersten und `10.0.0.54` als zweiten Zielserver ein. Die Reihenfolge bleibt erhalten - RootGuard fragt sie in genau dieser Reihenfolge ab.
3. RootGuard prüft beide Ziele **live gegen den laufenden Unbound-Container**: Erst wenn die konfigurierte Zone von einem Ziel mit `NOERROR` und einem gültigen SOA-Eintrag beantwortet wird, lässt sich die Zone aktivieren. `NXDOMAIN`, `REFUSED`, Timeouts oder leere Erfolgsantworten werden als Diagnose angezeigt, blockieren aber die Aktivierung.
4. Optionale Opt-ins, jeweils einzeln und sichtbar:
      - **Rekursiver Fallback**, falls keiner der Ziele antwortet.
      - **Unsignierte Antworten erlauben** (`domain-insecure`), falls der interne Server keine DNSSEC-signierten Antworten liefert - nötig, weil DNSSEC-Validierung sonst standardmäßig aktiv bleibt.
      - **Private RFC1918-Antworten erlauben**, falls `corp.example` legitim auf private Adressen zeigt - ohne diesen Opt-in bleibt der Rebinding-Schutz für private Adressbereiche aktiv.
5. Aktivieren. Wie bei jeder Unbound-Änderung durchläuft das die gemeinsame Vorschau-, Checkconf-, Versions- und Rollback-Kette.

## Nach der Aktivierung prüfen

```shell
dig @192.168.178.10 host.corp.example A
```

Antwortet `10.0.0.53` nicht innerhalb der konfigurierten Zeit, greift automatisch `10.0.0.54` - ohne dass du etwas manuell umschalten musst.

!!! warning "Schleifenerkennung"
    RootGuard verweigert Ziele, die auf RootGuard selbst, auf AdGuard Home oder auf eine andere bereits konfigurierte Weiterleitungszone zurückzeigen würden - eine Weiterleitungsschleife lässt sich damit gar nicht erst aktivieren.

!!! note "Authentifiziertes DNS-over-TLS"
    Forwarding über DNS-over-TLS mit Zertifikatsprüfung ist aktuell **bewusst noch nicht unterstützt**, bis sich Zertifikatsidentität sicher modellieren lässt. Reine Adress-Ziele wie im Beispiel oben funktionieren unabhängig davon bereits vollständig.

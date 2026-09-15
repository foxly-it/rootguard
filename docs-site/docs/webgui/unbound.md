# Resolver sicher konfigurieren

## Profile und Einstellungen

Balanced, Privacy, Resilience und Performance laden ausschließlich einen Entwurf. QNAME-Minimierung, Prefetch, Serve Expired, Cache-TTLs und Threads werden erklärt und vor der Aktivierung als Änderungsvorschau dargestellt.

## IPv4 und IPv6

IPv4 ist der kompatible Standard. Dual Stack und IPv6-only werden erst freigegeben, wenn der laufende Unbound-Container einen autoritativen Root-Server über IPv6 erreicht. Core wiederholt diese Prüfung bei der Aktivierung. Client-Zugriffsregeln bleiben bewusst in AdGuard Home, da Netzwerkgeräte nicht direkt mit Unbound sprechen.

## Lokale Zonen

Der geführte Assistent erzeugt A-, AAAA- und CNAME-Einträge ohne manuelle Unbound-Syntax. Für eindeutige A-/AAAA-Adressen kann er passende PTR-Einträge ableiten. Er erkennt parallele Änderungen und verwendet dieselbe Checkconf-, Versions- und Rollback-Kette.

!!! example "Anwendungsbeispiel"
    Eine vollständige, durchgerechnete Zone (`home.lan` mit A-, AAAA- und abgeleiteten PTR-Einträgen) findest du unter [Lokale DNS-Zonen](../guides/unbound-examples.md#lokale-dns-zonen).

## Geräte aus der FRITZ!Box importieren

Findet Hosts über die FRITZ!Box (TR-064) oder begrenzte Reverse-DNS-Abfragen in ausgewählten privaten IPv4- oder Unicast-IPv6-Netzen (max. 256 Adressen je Präfix und insgesamt). Zugangsdaten für die FRITZ!Box sind nur nötig, wenn TR-064-Anfragen eine Anmeldung verlangen, werden ausschließlich für diese eine Abfrage verwendet und nie gespeichert. Gefundene Geräte werden vor der Übernahme einzeln ausgewählt und umbenennbar - nichts wird automatisch importiert. Übernommene Hosts durchlaufen dieselbe Vorschau-, Checkconf- und Aktivierungskette wie die geführten lokalen Zonen.

!!! example "Anwendungsbeispiel"
    Schritt-für-Schritt-Anleitung unter [Geräte aus der FRITZ!Box importieren](../guides/unbound-examples.md#fritzbox-import), inklusive der router-unabhängigen Reverse-DNS-Alternative.

## Private Domains und Reverse DNS

Private Domains werden als geprüfte Liste verwaltet. Für `10/8`, `172.16/12` und `192.168/16` wählst du getrennt zwischen sicherem NXDOMAIN und transparenter öffentlicher Weiterauflösung. NXDOMAIN ist die Voreinstellung; RootGuard warnt sichtbar, bevor ein privater Rückwärts-Lookup nach außen gelangen kann.

!!! example "Anwendungsbeispiel"
    Ein durchgerechnetes Beispiel findest du unter [Private Domains und Reverse DNS](../guides/unbound-examples.md#private-domains-und-reverse-dns).

## Conditional Forwarding

Mehrere interne Zonen lassen sich an geordnete IPv4- und IPv6-DNS-Server weiterleiten. RootGuard normalisiert Zonennamen und Adressen, blockiert Schleifen und gibt die Aktivierung erst frei, wenn jedes Ziel die konfigurierte Zone mit NOERROR und einem SOA-Eintrag bestätigt. Rekursiver Fallback, unsignierte private Zonen und private RFC1918-Antworten besitzen getrennte, klar erklärte Opt-ins; DNSSEC und Rebinding-Schutz bleiben sonst aktiv.

!!! example "Anwendungsbeispiel"
    Ein durchgerechnetes Weiterleitungsbeispiel findest du unter [Conditional Forwarding](../guides/unbound-examples.md#conditional-forwarding).

## Expertenmodus und Live-Konfiguration

Der Editor besitzt nur `90-rootguard-custom.conf` und blockiert gefährliche Includes, Listener, Remote Control sowie DNSSEC-Umgehungen. Die Live-Ansicht liest die tatsächlich aktiven Dateien read-only aus dem Container.

## Konfiguration exportieren und übertragen

Die vollständige Resolver-Konfiguration (geführte Einstellungen und die Expertenkonfiguration zusammen) lässt sich als eine Datei herunterladen und auf einer anderen RootGuard-Instanz wieder hochladen - für Backups oder eine Migration. Der Import durchläuft dieselbe Vorschau- und Checkconf-Prüfung wie jede andere Aktivierung.

## Bestehende unbound.conf übernehmen

Eine vorhandene, handgeschriebene `unbound.conf` lässt sich einfügen oder hochladen. RootGuard klassifiziert jede Direktive gegen das eigene Ownership-Modell (geführt, feste Basis, Experte oder blockiert) und bietet nicht abgebildete Direktiven wie `forward-zone` oder `local-zone` vollständig für den Expertenmodus an, statt sie stillschweigend zu verwerfen - dasselbe Ergebnis wie ein manuelles Einfügen in den Expertenmodus.

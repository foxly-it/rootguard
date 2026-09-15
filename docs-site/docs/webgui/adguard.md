# Geschützte native Verwaltung

AdGuard Home erhält keinen öffentlichen Admin-Port. RootGuard führt den offiziellen Einrichtungsablauf intern durch, erzeugt Zugangsdaten im geschützten Core-Volume und stellt die Oberfläche ausschließlich unter `/adguard-ui/` über die angemeldete WebApp bereit.

!!! note "Upstream-Schutz"
    RootGuard erwartet Unbound fest unter `172.29.53.2:5335` und prüft diese Kette nach Start und Update.

## Blockseite

Statt AdGuards generischer Standardantwort zeigt eine eigene, RootGuard-gebrandete Seite an, warum eine Domain blockiert wurde, inklusive Domain, Zeitpunkt und Client-IP sowie einer verständlichen Erklärung der häufigsten Blockierungsgründe. Die Einrichtung aktiviert AdGuards `blocking_mode: custom_ip` automatisch und ist per Schalter im Setup deaktivierbar; empfohlen bleibt sie aktiv. Da eine DNS-seitige Blockadresse kein gültiges TLS-Zertifikat für beliebige blockierte Domains vorweisen kann, greift die Seite nur bei HTTP - HTTPS-Anfragen zeigen stattdessen die Zertifikatswarnung des Browsers, ein dokumentiertes Verhalten von AdGuard Home selbst.

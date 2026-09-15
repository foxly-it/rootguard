# Vertrauensgrenzen

- WebApp ohne Docker-Socket und ohne frei ausführbare Host-Befehle
- HttpOnly-, SameSite-Strict-Sitzung und Same-Origin-Prüfung für Schreibzugriffe
- Session-Inventar mit gezielter Sitzungs-Beendigung, Rate-Limiting und Audit-Log für Anmeldung und Passwort-Recovery
- Core und Updater nur in internen Docker-Netzen und mit Bearer-Token
- Unbound read-only, non-root, ohne zusätzliche Capabilities
- AdGuard-Administration ohne öffentlichen Port
- Dokumentiertes Threat Model sowie automatisierte Dependency-, Container-, Secret- und Static-Analysis-Scans in der CI

!!! note "HTTPS"
    RootGuard terminiert bewusst kein eigenes TLS - das übernimmt ein etablierter Reverse Proxy davor. [HTTPS über einen Reverse-Proxy](../guides/https-reverse-proxy.md) deckt die zwei Voraussetzungen (Host-Header-Weiterleitung, `X-Forwarded-Proto`) und Beispielkonfigurationen für Caddy, Zoraxy, Nginx Proxy Manager und HAProxy ab.

[Vollständiges Bedrohungsmodell öffnen](../security/threat-model.md)

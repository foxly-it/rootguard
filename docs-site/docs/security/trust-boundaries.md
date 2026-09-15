# Vertrauensgrenzen

- WebApp ohne Docker-Socket und ohne frei ausführbare Host-Befehle
- HttpOnly-, SameSite-Strict-Sitzung und Same-Origin-Prüfung für Schreibzugriffe
- Session-Inventar mit gezielter Sitzungs-Beendigung, Rate-Limiting und Audit-Log für Anmeldung und Passwort-Recovery
- Core und Updater nur in internen Docker-Netzen und mit Bearer-Token
- Unbound read-only, non-root, ohne zusätzliche Capabilities
- AdGuard-Administration ohne öffentlichen Port
- Dokumentiertes Threat Model sowie automatisierte Dependency-, Container-, Secret- und Static-Analysis-Scans in der CI

!!! note "HTTPS"
    RootGuard terminiert bewusst kein eigenes TLS - das übernimmt ein etablierter Reverse Proxy davor. Die Dokumentation deckt die zwei Voraussetzungen (Host-Header-Weiterleitung, `X-Forwarded-Proto`) und Beispielkonfigurationen für Caddy, Zoraxy, Nginx Proxy Manager und HAProxy ab.

    [HTTPS-Anleitung öffnen ↗](https://github.com/foxly-it/rootguard/blob/main/docs/https-reverse-proxy.md){: target="_blank" rel="noopener" }

[Vollständiges Threat Model öffnen ↗](https://github.com/foxly-it/rootguard/blob/main/docs/threat-model.md){: target="_blank" rel="noopener" }

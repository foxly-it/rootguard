# HTTPS über einen Reverse-Proxy

RootGuard terminiert selbst kein TLS und wird das auch nicht tun - das wäre
in weiten Teilen eine Neuimplementierung dessen, was etablierte
Reverse-Proxies wie Caddy, Zoraxy, Nginx Proxy Manager oder HAProxy bereits
robust, aktiv gepflegt und mit automatischer Zertifikatserneuerung anbieten.
Diese Seite beschreibt, was RootGuard von einem vorgeschalteten Proxy
erwartet, und liefert je ein lauffähiges Beispiel für die vier genannten
Optionen.

Betroffen ist ausschließlich die WebGUI (Standard-Port `8080`,
`ROOTGUARD_WEB_PORT`). Der DNS-Port (`53`, TCP+UDP) ist kein HTTP-Dienst und
läuft unabhängig vom hier beschriebenen Proxy-Setup weiter.

## Voraussetzungen

Diese zwei Punkte sind keine Empfehlungen, sondern Voraussetzungen - ohne
sie funktioniert die Anmeldung nicht oder das Session-Cookie wird nicht als
sicher markiert:

1. **Host-Header unverändert weiterreichen.** RootGuards
   Same-Origin-Schutz für schreibende Anfragen
   (`rootguard-webapp/backend/internal/httpapi/origin.go`) vergleicht den
   `Origin`-/`Referer`-Header des Browsers gegen den beim Backend
   ankommenden `Host`-Header. Reicht der Proxy statt des öffentlichen
   Hostnamens (z.B. `rootguard.example.com`) einen internen Namen weiter
   (z.B. `localhost:8080` oder den Docker-Servicenamen), schlägt jede
   Anmeldung und jede schreibende Aktion mit
   `403 Cross-origin administration request rejected` fehl. Das ist die mit
   Abstand häufigste Fehlerursache bei einem neu eingerichteten Proxy - siehe
   Troubleshooting unten.
2. **`X-Forwarded-Proto: https` setzen.** RootGuards Session-Cookie erhält
   das `Secure`-Flag nur, wenn die Anfrage entweder direkt über TLS ankommt
   oder `X-Forwarded-Proto` auf `https` steht
   (`rootguard-webapp/backend/internal/httpapi/auth.go`,
   `requestIsHTTPS`). Ohne diesen Header hält RootGuard die Verbindung für
   Klartext-HTTP und setzt das Cookie entsprechend ohne `Secure` - die
   Anmeldung funktioniert dann zwar noch, aber ohne den Schutz, den ein
   TLS-Reverse-Proxy eigentlich bieten soll.

Alle vier folgenden Beispiele erfüllen beide Punkte bereits. Bei einer
eigenen, abweichenden Konfiguration immer explizit prüfen, dass genau diese
zwei Header korrekt ankommen.

## Caddy

Automatische Zertifikatsbeschaffung/-erneuerung ohne weitere Konfiguration;
Caddy setzt `X-Forwarded-*` und reicht den Host standardmäßig korrekt durch.

```caddyfile
rootguard.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Läuft RootGuard selbst in Docker und Caddy auf demselben Host außerhalb des
RootGuard-Compose-Projekts, `127.0.0.1:8080` durch die tatsächlich
erreichbare Adresse ersetzen (z.B. den im Compose-Netzwerk veröffentlichten
Port, oder bei einem gemeinsamen Docker-Netzwerk den Servicenamen samt
Innenport `8080`).

## Zoraxy

In der Zoraxy-Oberfläche unter *HTTP Proxy* → *Add Proxy Rule*:

- **Matching Domain:** `rootguard.example.com`
- **Target:** `http://127.0.0.1:8080` (oder die tatsächliche
  RootGuard-Adresse im jeweiligen Netzwerk)
- **TLS/SSL:** aktivieren, Zertifikat automatisch beziehen lassen
- Unter den erweiterten Optionen sicherstellen, dass **„Host Header
  Override“ deaktiviert bleibt** (Standard) - Zoraxy reicht den
  ursprünglichen Host-Header damit unverändert durch, wie unter
  „Voraussetzungen“ gefordert.

Der `X-Forwarded-Proto`-Header wird von Zoraxy bei aktivem TLS-Rewrite
automatisch gesetzt.

## Nginx Proxy Manager

Unter *Proxy Hosts* → *Add Proxy Host*:

- **Domain Names:** `rootguard.example.com`
- **Forward Hostname/IP:** die RootGuard-WebApp-Adresse
- **Forward Port:** `8080`
- Reiter **SSL**: Zertifikat anfordern (Let's Encrypt) oder ein eigenes
  hinterlegen, **Force SSL** aktivieren.

Nginx Proxy Manager erzeugt seine Host-Blöcke standardmäßig mit
`proxy_set_header Host $host;` und `proxy_set_header X-Forwarded-Proto
$scheme;` - beide Voraussetzungen sind damit ohne manuellen Eingriff erfüllt.
Wer die generierte Konfiguration über *Advanced* → *Custom Nginx
Configuration* erweitert, darf diese beiden Zeilen nicht entfernen oder
überschreiben.

## HAProxy

```haproxy
frontend rootguard_https
    bind *:443 ssl crt /etc/haproxy/certs/rootguard.example.com.pem
    http-request set-header X-Forwarded-Proto https
    default_backend rootguard_webapp

backend rootguard_webapp
    server webapp 127.0.0.1:8080 check
```

HAProxy reicht den `Host`-Header in dieser Grundkonfiguration bereits
unverändert durch (kein `http-request set-header Host ...` vorhanden); das
explizite `X-Forwarded-Proto` ist nötig, da HAProxy das im Gegensatz zu
Caddy/Zoraxy/Nginx Proxy Manager nicht automatisch setzt. Zertifikatsbezug
und -erneuerung sind bei HAProxy selbst nicht eingebaut - typischerweise
über einen vorgelagerten `certbot`/ACME-Client oder ein separates
Cert-Management gelöst, das die PEM-Datei unter dem hier referenzierten Pfad
bereitstellt und HAProxy nach Erneuerung neu lädt.

## Troubleshooting

**„Cross-origin administration request rejected“ bei jedem Login/jeder
Aktion:** Der Proxy reicht nicht den öffentlichen Hostnamen als
`Host`-Header weiter. Prüfen, was tatsächlich beim Backend ankommt (z.B.
`docker compose logs webapp` während eines Login-Versuchs, oder ein
kurzzeitiger `tcpdump`/Debug-Header). Bei einer eigenen Nginx-basierten
Konfiguration `proxy_set_header Host $host;` ergänzen; bei Caddy/Zoraxy/NPM
sollte das bereits der Standardfall sein - siehe die jeweiligen Hinweise
oben.

**Login funktioniert, aber die Session geht bei jedem Neuladen der Seite
verloren:** `X-Forwarded-Proto` kommt nicht als `https` an, das
Session-Cookie wird ohne `Secure` gesetzt und vom Browser bei einer
gemischten HTTP/HTTPS-Situation unter Umständen verworfen. Header explizit
setzen wie im HAProxy-Beispiel oben.

## Siehe auch

- `docs/architecture.md` - Session-/Cookie-Verhalten im Detail.
- `docs/threat-model.md` - Browser-/Session-Vertrauensgrenze.
- [ROADMAP.md](../ROADMAP.md), Abschnitt 0.5 - Scope-Entscheidung: dokumentierter
  Reverse-Proxy-Betrieb statt eigener TLS-Implementierung.

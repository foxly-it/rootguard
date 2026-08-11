# Bedrohungsmodell

Stand: 2026-08-08. Ergänzt `docs/architecture.md` um eine explizite
Betrachtung, wem RootGuard wie weit vertraut, was ein kompromittierter
Akteur jeweils erreichen kann, welche Gegenmaßnahmen bereits greifen und
welche Restrisiken bewusst offen oder nicht Ziel dieses Projekts sind.

## Geltungsbereich

Betrachtet werden die sechs in [ROADMAP.md](../ROADMAP.md) 0.5 benannten
Akteure/Grenzen: Docker-Socket-Inhaber, Browser, interne Netzwerke,
Update-Supply-Chain, Backups und das AdGuard-Gateway. Außerhalb des
Geltungsbereichs: Kompromittierung des Hosts selbst (physischer Zugriff,
Kernel-Exploits, kompromittierte Basis-Images von Debian/Alpine/Docker) -
RootGuard vertraut dem Host-Betriebssystem und der Docker-Engine, auf denen
es läuft, so wie jede containerisierte Anwendung.

## Akteure und Vertrauensgrenzen

### 1. Docker-Socket-Inhaber (Core, Updater)

**Zugriff:** `rootguard-core` und `rootguard-updater` haben `/var/run/docker.sock`
gemountet. Wer diesen Socket kontrolliert, kontrolliert effektiv den Host -
ein neuer privilegierter Container mit beliebigem Bind-Mount ist daraus
trivial erreichbar.

**Wenn kompromittiert:** Vollständige Host-Kompromittierung. Das ist die mit
Abstand größte Einzel-Vertrauensgrenze im System.

**Bestehende Gegenmaßnahmen:**
- Beide Images laufen aktuell als `root` (siehe die Begründung direkt im
  jeweiligen `Dockerfile`) - kein zusätzlicher Privilege-Escalation-Schritt
  nötig, aber auch keine Reduktion der Angriffsfläche innerhalb des
  Containers selbst.
- Core spricht nur mit einem fest definierten, kontrollierten Satz von
  Images, Volumes, Netzwerken und Befehlen; Browser-Anfragen können weder
  Image-Namen noch Compose-Argumente oder Container frei wählen (siehe
  `docs/architecture.md`, Abschnitte „Kontrollierte Container-Updates" und
  „AIO-Bootstrap"). Ein kompromittierter Browser oder eine kompromittierte
  WebApp kann Core also nicht direkt zu beliebigen Docker-Befehlen bewegen -
  nur zu den engen, typisierten Operationen, die die Core-API tatsächlich
  anbietet.
- Der Updater kennt ausschließlich `core` und `webapp` als Austauschziele,
  zieht nur konfigurierte Ziel-Images und bleibt vom Host aus nicht
  erreichbar (`docs/architecture.md`, „Kontrollierte Container-Updates").
- Manuelle Docker-Bereinigung akzeptiert keine Ressourcennamen aus dem Browser:
  Core leitet Kandidaten ausschließlich aus seiner erfolgreichen Update-Historie
  oder dem festen Volume-Label `io.rootguard.cleanup=true` ab, prüft die Nutzung
  vor Vorschau und Ausführung erneut und ruft keine globalen Prune-Befehle auf.
- Digest-gepinnte Core-/WebApp-Releases durchlaufen vor der Aktivierung eine
  Cosign-/SLSA-Provenienzprüfung gegen den erwarteten GitHub-Workflow-
  Unterzeichner (siehe Abschnitt 4).

**Bekannte Restrisiken / offen:**
- Ein Bug oder eine Schwachstelle *innerhalb* von Core oder Updater, der
  einem Angreifer erlaubt, eigene Docker-API-Aufrufe statt der
  vorgesehenen engen Operationen abzusetzen, würde direkt zur
  Host-Kompromittierung führen - es gibt keine zweite Verteidigungslinie
  zwischen „Code-Fehler in Core" und „voller Docker-Zugriff".
- Konkreter geplanter Härtungsschritt: ein dediziertes
  `docker-socket-proxy`-Sidecar, das den Socket selbst hält und nur eine
  eng allowlistete Teilmenge der Docker-API durchlässt, während Core und
  Updater selbst unprivilegiert laufen. Noch nicht umgesetzt - siehe die
  Kommentare in `rootguard-core/Dockerfile` und
  `rootguard-updater/Dockerfile`.

### 2. Browser / authentifizierter Nutzer

**Zugriff:** HttpOnly-/SameSite=Strict-Session gegen die WebApp; darüber
alle geführten und Experten-Funktionen der Oberfläche.

**Wenn kompromittiert (gestohlene Session, XSS, o.ä.):** Voller
Administrator-Zugriff auf alle RootGuard-Funktionen - Konfigurationsänderung,
Updates, AdGuard-Verwaltung.

**Bestehende Gegenmaßnahmen:**
- PBKDF2-SHA256 mit 600.000 Iterationen für das Admin-Passwort, konstante
  Vergleichszeit, HttpOnly-/SameSite=Strict-Cookies mit `Secure` sobald HTTPS
  aktiv ist (`rootguard-webapp/backend/internal/httpapi/auth.go`).
- Mutierende Anfragen müssen vom gleichen Origin stammen (Same-Origin-Write-
  Check) - klassisches CSRF über ein fremdes Origin greift damit nicht.
- Der Experteneditor für Unbound ist auf eine einzelne Datei
  (`90-rootguard-custom.conf`) begrenzt; Includes, Listener, Remote Control,
  Containerpfade und Trust-Anker bleiben gesperrt (`docs/architecture.md`,
  „Unbound-Expertenkonfiguration") - selbst ein vollständig kompromittierter
  Browser-Zugang kann darüber nicht auf Host-Dateien oder beliebige
  Unbound-Direktiven zugreifen.
- Der separate `ROOTGUARD_RECOVERY_TOKEN` für Passwort-Reset gewährt für
  sich genommen weder Session noch Zugriff auf Core oder AdGuard.

**Bestehende Gegenmaßnahmen (ergänzt):**
- Session-Inventar mit gezielter Revocation: `GET /api/auth/sessions` /
  `DELETE /api/auth/sessions/{id}`, erreichbar über "Aktive Sitzungen" im
  Kontomenü - eine gestohlene Session muss nicht mehr bis zum TTL-Ablauf
  gültig bleiben.
- Rate-Limiting auf Login und Passwort-Recovery: 5 Fehlversuche pro
  5-Minuten-Fenster sperren weitere Versuche - auch ein danach korrektes
  Passwort wird während einer aktiven Sperre abgelehnt, damit eine Sperre
  nicht durch schlichtes Weiterprobieren umgangen werden kann.
- Ein begrenztes, persistiertes Audit-Log (`GET /api/auth/audit`,
  max. 500 Einträge) zeichnet Login-Erfolg/-Fehlschlag, Rate-Limiting,
  Logout, Passwort-Recovery und Session-Revocation auf - sichtbar im
  selben "Aktive Sitzungen"-Panel.

**Bekannte Restrisiken / offen:**
- Rate-Limits und Audit-Events decken bislang nur die Authentifizierung ab,
  nicht destruktive Aktionen an anderer Stelle der Anwendung (Unbound-
  Aktivierung, Service-Updates/-Rollbacks, AdGuard-Bootstrap und
  Vergleichbares) - siehe ROADMAP.md 0.5.

### 3. Interne Netzwerke (control, edge, DNS-Netz)

**Zugriff:** Drei Docker-Netzwerke trennen Zuständigkeiten: `edge`
(WebApp-Host-Port), `control` (WebApp↔Core↔Updater), und das interne
DNS-Netz (AdGuard↔Unbound). Nur WebApp und der DNS-Port erhalten Host-Ports;
AdGuards native Administration bleibt ausschließlich intern erreichbar.

**Wenn kompromittiert (z.B. ein anderer Container im selben
Docker-Host/-Netzwerk):** Zugriff auf interne, eigentlich nicht öffentlich
gedachte Schnittstellen (AdGuard-Admin-API, Core-Bearer-Token-API).

**Bestehende Gegenmaßnahmen:**
- Netzsegmentierung wie oben; Core erreicht Unbound nur über eine fest
  definierte interne Adresse (`172.29.53.2:5335`), kein öffentlicher
  Fallback.
- Core-API ist bearer-token-geschützt (`ROOTGUARD_API_TOKEN`), nicht nur
  netzwerk-isoliert.

**Bekannte Restrisiken / offen:**
- Wer bereits *im selben Docker-Host* beliebige Container starten kann, kann
  sich in der Regel auch in `control`/das interne DNS-Netz hängen (ohne
  zusätzliche Docker-Netzwerkrichtlinien/Firewalling auf Host-Ebene) - die
  Netzsegmentierung schützt vor externen Angreifern und vor anderen,
  nicht-privilegierten Workloads auf demselben Host, nicht vor einem
  Angreifer, der bereits Docker-Zugriff auf demselben Host hat. Das deckt
  sich mit Akteur 1: Wer den Docker-Socket kontrolliert, kontrolliert auch
  die Netzwerke.

### 4. Update-Supply-Chain

**Zugriff:** GHCR-Images für alle fünf Komponenten; der Updater-Helper zieht
und aktiviert neue Core-/WebApp-/AdGuard-/Unbound-Images.

**Wenn kompromittiert (z.B. gestohlene GHCR-Publish-Credentials, kompromittierter
CI-Runner):** Ein bösartiges Image könnte als legitimes RootGuard-Release
verteilt und von bestehenden Installationen automatisch übernommen werden -
letztlich gleichbedeutend mit Akteur 1 (Host-Kompromittierung), nur über den
Update-Pfad statt direkt.

**Bestehende Gegenmaßnahmen:**
- Digest-gepinnte Core-/WebApp-Releases werden vor der Aktivierung per
  Cosign gegen die signierte SLSA-Provenienz geprüft: erwarteter
  GitHub-Repository- und Workflow-Unterzeichner, erwarteter
  GitHub-Actions-OIDC-Aussteller, Prüfung der Sigstore-Transparenzdaten
  (`docs/architecture.md`, „AIO-Bootstrap"). Der eingebettete
  Cosign-Verifier selbst ist per Digest gepinnt - keine bewegliche
  Abhängigkeit an dieser Stelle.
- Lokale Builds, veränderliche Tags (`:latest` o.ä.) und Fremdimages
  erhalten explizit keine RootGuard-Vertrauensfreigabe.
- Ein Update-Fehlschlag (Healthcheck nach Austausch) pinnt automatisch die
  vorherige Image-ID zurück und prüft erneut (`docs/architecture.md`,
  „Kontrollierte Container-Updates").
- CI-seitig: `trivy` prüft alle fünf Komponenten-Images/-Dockerfiles auf
  bekannte Schwachstellen und Fehlkonfigurationen, `govulncheck` und
  `staticcheck` laufen gegen jedes Go-Modul, `gitleaks` gegen die gesamte
  Git-Historie (`.github/workflows/ci-security.yml`) - reduziert das
  Risiko, dass eine bekannte Schwachstelle oder ein versehentlich
  committetes Secret unbemerkt in ein veröffentlichtes Release gelangt.

**Bekannte Restrisiken / offen:**
- AdGuard Home und Unbound (Basis-Images von Drittanbietern bzw. eigener
  Reproducible Build) durchlaufen aktuell keine Cosign-Provenienzprüfung wie
  Core/WebApp - Vertrauen basiert hier auf Digest-Pinning und der
  Upstream-Signatur/dem eigenen SHA-256-verifizierten Reproducible-Build,
  nicht auf einer RootGuard-eigenen Signaturkette.
- Kein SBOM/keine Provenance für jedes Release (ROADMAP.md 0.6) - erschwert
  aktuell eine nachträgliche forensische Analyse eines betroffenen Releases.
- Kein Image-Signing im eigentlichen Sinn über Cosign hinaus für alle fünf
  Komponenten einheitlich (ROADMAP.md 0.6).

### 5. Backups

**Zugriff:** Persistente RootGuard-Volumes (Konfigurationshistorie,
Sessions, AdGuard-Zugangsdaten, Installationszustand) und deren Sicherungen
vor jedem Update/Austausch.

**Wenn kompromittiert (Zugriff auf ein Backup/Volume-Snapshot):** Offenlegung
aller darin enthaltenen Zugangsdaten und Konfiguration - AdGuard-Admin-
Zugangsdaten liegen mit Besitzerrechten im persistenten Volume
(`docs/architecture.md`, „Laufzeitarchitektur"), Sessions liegen serverseitig
im Session-Volume.

**Bestehende Gegenmaßnahmen:**
- Backups/Snapshots entstehen ausschließlich serverseitig vor einem
  kontrollierten Austausch, nicht browserseitig abrufbar.
- Interne Update-Backups sind pro Dienst auf konfigurierbare 2–50
  Wiederherstellungspunkte begrenzt (Standard 5); Speichernutzung und
  nicht erkannte Daten bleiben auf der Backups-Seite sichtbar.
- Automatische Bereinigung akzeptiert nur kanonische Zeitstempel-/Dienstpfade
  mit einem Manifest, das zum erlaubten Dienst und Container passt. Unbekannte
  Daten und Symlinks werden nicht gelöscht.
- Passwort-Hashes sind PBKDF2-SHA256-gesalzen, nicht im Klartext.
- Portable Vollbackups werden vor dem Download interoperabel mit age-v1 und
  einer scrypt-abgeleiteten Passwortidentität authentifiziert verschlüsselt.
  Ein versioniertes Manifest enthält SHA-256-Prüfsummen; Sitzungen und externe
  `.env`-Geheimnisse sind nicht Teil des Exports. Fest verdrahtete Quellen,
  Symlink-Ablehnung und privates, immer entferntes Klartext-Staging begrenzen
  Pfad- und Restdatenrisiken.
- Der geführte Restore prüft vor jeder Änderung Schema, Pflichtdateien,
  erlaubte Pfade/Typen, exaktes Manifest, Größen und SHA-256 sowie harte
  Upload-/Entpack-/Dateigrenzen. Apply validiert erneut, verlangt Bestätigung
  und scheitert geschlossen, wenn Installation oder verwaltete Docker-
  Ressourcen nicht sauber sind. Klartext- und Rollback-Staging werden immer
  entfernt; partielle neue Docker-Ressourcen werden nach Fehlern abgebaut.

**Bekannte Restrisiken / offen:**
- Das Exportpasswort wird nicht gespeichert und muss für jede Vorschau und
  Wiederherstellung erneut eingegeben werden. Über reines HTTP ist das fertige
  Archiv verschlüsselt, das Passwort auf dem Weg vom Browser zur lokalen
  WebApp jedoch nicht transportgeschützt; die Oberfläche warnt sichtbar und
  `docs/https-reverse-proxy.md` beschreibt den unterstützten HTTPS-Betrieb.
- Live-Daten können sich während einzelner Dateikopien verändern. Updates sind
  zwar ausgeschlossen, ein transaktionsartiger Dienst-Snapshot und seine
  Restore-Verifikation bleiben jedoch eigene offene 0.4-Punkte.

### 6. AdGuard-Gateway

**Zugriff:** RootGuard proxied die native AdGuard-Home-Oberfläche unter dem
festen Pfad `/adguard-ui/`; Core setzt dabei die internen
AdGuard-Zugangsdaten ein.

**Wenn kompromittiert (Schwachstelle in AdGuard Home selbst oder im
Proxy-Pfad):** Zugriff auf AdGuards Filterregeln, DNS-Anfrage-Logs (soweit
aktiviert) und die AdGuard-Konfiguration - nicht jedoch auf Core, Unbound
oder den Docker-Socket, da AdGuard selbst keinen dieser Zugriffe besitzt.

**Bestehende Gegenmaßnahmen:**
- AdGuards native Administration ist ausschließlich intern erreichbar, nie
  über einen Host-Port (`docs/architecture.md`, „Laufzeitarchitektur").
- Nur der feste, authentifizierte Proxy-Pfad existiert; frei wählbare Ziele
  und ein öffentlicher Administrationsport sind ausgeschlossen
  (`docs/architecture.md`, „AdGuard-Ersteinrichtung").
- Der Reverse-Proxy migrierte von `Director` auf `Rewrite`
  (`rootguard-core/internal/adguard/proxy.go`,
  `rootguard-webapp/backend/internal/coreclient/client.go`): client-seitig
  gesetzte `X-Forwarded-*`-Header werden vor dem Weiterreichen verworfen und
  serverseitig neu gesetzt statt unverändert durchgereicht - schließt einen
  Header-Spoofing-Pfad, der zuvor offen war.
- Mutierende Anfragen über den Proxy-Pfad müssen ebenfalls vom gleichen
  Origin stammen.

**Bekannte Restrisiken / offen:**
- AdGuard Home selbst liegt außerhalb der RootGuard-Codebasis - eine
  Schwachstelle in AdGuard Home wirkt sich über diesen Pfad direkt auf
  RootGuard-Installationen aus. Mitigiert nur durch Digest-Pinning und
  zeitnahes Nachziehen von AdGuard-Releases, nicht durch eine
  RootGuard-eigene zusätzliche Schutzschicht.

## Bewusst nicht Ziel (Non-Goals)

- **Böswilliger Administrator:** Wer legitime Admin-Zugangsdaten besitzt,
  ist per Definition vertrauenswürdig - RootGuard schützt nicht vor einem
  autorisierten, aber böswillig handelnden Betreiber.
- **Physischer Host-Zugriff:** Wer physischen oder Hypervisor-Zugriff auf
  den Host hat, kann jede containerisierte Anwendung umgehen.
- **Kompromittierte Basis-Images/Registries selbst** (Docker Hub, GHCR,
  Debian/Alpine-Paketquellen) - RootGuard verifiziert, was es referenziert
  (Digest-Pinning, Cosign wo verfügbar), vertraut aber letztlich denselben
  Wurzeln wie jede andere containerisierte Anwendung.
- **Mehrbenutzer-/Rollenmodell und externe Identity-Provider** - laut
  ROADMAP.md 0.5 „Later", nur bei echtem 1.0-Bedarf.
- **HTTPS/TLS-Terminierung durch RootGuard selbst** - bewusste
  Scope-Entscheidung, siehe ROADMAP.md 0.5: dokumentierter Betrieb hinter
  einem etablierten Reverse-Proxy (Caddy, Zoraxy, Nginx Proxy Manager,
  HAProxy) statt einer eigenen TLS-Implementierung.

## Verweise

- `docs/architecture.md` - detaillierte Beschreibung der hier referenzierten
  Mechanismen.
- `SECURITY.md` - Meldeweg für Sicherheitslücken.
- `ROADMAP.md`, Abschnitt 0.5 - offene Punkte, die direkt aus diesem Modell
  folgen (Docker-Socket-Proxy, Session-Revocation, Rate-Limits,
  Audit-Events, Backup-Verschlüsselung).

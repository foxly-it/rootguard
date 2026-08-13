[English](architecture.md) · **Deutsch**

# Repository-Struktur

Das RootGuard-Hauptrepository koordiniert die Versionen der eigenständigen
Komponenten. Jede Komponente behält ihre eigene Historie, Abhängigkeiten,
Releases und Entwicklungsabläufe.

```text
rootguard/
├── docs/
├── rootguard-core/
├── rootguard-updater/
├── rootguard-webapp/
└── rootguard-unbound/
```

Ein Commit im Hauptrepository verweist auf genau einen Commit je Submodule.
Dadurch lässt sich jederzeit nachvollziehen, welche Kombination der
Komponenten gemeinsam verwendet wurde.

Lokale Sicherungen und veraltete Arbeitskopien sind bewusst nicht Bestandteil
des Hauptrepositories.

## Laufzeitarchitektur

```text
Browser
  │ Login + HttpOnly-Session
  ▼
RootGuard WebApp
  │ internes Bearer-Token
  ▼
RootGuard Core ────── Docker API ─── AdGuard Home + Unbound
  │
  │ enge, interne Update-Aufträge
  ▼
Updater-Helper ────── Docker API ─── Core + WebApp
  │
  └── persistenter Status und atomarer Image-Pin
Persistente RootGuard-Daten
```

Nur Webapp und DNS erhalten Host-Ports. AdGuard Home ist ausschließlich in den
internen Netzen für Core erreichbar; seine zufällig erzeugten Zugangsdaten
liegen mit Besitzerrechten im persistenten RootGuard-Volume. Core und der
separate, intern erreichbare Updater-Helper besitzen Docker-Zugriff für
unterschiedliche, fest freigegebene Aufgaben. Die Webapp kennt weder den Socket
noch Host-Systembefehle.

## Anmeldung und lokales Passwort-Recovery

Die WebApp verwaltet HttpOnly-/SameSite-Strict-Sitzungen serverseitig im
geschützten Session-Volume. Ein separater `ROOTGUARD_RECOVERY_TOKEN` ermöglicht
auf der Login-Seite ausschließlich das Setzen eines neuen Admin-Passworts. Er
gewährt weder eine Sitzung noch Zugriff auf Core oder AdGuard und muss
unabhängig von Admin-Passwort und internem API-Token erzeugt werden.

Nach einem erfolgreichen Reset liegt nur ein gesalzener
PBKDF2-SHA256-Verifier mit 600.000 Iterationen im Session-Volume. Alle
bestehenden Sitzungen werden ungültig. Ohne konfigurierten Recovery-Schlüssel
bleibt der lokale Betreiberpfad über `.env` und eine kontrollierte Neuerstellung
des WebApp-Containers; es gibt bewusst keinen E-Mail- oder Cloud-Recovery-Dienst.

## AIO-Bootstrap und Stack-Lebenszyklus

Die öffentliche `compose.yaml` startet die dauerhafte Control Plane aus WebApp,
Core und dem internen Updater-Helper. Die DNS Data Plane wird erst nach einer
authentifizierten Einrichtung erzeugt:

```text
Bootstrap Compose
  → WebApp + Core + Updater-Helper
  → IP-/Port-Preflight
  → fest definierte DNS-Compose-Spezifikation
  → Image Pull
  → Unbound + Healthcheck
  → AdGuard Home + geschützter Bootstrap
```

Core speichert Konfiguration, Einzelschritte und Fehler atomar unter
`/var/lib/rootguard/installation`. Die WebApp sendet nur typisierte
Netzwerkangaben und den erlaubten AdGuard-Release-Kanal `stable` oder `beta`.
Fehlende Kanalangaben älterer Installationen werden als `stable` behandelt;
andere Werte weist Core zurück. Image-Namen, Containerprivilegien, Volumes, Netzwerke und
Kommandos stammen aus der kontrollierten Core-Spezifikation und sind nicht
über die Browser-API frei wählbar.

Der Controller wird nach der Installation mit dem privaten DNS-Netz verbunden.
Beim Neustart oder Austausch des Core-Containers stellt er diese Verbindung
anhand des persistenten Installationszustands wieder her. AdGuard veröffentlicht
weiterhin ausschließlich TCP/UDP 53; seine native Administration bleibt privat.
Der Bootstrap wartet begrenzt auf die tatsächliche AdGuard-Installer-API; ein
laufender Container allein gilt noch nicht als betriebsbereit.

Für unveränderlich per Digest referenzierte Core- und WebApp-Releases prüft
Core zusätzlich die signierte SLSA-Provenienz. Der eingebettete, selbst per
Digest gepinnte Cosign-Verifier erzwingt den erwarteten GitHub-Repository- und
Workflow-Unterzeichner sowie den GitHub-Actions-OIDC-Aussteller und prüft die
Sigstore-Transparenzdaten. Die Ergebnisse werden zehn Minuten gecacht. Ein
fehlender Nachweis, eine kryptografisch ungültige Attestierung und eine
vorübergehend nicht erreichbare Registry werden absichtlich getrennt
ausgewiesen. Lokale Builds, veränderliche Tags und Fremdimages erhalten keine
RootGuard-Vertrauensfreigabe.

## AdGuard-Ersteinrichtung

Die Webapp bietet ausschließlich Status und den expliziten Bootstrap-Vorgang
an. Core nutzt dafür die typisierten AdGuard-Installer- und DNS-Endpunkte,
prüft Unbounds feste Adresse `172.29.53.2:5335` im internen DNS-Netz vor der
Aktivierung und konfiguriert keinen öffentlichen Fallback. Für die native
Oberfläche existiert ausschließlich der feste, authentifizierte Pfad
`/adguard-ui/`: WebApp und Core leiten ihn an das fest konfigurierte interne
Ziel weiter, Core setzt die internen AdGuard-Zugangsdaten ein und mutierende
Browser-Anfragen müssen vom gleichen Origin stammen. Frei wählbare Ziele,
AdGuard-Zugangsdaten und ein öffentlicher Administrationsport bleiben
ausgeschlossen.

## Kontrollierte Container-Updates

Der Stack-Bereich kann ausschließlich die fest in Core freigegebenen
DNS-Dienste AdGuard Home und Unbound prüfen und aktualisieren. Browser-Anfragen
können weder Image-Namen noch Compose-Argumente oder Container festlegen.
Eine Prüfung lädt das serverseitig konfigurierte Ziel-Image und vergleicht
dessen tatsächliche Image-ID mit dem laufenden Container.

Vor einem Austausch kopiert Core die persistenten Dienstpfade in sein
geschütztes Daten-Volume. Anschließend wird genau ein Compose-Dienst ersetzt
und die vollständige dienstspezifische Gesundheitsprüfung ausgeführt. Bei
einem Fehler pinnt Core wieder die vorherige Image-ID, stellt die Sicherung
wieder her und prüft den zurückgerollten Dienst erneut.

Core und WebApp werden ausschließlich als gemeinsame Control Plane durch den
separaten Updater-Helper ersetzt. Der Browser kann weder Images noch Compose-
Dienste oder Argumente angeben. Der Helper kennt nur `core` und `webapp`, zieht
die konfigurierten Ziel-Images, schreibt ein enges Compose-Override und prüft
anschließend beide tatsächlichen Image-IDs sowie Core- und WebApp-Health.
Scheitert eine Prüfung, werden beide vorherigen Image-IDs gemeinsam gepinnt und
erneut geprüft. Der Helper selbst bleibt während des Vorgangs unverändert und
ist nicht vom Host aus erreichbar. WebApp-Sitzungen liegen in einem eigenen
Volume und überstehen den kontrollierten WebApp-Austausch.

Die automatische und manuelle Docker-Bereinigung teilen dieselbe serverseitige
Kandidatenermittlung. Images werden nur aus erfolgreichen, persistenten
RootGuard-Update-Einträgen abgeleitet; das aktive und das vorherige erfolgreiche
Image bleiben je Dienst geschützt. Volumes benötigen zusätzlich das feste Label
`io.rootguard.cleanup=true` und dürfen von keinem Container verwendet werden.
Die manuelle Vorschau zeigt Dockers gerundete `UniqueSize`-/Volume-Schätzung und
übersprungene Ressourcen. Nach der Bestätigung wird die Auswahl vollständig neu
berechnet, statt einer möglicherweise veralteten Browser-Vorschau zu vertrauen.
Globale Prune-Befehle und frei übergebene Ressourcennamen bleiben ausgeschlossen.

## Backup- und Restore-Zuständigkeit für AdGuard Home

AdGuards Zustand liegt in zwei separaten, benannten Docker-Volumes statt in
Core-eigenen Pfaden:

- `rootguard-adguard-config` (`/opt/adguardhome/conf`) - Filterlisten,
  Zulassungslisten, DNS-Umschreibungen, Client-/DHCP-Einstellungen,
  Verschlüsselungskonfiguration; alles, was der Betreiber über die native
  AdGuard-Oberfläche einstellt.
- `rootguard-adguard-work` (`/opt/adguardhome/work`) - Abfrageprotokoll und
  Statistiken.

Beide Pfade sind über `BackupPaths` Teil desselben Sicherungsmechanismus wie
oben beschrieben: Vor jedem AdGuard-Update kopiert Core sie in sein
geschütztes Daten-Volume; schlägt die anschließende Gesundheitsprüfung fehl,
stellt Core beide Pfade automatisch wieder her und rollt auf die vorherige
Image-ID zurück. Diese Sicherung ist ausschließlich ein interner
Update-Schutz - kein vom Betreiber auslösbarer, herunterladbarer oder über
die WebApp direkt wiederherstellbarer Export. RootGuard bewahrt standardmäßig
die fünf neuesten Update-Restore-Punkte je Dienst auf; auf der Backups-Seite kann
der Betreiber diesen Wert zwischen 2 und 50 konfigurieren und Anzahl,
Speichernutzung sowie den neuesten Zeitpunkt pro Dienst einsehen. Bereinigt
werden ausschließlich kanonische, per passendem Manifest eindeutig RootGuard
und dem erlaubten Dienst zugeordnete Verzeichnisse. Unbekannte Dateien,
Verzeichnisse und Symlinks werden separat als nicht verwalteter Speicher
angezeigt und niemals gelöscht.

Für Datensicherung über den reinen Update-Schutz hinaus erzeugt die
Backups-Seite ein portables, passwortverschlüsseltes age-v1-Archiv. Es enthält
RootGuards Unbound-/AdGuard-/Installationszustand, AdGuards live Konfigurations-
und Arbeitsdaten sowie Unbounds Laufzeitstatus. Browser-Sitzungen, externe
`.env`-Geheimnisse, interne Update-Restore-Punkte und temporäre Exportdaten
bleiben ausgeschlossen. Ein versioniertes Manifest erfasst jede reguläre Datei
mit Größe und SHA-256-Prüfsumme. Sämtliche Quellpfade und Container sind fest in
Core definiert; Symlinks werden abgelehnt. Docker-Kopien liegen nur während der
Erstellung in einem privaten `0700`-Verzeichnis im geschützten Core-Volume und
werden auf jedem Erfolgs-/Fehlerpfad entfernt. Der Download wird direkt durch
age/scrypt verschlüsselt und blockiert parallel laufende Daten-Updates. Der
geführte Restore validiert dasselbe Artefakt vollständig, akzeptiert nur eine
saubere Installation ohne kollidierende verwaltete Docker-Ressourcen, befüllt
neu angelegte gestoppte Service-Volumes und startet anschließend die
gesundheitsgeprüfte DNS-Kette. Ein Fehler entfernt die neuen Docker-Ressourcen
und stellt zuvor vorhandene lokale Volume-Inhalte wieder her.

## Unbound-Konfigurationszyklus

```text
WebGUI-Entwurf
  → Vorschau und Feldvergleich
  → unbound-checkconf im Resolver
  → atomare Aktivierung
  → Resolver-Neustart
  → versionierter Snapshot
```

Core speichert maximal 20 validierte Versionen. Ein manueller Rollback wird
wie jede andere Änderung erneut gerendert und validiert. Scheitert ein Neustart
nach der Aktivierung, stellt Core die zuvor gelesenen Konfigurations- und
Einstellungsdateien wieder her und startet Unbound erneut. Die Webapp erhält
keinen generischen Datei- oder Kommandozugriff.

Die Live-Ansicht besitzt ebenfalls keinen generischen Dateizugriff. Core liest
ausschließlich die fest vorgegebenen Unbound-Basis- und Managed-Dateien aus dem
laufenden Resolver und liefert sie read-only an die authentifizierte Webapp.
Dadurch zeigt die Oberfläche den effektiven Containerstand statt lediglich
einen erneut gerenderten Entwurf.

Vordefinierte Betriebsprofile und der RootGuard Advisor arbeiten ausschließlich
auf dem Entwurf. Die Empfehlungen sind deterministisch, verändern keine Dateien
und werden vor ihrer Rückgabe gegen dieselben Wertebereiche wie eine spätere
Aktivierung geprüft. Dadurch umgehen weder Profile noch Vorschläge die
Sicherheitskette.

Conditional Forwarding ist Teil der typisierten Managed Config. Zonen müssen
kanonische FQDNs sein; Zielserver sind kanonische IPv4-/IPv6-Adressen. Root-Zone,
Loopback, Link-Local, Multicast, das interne RootGuard-DNS-Netz, Duplikate und
parallele Experten-`forward-zone`-Blöcke werden abgewiesen. Ein
authentifizierter Reachability-Endpunkt führt ausschließlich DNS-SOA-Proben per
`dig` aus dem laufenden Unbound-Container aus. Nur `NOERROR` zusammen mit einem
SOA-Eintrag für die konfigurierte Zone gilt als erfolgreich; `NXDOMAIN`,
`REFUSED`, Transportfehler und leere erfolgreiche Antworten bleiben
Diagnoseergebnisse, geben die Aktivierung aber nicht frei. Anzahl, Parallelität,
Ausgabe und Laufzeit sind begrenzt; der Endpunkt schreibt keine Konfiguration.
Die spätere Aktivierung durchläuft weiterhin den vollständigen Checkconf-,
Snapshot- und Rollback-Zyklus. DNSSEC bleibt für jede Weiterleitungszone
standardmäßig aktiv.
Nur ein explizites `allow_unsigned` rendert innerhalb des `server`-Blocks eine
zonenspezifische `domain-insecure`-Direktive. Damit funktionieren
vertrauenswürdige unsignierte Split-DNS-Zonen, ohne die globale
DNSSEC-Validierung abzuschalten. Entsprechend rendert nur
`allow_private_addresses` eine zonenspezifische `private-domain`-Direktive.
Unbounds Rebinding-Schutz bleibt damit global aktiv, während ausdrücklich
vertrauenswürdige interne Zonen RFC1918- und andere geschützte private
Adressantworten liefern dürfen.

## Unbound-Expertenkonfiguration

Die unveränderliche `/etc/unbound/unbound.conf` bindet Konfigurationsmodule aus
`/etc/unbound/unbound.d/*.conf` ein. Der Experteneditor besitzt ausschließlich
die Datei `90-rootguard-custom.conf`; `50-rootguard.conf` bleibt der typisierten
WebGUI vorbehalten. Includes, Listener, Remote Control, Containerpfade,
Trust-Anker und geführte Werte sind im freien Editor gesperrt.

Vor einer Aktivierung prüft Core eine kombinierte Kandidatendatei. Danach werden
Settings, Managed Config und Custom Config atomar geschrieben und die effektive
`/etc/unbound/unbound.conf` erneut mit `unbound-checkconf` geprüft. Bei einem
Prüf- oder Neustartfehler stellt Core alle drei vorherigen Dateien wieder her.
Ein History-Eintrag bildet deshalb stets den gemeinsamen Resolverzustand ab.

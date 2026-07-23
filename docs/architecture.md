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
Netzwerkangaben. Image-Namen, Containerprivilegien, Volumes, Netzwerke und
Kommandos stammen aus der kontrollierten Core-Spezifikation und sind nicht
über die Browser-API frei wählbar.

Der Controller wird nach der Installation mit dem privaten DNS-Netz verbunden.
Beim Neustart oder Austausch des Core-Containers stellt er diese Verbindung
anhand des persistenten Installationszustands wieder her. AdGuard veröffentlicht
weiterhin ausschließlich TCP/UDP 53; seine native Administration bleibt privat.
Der Bootstrap wartet begrenzt auf die tatsächliche AdGuard-Installer-API; ein
laufender Container allein gilt noch nicht als betriebsbereit.

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
`dig` aus dem laufenden Unbound-Container aus. Anzahl, Parallelität, Ausgabe und
Laufzeit sind begrenzt; der Endpunkt schreibt keine Konfiguration. Die spätere
Aktivierung durchläuft weiterhin den vollständigen Checkconf-, Snapshot- und
Rollback-Zyklus. DNSSEC bleibt für jede Weiterleitungszone standardmäßig aktiv.
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

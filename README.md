# RootGuard

RootGuard bündelt die gemeinsam entwickelten Komponenten des Projekts in einem
übergeordneten Repository. Die einzelnen Komponenten bleiben eigenständige
Git-Repositories und werden hier als Submodules eingebunden.

## Komponenten

- `rootguard-core` – zentrale RootGuard-Anwendung
- `rootguard-updater` – separater Helper für atomare Core-/WebApp-Updates
- `rootguard-webapp` – Weboberfläche und zugehöriges Backend
- `rootguard-unbound` – Unbound-basierter DNS-Dienst

## Repository klonen

```sh
git clone --recurse-submodules git@github.com:foxly-it/rootguard.git
cd rootguard
```

Bei einem bereits vorhandenen Checkout werden die Submodules so geladen:

```sh
git submodule update --init --recursive
```

## Entwicklungsstack starten

RootGuard benötigt ein internes API-Token, ein Passwort für die Weboberfläche
und einen davon unabhängigen Recovery-Schlüssel. Alle Werte werden
ausschließlich lokal in `.env` gespeichert:

```sh
cp .env.example .env
openssl rand -hex 32
openssl rand -hex 32
```

Die beiden Zufallswerte getrennt als `ROOTGUARD_API_TOKEN` und
`ROOTGUARD_RECOVERY_TOKEN` eintragen, ein starkes
`ROOTGUARD_ADMIN_PASSWORD` setzen und anschließend die RootGuard Control Plane
starten:

```sh
docker compose up --build -d
```

Danach sind erreichbar:

- RootGuard WebApp: `http://localhost:8080`

Über **Passwort vergessen?** auf der Login-Seite lässt sich mit dem
Recovery-Schlüssel ein neues Admin-Passwort setzen. Der Schlüssel ist kein
Ersatzpasswort: RootGuard speichert nach dem Reset nur einen gesalzenen
PBKDF2-SHA256-Verifier im geschützten Session-Volume und beendet alle
bestehenden Sitzungen.

`compose.yaml` startet WebApp, Core und den nur intern erreichbaren
Updater-Helper. Unter **Setup** wird
anschließend eine bereits auf dem Docker-Host vorhandene LAN-IP ausgewählt.
RootGuard prüft die Angaben, lädt die DNS-Images, erstellt Netzwerke und
Volumes und startet Unbound und AdGuard Home in der richtigen Reihenfolge.
Nach dem Healthcheck erzeugt Core die internen AdGuard-Zugangsdaten und trägt
`172.29.53.2:5335` als einzigen Upstream-DNS ein.

Der Installationsfortschritt wird persistent gespeichert. Ein abgebrochener
Vorgang kann erneut gestartet werden, ohne eine frei editierbare Compose-Datei
oder beliebige Docker-Befehle aus der WebApp anzunehmen. Core wird nicht nach
außen veröffentlicht. Die native AdGuard-Oberfläche ist nach der Einrichtung
über den authentifizierten RootGuard-Pfad `/adguard-ui/` erreichbar; ein
eigener AdGuard-Administrationsport wird weiterhin nicht veröffentlicht.

Unter **Stack & Updates** lassen sich die freigegebenen AdGuard- und
Unbound-Images prüfen und einzeln aktualisieren. Vor dem Austausch erstellt
RootGuard eine Sicherung und prüft danach DNS, DNSSEC und den geschützten
Upstream. Schlägt die Prüfung fehl, wird automatisch auf die vorherige
Image-ID zurückgerollt. Core und WebApp werden separat davon als gemeinsame
Control Plane aktualisiert: Ein unabhängiger Helper bleibt während des
Austauschs aktiv, prüft beide neuen Container und setzt bei einem Fehler beide
vorherigen Image-IDs zurück. Image-Namen und Compose-Argumente sind nicht über
den Browser wählbar.

### DNS im Router eintragen

`localhost` bezeichnet ausschließlich den Zugriff auf die WebApp vom
RootGuard-Host selbst. Im Setup wird beispielsweise `192.168.178.10:53`
ausgewählt. Genau diese feste LAN-IP wird anschließend im Router eingetragen –
nicht `127.0.0.1` und nicht die interne Docker-Adresse `172.29.53.2`.

Der RootGuard-Host sollte dafür eine DHCP-Reservierung besitzen, dauerhaft
laufen und eingehenden DNS-Verkehr auf TCP/UDP 53 erlauben. `0.0.0.0` ist als
Bind für alle vorhandenen Host-Adressen möglich; eine konkrete LAN-IP ist
jedoch die engere Einstellung.

### Unbound über die WebGUI konfigurieren

Unter **Unbound Settings** können Resolver- und Cache-Einstellungen bearbeitet
werden. Vor dem Speichern zeigt RootGuard die einzelnen Änderungen und die
generierte Unbound-Konfiguration. Core validiert sie im laufenden Resolver,
aktiviert sie atomar und hält bis zu 20 Versionen bereit. Scheitert der
Neustart, wird automatisch die vorherige Konfiguration wiederhergestellt.

Der Bereich **Geführte Erweiterungen** erlaubt lokale DNS-Zonen mit mehreren
A-, AAAA- und CNAME-Einträgen über ein Formular. Die erzeugten Regeln bleiben
im Experteneditor sichtbar, werden zusammen mit der gesamten effektiven
Konfiguration durch `unbound-checkconf` geprüft und erst nach einer expliziten
Vorschau aktiviert.

Unter **Conditional Forwarding** können mehrere kanonische DNS-Zonen an
geordnete IPv4- und IPv6-Zielserver weitergeleitet werden. RootGuard blockiert
Schleifen zur eigenen DNS-Kette, prüft jedes Ziel aus dem Unbound-Container und
akzeptiert es nur, wenn die konfigurierte Zone mit `NOERROR` und einem
SOA-Eintrag bestätigt wird. Die Oberfläche erklärt außerdem den optionalen
rekursiven Fallback. DNSSEC bleibt pro Zone standardmäßig aktiv; für
vertrauenswürdige interne Server ohne signierte Antworten kann eine
klar gekennzeichnete, zonenspezifische Ausnahme eingeschaltet werden. Der
Rebinding-Schutz bleibt ebenfalls Standard; private RFC1918-Antworten werden nur
nach einer zweiten, auf diese Zone begrenzten Freigabe akzeptiert. Der Entwurf
wird anschließend über denselben Vorschau-, Versions- und Rollback-Pfad
aktiviert.

Die WebGUI bietet Deutsch und Englisch über den Sprachumschalter im Header.
Die Auswahl folgt beim ersten Aufruf der Browsersprache und wird anschließend
lokal gespeichert. Weitere Übersetzungen können als Sprachkatalog über die
öffentliche `registerLocale`-Schnittstelle ergänzt werden.

Die Oberfläche bietet außerdem einen bestätigten Rollback und Diagnosen für
Konfigurationssyntax, rekursive DNS-Auflösung und DNSSEC-Validierung. Vier
geprüfte Betriebsprofile können als Entwurf geladen werden. Der RootGuard
Advisor bewertet jeden Entwurf automatisch hinsichtlich Datenschutz,
Verfügbarkeit, Cache-Effizienz und Ressourcenbedarf. Ein Profil wird niemals
direkt aktiviert, sondern durchläuft denselben Vorschau-, `unbound-checkconf`-,
Versions- und Rollback-Pfad wie eine manuelle Änderung.

Der einklappbare **Expertenmodus** verwaltet zusätzlich die separate Datei
`90-rootguard-custom.conf`. Der Editor bietet Syntaxhervorhebung, Vorlagen,
Vervollständigung, kontextsensitive Erklärungen und Advisor-Hinweise. RootGuard
blockiert systemkritische Direktiven, prüft zunächst einen kombinierten Entwurf
und anschließend die vollständige effektive Konfiguration. Geführte und freie
Konfiguration werden gemeinsam versioniert und wiederhergestellt.

> Der aktuelle Stack ist ein Entwicklungsstand. Vor dem Einsatz als DNS für
> ein gesamtes Netzwerk müssen insbesondere HTTPS, Wiederherstellungstests und
> die RootGuard-Oberflächen für Filter, Clients und Abfragestatistiken ergänzt
> werden.

## Öffentliche Alpha ohne lokalen Build testen

Die Alpha-Compose lädt versionierte Multi-Arch-Images aus GHCR. Ein Checkout
der Komponenten und ein lokaler Image-Build sind dafür nicht erforderlich:

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.2/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.2/.env.alpha.example
```

In `.env` müssen anschließend `ROOTGUARD_API_TOKEN`,
`ROOTGUARD_ADMIN_PASSWORD` und der unabhängige `ROOTGUARD_RECOVERY_TOKEN`
ersetzt werden. API- und Recovery-Token können jeweils separat mit
`openssl rand -hex 32` erzeugt werden. Danach startet eine einzige Compose die
vollständige RootGuard Control Plane:

```sh
docker compose -f compose.alpha.yaml up -d
```

Die WebGUI ist anschließend standardmäßig unter
`http://<IP-des-Docker-Hosts>:8080/login` erreichbar. Der geführte Setup-Dialog
prüft die gewählte Host-Adresse und stellt danach AdGuard Home und Unbound als
geschützte DNS-Kette bereit. Die Alpha ist zum Evaluieren und Melden
reproduzierbarer Fehler gedacht, noch nicht als Produktionsempfehlung.

Enthaltene Funktionen, bekannte Einschränkungen, Betriebsbefehle und Hinweise
für Fehlerberichte stehen in den
[`0.1.0-alpha.2` Release Notes](RELEASE_NOTES_0.1.0-alpha.2.md).

## Website und Dokumentation

Die Projektwebsite wird aus `site/` über GitHub Pages veröffentlicht und ist
für `https://rootguard.foxly.de` vorgesehen. Die eigenständige
[RootGuard-Dokumentation](https://rootguard.foxly.de/docs.html) beschreibt
Installation, Ersteinrichtung, Router- und Client-Konfiguration, WebGUI,
Unbound, AdGuard Home, Updates, Sicherheit, Betrieb und Fehlerbehebung auf
Deutsch und Englisch.

Das [RootGuard Wiki](https://rootguard.foxly.de/wiki.html) ist der zentrale
Einstieg in Systemwissen und Entwicklungsstand. Die öffentliche
[Roadmap bis 1.0](https://rootguard.foxly.de/roadmap.html) fasst den
kanonischen, überprüfbaren [`ROADMAP.md`](ROADMAP.md) zusammen. Änderungen an
Funktionen gelten erst als vollständig, wenn WebGUI, Tests, Wiki,
Dokumentation und Projektstatus gemeinsam aktualisiert wurden.

## Komponenten aktualisieren

Änderungen werden zuerst im jeweiligen Komponenten-Repository committed und
gepusht. Anschließend wird der neue Komponenten-Stand im Hauptrepository
festgehalten:

```sh
git submodule update --remote
git add rootguard-core rootguard-webapp rootguard-unbound
git commit -m "Update RootGuard components"
```

Weitere Hinweise zur Struktur stehen in
[`docs/architecture.md`](docs/architecture.md).

## Lizenz und Marken

RootGuard ist unter der **GNU Affero General Public License v3.0 oder später**
(AGPL-3.0-or-later) veröffentlicht. Nutzung, Prüfung, Änderung und Weitergabe
sind erlaubt. Wer eine geänderte Version verteilt oder Nutzern über ein
Netzwerk bereitstellt, muss den zugehörigen Quellcode unter denselben
Lizenzbedingungen zugänglich machen. Einzelheiten stehen in [`LICENSE`](LICENSE).

Die Softwarelizenz gewährt keine Rechte an den Namen und Logos von RootGuard
oder Foxly IT. Hinweise für Forks und sachliche Namensnennung enthält
[`TRADEMARKS.md`](TRADEMARKS.md).

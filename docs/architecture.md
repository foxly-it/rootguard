# Repository-Struktur

Das RootGuard-Hauptrepository koordiniert die Versionen der eigenständigen
Komponenten. Jede Komponente behält ihre eigene Historie, Abhängigkeiten,
Releases und Entwicklungsabläufe.

```text
rootguard/
├── docs/
├── rootguard-core/
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
  │ Basic Auth
  ▼
RootGuard WebApp
  │ internes Bearer-Token
  ▼
RootGuard Core ────── Docker API
  │                     │
  │ enge APIs           ├── AdGuard Home
  ▼                     └── Unbound
Persistente RootGuard-Daten
```

Nur Webapp und DNS erhalten Host-Ports. AdGuard Home ist ausschließlich in den
internen Netzen für Core erreichbar; seine zufällig erzeugten Zugangsdaten
liegen mit Besitzerrechten im persistenten RootGuard-Volume. Core liegt in
einem internen Docker-Netz und ist der einzige RootGuard-Dienst mit Zugriff auf
den Docker-Socket. Die Webapp kennt weder den Socket noch Host-Systembefehle.

## AdGuard-Ersteinrichtung

Die Webapp bietet ausschließlich Status und den expliziten Bootstrap-Vorgang
an. Core nutzt dafür die typisierten AdGuard-Installer- und DNS-Endpunkte,
prüft Unbounds feste Adresse `172.29.53.2:5335` im internen DNS-Netz vor der
Aktivierung und konfiguriert keinen öffentlichen Fallback. Eine generische
Weiterleitung der AdGuard-API ist
absichtlich ausgeschlossen.

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

Vordefinierte Betriebsprofile und der RootGuard Advisor arbeiten ausschließlich
auf dem Entwurf. Die Empfehlungen sind deterministisch, verändern keine Dateien
und werden vor ihrer Rückgabe gegen dieselben Wertebereiche wie eine spätere
Aktivierung geprüft. Dadurch umgehen weder Profile noch Vorschläge die
Sicherheitskette.

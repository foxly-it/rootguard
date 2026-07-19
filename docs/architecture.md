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

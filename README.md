# RootGuard

RootGuard bündelt die gemeinsam entwickelten Komponenten des Projekts in einem
übergeordneten Repository. Die einzelnen Komponenten bleiben eigenständige
Git-Repositories und werden hier als Submodules eingebunden.

## Komponenten

- `rootguard-core` – zentrale RootGuard-Anwendung
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

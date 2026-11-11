# Was RootGuard bereitstellt

RootGuard kombiniert AdGuard Home als Netzwerkfilter mit Unbound als eigenem rekursiven und DNSSEC-validierenden Resolver. WebApp und Core verwalten den Stack; ein separater Updater tauscht Core und WebApp sicher als Paar aus.

## Sieben Komponenten, klare Verantwortung

| Komponente | Rolle | Beschreibung |
| --- | --- | --- |
| [WebApp](https://github.com/foxly-it/rootguard/tree/main/rootguard-webapp) | UI | Login, Dashboard, geführte Bedienung und enger API-Proxy |
| [Core](https://github.com/foxly-it/rootguard/tree/main/rootguard-core) | CONTROL | Orchestrierung, Konfiguration, Validierung und DNS-Lebenszyklus |
| [Updater](https://github.com/foxly-it/rootguard/tree/main/rootguard-updater) | UPDATE | Atomarer Austausch von Core und WebApp mit gemeinsamem Rollback |
| [Unbound](https://github.com/foxly-it/rootguard/tree/main/rootguard-unbound) | DNS | Gehärteter rekursiver und DNSSEC-validierender Resolver |
| [Blockpage](https://github.com/foxly-it/rootguard/tree/main/rootguard-blockpage) | UI | Eigene, verständliche Blockseite statt AdGuards Standardantwort |
| [Attestation Proxy](https://github.com/foxly-it/rootguard/tree/main/rootguard-attestation-proxy) | TRUST | Enger Egress-Pfad für signierte Release-Prüfungen, selbst per Update-Kanal verwaltet |
| [Docker-Proxy](https://github.com/foxly-it/rootguard/tree/main/rootguard-docker-proxy) | TRUST | Einziger Container mit Docker-Socket; lässt für Core und Updater nur freigegebene Docker-API-Aufrufe durch. Hat keinen eigenen Update-Kanal (siehe [Betrieb](operations.md)) |

!!! tip "Schnell loslegen"
    Neu hier? Weiter mit [Erste Schritte](getting-started.md).

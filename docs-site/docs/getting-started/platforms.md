# Saubere Installation, gleiche Prüfung

Jedes Release wird mit demselben geschützten Ende-zu-Ende-Test auf nativen Linux-amd64-/arm64-Runnern und Docker Desktop geprüft. Der Test umfasst Login, AIO-Bereitstellung, rekursive DNS-Auflösung und die Ablehnung einer ungültigen DNSSEC-Kette.

| Plattform | Nachweis |
| --- | --- |
| Linux amd64 | Nativer automatischer GitHub-Runner |
| Linux arm64 | Nativer automatischer GitHub-Runner |
| Docker Desktop | Apple Silicon / arm64 am 28.07.2026 erfolgreich |

[Prüfmatrix und sicheren Wiederholungstest öffnen ↗](https://github.com/foxly-it/rootguard/blob/main/docs/platform-support.md){: target="_blank" rel="noopener" }

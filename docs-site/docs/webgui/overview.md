# Die Bereiche der Oberfläche

| Bereich | Beschreibung |
| --- | --- |
| **Dashboard** | Installation, DNS-Endpunkt, Dienste, DNSSEC und Verbindungskette sowie echte CPU-/RAM- und aggregierte AdGuard-Kennzahlen auf einen Blick. |
| **Einrichtung** | Netzwerk-Preflight und persistenter AIO-Bereitstellungsfortschritt. |
| **Stack & Updates** | Dienstzustand, sichere Updates und Verlauf bleiben sofort sichtbar; technische Release-Details sind einklappbar und die geschützte Docker-Bereinigung wird erst auf Anforderung geprüft. |
| **Backups** | Wiederherstellungspunkte, verschlüsselter Export und saubere Vollwiederherstellung. Eine Auswahl führt außerdem direkt zum RootGuard-Unbound-Paket oder zum Import einer vorhandenen `unbound.conf`. |
| **Logs & Diagnose** | Zentrale, durchsuchbare Protokolle für alle verwalteten Dienste mit lokaler Filterung, optionaler Aktualisierung und redigiertem Diagnosebericht. |
| **Unbound** | Profile, geführte Einstellungen, Zonen, Live-Konfiguration und Direktiven in großen Detailansichten, Experteneditor und Rollback. |
| **AdGuard Home** | RootGuard-Status und geschützter Zugriff auf die native AdGuard-Oberfläche. |

## Live-Kennzahlen

Das Dashboard aggregiert ausschließlich die CPU- und RAM-Nutzung der fünf fest freigegebenen RootGuard-Container. Zusätzlich liest Core über die intern authentifizierte AdGuard-Home-API die Gesamtzahl der DNS-Anfragen und blockierten Anfragen aus und berechnet daraus die Filterquote. Query-Namen und Clientdaten verlassen AdGuard dabei nicht. Die Anzeige aktualisiert sich automatisch alle zehn Sekunden und kennzeichnet nicht verfügbare Messwerte ausdrücklich.

# Anwendungsbeispiel: Geräte aus der FRITZ!Box importieren

Statt jeden Host manuell in **Unbound → Lokale Zonen** anzulegen, kann RootGuard bekannte Geräte direkt aus deiner FRITZ!Box übernehmen.

## Voraussetzungen

- Die FRITZ!Box muss über TR-064 im lokalen Netz erreichbar sein (Standard bei den meisten FRITZ!OS-Versionen).
- Zugangsdaten sind nur nötig, wenn deine FRITZ!Box eine Anmeldung für TR-064-Anfragen verlangt.

## Schritte

1. Öffne **Unbound → Lokale Zonen → Aus FRITZ!Box importieren**.
2. Trage die FRITZ!Box-Adresse ein (z. B. `192.168.178.1`). Falls eine Anmeldung nötig ist, gib Benutzername/Passwort ein - beides wird **ausschließlich für diese eine Abfrage** verwendet und danach nicht gespeichert.
3. RootGuard fragt die Geräteliste ab und zeigt einen **Entwurf**: Hostname, erkannte Adresse(n) und eventuelle Konflikte mit bereits vorhandenen Einträgen.
4. Wähle einzeln aus, welche Geräte übernommen werden sollen. Nichts wird automatisch importiert. Du kannst Hostnamen vor der Übernahme anpassen, z. B. `fritzbox-erkannt-iphone-mark` in `iphone-mark.home.lan` umbenennen.
5. Übernommene Hosts durchlaufen **dieselbe Vorschau-, Checkconf- und Aktivierungskette** wie manuell angelegte lokale Zonen.

## Alternative: Router-unabhängige Reverse-DNS-Erkennung

Ohne FRITZ!Box (oder zusätzlich dazu) lässt sich ein Netzwerkbereich direkt per Reverse-DNS abfragen:

```text
Präfix: 192.168.178.0/24
```

- Nur private IPv4- oder Unicast-IPv6-Präfixe sind erlaubt.
- Maximal 256 Adressen pro Präfix und insgesamt - größere Bereiche werden abgelehnt.
- Sechzehn parallele Abfragen mit gemeinsamem 15-Sekunden-Zeitlimit; einzelne fehlgeschlagene Lookups tauchen als solche auf, verhindern aber nicht die erfolgreichen Ergebnisse.

!!! note "Datenschutz bei FRITZ!Box-Zugangsdaten"
    Zugangsdaten werden ausschließlich serverseitig für die eine Abfrage verwendet - niemals im Browser gespeichert, niemals geloggt, niemals in der generierten Konfiguration, im Verlauf oder in Diagnoseberichten sichtbar.

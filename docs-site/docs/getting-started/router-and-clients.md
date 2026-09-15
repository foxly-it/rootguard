# RootGuard im Netzwerk verwenden

Trage die im Setup angezeigte feste Host-IP als DNS-Server im Router ein. Verwende niemals `127.0.0.1` oder die interne Docker-Adresse `172.29.53.2` auf anderen Geräten. Port 53 muss für TCP und UDP erreichbar sein.

```shell title="Prüfung von einem Client"
dig @192.168.178.10 example.com A
dig +dnssec @192.168.178.10 dnssec-failed.org A
```

Die erste Abfrage muss eine Adresse liefern. Die zweite muss mit `SERVFAIL` enden; dadurch wird eine ungültige DNSSEC-Kette korrekt verworfen.

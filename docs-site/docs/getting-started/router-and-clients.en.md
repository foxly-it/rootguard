# Use RootGuard on your network

Enter the fixed host IP shown by Setup as DNS server in your router. Never use `127.0.0.1` or the internal Docker address `172.29.53.2` on other devices. Port 53 must be reachable over TCP and UDP.

```shell title="Check from a client"
dig @192.168.178.10 example.com A
dig +dnssec @192.168.178.10 dnssec-failed.org A
```

The first query must return an address. The second must end with `SERVFAIL`, proving that an invalid DNSSEC chain is rejected.

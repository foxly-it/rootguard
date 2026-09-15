# Worked example: local DNS zones

The guided assistant under **Unbound → Local zones** creates A, AAAA, and CNAME records for devices on your network without writing any Unbound syntax by hand.

## Goal

A zone `home.lan` with three hosts:

| Host | IPv4 | IPv6 | Derive PTR |
| --- | --- | --- | --- |
| `nas.home.lan` | `192.168.178.20` | – | yes |
| `printer.home.lan` | `192.168.178.30` | – | yes |
| `ap1.home.lan` | `192.168.178.40` | `fd00::40` | yes |

## Steps

1. Open **Unbound → Local zones** and create the zone `home.lan`.
2. Add one entry per host with its name and address(es). `nas` and `printer` only need IPv4; for `ap1` also add the IPv6 address.
3. Enable "Derive PTR record" per host. RootGuard automatically derives the matching reverse zone (`178.168.192.in-addr.arpa`) - provided the address is unambiguous across all guided zones.
4. Review the change preview: RootGuard shows the generated `local-zone`/`local-data`/`local-data-ptr` directives before anything goes live.
5. Activate. RootGuard runs `unbound-checkconf` against the effective configuration before writing it out; on failure, the previous version stays active.

## Verify after activation

```shell
dig @192.168.178.10 nas.home.lan A
dig @192.168.178.10 -x 192.168.178.20
```

The first query must return `192.168.178.20`; the second (reverse lookup) must return `nas.home.lan.`

!!! note "Limits of the guided surface"
    CNAME records can be created, but **per-host TTL and more complex CNAME chains are deliberately out of scope for this guided surface** ([issue #131](https://github.com/foxly-it/rootguard/issues/131)). For those rare cases, use expert mode - it goes through the same preview, checkconf, and rollback pipeline.

!!! tip "Client access rules"
    Who is allowed to query your network at all stays deliberately managed in AdGuard Home, not here - Unbound zones only affect how names resolve, not who may ask.

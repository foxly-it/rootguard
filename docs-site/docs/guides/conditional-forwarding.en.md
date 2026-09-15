# Worked example: conditional forwarding

Conditional forwarding sends queries for specific internal zones to your own DNS servers instead of resolving them recursively through the public root hierarchy - useful for a corporate or VPN zone that's only known internally.

## Goal

Forward the zone `corp.example` to two ordered servers:

1. `10.0.0.53` (primary)
2. `10.0.0.54` (secondary, if the first doesn't answer)

## Steps

1. Open **Unbound → Conditional Forwarding** and create a new zone `corp.example`.
2. Enter `10.0.0.53` as the first and `10.0.0.54` as the second target. Order is preserved - RootGuard queries them in exactly this order.
3. RootGuard verifies both targets **live against the running Unbound container**: activation only unlocks once the configured zone is answered by a target with `NOERROR` and a valid SOA record. `NXDOMAIN`, `REFUSED`, timeouts, or empty successful responses show up as diagnostics but block activation.
4. Optional opt-ins, each separate and visible:
      - **Recursive fallback** if none of the targets answer.
      - **Allow unsigned answers** (`domain-insecure`) if the internal server doesn't return DNSSEC-signed answers - needed because DNSSEC validation otherwise stays on by default.
      - **Allow private RFC1918 answers**, if `corp.example` legitimately points at private addresses - without this opt-in, rebinding protection for private address ranges stays active.
5. Activate. Like any Unbound change, this goes through the shared preview, checkconf, versioning, and rollback pipeline.

## Verify after activation

```shell
dig @192.168.178.10 host.corp.example A
```

If `10.0.0.53` doesn't respond within the configured time, `10.0.0.54` takes over automatically - no manual switch needed.

!!! warning "Loop detection"
    RootGuard refuses targets that would point back at RootGuard itself, at AdGuard Home, or at another already-configured forwarding zone - a forwarding loop can't be activated in the first place.

!!! note "Authenticated DNS-over-TLS"
    Forwarding over DNS-over-TLS with certificate verification is currently **deliberately not supported** until certificate identity can be modeled safely. Plain address targets, as in the example above, already work fully regardless.

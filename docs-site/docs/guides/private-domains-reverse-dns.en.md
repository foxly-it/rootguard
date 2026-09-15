# Worked example: private domains and reverse DNS

RootGuard decides independently, for each of the three private RFC1918 address ranges, how reverse lookups are handled.

## The three ranges

| Range | Typical network |
| --- | --- |
| `10/8` | Large private networks, VPNs |
| `172.16/12` | Default Docker networks, some routers |
| `192.168/16` | Most home networks |

## Two options per range

- **NXDOMAIN (default)** - a reverse lookup for an address in this range that RootGuard doesn't know about itself is safely answered with "not found." No internal address name leaves your network toward the public DNS hierarchy.
- **Transparent public fallback** - the lookup is instead resolved normally, recursively. RootGuard shows a **visible warning** before activation that this can let private reverse lookups leave the network.

## Example: deliberately letting `192.168.178.0/24` resolve publicly

If your setup needs public reverse-DNS answers for your home network for some specific reason (e.g. a device that expects public PTR records):

1. Open **Unbound → Private Domains**.
2. For `192.168/16`, explicitly choose "Transparent fallback" instead of the NXDOMAIN default.
3. Confirm the warning shown - it explains exactly what this changes.
4. Activation goes through the same preview and checkconf verification as any other Unbound change.

In this example, `10/8` and `172.16/12` keep the safe NXDOMAIN default unchanged - the three ranges are configured independently of each other.

## PTR records from local zones

If you instead just want `192.168.178.20` to resolve as `nas.home.lan` (without a range-wide fallback), this is the wrong place for that - see the [local DNS zones worked example](local-dns-zones.md) instead: PTR records derived from your own A/AAAA entries work independently of the range-wide NXDOMAIN/fallback setting described here.

!!! note "Why NXDOMAIN is the safe default"
    A reverse lookup for a private address that leaves the network can expose internal naming conventions or network structure. New and migrated installations therefore start with NXDOMAIN for all three ranges; any exception is a deliberate, individually visible choice.

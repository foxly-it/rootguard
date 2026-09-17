# Local DNS on the home network

Most routers only resolve names for devices on their own network in a rudimentary way - often only for DHCP-assigned names, with no wildcard domains, no working reverse DNS for every device, and no control over what happens for internal versus public names. A dedicated recursive resolver with local zones fixes this at the root, regardless of which specific software runs it.

## Why run your own resolver

- **Readable names instead of IP addresses.** `nas.home.lan` instead of `192.168.178.20` - for backups, SSH configs, bookmarks.
- **Reverse DNS (PTR) that's actually correct.** Log files, `dig -x`, and many admin interfaces display hostnames - without working PTR, that stays a bare IP.
- **Control over internal versus public resolution.** Some internal domains should never leave the network (split DNS); others should deliberately stay publicly resolvable even though they sit in a private address range.
- **One place for conditional forwarding.** A corporate VPN, a second internal network, a separate nameserver for one specific domain - all of that can live in one place instead of being configured on every client individually.

## The building blocks

**Local zones (A/AAAA/CNAME).** A resolver that answers authoritatively for a self-defined zone (e.g. `home.lan`) instead of resolving the query recursively. Every device gets a name, optionally with IPv4 and IPv6 side by side.

**Reverse DNS (PTR).** The reverse direction: `192.168.178.20` becomes `nas.home.lan`. It has to match forward resolution, or log entries and actual reachability start contradicting each other.

**Conditional forwarding.** A specific domain isn't resolved recursively but forwarded to a different, dedicated DNS server - typical for corporate networks, VPN domains, or a second internal zone.

**Private-domain policy.** What happens when someone outside the network tries to resolve a `192.168.x.x` address? The safe default answer is NXDOMAIN; deliberately opening it up is possible but should be the exception.

## Adopting devices automatically instead of maintaining them by hand

Entering every device individually doesn't scale. Two ways to avoid that:

- **Import from the router**, if it exposes device information through an interface (e.g. a FRITZ!Box via TR-064) - hostnames and addresses are adopted, not reinvented.
- **Reverse-DNS discovery** over an IP range, if devices are already reachable via DHCP hostnames but no central device inventory exists.

## How RootGuard implements this

RootGuard manages exactly these four building blocks through a guided interface on top of a hardened Unbound resolver: a form instead of Unbound syntax, a live check against the running resolver before every activation, loop detection for conditional forwarding, and versioning with rollback. Concrete, worked examples for all four building blocks: [Unbound examples](unbound-examples.en.md).

## See also

- [Unbound examples](unbound-examples.en.md) - the same concepts, step by step in RootGuard's interface.
- [Getting started with RootGuard](../getting-started.en.md) - installation and first-time setup.

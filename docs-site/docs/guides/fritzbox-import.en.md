# Worked example: importing devices from your FRITZ!Box

Instead of creating every host manually under **Unbound → Local zones**, RootGuard can adopt known devices directly from your FRITZ!Box.

## Requirements

- The FRITZ!Box must be reachable via TR-064 on the local network (the default on most FRITZ!OS versions).
- Credentials are only needed if your FRITZ!Box requires a login for TR-064 requests.

## Steps

1. Open **Unbound → Local zones → Import from FRITZ!Box**.
2. Enter the FRITZ!Box address (e.g. `192.168.178.1`). If a login is required, provide username/password - both are used **only for this one lookup** and are never stored afterward.
3. RootGuard queries the device list and shows a **draft**: hostname, detected address(es), and any conflicts with existing entries.
4. Individually select which devices to adopt. Nothing is imported automatically. You can rename hostnames before adoption, e.g. turning `fritzbox-detected-iphone-mark` into `iphone-mark.home.lan`.
5. Adopted hosts go through **the same preview, checkconf, and activation pipeline** as manually created local zones.

## Alternative: router-independent reverse-DNS discovery

Without a FRITZ!Box (or in addition to it), you can query a network range directly via reverse DNS:

```text
Prefix: 192.168.178.0/24
```

- Only private IPv4 or unicast IPv6 prefixes are allowed.
- A maximum of 256 addresses per prefix and in total - larger ranges are rejected.
- Sixteen parallel lookups share a 15-second deadline; individual failed lookups show up as such but don't block the successful results.

!!! note "Privacy of FRITZ!Box credentials"
    Credentials are used server-side only, for that one lookup - never stored in the browser, never logged, and never visible in the generated configuration, history, or diagnostic reports.

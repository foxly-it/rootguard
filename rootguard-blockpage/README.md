# RootGuard Blockpage

**A RootGuard-branded landing page for AdGuard Home's "custom IP for blocked
hosts."** Instead of a browser connection error, devices on the network see a
clear, on-brand explanation of why a request was blocked - with the domain,
time, and client IP, and a link to more detail. Static HTML/CSS/JS, no build
step, no external dependencies, automatic light/dark theme.

[![License](https://img.shields.io/badge/license-AGPL--3.0--or--later-72c483)](LICENSE)

[RootGuard](https://github.com/foxly-it/rootguard) ·
[Manual](https://rootguard.foxly.de/docs.html) ·
[Security](../SECURITY.md)

## How it fits into RootGuard

A DNS-level blocking IP cannot present a valid TLS certificate for the
originally requested domain, so this page is served over plain HTTP only -
that's a property of how DNS blocking works, not a limitation specific to this
image. During guided setup, RootGuard Core configures AdGuard Home's
`blocking_mode: custom_ip` to point at this container's address automatically.
The feature is optional and enabled by default; it can be turned off in Setup.

## Quick start

```sh
docker build -t rootguard-blockpage:test .
docker run --rm -p 8080:8080 rootguard-blockpage:test
```

Open `http://localhost:8080/`. `/clientip.txt` returns the caller's address as
plain text; `/info/` explains common blocking reasons in more detail.

## Design

Colors and spacing mirror the semantic tokens used by the RootGuard WebGUI
(`--bg`, `--surface`, `--accent`, `--danger`, ...) so this page reads as part
of the same product rather than a bolted-on extra, without sharing any actual
code with the WebApp. Theme follows the system preference by default; a
toggle persists an explicit choice in `localStorage`.

## License

AGPL-3.0-or-later, see [LICENSE](LICENSE). See
[TRADEMARKS.md](https://github.com/foxly-it/rootguard/blob/main/TRADEMARKS.md)
in the main repository for the RootGuard and Foxly IT name and logo policy.

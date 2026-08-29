# Contributing to RootGuard Unbound

Thanks for helping improve the RootGuard recursive DNS image.

## Before you start

Check the [issues](https://github.com/foxly-it/rootguard-unbound/issues) and the
main [RootGuard roadmap](https://github.com/foxly-it/rootguard/blob/main/ROADMAP.md).
Discuss changes to the immutable base configuration, trust-anchor handling,
access controls, image tags, or runtime privileges before implementation.

Security vulnerabilities must be reported privately through
[SECURITY.md](../SECURITY.md).

## Development

```sh
git clone https://github.com/foxly-it/rootguard-unbound.git
cd rootguard-unbound
docker build -t rootguard-unbound:test .
docker run --rm rootguard-unbound:test \
  unbound-checkconf /etc/unbound/unbound.conf
```

Test recursive resolution and DNSSEC rejection before opening a pull request.
Preserve non-root execution, read-only compatibility, dropped capabilities,
private-network access control, and the separation between immutable base and
managed includes.

## Pull requests

Explain the problem, solution, security implications, image-size impact, and
checks performed. Link the related issue. Contributions are accepted under
[AGPL-3.0-or-later](LICENSE).

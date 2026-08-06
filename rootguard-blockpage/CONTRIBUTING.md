# Contributing to RootGuard Blockpage

Thanks for helping improve the page RootGuard shows for blocked requests.

## Before you start

Check the [issues](https://github.com/foxly-it/rootguard/issues) and the main
[RootGuard roadmap](https://github.com/foxly-it/rootguard/blob/main/ROADMAP.md).
This directory is part of the `rootguard` monorepo; open one pull request
there touching whichever directories a change needs.

Security vulnerabilities must be reported privately through
[SECURITY.md](SECURITY.md).

## Development

```sh
git clone https://github.com/foxly-it/rootguard.git
cd rootguard/rootguard-blockpage
docker build -t rootguard-blockpage:test .
docker run --rm -p 8080:8080 rootguard-blockpage:test
```

Open `http://localhost:8080/` and `http://localhost:8080/info/`. Check both
light and dark rendering, and that `/clientip.txt` returns the caller's
address. This page is served over plain HTTP only - see `docs/architecture.md`
in the main repository for why.

## Pull requests

Explain the problem, solution, security implications, and checks performed.
Link the related issue. Contributions are accepted under
[AGPL-3.0-or-later](LICENSE).

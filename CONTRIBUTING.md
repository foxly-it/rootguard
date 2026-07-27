# Contributing to RootGuard Core

Thanks for helping improve the RootGuard control plane.

## Before you start

Check the [issues](https://github.com/foxly-it/rootguard-core/issues) and the
main [RootGuard roadmap](https://github.com/foxly-it/rootguard/blob/main/ROADMAP.md).
Use an issue to discuss changes that affect API contracts, security boundaries,
managed Compose content, or persistent configuration formats.

Security vulnerabilities must be reported privately through
[SECURITY.md](SECURITY.md).

## Development

```sh
git clone https://github.com/foxly-it/rootguard-core.git
cd rootguard-core
go test ./...
go vet ./...
go build ./...
docker build -t rootguard-core:test .
```

Keep changes deterministic and narrowly scoped. Browser-controlled input must
never select arbitrary images, mounts, paths, Compose arguments, or commands.
Add tests for changed behavior and update the main project documentation when
an API or visible capability changes.

## Pull requests

Explain the problem, solution, security implications, and checks performed.
Link the related issue. Keep generated files and secrets out of commits.
Contributions are accepted under [AGPL-3.0-or-later](LICENSE).

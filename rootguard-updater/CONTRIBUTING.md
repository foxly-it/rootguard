# Contributing to RootGuard Updater

RootGuard Updater is intentionally small and narrowly privileged. Before
opening a pull request:

1. keep Docker services, containers, images, and Compose paths server-controlled;
2. never accept Docker commands or resource names from an HTTP request;
3. preserve paired Core/WebApp verification and rollback;
4. retain the active and previous successful image;
5. never introduce a global Docker prune command;
6. add tests for success, failed verification, rollback, and cleanup boundaries;
7. run `go test ./...`, `go vet ./...`, and a local container build.

Use a focused branch and explain the security impact and failure behavior in
the pull request. General RootGuard architecture and roadmap changes belong in
the [main repository](https://github.com/foxly-it/rootguard).

# Contributing to RootGuard Docker Proxy

This component is the one, narrow, auditable holder of the real Docker
socket, standing in front of it so Core and the Updater no longer need
direct, unrestricted access. Before opening a pull request:

1. never widen the method+path allowlist or a body validator's accepted
   fields without confirming, against Core's/the Updater's real code,
   that the new operation is actually issued somewhere - don't guess or
   "just to be safe" allow something wider;
2. never allow `Privileged`, `NetworkMode`/`PidMode`/`IpcMode`/
   `UTSMode: host`, arbitrary `Devices`, an uncontrolled `CapAdd`, or a
   bind/mount source outside RootGuard's own named volumes through
   `POST /containers/create` - that's the exact primitive this proxy
   exists to close off;
3. keep the runtime image `scratch`-based; the only container in this
   repository allowed to run as root is this one, and only because
   Docker-socket access structurally requires it (see the Dockerfile's
   own comment) - don't add a `USER` instruction here without first
   solving the docker-group-GID problem for real, not by disabling the
   check;
4. the health check must never depend on live reachability to the real
   Docker socket - see `main.go`'s `runHealthcheck` doc comment;
5. add both a positive test (the real call still passes) and a negative
   test (the specific dangerous variant is rejected with 403 and never
   reaches the fake upstream) for any allowlist/validator change, then
   run `go test ./...`, `go vet ./...`, and a local container build.

Use a focused branch and explain the security impact in the pull
request. General RootGuard architecture and roadmap changes belong in
the [main repository](https://github.com/foxly-it/rootguard).

# Contributing to RootGuard Attestation Proxy

This component is intentionally tiny and narrowly scoped: a CONNECT-only
forward proxy with a hardcoded, 3-host allowlist, existing purely so
Core/the Updater can reach the internet for cosign's own attestation
verification without `control`'s own network isolation being reopened
wholesale. Before opening a pull request:

1. never widen the allowlist without live confirmation of what host a
   real, successful `cosign verify-attestation` call actually needs -
   don't guess or "just to be safe" add a host;
2. never add general-purpose HTTP proxying, only CONNECT tunneling;
3. never terminate TLS here - this proxy must stay content-blind to
   everything except the CONNECT target itself;
4. keep the runtime image `scratch`-based, non-root, with no Docker
   socket and no host mounts - this is deliberately the least-privileged
   component in the repository;
5. the health check must never depend on live reachability to any
   allowlisted host - see `main.go`'s `runHealthcheck` doc comment;
6. add tests for both directions (allowed hosts tunnel correctly,
   everything else is rejected) and run `go test ./...`, `go vet ./...`,
   and a local container build.

Use a focused branch and explain the security impact in the pull
request. General RootGuard architecture and roadmap changes belong in
the [main repository](https://github.com/foxly-it/rootguard).

# Clean-install platform verification

Tracked by [issue #39](https://github.com/foxly-it/rootguard/issues/39).

RootGuard's public beta uses one immutable Compose model on every supported
Docker platform. The clean-install verifier proves more than image
availability: it starts the control plane, signs in, runs the AIO preflight,
deploys AdGuard Home and Unbound, resolves a public name, and verifies that an
invalid DNSSEC chain is rejected.

## Verification matrix

| Platform | Architecture | Verification | Status |
| --- | --- | --- | --- |
| Linux, GitHub-hosted Ubuntu 24.04 | `amd64` | Native automated runner | Passed 2026-07-28 |
| Linux, GitHub-hosted Ubuntu 24.04 | `arm64` | Native automated runner | Passed 2026-07-28 |
| Docker Desktop 4.x on macOS | Apple Silicon / `arm64` | Same portable verifier | Passed 2026-07-28 |

The Docker Desktop result used Docker Engine 29.6.2 and Docker Compose 5.3.1.
It installed the immutable `v0.1.0-alpha.2` images, completed the managed DNS
deployment, returned a recursive IPv4 answer, and rejected
`dnssec-failed.org` with `SERVFAIL`.

Docker Desktop on Intel uses the same published `amd64` manifests but is not a
separately verified platform in the current verification matrix.

## Repeat the test

This test is intended only for an empty disposable Docker environment:

```sh
git clone https://github.com/foxly-it/rootguard.git
cd rootguard
./scripts/verify-clean-install.sh
```

Requirements are Docker Engine or Docker Desktop with Compose v2, `curl`,
`dig`, and `jq`. Set `ROOTGUARD_TEST_ARCH=amd64` or `arm64` to require an exact
Docker architecture.

The verifier refuses to start if any RootGuard container, named data volume, or
DNS network already exists. On a clean host it creates only RootGuard resources
and removes those resources when the test finishes. It never calls a global
Docker prune command and does not inspect or delete unrelated containers,
images, networks, or volumes.

## Automated evidence

The `Public clean install` workflow executes the verifier on native
GitHub-hosted `amd64` and `arm64` Linux runners. Each job records the runner,
Docker architecture, Docker Engine version, and Compose version in its job
summary. A failed install prints bounded control-plane and DNS-service logs
before cleaning up its own resources.

The first complete native matrix passed in
[GitHub Actions run 30353823582](https://github.com/foxly-it/rootguard/actions/runs/30353823582).

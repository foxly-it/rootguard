# RootGuard agent context

Before performing broad repository discovery, read `docs/project-state.md`.
It records the current architecture, delivered capabilities, verified behavior,
and the next production milestones.

RootGuard is a monorepo: `rootguard-core/`, `rootguard-webapp/`,
`rootguard-unbound/`, and `rootguard-updater/` are top-level directories here,
each independently buildable with its own Dockerfile and path-filtered CI
workflow. Implement a change as one PR in this repository, touching whichever
component directory(ies) it needs, and update integration tests, documentation,
and `docs/project-state.md` in that same PR.

Preserve the security boundaries documented in `docs/architecture.md`: the
WebApp has no Docker socket, Core is internal and token protected, AdGuard has
no public administration port, and invalid Unbound changes must never replace
the active resolver configuration.

Update `docs/project-state.md` whenever a material feature is merged so future
sessions can continue without repeating a full repository audit.

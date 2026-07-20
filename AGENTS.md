# RootGuard agent context

Before performing broad repository discovery, read `docs/project-state.md`.
It records the current architecture, delivered capabilities, verified behavior,
and the next production milestones.

RootGuard is a main repository with independent component repositories included
as Git submodules. Implement and publish component changes in their own
repositories first. After their PRs are merged, update the submodule revisions,
integration tests, documentation, and `docs/project-state.md` here.

Preserve the security boundaries documented in `docs/architecture.md`: the
WebApp has no Docker socket, Core is internal and token protected, AdGuard has
no public administration port, and invalid Unbound changes must never replace
the active resolver configuration.

Update `docs/project-state.md` whenever a material feature is merged so future
sessions can continue without repeating a full repository audit.

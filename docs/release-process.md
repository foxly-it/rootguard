# RootGuard release process

`.github/workflows/release-alpha.yml` is the release pipeline for every
component image (`rootguard-core`, `rootguard-webapp`, `rootguard-updater`,
`rootguard-unbound`, `rootguard-blockpage`). This document is the
architecture-level map of what it does and why it's shaped the way it is;
the workflow file itself keeps its own inline comments for the specific,
often incident-driven reasoning behind individual steps - this doc doesn't
duplicate those, it gives the overall picture they each sit inside.

## Trigger and identity

A release starts either as a `v*.*.*` tag push (the normal path -
`release-version-bump.yml` computes the next version, commits a changelog
entry, pushes the commit and tag atomically via `git push --atomic`, then
dispatches this workflow with that exact commit as `source_sha`) or a
manual `workflow_dispatch` with an explicit `version` and, for automation,
`source_sha`.

Every job checks out `source_sha` (falling back to `github.sha` for a
by-hand dispatch with nothing else supplied) rather than the ambient
`github.sha` directly - a `workflow_dispatch` run's own `github.sha`
resolves from whatever the target ref is *when the run's checkout actually
executes*, not at dispatch-request time, so anything else landing on
`main` in that window would otherwise get silently pulled into the release.
`source_ref` and a single computed `candidate_tag` (see below) are both
resolved once, in the `version` job, and threaded through every later job's
`needs.version.outputs.*` rather than each re-deriving them.

## Build and test gate

`test` runs the same checks each component's own PR-level CI does (Go
vet/test for all three modules, frontend lint/test/build) - a release cut
has no PR of its own, so this is the only place that would ever catch a
regression before it ships. `security` calls `ci-security.yml` (gitleaks,
trivy, govulncheck, staticcheck) as a reusable workflow, pinned to the
release's own `source_ref`, so the release gate depends on it directly
instead of trusting that it happened to run against the right commit at
some earlier, disconnected point via its own `push: branches: [main]`
trigger. `publish` requires both to have passed.

## Candidate tags, not the final tag, until tests pass

`publish` builds and pushes each component to a commit-scoped **candidate**
tag (`VERSION-candidate-<12-char-sha>`), never the final `VERSION` tag
directly. `smoke-test` and `upgrade-test` run against those candidate
images. Only after both pass does `update-alpha-pins` promote the exact
tested manifest to the final tag, via `docker buildx imagetools create`
(no rebuild - same digest, same attestation, same provenance label as what
was actually tested), with an immediate re-inspection to confirm the
promoted digest matches.

This exists specifically so a failing E2E test never leaves a public,
pullable image under the final version tag, and so a retry of a genuinely
new commit can't be satisfied by an old candidate that happened to share a
tag - the candidate tag encodes the commit, so a different commit always
gets a different candidate.

## Pin update, release tag, and GitHub Release

Still inside `update-alpha-pins`, after promotion: `compose.release.yaml`
and `.env.release.example` get their image references rewritten to the
newly promoted digests, `site/*.html`'s version references are refreshed,
the result is validated (compose config parses, every image pin carries
`@sha256:`, nothing points at a mutable `:latest`), and the pin-only commit
is pushed to `main` (`[skip ci]`, since a mechanical pin refresh shouldn't
re-trigger the whole CI matrix). The release *tag* is then moved to point
at that pin commit - not left at the earlier commit the images were built
from - so the documented quick start (`curl .../vX.Y.Z/compose.release.yaml`)
always resolves to a compose file pinned to that same release's own
images. The GitHub Release itself is created last, after every check above
has already passed - a failed E2E test used to still leave a real, publicly
visible Release behind (auto-discoverable by any live installation's own
update check) needing manual cleanup.

## Upgrade testing

`upgrade-test` doesn't synthesize a fixture - it deploys the actual
immediately-preceding published release exactly as it shipped, then drives
the real control-plane updater to move it onto the candidate just built,
verifying DNS both before and after. It intentionally bypasses Core's own
live "latest release" discovery and calls the updater directly with an
explicit target, because that discovery logic is itself sometimes what
changed in the release being tested - relying on it here would test the
new release's ability to discover itself, not the upgrade path a real
operator running the previous release would actually take.

## What's deliberately not extracted (yet)

The pin-update, compose-verification, and release-notes generation logic
above all lives inline in `update-alpha-pins`'s own steps rather than as
separate, independently-testable scripts (the pattern
`scripts/lib/semver-validate.sh` and `scripts/bump-site-versions.sh`
already use elsewhere in this pipeline). That's a reasonable next
compression step, but a release pipeline is exactly the kind of file where
a rushed refactor risks introducing a real regression for a marginal
readability gain - deferred as a deliberate, separate piece of work rather
than folded into a documentation pass.

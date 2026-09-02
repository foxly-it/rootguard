# RootGuard release process

`.github/workflows/release-alpha.yml` is the release pipeline for every
component image (`rootguard-core`, `rootguard-webapp`, `rootguard-updater`,
`rootguard-unbound`, `rootguard-blockpage`, `rootguard-attestation-proxy`).
This document is the architecture-level map of what it does and why it's
shaped the way it is; the workflow file itself keeps its own inline
comments for the specific, often incident-driven reasoning behind
individual steps - this doc doesn't duplicate those, it gives the overall
picture they each sit inside.

## Trigger and identity

A release starts as a `workflow_dispatch` - normally from
`release-version-bump.yml` (computes the next version, commits a changelog
entry, pushes that single commit to `main`, then dispatches this workflow
with that exact commit as `source_sha`), or by hand with an explicit
`version` and, for automation, `source_sha`. This workflow used to also
trigger directly on a pushed `v*.*.*` tag - removed (a follow-up review's
finding) once the release tag itself stopped being created up front: this
workflow now creates it exactly once, after every gate has passed, and
never moves it again under any circumstance (see "Pin update, release
tag, and GitHub Release" below) - a tag pushed by hand already exists
before a single test has run, which that never-move guarantee then has no
way to honor.

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

Both `release-version-bump.yml`'s changelog commit and this job's own pin
commit/tag push authenticate as `secrets.RELEASE_PAT` (a fine-grained
token, Contents: Read and write only, scoped to this repository), not
the default `GITHUB_TOKEN` - found live, cutting 1.0.0-rc.2: `main`
requires all 20 status checks on every push, not just PR merges, and the
default token authenticates as the "github-actions" app, which isn't a
repository admin and so can't bypass that even with `contents: write`
granted. `RELEASE_PAT` belongs to an account that does have admin rights
on this repo - admins bypass required status checks on a direct push
(`enforce_admins` is off), the same way a maintainer pushing either
commit by hand always could. Both checkout steps pass this token
explicitly so every subsequent git command in that job authenticates
with it.

Before any of this, `update-alpha-pins` re-verifies `origin/main` still
equals `source_ref` - twice: once immediately, before promoting a single
image, and once more right before the pin commit itself. If `main` has
moved on since this release was tested, the run aborts outright rather
than folding untested commits into the release (a follow-up review's
finding, after this had actually happened live: a real RC's tag and its
own Core image's OCI revision label ended up pointing at two different
commits, because the only such check used to run *after* image promotion
had already happened).

After promotion: `compose.release.yaml` and `.env.release.example` get
their image references rewritten to the newly promoted digests,
`site/*.html`'s version references are refreshed, the result is validated
(compose config parses, every image pin carries `@sha256:`, nothing
points at a mutable `:latest`), and the pin-only commit is pushed straight
to `main` - never rebased onto whatever `main` happens to be by then
(`[skip ci]`, since a mechanical pin refresh shouldn't re-trigger the
whole CI matrix). The release *tag* is then created, pointing at that pin
commit rather than the earlier commit the images were built from, so the
documented quick start (`curl .../vX.Y.Z/compose.release.yaml`) always
resolves to a compose file pinned to that same release's own images -
created exactly once, here, the *only* place this tag is ever written,
and never force-moved again afterward: an existing tag that already
matches is a no-op, one that doesn't is a hard error demanding a by-hand
fix, never a silent overwrite. The GitHub Release itself is created last,
after every check above has already passed - a failed E2E test used to
still leave a real, publicly visible Release behind (auto-discoverable by
any live installation's own update check) needing manual cleanup.

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
`scripts/lib/semver-validate.sh`, `scripts/lib/semver-compare.sh`, and
`scripts/bump-site-versions.sh` already use elsewhere in this pipeline -
`semver-compare.sh` joined that pattern only once a real gap needed real
SemVer 2.0 precedence logic, not the other way around: extraction happens
when a piece of logic earns its own file by being genuinely reusable and
independently testable, not as a wholesale rewrite). That's a reasonable
next compression step for the rest, but a release pipeline is exactly the
kind of file where a rushed refactor risks introducing a real regression
for a marginal readability gain - deferred as a deliberate, separate
piece of work rather than folded into a documentation pass.

## Self-update can never deliver a compose-topology change

Self-update (both Core's own self-replacement of Core/WebApp and the
Updater's self-replacement) only ever swaps container *images* in place
(`docker compose ... up -d --no-deps <service>`) against whatever
compose file already exists on the operator's disk - it never re-fetches
`compose.release.yaml` itself. Adding `rootguard-attestation-proxy` (a
new service, a new `egress` network) is exactly the kind of change
self-update structurally cannot carry to an existing installation: an
operator already running an older release, updating via the WebGUI
alone, ends up with new core/updater binaries that expect
`ROOTGUARD_ATTESTATION_PROXY_URL` and the proxy service to exist, but
neither does. This is not a silent failure, though: `RequireAttestation`
(Core) and `verifyAttestation` (the Updater) both check the proxy is
actually configured and reachable *before* ever invoking cosign - an
empty `ROOTGUARD_ATTESTATION_PROXY_URL` refuses the update outright with
"no attestation proxy configured ... this installation's compose
topology likely predates rootguard-attestation-proxy; a fresh install or
a manual compose.release.yaml refresh is required" (or, if the variable
is set but nothing answers on it, an equally specific "configured but
unreachable" message) - not cosign's own generic network-error text, and
not a hang. Only a fresh `install.sh` run, or an operator manually
refreshing their local `compose.release.yaml`, can actually cross a
topology change like this; the clear error is a diagnosis, not a fix.
This is a pre-existing, structural property of the whole self-update
mechanism - not something this change
introduced, and not something worth solving here (making self-update
topology-aware is a materially bigger, separate undertaking) - just
worth knowing next time a release adds a new service or network.

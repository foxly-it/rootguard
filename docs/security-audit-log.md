# RootGuard security audit log

The chronological record of every independent security audit run against
this codebase: what each one found, how Claude verified the finding
independently, what was fixed, and how the fix was proven to actually
work (revert-and-confirm-fails, where achievable). Split out of
`docs/project-state.md` on 2026-08-29 - a follow-up review's own
code-compression suggestion, since the combined file had grown past
~3,500 lines and become a hard-to-review event journal mixed in with the
architecture/feature-state summary a session actually needs first. See
`docs/project-state.md` for that, and `docs/release-history.md` for
version-by-version release notes.

## Full security audit against 60dc447 (2026-08-28)

User ran a complete independent audit of the codebase at commit `60dc447`
(the RC.1 tree plus the site-polish work above). Found 3 release-blocking
issues, 7 medium findings, and several minor ones - Claude verified each
high-severity finding directly against the code before agreeing, all
confirmed accurate. User chose to work the full list in priority order,
one issue/PR each, same discipline as always.

**Blocker 1, fixed: attestation was never enforced on activation, only
displayed.** `stack.CheckStackAttestations` was wired into the stack
status API only (`routes.go`'s `stackStatusHandler`/`servicesHandler`),
never into either updater's actual `update()` path - a pulled,
digest-resolved image was activated (`selectImage`/`writeOverride` +
`compose up`) the moment its post-swap health check passed, with no
attestation check anywhere in between. Directly contradicted
`docs/threat-model.md`'s explicit claim that releases are "checked via
Cosign against the signed SLSA provenance before activation" - a real gap
between documented and actual behavior, not just a missing feature.

Fixed in both updaters:
- `rootguard-core/internal/stack/attestation.go`: new exported
  `RequireAttestation(ctx, service, image) error`, wrapping the existing
  `verifyReleaseAttestation` - no-ops for a service with no RootGuard
  signing policy (AdGuard, a third-party image), demands literally
  `"verified"` and nothing else (not `"missing"`, `"failed"`,
  `"unavailable"`, or an unexpected `"not_applicable"`) for every other
  service.
- `rootguard-core/internal/updater/manager.go`: new injectable
  `Options.AttestationVerifier` (defaults to `stack.RequireAttestation`),
  called right before `selectImage`/`composeUp` - the actual point of no
  return, not just logged for display.
- `rootguard-updater/main.go` + new `attestation.go`: a separate Go
  module, can't import Core's internal package, so it carries its own
  minimal standalone cosign-verify-attestation call (core/webapp are the
  only two services it ever manages, both always requiring verification -
  no AdGuard-style exemption needed here). Injectable
  `manager.attestationVerifier`, defaults to the real `verifyAttestation`.
- `rootguard-updater/Dockerfile`: didn't carry the `cosign` binary at all
  before this - added the same digest-pinned
  `ghcr.io/sigstore/cosign/cosign:v3.0.6@sha256:de9c65...` COPY step Core's
  Dockerfile already uses. Built for real on the `.7` test host (no local
  Docker daemon) to confirm the multi-stage COPY actually works and
  `cosign version` runs inside the image - it does.

Regression tests in both modules prove the gate has teeth: a failing
verifier must stop the update before any `compose`/activation command
runs at all, not just fail afterward - verified by temporarily reverting
each fix and confirming the new test fails without it, then restoring.
Existing tests that exercise a full successful `update()` needed an
explicit no-op `AttestationVerifier` injected (they use fixture image
names that don't match a real `ghcr.io/foxly-it/...` prefix, which the
real, now-default verifier correctly refuses as `"not_applicable"` -
fail-closed, working as intended, just not what those particular tests
were testing).

**Found by PR #365's own CI, not by local testing**: `ci-updater.yml`'s
real Docker E2E integration test (`integration/run.sh`, a genuinely
separate thing from the Go unit tests above - builds real local fixture
images and runs the real compiled binary through a real paired
core/webapp update+rollback) failed the same way, for the same reason -
its fixture images (`rootguard-e2e-core:new`, no `ghcr.io/foxly-it/...`
prefix, and with `ROOTGUARD_UPDATER_SKIP_PULL=true` already set, never
even digest-qualified) correctly fail the new attestation gate every
time, so the update can never reach `idle`. Fixed with the same kind of
test-only escape hatch `ROOTGUARD_UPDATER_SKIP_PULL` already is -
`ROOTGUARD_UPDATER_SKIP_ATTESTATION`, read once in `main()`, set only in
`integration/compose.e2e.yaml`. Documented plainly in the code that this
one is a real security control, not just a convenience skip like
`SKIP_PULL` - anything setting it in production loses the whole point of
this fix. Verified the exact env-var-driven override logic in isolation
(a standalone Go snippet mirroring `main.go`'s wiring, run with and
without the variable set) since the shared `.7` test host's own
long-running containers made a real `integration/run.sh` pass there
unsafe - fixed `container_name:` values in `compose.e2e.yaml` would have
collided with (and risked disrupting) an unrelated 3-day-old deployment
already running there.

**Blocker 2, fixed: releases became publicly visible before smoke-test/
upgrade-test passed.** `update-alpha-pins` (pin commit + moved release tag)
ran with `needs: [version, publish]` - a sibling of, not gated behind,
`smoke-test`/`upgrade-test`. The GitHub Release itself was created even
earlier, by `release-version-bump.yml`, before `release-alpha.yml` had even
started. A failing `upgrade-test` (which has happened live, for exactly
this reason, during the `1.0.0-rc.1` cut) still left the final image tags,
a real GitHub Release, committed pins on `main`, and a moved release tag
all publicly visible - and since Core's/`install.sh`'s live release
discovery both query the Releases API (not raw git tags), a
published-but-broken Release was immediately eligible for auto-discovery
by any existing installation checking for updates.

Fixed: `release-version-bump.yml` no longer creates the GitHub Release (it
still tags + pushes the version-bump commit itself, a low-risk internal
history marker, not a publicly promoted artifact). `update-alpha-pins` now
`needs: [version, publish, smoke-test, upgrade-test]` and creates the
GitHub Release itself as its last step, after the tag has already been
moved to the correctly-pinned commit - notes are regenerated with
git-cliff right there (deterministic from commit history, so identical to
what the other workflow would have produced) rather than passed across the
two separate workflow runs. Both the tag-move and the new Release-creation
step stay idempotent, matching the rest of this job, since a re-run against
an already-published version (exactly what happened live for `1.0.0-rc.1`)
must not fail just because an earlier run already did the work.

Also closed the two smaller gaps the same review flagged in this job:
`docker compose -f compose.release.yaml config` (plus the same
digest-pinned/no-`:latest` assertions `ci.yml`'s own PR check already
runs) now validates the pin-sed's output before it's committed - the
`[skip ci]` pin commit's own comment already documented a *real* past
incident where a broken sed match silently shipped invalid YAML because
nothing re-checked it. And the release pipeline's own `test` job gained
`go vet` (all three Go modules) and `npm test` (the frontend unit suite,
previously not run there at all, only by `ci-webapp.yml`'s own PR checks -
a release cut has no PR).

Deliberately deferred, not folded into this fix: making the release
pipeline depend on `ci-security.yml`'s scans (trivy/gitleaks/govulncheck/
staticcheck) passing. They already run on every push to main via that
separate workflow, but - found while investigating this - `main` isn't
actually a GitHub-protected branch (`gh api .../branches/main/protection`
returns 404), so that's convention, not an enforced gate. Wiring a
security-scan dependency into the release pipeline for real means turning
`ci-security.yml` into a reusable `workflow_call` workflow, a larger,
separate change; flagged to the user rather than rushed into this PR.

**Follow-up (2026-08-29):** done - see the "release gate" entry further
down under "Follow-up review, round 2" for the actual `workflow_call`
conversion and the new `security` job in `release-alpha.yml` that
depends on it.

**Blocker 3, fixed: the one-line installer ran an unpinned, unverified
script as root.** `install.sh`'s `install_docker()` downloaded
`dockerinstall.sh` from `foxly-it/dockerinstall`'s moving `main` branch
and immediately ran it via `sudo`, with no digest, signature, or checksum
check at all - a compromised commit to that separate repository would
have been live on every fresh RootGuard install within minutes. That repo
has no tags or releases to pin to instead (confirmed via the GitHub API),
so there's no "latest stable version" to point at.

Fixed with the two-part guarantee the review itself recommended: pinned
`DOCKERINSTALL_URL` to a specific commit SHA (fixes *which* content is
expected - a real git commit's content is immutable, unlike a branch tip)
plus a hardcoded `DOCKERINSTALL_SHA256`, checked against the actual
download before it's ever `chmod +x`'d or executed (the part that
actually verifies what was *received* still matches that commit - a
commit-pinned URL alone doesn't protect against a compromised CDN edge,
DNS spoofing, or a tampered mirror serving something else at the same
URL). Verified both directions live against the real pinned URL: the
real download's checksum matches, and a deliberately wrong expected value
is correctly rejected. Documented directly in the script how to
intentionally update the pin later (bump the commit SHA, recompute the
checksum from that exact URL).

## Medium findings from the same audit

**Medium 1, fixed (the digest-pinning half): blockpage wasn't
reproducibly bound to a release.** `release-alpha.yml`'s publish job
explicitly excluded blockpage from digest capture
(`if: matrix.component != 'blockpage'`), and neither
`compose.release.yaml` nor `.env.release.example` referenced
`ROOTGUARD_BLOCKPAGE_IMAGE` at all - Core's own Go default
(`ghcr.io/foxly-it/rootguard-blockpage:latest`) was always what actually
ran. A fresh install could get a blockpage image from an entirely
different commit than the rest of the release, with no digest pin and no
attestation ever checked for it (unlike core/webapp/unbound/updater,
which already all have real signing policies in
`attestationPolicies["blockpage"]` - that policy existed already, it was
just never reachable because nothing pinned a digest-qualified reference
to check it against).

Fixed: `release-alpha.yml` now captures blockpage's digest like every
other component, `rootguard-blockpage` was added to the pin-rewrite loop,
and both `compose.release.yaml` and `.env.release.example` now pin
`ROOTGUARD_BLOCKPAGE_IMAGE` explicitly (with today's real digest,
verified live via `docker buildx imagetools inspect`). The `docker
compose config` digest-pinned assertion (both the copy in `ci.yml` and
the one added to `release-alpha.yml` for the earlier release-gate fix)
now checks it too, so the two can't drift apart.

**Deliberately deferred, not folded into this fix** (two related but
separable pieces the same finding raised):
- Full dashboard/attestation-status visibility for blockpage
  (`StackStatus` has no `Blockpage` field at all yet - adding one means
  wiring container inspection, health, and attestation status through
  the API and frontend end-to-end, a materially bigger change than
  closing the actual "unpinned mutable image" security gap above).
- The architectural fix for blockpage holding a reversible (base64,
  trivially decodable) copy of the real AdGuard admin credentials
  (`publishBlockpageAuthToken` in `rootguard-core/internal/adguard/
  manager.go`) - the review's own recommendation frames this as a
  longer-term redesign (a narrow Core-side endpoint the blockpage
  queries instead of ever holding real credentials at all), not a
  same-size fix as the others in this list.
  **Follow-up (2026-08-29): done** - see the "blockpage no longer holds
  any AdGuard credential" entry further down under "Follow-up review,
  round 2" for the actual Core-side endpoint and service-token redesign.
**Medium 2, fixed: expert config could disable DNSSEC entirely.**
`blockedDirectives` in `rootguard-core/internal/unbound/custom.go`
blocked `val-permissive-mode` and the trust-anchor directives, but not
`domain-insecure` at all - a bare `domain-insecure: "."` disables DNSSEC
validation for the whole namespace, not just one zone. Separately,
`harden-dnssec-stripped: no` (accepting a stripped-DNSSEC-data attack
instead of treating it as an error) only produced a soft "warning"
advisory, lumped in with cosmetic settings like `hide-identity` - a user
could activate it anyway. Both directly contradicted `docs.html`'s own
claim (written earlier this same session) that the expert editor
"blocks... DNSSEC bypasses".

Fixed: `domain-insecure` added to `blockedDirectives` (doesn't affect the
guided private-domain/reverse-DNS feature, which renders this directive
itself from validated Go code - a separate path from this free-text
editor entirely); `harden-dnssec-stripped: no` specifically (not the key
unconditionally - `: yes`, the recommended default, must stay accepted)
is now a hard `normalizeCustom` rejection instead of an `adviseCustom`
warning, removed from that warning's case since it's now unreachable
there. New regression test proves both refusals have teeth (reverted
each, confirmed the test fails, restored) and that `harden-dnssec-
stripped: yes` still passes.

**Medium 3, fixed: logout could be silently undone by a restart.**
`handleLogout` only cleared the browser's session cookie *after*
`persistLocked` succeeded - a persist failure returned a 500 with the
cookie still live, even though the comment right there claimed it was
"already gone" (false: the delete-cookie call was below the early
return, unreached). If a stale `sessions.json` then revived that exact
session on the next restart, the browser would still hold a valid
cookie for it, undoing a logout the user had already been told
succeeded. `handleRevokeSession` had the same durability gap for an
*admin*-revoked foreign session (no cookie involved there, but the same
"in-memory revoked, not-yet-persisted, restart brings it back" window).

Fixed: the cookie-clearing call in `handleLogout` now runs
unconditionally, before the persist attempt - the browser-side effect of
logout is durable regardless of what happens to the server-side record.
`handleRevokeSession` gained a bounded retry (3 attempts, 50ms apart)
around its persist call, closing the common transient-I/O-hiccup case;
a permanently broken write still can't be made durable by retrying, so
it still surfaces as an error rather than a silent success, and the
revoked session still stays removed from memory immediately either way.
Two new regression tests (a real broken-path filesystem trick: point
`persistencePath` through a file that was just written, so `MkdirAll`
reliably fails on any OS) prove: logout's cookie is cleared even when
persistence fails (reverted, confirmed the test fails, restored), and a
broken-persist revoke still returns a clear error while the session
stays gone from memory.

**Medium 4, fixed: a corrupted credentials.json silently reactivated the
old env password.** `loadCredentials` treated every failure identically
- a genuinely missing file, an unreadable one, corrupt JSON, an
unexpected algorithm, a malformed salt/hash - as a silent no-op that
left the env-configured initial username/password active. Correct for a
fresh install that never changed its password (file genuinely absent),
wrong for real corruption: an admin who'd deliberately changed the
password in the UI would have that change silently, invisibly reverted
to the original env password on the next restart - a real risk if that
original password was ever exposed (an old `.env` backup, a log,
whoever was present during initial setup).

Fixed: `loadCredentials` now returns an error, distinguishing
`os.ErrNotExist` (the one legitimate silent case) from every other
failure (returns a real error). `NewSessionAuth` panics on that error
the same way it already does for a password-hashing init failure -
consistent with this constructor's own existing "fail loudly at
startup" convention, not a new one. Two of this package's existing
tests deliberately made `credentials.json` a directory *before*
constructing `SessionAuth` (to force a later *persist* failure) - moved
that setup to *after* construction in both, since it would otherwise now
correctly panic during construction itself (a directory where a file
should be is exactly the kind of corruption this fix is meant to catch).
New regression test proves the panic has teeth (reverted, confirmed the
test fails, restored) alongside a companion positive test confirming a
genuinely missing file still doesn't panic.

**Medium 5, fixed: the destructive-action rate limit was bypassable under
concurrency and shared across an account's sessions.** `guardDestructive`
had the same `blocked()`-then-`recordFailure()` TOCTOU gap already fixed
for login/recovery this session - many truly concurrent requests could
all observe zero recorded uses and all be admitted before any got
counted. Separately, it keyed the limiter by *username*, so every session
the same admin account happens to have open (session inventory
explicitly allows more than one) shared a single combined budget -
directly contradicting the limiter's own documented purpose ("bound how
much a single... session can do", right there in its own construction in
`NewSessionAuth`).

Fixed: `guardDestructive` now uses `beginAttempt`/`endAttempt` (every
attempt counts, matching its pre-existing "bound request volume, not
repeated wrong guesses" semantics), keyed by a new `authenticatedSessionID`
helper (the session's own opaque `.ID`, already designed to be safe to
hand out, not the bearer token itself) instead of username, falling back
to the IP-based key only when there's genuinely no session. New
per-session regression test proves the keying fix has teeth (reverted,
confirmed the test fails with a 429 that should have succeeded,
restored). The concurrency test is honest about its own limits: unlike
login/recovery's PBKDF2-widened window, there's no expensive work between
the old two calls here, so Go's scheduler rarely interleaves into that
gap in a synchronous test even at high goroutine counts - confirmed the
race is real a different way instead (a throwaway direct probe against
the raw rate limiter, bypassing the HTTP layer, showed extra accepted
attempts in roughly 1 of 3 runs).

**Medium 6, fixed: Setup preflight didn't check the blockpage's port 80.**
`Preflight` only ever checked the DNS port; a host with something already
bound to :80 (a common case - many hosts run a web server) was reported
"ready", then failed only later, during deployment, when the blockpage
container's own port publish (`composeDNSFile`, gated on
`config.BlockpageEnabled`) collided with it.

Fixed: `Preflight` now runs the same `occupiedDockerPort`/
`probeHostPortBusy` pair it already uses for the DNS port against port 80
too, gated on `config.BlockpageEnabled` (the same flag that decides
whether the blockpage - and therefore that port publish - exists at
all), with its own `blockpage_port_occupied`/`blockpage_port_available`
check codes and a blockpage-specific action message. Two new regression
tests: one proves the occupied-port case fails preflight (reverted,
confirmed the test fails, restored), the other proves a disabled
blockpage doesn't fail preflight just because something else happens to
hold port 80.

**Medium 7, fixed: the WebApp sent no browser security headers.** Every
response - the SPA, the API, the proxied AdGuard UI - carried no
`Content-Security-Policy`, `X-Content-Type-Options`, `X-Frame-Options`,
`Referrer-Policy`, or `Permissions-Policy`. None of this is the primary
defense against anything (that's `RequireSameOriginWrites` and
`SessionAuth`), but its absence meant a same-origin script-injection bug
anywhere else in the app would have had no browser-side backstop, and the
admin UI could be framed by another site.

Fixed: a new `SecurityHeaders` middleware, wrapped outermost around the
whole router in `main.go`, sets all five. The CSP's `script-src` is
same-origin plus a hash for the one inline script the SPA actually has
(`frontend/index.html`'s theme-flash-prevention script, which can't be
externalized without reintroducing the flash it exists to avoid) -
locked down, since script execution is the directive that actually
matters for XSS. `style-src` keeps `'unsafe-inline'` deliberately: a
handful of components apply the React `style` prop, which CSS-in-JS
research shows browsers don't reliably attribute to CSP's HTML-attribute
style-src restriction the same way across versions - not worth risking a
silent rendering break in the field to tighten a directive that isn't
the injection-relevant one. The CSP is withheld specifically for
`/adguard-ui/` (AdGuard Home's own reverse-proxied admin UI, a separate
frontend whose asset layout this app doesn't control); the other four
headers still apply there.

Three new tests: the headers are actually set (and withheld for
`/adguard-ui/`, both revert-verified - reverted to a no-op middleware,
confirmed both tests fail, restored), plus a drift guard that hashes the
real `frontend/index.html` inline script at test time and fails if it no
longer matches the hardcoded CSP hash, so a future edit to that script
can't silently start breaking under the new CSP.

Not verified in a live browser (no local Docker daemon, same constraint
as the rest of this session) - worth a manual DevTools-console check for
CSP violations on the actual built SPA before the next release.

**Medium 8, fixed (audit rated it "gering"/low): router-import allowed
SSRF via redirects and arbitrary addresses.** `FritzBoxClient` only ever
gave `address` a syntax check (`normalizeRouterAddress`); it was never
restricted to plausible router locations, and its `http.Client` followed
redirects with its default behavior. An admin (or anything able to drive
this endpoint through them) could point host discovery at any reachable
address, public or otherwise, or have a redirect steer the request - with
its Digest `Authorization` header intact on a same-host redirect -
somewhere else entirely.

Fixed with the two changes the audit itself suggested: `CheckRedirect`
now refuses every redirect outright, and a custom `DialContext` checks
the *actually-dialed* remote IP - not the pre-resolution hostname, which
closes a DNS-rebinding gap a string-only check would leave open - against
private/link-local/loopback ranges only, rejecting anything globally
routable. Five new tests: the happy path still works through both new
guards (a real `httptest.Server` on loopback), redirect rejection is
revert-verified against a real HTTP redirect, and the address-range logic
is covered directly (a table test on the pure range-check function, and a
dial-level test using a fake `net.Conn` with an arbitrary `RemoteAddr` so
rejecting a public IP doesn't depend on outbound network access in CI) -
both of those also revert-verified.

**Medium 9, fixed: GitHub Actions supply-chain and permission hygiene.**
Every third-party action was pinned by a mutable major-version tag
(`@v7`, `@v4`, ...) rather than a commit SHA; two scanner installs
(gitleaks, trivy) downloaded a binary over HTTPS with no checksum
verification, and trivy's installer additionally came from the `main`
branch of a script - a genuinely moving target, re-fetched fresh (and
potentially different) on every run; `govulncheck`/`staticcheck` ran via
`@latest`, so a new upstream release could change CI behavior with zero
review; and several workflows granted `packages: write`/`id-token: write`/
`attestations: write` at the workflow level, so their test-only jobs
inherited registry-write and OIDC-attestation capabilities they never use.

Fixed: every `uses:` across all 15 workflow files now pins a full commit
SHA with the original tag kept as a trailing comment. `govulncheck` and
`staticcheck` are pinned to specific versions (`v1.7.0`/`v0.8.1`, the
versions already in use) instead of `@latest` - `go run pkg@version` also
gets Go's own module checksum verification (GOSUMDB) for free, unlike a
raw download. gitleaks and trivy are now installed by downloading the
exact release asset directly (bypassing trivy's third-party install
script entirely) and verifying its sha256 against a hardcoded hash before
extracting; git-cliff's existing download (both release workflows) gained
the same checksum check. Elevated permissions moved from workflow-level
to job-level in `ci-unbound.yml`, `ci-updater.yml`, `ci-webapp.yml`, and
`release-alpha.yml`'s `publish` job (the only jobs that actually push
images or attest), with `smoke-test`/`upgrade-test` getting `packages:
read` (they pull published images but never push). `ci-blockpage.yml` and
`pages.yml` are single-job workflows where the one job both builds and
(conditionally) publishes - permission scoping doesn't help there without
splitting into a build/publish pair with an image handoff between them, a
larger restructure left for later since the practical exposure is low
(fork-PR runs already get a read-only token from GitHub regardless of
what a workflow declares, and same-repo runs push in that same job
anyway).

**Minor 1, fixed: backup-restore's entry-limit error message overclaimed
what was found.** The guard rejects at `count >= MaxFiles`, i.e. as soon
as `MaxFiles` entries have already been read successfully - without ever
checking whether a real `MaxFiles+1`th entry exists (that's correct: see
the surrounding comment for why `>=` and not `>`, from an earlier fix
this session). But its message said "backup contains more than %d
entries", which isn't always true at that point - an archive with
*exactly* `MaxFiles` entries and no more triggers the same rejection with
the same overclaiming message. Reworded to "backup contains too many
entries (limit: %d)" - accurate regardless of which case triggered it.
New regression test builds an archive with exactly `MaxFiles` (100000)
entries and checks both that it's still rejected and that the message no
longer says "more than" (revert-verified).

**Minor 2, fixed: the AdGuard UI proxy's Set-Cookie rewrite was a fragile
string match.** `rewriteAdGuardUIResponse` repoints AdGuard's root-scoped
session cookie (`Path=/`) to this proxy's own mount point
(`/adguard-ui/`), so the browser keeps sending it back on later requests.
It did that with `strings.Replace(cookie, "Path=/;", ...)`, which missed
every one of: `Path=/` as the *last* attribute (no trailing semicolon - a
completely ordinary, spec-legal shape), different attribute-name casing
(`path=/`), and extra whitespace around the attribute.

Fixed: `rewriteAdGuardSetCookie` now parses the cookie properly via
`http.ParseSetCookie` (net/http already implements RFC 6265 for this)
and only touches the `Path` field, instead of substring-matching the raw
header text - which also means every other attribute (`Secure`,
`HttpOnly`, `SameSite`, `Expires`, ...) survives untouched regardless of
where `Path=/` appears in the string. A cookie whose path isn't exactly
`/`, or that fails to parse at all, is passed through unchanged - the
same scope the old code had, just reliably. Two new tests cover the
realistic variance the audit named (all revert-verified against the old
implementation) and confirm the "leave it alone" cases still work.

**Minor 3, fixed: the blockpage used innerHTML with location.hostname
unnecessarily.** The block-reason renderer built its "lead" paragraph via
`element.innerHTML = "...<strong>" + location.hostname + "</strong>..."`.
`location.hostname` can't practically carry HTML-special characters (DNS
hostname syntax and browser URL parsing both rule that out), so the
audit rated this low practical exploitability - but it's still needless:
nothing about this line needs raw HTML construction.

Fixed: rebuilt via `textContent`/`createElement`/`appendChild`, the same
pattern this file already uses two lines above for the headline's
reason span - no `innerHTML` anywhere in the reason-rendering path
anymore. (The one other `innerHTML` use in this component, `theme.js`'s
icon swap, stays as-is: it only ever assigns one of three hardcoded SVG
strings keyed by an internal `mode` value, never anything
request-derived.) No automated test covers this specific rendering path
- this component has no JS test harness at all (a static
Node.js-syntax-checked, curl-smoke-tested-only script), and building one
for a single low-severity hygiene fix wasn't judged worth the new CI
surface; verified by inspection and by `node --check` for syntax only.

**Minor 4, fixed: install.sh wrote the WebGUI username/password into
.env unquoted.** The awk rewrite that injects the generated tokens and
typed credentials into the downloaded `.env` file printed each value
raw. Docker Compose re-reads that file later and treats an unquoted `#`
as starting a comment - a password containing one would get silently
truncated, corrupting it without any visible error - and a literal
newline as starting a new `KEY=VALUE` line, which a value supplied via
`ROOTGUARD_ADMIN_PASSWORD` in a non-interactive install could exploit to
inject an extra variable into `.env` entirely.

Fixed two ways: every value the awk script writes is now double-quoted,
with any backslash or embedded double-quote in the value itself escaped
first so the quoting can't be broken out of (verified locally against
values containing `#`, embedded quotes, and backslashes - all come out
correctly preserved inside the quotes). Separately, `admin_user`/
`admin_password` are now rejected outright if either contains a literal
newline - not relied on the quoting alone to contain one, since whether
a dotenv-style parser treats a quoted value as continuing past a
newline varies enough across implementations that it wasn't worth
trusting for something that becomes an actual account password. A typed
value can never contain a newline in the first place (`read -r` stops
at the first one); this only matters for the non-interactive,
environment-supplied path.

No CI coverage exists for install.sh's own logic at all (the existing
`clean-install.yml` workflow exercises `compose.release.yaml`/
`.env.release.example` directly, never invokes `install.sh` itself) -
this is a pre-existing gap, not introduced or widened here, and out of
scope for this fix. Verified locally: `bash -n` syntax check, and the
awk pipeline and newline-rejection logic run standalone against a set
of adversarial values (`#`, embedded `"` and `\`, an injected newline).

**Minor 5, fixed: persistence errors were swallowed via `_ = persist...`
in many places.** ~20 call sites across `rootguard-core`'s installer and
updater managers (`_ = m.persistLocked()`) and `rootguard-webapp`'s
session/audit stores (`_ = a.persistLocked()`, `_ = a.persistAuditLocked()`)
discarded the write error entirely. On a full disk or a permissions
problem, an install step, update, cleanup, rollback, session change, or
audit entry could report success while its outcome was never actually
written down - invisible anywhere, not even in the container's own logs.

Fixed at the single choke point each package already had rather than at
every call site: `persistLocked` (both `installer` and `updater` in
`rootguard-core`) now takes an injectable `OnPersistError` hook -
defaults to a no-op, matching this codebase's existing
`CommandRunner`/`BootstrapFunc`-style option pattern - and calls it on
every failure before returning, so all ~15 existing `_ =` call sites get
visibility for free without themselves changing. `cmd/rootguard/main.go`
wires all three manager instances to log it. `rootguard-webapp`'s
`SessionAuth` doesn't have that same options-struct constructor (its
`NewSessionAuth` has a fixed positional signature used at 29+ existing
call sites, mostly in tests - not worth restructuring for this), so
`persistLocked`/`persistAuditLocked` log directly instead, at the one
place each already knows a write failed.

None of this changes what any caller does with the returned error or
what the UI reports on success/failure - it only makes an already-silent
failure mode observable in logs, closing the "invisible even to someone
troubleshooting a full disk" gap the audit described. A full fail-closed
redesign (surfacing every one of these to the UI, refusing to report
success at all) would be a much larger change or a longer-term goal,
matching how the same call was made for logout/session persistence
earlier this session.

Four new regression tests (two per repo, all revert-verified: reverted
each hook wire-up/log call, confirmed the corresponding test fails,
restored) using the same broken-path-through-a-file trick already
established this session (point a path through a file that was just
written, so `MkdirAll` reliably fails on any OS).

**Minor 6, fixed: the release tag's move to a pin-update commit
complicated forensic traceability.** An earlier RC-blocker fix this
session moved the release tag to point at the pin-update commit (the one
that records this release's own image digests in
`compose.release.yaml`/`.env.release.example`), so the documented quick
start resolves to the right files. But the published images are built
*before* that pin commit exists (the pin needs the digests the build
produces - chicken-and-egg), so each image's own
`org.opencontainers.image.revision` label and provenance attestation
reference the earlier build commit, not the tag. Someone tracing a
published image back to source without already knowing this would land
on the wrong commit.

Not fixable by skipping the tag move (that's the RC-blocker this would
revert to) or by building from the pin commit (still impossible for the
same chicken-and-egg reason) - recorded explicitly instead. The release
notes now get a "Build provenance" section naming both commits: the one
the images were actually built from (`github.sha`, matching their labels
and attestations) and the one the tag now points at (the pin-update
commit, one commit ahead). A one-line lookup instead of something to
reverse-engineer from git history.

Workflow-only change with no way to exercise a real release run in CI -
verified locally instead: the YAML parses, the appended-section shell
logic was run standalone against a fixture notes file and produces the
expected output.

**Code compression 1, done: a shared atomic-file writer.** The
audit's own lower-priority code-compression suggestions, worked through
after the full findings list: the write-temp-then-rename pattern was
hand-rolled separately in `installer`, `updater` (twice - one copy in
`manager.go`, another in `backups.go`), `unbound` (twice), and `adguard`
- six call sites within `rootguard-core` alone, and they'd genuinely
drifted: some cleaned up their temp file on a failed rename, some
didn't (a stray `.tmp` left behind on error); one used `json.Marshal`,
another `json.MarshalIndent`.

New `internal/atomicfile` package (`WriteFile`, `WriteJSON`) replaces
all six - only `adguard/manager.go`'s credentials write is left as-is,
since it deliberately delays its rename past an unrelated HTTP call
(a real two-phase-commit shape, not the same pattern). Behavior-
preserving: every existing test in the module passes unchanged, plus
three new tests for the new package itself (one a genuine regression
test - proves the failed-rename cleanup the consolidation fixed for
free, revert-verified).

`rootguard-updater` and `rootguard-webapp` keep their own small,
independent copies - they're separate Go modules from `rootguard-core`
(and from each other), and this codebase already established this
session that sharing code across that boundary means a new shared
module, real machinery for a handful of lines each already keeps
correct on its own.

**Code compression 2, done: split rootguard-updater/main.go (851 lines)
into files by concern.** Everything - HTTP wiring, the manager's whole
state machine, digest/version-resolution helpers, and raw docker-CLI
primitives - lived in one file. Split into `main.go` (156 lines: entry
point, env wiring, secret-strength check), `manager.go` (the `manager`
type, its persisted `status`, and every method - the actual update/
check/rollback state machine), `image.go` (`digestQualify`,
`digestFromPullOutput`, `isOlderReleaseVersion` - the same grouping as
the digest/SemVer logic this session's other compression item already
touched in `rootguard-core`), `http.go` (request/response glue), and
`docker.go` (the two raw `docker`/filesystem primitives) -
`attestation.go` already existed as its own file.

Scoped narrower than the audit's literal "separate packages" wording:
kept everything in `package main` rather than introducing real
sub-packages. This binary has no other consumer (a standalone
control-plane updater, not a library), so sub-packages would mean
export/visibility bookkeeping and import-cycle risk for zero reuse
benefit - multiple files in one package gets the actual readability win
this suggestion was about without that cost.

Purely a reorganization: every function's body is unchanged, byte for
byte, just relocated - confirmed by every existing test passing
unchanged (gofmt/go vet/go build/go test -race/staticcheck all clean),
which requires nothing about behavior or the public shape of any type
to have shifted.

**Code compression 3, done: shared SemVer validation between the two
release workflows.** `release-alpha.yml` and `release-version-bump.yml`
each kept a byte-for-byte identical SemVer 2.0 regex, deliberately - the
existing comment on the second copy said sharing "needs a composite
action" and wasn't worth it. That's not quite right: both jobs already
check out the full repo, so a plain shared shell script works without a
composite action. Worth fixing given the history the same comment
recorded: this exact duplication had already drifted apart for real once
- only `release-alpha.yml` validated a version at all until the second
check was added by hand, so a typo'd override had already produced a
real commit, tag, and GitHub Release before anything caught it.

New `scripts/lib/semver-validate.sh` (`require_semver "$value"`, exits
non-zero with a message on an invalid version) replaces both inline
copies; `release-alpha.yml`'s `version` job gained a checkout step it
didn't need before, purely to read this one file (a small, deliberate
trade - cheap CI overhead for one source of truth on a check with a real
past-incident history). `install.sh`'s own, much larger SemVer
comparator (real precedence comparison, not just format validation) is
untouched - it runs standalone via `curl | bash` with no repo access at
that point, so it can't source this file the same way; already
established this session as a case where duplication is the correct
trade, not an oversight.

Verified locally (no way to exercise a real release run in CI): the
regex's behavior is unchanged (same pattern, byte for byte) - checked
against every case the removed comments themselves named
(`1.0.0-rc.01`, `1foo.2bar.3baz`, `1.0.0+build`, `01.2.3`, plus valid
alpha/beta/rc/bare versions), all matching the prior inline behavior
exactly.

**Code compression 4, done: the last item on the audit's list -
cross-referenced (not merged) the digest-resolution duplication between
Core's updater and the control-plane updater.** `digestQualify` and
`digestFromPullOutput` are near-identical between
`rootguard-core/internal/updater/github_release.go` and
`rootguard-updater/image.go` - the audit's own suggestion was to share
this logic. Deliberately not done: the two are separate Go modules (this
session already established that pattern for the attestation verifier
and, again, for the atomic-file-write consolidation two items back), and
standing up a third shared module for ~30 lines of stable logic that
hasn't meaningfully drifted in practice wasn't judged worth the ongoing
versioning/import overhead.

What actually closes the audit's real concern (silent drift between the
two copies) without that cost: each function's doc comment on both sides
now names the other file explicitly and says to check it too on any
change - previously only loose, non-actionable prose ("a different Go
module, so its own copy") pointed at the duplication's existence at all,
with no precise pointer either way.

With this, all four of the audit's code-compression suggestions are
addressed - three with real consolidation (atomic-file writer, the
updater's file split, shared SemVer validation) and this one with an
explicit, deliberate "not worth a new module" call plus the
cross-referencing that keeps the trade-off honest going forward. Every
finding in the original audit, from the three RC-blockers down to this
lowest-priority suggestion, has now been worked through.

## Follow-up review, round 2 (2026-08-29)

A second, independent review of the round-1 fixes found genuine gaps in
several of them plus a few new issues - worked through the same way as
round 1: verify directly in code, scope the fix, test with real teeth,
document, PR, merge.

**Medium, fixed: the guided setup's first-ever DNS stack deploy skipped
attestation entirely.** `RequireAttestation` was wired into both updater
packages (round 1's first RC-blocker), but never into `installer.Manager`
- so a fresh install's very first Unbound/Blockpage activation, often the
only deployment event most installations ever have, pulled and started
those images with zero attestation check, contradicting
`docs/threat-model.md`'s claim exactly the way the original finding did
for updates.

Fixed with the identical pattern already used for both updater packages:
`installer.Manager` gained an injectable `AttestationVerifier` (defaults
to `stack.RequireAttestation`), called right before `compose up` in both
`deploy()` (fresh install) and `restoreDeploy()` (backup restore) - after
`pull`, at the actual point of no return. AdGuard stays unchecked
(no RootGuard signing policy, same as everywhere else); Blockpage is
checked only when `config.BlockpageEnabled`. `unbound`/`blockpage`
already had real signing policies registered in `stack.attestationPolicies`
from round 1's blockpage-pinning work - this was purely a missing call
site, not a missing policy.

Two new regression tests (`TestDeployRefusesActivationWhenAttestationFails`,
`TestRestoreRefusesActivationWhenAttestationFails`), both revert-verified
- reverting the call in either `deploy()` or `restoreDeploy()` made the
corresponding test hang until Go's test timeout rather than failing
cleanly, itself proof the gate is what unblocks the flow. Also added a
dedicated `attestation_failed` diagnostic code/message (previously an
attestation failure fell into the generic "could not be deployed"
classification, same bucket as an unrelated Docker error) and fixed four
existing tests that now correctly hit the real `stack.RequireAttestation`
default and need the same noop-verifier injection round 1's updater
tests already use.

**Follow-up to the fix above:** this PR's own CI caught what it broke -
`backup-restore.yml`'s "Verify backup export and restore" builds
`rootguard-core` locally from the checkout under test and deploys it via
the real installer path, so the new attestation gate correctly (if
inconveniently, for that test) rejected the unattested local build.
`cmd/rootguard/main.go` gained `ROOTGUARD_SKIP_ATTESTATION`, the same
shape and purpose as `rootguard-updater`'s own `ROOTGUARD_UPDATER_SKIP_ATTESTATION`
- disables every attestation gate this binary enforces (installation
deploy, service updates, updater self-update) uniformly, wired through
`compose.release.yaml` and set only in that one E2E job.
`clean-install.yml` (the other workflow deploying via `compose.release.yaml`)
needed no change - it deploys the real, published, genuinely-attested
images, not a local build, so it's an actual live check of the
attestation chain working end-to-end rather than something that needs
to bypass it.

**Unrelated cleanup, needed to unblock this PR's own CI:** `trivy`
flagged CVE-2026-56854 in `golang.org/x/crypto` (v0.52.0, an indirect
dependency, fixed in 0.55.0) - present in `main` already, not something
this PR introduced, but it fails `main`'s own security-scan gate for any
PR touching `rootguard-core` right now. Bumped via `go get
golang.org/x/crypto@v0.55.0 && go mod tidy` (pulled `golang.org/x/sys`
along with it); `rootguard-updater`/`rootguard-webapp/backend` don't
depend on it at all, so this is scoped to `rootguard-core` alone.

**Medium, fixed: build-time base images weren't digest-pinned.**
`golang:1.26-alpine`/`docker:29-cli` (`rootguard-core`, `rootguard-updater`)
and `node:22-bookworm`/`golang:1.26-bookworm` (`rootguard-webapp`'s two
builder stages) were still tag-only - `rootguard-unbound`,
`rootguard-blockpage`, cosign, and the webapp runtime's own
`gcr.io/distroless` base were already digest-pinned, this closes the
remaining gap. Matters even for a builder stage that never ships in the
final image: it's what compiled the shipped binary, so a repointed tag
(a compromised or re-pushed upstream image) could tamper with the build
toolchain itself without any change to this repo.

Digests resolved live against the real registry (`docker buildx
imagetools inspect <image> --format '{{.Manifest.Digest}}'`) and
independently re-verified a second time before committing, after an
early copy-paste slip in this same change corrupted one digest and was
caught by a `diff` between the two Dockerfiles' identical cosign pins
before it ever reached a commit.

**Medium, fixed: the release pipeline could build from, tag, or attribute
provenance to the wrong commit under a race.** Two distinct races, both
from trusting a mutable git ref re-resolved at a *later* moment than
when the intent to release was actually captured:

1. `release-version-bump.yml` dispatched `release-alpha.yml` with
   `--ref main` and no pinned commit. `github.sha` for that dispatched
   run resolves from whatever `main` is *when its own checkout actually
   runs* - not necessarily the commit `release-version-bump.yml` just
   committed and tagged, if anything else landed on `main` in the
   window between the two. Every job's checkout, the published images'
   `org.opencontainers.image.revision` label and `COMMIT` build-arg, and
   the release notes' own provenance section all inherited this
   ambiguity.
2. `update-alpha-pins`'s "Point the release tag" step re-fetched
   `origin/main`'s tip at that later point, independent of what its own
   preceding "Commit updated pins" step had just pushed - a second,
   narrower race window with the same shape.

Fixed by threading an explicit commit through instead of re-resolving a
ref: `release-version-bump.yml` now pushes commit+tag atomically
(`git push --atomic`, closing the window between the two ref updates
too) and passes the exact resulting SHA as a new `source_sha`
`workflow_dispatch` input; `release-alpha.yml`'s `version` job resolves
`source_ref` once (`inputs.source_sha || github.sha` - a manual dispatch
with no `source_sha` keeps today's behavior) and every other job's
checkout, the image labels, and the release notes all reference that
same value. The tag-move step now uses the exact commit
"Commit updated pins" just pushed (captured as that step's own output)
instead of re-fetching `origin/main`, falling back to the fetch only on
its pre-existing idempotent "nothing to commit" path, where there's no
"just pushed" commit to reference. Both workflows also gained a shared
`concurrency: group: release-pipeline` lock (the audit's own
recommendation) - previously absent entirely, so two overlapping release
attempts could have raced each other's pushes directly, a strictly worse
version of the same problem.

Workflow-only change with no way to exercise a real release run in CI -
verified locally: YAML parses on both files, `git push --atomic` tested
against a real local bare repo (both refs land in one transaction), and
the new conditional shell logic (prefer the captured commit, fall back
to a fresh fetch only when nothing was pushed) checked standalone.

**Medium, fixed: install.sh's double-quoted .env values still let
Compose expand `$` references in them.** Round 1 double-quoted every
value the awk block writes to `.env`, closing the `#`-comment-truncation
and newline-injection gaps - but double-quoted dotenv values still get
`$VAR`/`${VAR}` expansion from Docker Compose's own parser. Reproduced
live with `docker compose config`: a password of
`abc$HOME${MISSING}` came back out with `$HOME` expanded to the invoking
user's real home directory - the actually-running admin password could
silently differ from the one typed in.

Fixed by switching to single quotes (`q()` now just wraps the value in
`'...'`, no escaping needed) - single-quoted dotenv values are fully
literal, no expansion or escape processing at all, confirmed the same
way: `docker compose config` against a single-quoted
`abc$HOME${MISSING}` now returns it completely unchanged. The one
consequence of true literalness: a value can't contain an embedded `'`
at all (no escape mechanism exists in single-quoted dotenv syntax) - so
`admin_user`/`admin_password` are now rejected outright if they contain
one, alongside the existing newline rejection (which also now covers a
bare `\r`, the audit's own additional recommendation). Deliberately not
attempting a quote-concatenation escape trick for `'` (`'\''`-style) -
its correctness would depend on parser behavior across dotenv
implementations this script has no way to verify, unlike the
single-vs-double-quote literalness question, which was checked directly
against the real `docker compose config`.

No regression test added (this script has no test harness - the
existing gap noted in round 1's install.sh fix) - verified instead the
same way round 1's install.sh fix was: the awk pipeline run standalone
against adversarial values, and this time also against a *real*
`docker compose config` invocation (works daemonless, since it's pure
config resolution) proving both the single-quote fix and the
previously-double-quoted bug it fixes are real.

**Medium, fixed: the FritzBox SSRF guard checked the dialed address after
connecting, not before.** `dialPrivateOnlyWith` dialed first and inspected
`conn.RemoteAddr()` afterwards - real protection against the client
speaking TR-064 to a public address, but a genuine `connect(2)` to the
disallowed address still happened before the connection was torn down,
letting a caller distinguish an open port from a closed one on a host
this client should never have touched at all (a classic SSRF port-scan
oracle, even though the actual TR-064 request itself never got sent).

Fixed by moving the check into a `net.Dialer.Control` hook
(`rejectNonPrivateControl`), which the standard library calls after DNS
resolution but before the `connect(2)` syscall - a disallowed address is
now never actually contacted. DNS rebinding is still closed the same way
as before: `Control` receives the concrete, already-resolved `ip:port`
about to be dialed, never the pre-resolution hostname. The old
`dialPrivateOnlyWith` test faked a `net.Conn`'s `RemoteAddr()` to exercise
the post-dial check without real network I/O; the new test calls
`rejectNonPrivateControl` directly with a `nil syscall.RawConn` instead,
since the function never touches it - simpler and no longer needs a fake
connection at all. Revert-verified: reverted the range check to
always-allow, confirmed the public-address test cases fail, restored.

Loopback stays allowed in the production path (the audit's own secondary
recommendation was to restrict it there too, "not only for testability")
- left as-is deliberately: this package's own tests bind to loopback via
`httptest.Server`, and a fully clean test/production split would need
materially more test scaffolding for a client whose blast radius (an
unauthenticated TR-064 host-discovery call) loopback doesn't meaningfully
worsen beyond what the private ranges above it already allow.

**Releasekritisch, fixed: final image tags were published before the
release's own E2E tests ran, and a re-run could silently reuse an image
built from the wrong commit.** Two compounding gaps in
`release-alpha.yml`'s `publish` job:

1. It pushed straight to the FINAL version tag
   (`ghcr.io/.../IMAGE:VERSION`), before `smoke-test`/`upgrade-test` had
   run - so a release whose E2E tests then failed still left a real,
   publicly pullable image behind under that tag.
2. The "does this need building" check keyed on that same final tag's
   mere *existence* - so a re-run after fixing whatever broke the tests
   could not correct it: the tag already "existed", build+attestation
   were silently skipped, and the stale (or, on a re-dispatch against a
   newer commit, actually-wrong-commit) image was reused - while the
   release notes' provenance section still claimed it was built from the
   current `source_ref`. Not theoretical - this happened on a real
   release re-run.

Fixed by publishing to a commit-scoped CANDIDATE tag first
(`VERSION-candidate-<12-char-sha>`, computed once in the `version` job so
every job agrees on it) instead of the final tag - nothing under the
final version tag is ever touched by `publish` itself. `smoke-test` and
`upgrade-test` now run against these candidate images. Only after both
have actually passed does `update-alpha-pins` (already gated on
`needs: [..., smoke-test, upgrade-test]` from an earlier round) run a new
"Promote candidate images to the final release tag" step:
`docker buildx imagetools create` republishes the exact already-tested
manifest under the final tag - not a fresh build, so the promoted image
is byte-for-byte what was actually tested (same digest, same
attestation, same `org.opencontainers.image.revision`) - followed
immediately by re-inspecting the promoted tag's digest and asserting it
matches the recorded one exactly, as defense-in-depth against promoting
from a stale digest file.

This also closes finding #2 as a side effect, not just #1: because the
candidate tag encodes the exact commit, "does this candidate already
exist" and "was this the right commit" collapse into the same question -
a genuinely new commit always gets its own, different candidate tag,
so there's no tag-existence check left that a wrong-commit re-run could
satisfy.

Also wired `gitleaks`/`trivy`/`govulncheck`/`staticcheck` directly into
the release gate (the audit's own additional recommendation) rather than
continuing to trust that `ci-security.yml`'s separate `push: branches:
[main]` trigger happened to run against the right commit at some
earlier, untimed point: `ci-security.yml` gained a `workflow_call`
trigger (with a `ref` input, since a workflow_dispatch release run's
`source_ref` can differ from its ambient `github.sha` - see the earlier
fix for that), and a new `security` job in `release-alpha.yml` calls it
pinned to the release's own `source_ref`, with `publish` now requiring
`needs: [version, test, security]`.

Workflow-only change with no way to exercise a real release run in CI -
verified locally: YAML parses on both files (`yaml.safe_load`), the
`docker buildx imagetools create`/`inspect` command shapes match the
pattern already verified live in an earlier round's base-image pinning
work, and every image-reference expression that needed to move from
`needs.version.outputs.value` to the new `needs.version.outputs.candidate_tag`
was swept via `grep` before and after to confirm none were missed (the
handful of remaining `value` references are all genuinely about the git
tag/GitHub Release name, not a Docker image reference, and were left
alone deliberately).

**Fixed (not deferred - see the correction above): blockpage no longer
holds any AdGuard credential.** Round 1 explicitly deferred this as a
"longer-term redesign"; the user's own correction on this second review
was direct: a mandate to fix a review's findings doesn't leave room for
unilaterally downgrading one to "document only" because it looks bigger
than the others, even when the review itself calls it a long-term
recommendation. Implemented as originally scoped:

`publishBlockpageAuthToken` (`base64(adguard_username+":"+adguard_password)`,
written to the volume shared with the blockpage container) is gone.
`Manager.publishBlockpageServiceToken` now writes a 32-byte random,
AdGuard-unrelated token there instead, and a new `GET
/api/blockpage/reason` endpoint in `rootguard-core/internal/api/routes.go`
performs the actual AdGuard `check_host` call server-side - only Core
ever holds the real AdGuard credentials now, and only returns the block
`reason` string (the only field blockpage's own frontend, `meta.js`,
actually reads). Registered on the root mux (not the bearer-token-gated
`apiMux` subtree) with its own, much narrower auth check
(`Manager.VerifyBlockpageServiceToken`, constant-time-compared) - putting
it behind Core's own admin token (shared with the WebApp and the updater)
would have been almost as bad as the problem being fixed. Host input is
validated against an RFC-1123-style hostname pattern
(`ErrInvalidBlockpageHost`) before ever reaching AdGuard's admin API.

`rootguard-blockpage`'s nginx template and its `19-render-blockpage-conf.sh`
entrypoint script both updated to match: `/api/reason` now proxies to
`http://rootguard-core:8081/api/blockpage/reason?host=$host` with `Authorization:
Bearer ${BLOCKPAGE_SERVICE_TOKEN}`, resolved the same Docker-embedded-DNS
way the old direct-to-AdGuard proxy already was. Reachability confirmed by
reading the actual network topology, not assumed: blockpage only joins the
dynamically-rendered `rootguard-dns` network (see `renderCompose` in
`installer/manager.go`), and Core is already `docker network connect`ed to
that same network at deploy time (for its own DNSSEC-adjacent reasons,
predating this fix) - so `rootguard-core:8081` was already reachable from
there, no new network wiring needed.

Regenerating the token on every `Bootstrap` call (rather than persisting
and reusing one) is intentional, not an oversight: `installer.Manager`
already re-renders blockpage's nginx config and reloads it
(`docker exec rootguard-blockpage sh /docker-entrypoint.d/19-render-blockpage-conf.sh`
+ `nginx -s reload`) immediately after `Bootstrap` returns, in the same
deploy step that existed before this fix - so there's no window where
blockpage would be left holding a stale token.

Tests: `adguard` package - the token is 32 random bytes (64 hex chars),
provably unrelated to the AdGuard credentials, and round-trips through
`VerifyBlockpageServiceToken` (accepts the real one, rejects a tampered
one, rejects empty, fails closed before any `Bootstrap` has ever run);
`ReasonForHost` proxies correctly and rejects malformed hosts. `api`
package - the handler rejects a missing/wrong service token, rejects a
malformed host with a valid token, and - the routing half of the fix,
proven separately - a request bearing the service token (never Core's
admin token) reaches past `requireBearerToken`, revert-verified: removing
the route's own `root.HandleFunc` registration and re-running that exact
test flips it to a 401, confirming the route precedence is what the test
depends on, not something it would pass by accident either way.

**Small, fixed: `atomicfile.WriteFile` wrote to a fixed, predictable temp
name (`path+".tmp"`) via a plain `os.WriteFile`.** Three compounding
gaps in that: not concurrency-safe (two overlapping callers for the same
`path` shared that one temp name, racing each other's writes); `os.WriteFile`
opens-and-truncates an *existing* file rather than refusing one, so a
pre-existing `path+".tmp"` - a stale leftover from an earlier failed run,
or a symlink someone else in the same directory could plant - would be
written through rather than replaced; and since `os.OpenFile` only applies
the requested permission bits when it actually *creates* a file, an
existing leftover at that name silently donated its own (possibly wrong)
mode to every future write, ignoring whatever mode the caller asked for.

Fixed by switching to `os.CreateTemp(dir, ...)` in the target directory:
it always creates a brand-new, uniquely-named file, so there's never an
existing name (symlink or otherwise) to collide with, and this code now
explicitly `Chmod`s it to the requested mode regardless of what any
earlier leftover had. Also added `Sync()` on both the file (before the
rename that makes it visible) and the parent directory (after the rename
- a rename is itself a directory-entry change that needs its own fsync to
survive a crash, which the previous version never did either).

`TestWriteFileIgnoresStaleLegacyTempFile` is the regression test:
pre-plants a stale `path+".tmp"` at world-writable mode `0777`, then
asserts a fresh `WriteFile(path, ..., 0600)` call is completely unaffected
by it - both in the final file's content and in its mode actually being
`0600`, not the leaked `0777`. Revert-verified: reverting to the old
fixed-name implementation makes exactly the mode assertion fail (got
`0777`, wanted `0600`), confirming the test targets the real bug, not
just the file-content half of it.

**Medium, fixed: the DNSSEC-bypass check still had reachable bypasses.**
Round 1's fix for `harden-dnssec-stripped: no` matched a literal `": no"`
suffix against the raw config line - real, but still bypassable with any
of `harden-dnssec-stripped:    no` (extra internal whitespace),
`harden-dnssec-stripped: no # allow it` (a trailing comment - the *key*
extraction already stripped comments, the value check never did), or
`harden-dnssec-stripped:no` (no space after the colon) - all ordinary,
spec-legal Unbound config shapes.

Fixed by parsing the value the same principled way the key was already
parsed: new `directiveValue` (comment-stripped, colon-split, trimmed -
mirrors the existing `directiveKey`) replaces the raw-line suffix match,
compared via `strings.EqualFold` instead of a pre-lowercased literal
match. Four new cases added to the existing
`TestCustomConfigRejectsDNSSECBypasses` (the three real bypasses plus a
non-regression casing case, kept as a companion now that the value
comparison is its own explicit step rather than inherited from a
whole-line lowercase) - revert-verified: reverted to the old suffix
match, confirmed the whitespace case fails, restored.

**Small, fixed: an archive with exactly `MaxFiles` entries was still
rejected.** Round 1 fixed this same off-by-one's *message* (it used to
claim "more than %d entries", not always true at that point) but not the
underlying semantics: the entry-count guard checked `count` - starting at
0, incremented per loop iteration before `archive.Next()` was even called
- against `MaxFiles` *before* reading each entry. Once exactly `MaxFiles`
entries had already been read successfully, the guard fired on the next
iteration without ever looking far enough to confirm a genuine
`MaxFiles+1`th entry actually exists - rejecting an archive precisely at
the limit, not over it.

Fixed by restructuring the loop to count (and check) only after a real
entry has actually been read: `count++` moved to immediately after a
successful `archive.Next()`, with the guard now `count > MaxFiles` -
`MaxFiles` entries read is no longer over the limit, only the
genuine `MaxFiles+1`th read trips it.

`TestExtractAcceptsExactlyTheEntryLimit` builds an archive with exactly
`MaxFiles` entries and confirms the entry-count guard no longer rejects
it (deliberately narrower than "the whole restore succeeds" - a
100000-entry archive's manifest would need matching hashes for every one
of them, unrelated overhead for what this specifically targets; whatever
Extract fails on afterward, here the always-expected missing manifest, is
a separately-covered path). `TestExtractRejectsEntryCountOverTheLimit`
is the counterpart: one entry over the limit is still correctly rejected,
with the message a prior round already corrected staying accurate.

**Small, fixed: the AdGuard UI cookie rewrite silently dropped unknown
`Set-Cookie` attributes.** `rewriteAdGuardSetCookie` parses each cookie
with `http.ParseSetCookie`, rewrites `Path`, and re-serializes with
`Cookie.String()`. Verified live: `Cookie.Unparsed` holds any
attribute-value pair `ParseSetCookie` couldn't map to one of its own
known fields (a vendor-specific or not-yet-standard attribute - a
`FutureAttr=xyz` pair lands there in practice), but `Cookie.String()`
never serializes it back - so the rewrite was silently dropping anything
it didn't specifically recognize, not just leaving it untouched.

Fixed by appending `cookie.Unparsed` back onto the serialized result
after `Cookie.String()` runs - order doesn't matter for Set-Cookie
attributes (RFC 6265), so this round-trips correctly regardless of where
the unknown attribute originally sat.

`TestRewriteAdGuardSetCookieKeepsUnknownAttributes` is the regression
test: a cookie carrying a `FutureAttr=xyz` pair must still carry it after
the rewrite. Revert-verified: removing the `Unparsed` re-append makes the
rewritten cookie silently lose it, exactly reproducing the finding.

**Small, fixed: the WebApp's audit log and session inventory trusted a
freely-settable `X-Forwarded-For` header.** Rate limiting already used
the real TCP peer address (`rateLimitKey`, fixed in an earlier round) -
but `clientAddress`, used for the audit log and session inventory shown
to an operator, still trusted `X-Forwarded-For` unconditionally. Not just
a display quirk: an operator reviewing "who logged in from where" for
incident response would see attacker-controlled garbage instead of the
real peer address, since anyone who can reach this container at all could
set an arbitrary value and have it recorded as their own session's
address. RootGuard's own documented reverse-proxy setups
(`docs/https-reverse-proxy.md`) never ask an operator to forward
`X-Forwarded-For` either - only `X-Forwarded-Proto` - so there's no
legitimate deployment this header could be trustworthy in.

Fixed by making `clientAddress` delegate to `rateLimitKey` directly -
kept as its own named function (rather than every caller switching to
`rateLimitKey`) purely to preserve the distinct display-vs-access-control
intent already documented at each call site.

`TestAuditLogIgnoresSpoofedForwardedForHeader` is the regression test: a
login with a spoofed `X-Forwarded-For` must still record the real peer
address in every resulting audit event. Revert-verified: reverting
`clientAddress` to trust the header again makes the audit log record the
spoofed value instead.

**Small, fixed: `theme.js` still had an `innerHTML` sink.** The blockpage
theme toggle set `btn.innerHTML = icons[mode]`, where `icons` is a
hardcoded, developer-written object of three fixed SVG strings - not
exploitable today, since nothing attacker-influenced ever reaches it, but
the audit's point stands: a sink that happens to be safe today is still a
sink, and removing it is cheap here.

Fixed by building each icon as real DOM nodes
(`document.createElementNS`/`setAttribute`) instead of an HTML string, and
swapping the button's child via `removeChild`/`appendChild` instead of
`innerHTML`. Applied identically to both copies of this file -
`rootguard-blockpage/web/theme.js` (the real blockpage) and
`rootguard-webapp/frontend/public/blockpage-preview/theme.js` (the
WebApp's live preview of it, a manually-kept-in-sync copy, confirmed
byte-identical to the original before and after this change).

No existing test harness for this file (no Playwright/DOM test
infrastructure in this repo yet) - verified instead with a real `jsdom`
instance (already a frontend devDependency): loaded the actual file,
confirmed initialization produces exactly one `<svg>` child, and that
cycling through all three modes (three simulated clicks) toggles
`data-theme` correctly and never leaves more than one child node behind.
`grep` confirms no `.innerHTML =` assignment remains in either file;
`npm run lint` stays clean.

**Small, fixed: the frontend production bundle was a single 578kB chunk
(163kB gzipped), triggering Vite's own &gt;500kB warning.** `App.tsx`
imported all seven authenticated pages (`Overview`, `Unbound`, `AdGuard`,
`Setup`, `Stack`, `Backups`, `Logs`) eagerly at the top - every page's
code shipped on first load regardless of which one, if any, a given
session actually visits.

Fixed by switching those seven to `React.lazy(() => import(...))`,
wrapped in a single `<Suspense>` boundary around the authenticated
`<Routes>` (a small, non-full-viewport spinner as the fallback, styled to
render inside the already-mounted sidebar layout rather than covering
it - full-viewport would flash over the sidebar on every navigation, not
just the first load). `Login` deliberately stays eager: it's the one page
every unauthenticated visit needs immediately, so splitting it would just
trade one round-trip for another on the most universal path; its own
`<Routes>` branch never touches the lazy imports at all, so it needed no
`Suspense` boundary of its own.

Verified with a real production build: the &gt;500kB warning is gone, the
main chunk dropped from 578kB to 426kB (163kB → 129kB gzipped), and each
page now ships as its own separate chunk (5-90kB each) fetched on
navigation instead of upfront. `tsc -b`, `npm run lint`, and the existing
26-test suite all stay clean - none of them render pages through `App.tsx`'s
routing (the one component test, `AdGuard.test.tsx`, imports the page
module directly), so none needed a `Suspense`-aware update.

**Code-compression, revisited and partly implemented: the atomic-write
pattern is now consistently fixed everywhere it's duplicated, not just in
`rootguard-core`.** Round 1 explicitly judged a shared module "not worth
it" for ~40 lines of stable atomic-write logic; the user re-raised this
suggestion in this round's review. Re-examining it turned up something
the compression framing alone wouldn't have: `rootguard-updater`
(`writeAtomic` in `docker.go`) and `rootguard-webapp/backend`
(`internal/httpapi`'s three separate call sites for credentials,
sessions, and the audit log) still had the *exact* old, vulnerable
`path+".tmp"` pattern this round's own `atomicfile.WriteFile` fix (see
above) had just closed in `rootguard-core` alone - not concurrency-safe,
follows an existing file/symlink at that name rather than refusing it,
and silently inherits a stale leftover's permissions instead of applying
the requested mode.

Fixed by porting the same `os.CreateTemp`-based implementation into both
other modules - a small local `writeAtomic`/`writeAtomicFile` function
each, not a new shared Go module: separate Go modules in this repo can't
share an `internal/` package directly (an existing architectural
constraint, not new to this fix), and a real shared module would need
its own versioning/dependency-management overhead for logic that rarely
changes. Round 1's "not worth a new module" call stands - what changed is
that leaving the *duplication* unfixed had also left the *bug* it
inherited unfixed in two more places, which the original compression
framing didn't surface. `rootguard-webapp/backend` additionally
consolidated its own three previously-triplicated copies into one
package-local helper (`atomicfile.go`), so this pass also delivers a
literal reduction in duplicated code where a shared module wasn't
warranted.

Regression tests mirror `rootguard-core`'s own
`TestWriteFileIgnoresStaleLegacyTempFile`: `rootguard-updater`'s
`TestWriteAtomicIgnoresStaleLegacyTempFile` and
`rootguard-webapp/backend`'s `TestWriteAtomicFileIgnoresStaleLegacyTempFile`
both pre-plant a stale `path+".tmp"` at world-writable mode `0777` and
confirm a fresh write is unaffected by it. Revert-verified in both
modules: reverting to the old fixed-name implementation fails exactly
the mode assertion (`0777` leaked through instead of the requested
`0600`) in each.

**Code-compression, done: `rootguard-webapp/Dockerfile` trimmed from 123
to 60 lines.** Decorative box-drawn section headers, German inline
comments, and doubled blank lines between stages made this file a clear
outlier - `rootguard-core/Dockerfile` and `rootguard-updater/Dockerfile`
never adopted that style, keeping comments only where they carry real
information (a security rationale, a non-obvious decision). Trimmed to
match: every substantive comment (the two digest-pinning rationales, the
`ARG`-redeclaration note) is kept verbatim; everything else - headers,
translated filler like "Nur Dependency Files kopieren" or "Sicherheit:
kein root", and the extra blank lines - is gone.

Verified mechanically, not just by eye: every non-comment, non-blank
line (every `FROM`/`ARG`/`COPY`/`RUN`/`WORKDIR`/`LABEL`/`EXPOSE`/`USER`/
`ENTRYPOINT` instruction) diffed byte-for-byte identical between the old
and new file, sorted and compared with `diff` - confirming this is a
pure comment/whitespace trim with zero functional change, not just an
assumption. No local Docker daemon available to do a real build in this
environment; the real build/push verification happens via this PR's own
CI, which builds and pushes the actual multi-arch image from this exact
file.

**Code-compression, partly implemented: added `docs/release-process.md`,
the architecture-level map `release-alpha.yml`'s own inline comments
never tried to be.** The audit's suggestion had two halves: move some of
the workflow's incident-driven explanations out to a doc, and extract its
pin-update/compose-verification/release-notes logic into separate,
independently-testable scripts (the pattern
`scripts/lib/semver-validate.sh`/`scripts/bump-site-versions.sh` already
use elsewhere in this same pipeline).

Did the first half, deliberately scoped narrower than "move the
comments": the workflow's own inline comments stay exactly where they
are - they explain the specific, often incident-driven "why" behind
individual steps, positioned right where a future maintainer would
otherwise be tempted to "simplify" away a hard-won fix, which is the
opposite of what moving them elsewhere would achieve. What was actually
missing was the *overview* those comments don't individually provide -
`docs/release-process.md` is new, additive documentation covering the
trigger/identity flow, the build/test/security gate, the candidate-tag
promotion model, the pin-update/tag-move/Release-creation sequence, and
the upgrade-test rationale - written by reading the current workflow file
directly and cross-checking every specific claim (job names, output
names, the exact `docker buildx imagetools create` invocation, the
`[skip ci]` commit message, the two referenced scripts' actual paths)
against it via `grep`, not from memory.

Deliberately did not attempt the second half (extracting pin-update/
compose-verification/release-notes logic into standalone scripts) in this
pass: a release pipeline is exactly the kind of file where a rushed
refactor risks a real regression for a marginal readability gain,
explicitly noted as a separate, future piece of work in the new doc's own
closing section rather than folded in hastily here.

**Small, fixed: a failed state persist was diagnosable but still
invisible in Status().** Round 1 added `OnPersistError`/`PersistErrorHandler`
so `persistLocked`'s many `_ = m.persistLocked()` call sites (found in
that same review) would at least log a failed write instead of silently
discarding it - explicitly scoped at the time as "not a fix for the
underlying problem, just the difference between invisible and
diagnosable". This round's audit re-flagged the remaining half directly:
`Status()` itself still reported whatever the in-memory state was
(`installed`, an update's `history`, ...) with no indication the on-disk
record backing it might not survive a restart.

Fixed by having `persistLocked` (in both `installer.Manager` and
`updater.Manager` - the same pattern round 1 already used in both)
record the outcome directly on `m.status` itself: `PersistError`/
`PersistErrorAt` are cleared before every write attempt and set only in
the deferred failure branch, so a success always reports (and durably
records) a clean state, and a failure is visible in the very next
`Status()` call - self-healing the moment a later persist succeeds,
without any caller needing to notice or retry anything. Mirrored into
the WebApp's TypeScript `InstallationStatus`/`UpdateStatus` types for
contract completeness; no UI surfacing added yet (out of scope for this
fix - a follow-up if the backend/frontend team wants a visible warning
banner, not a correctness gap on its own).

`TestStatusSurfacesPersistFailureAndSelfHeals` (one per package,
mirroring each other) is the regression test: forces a persist to fail
against a path blocked by a file, confirms `Status()` reports both new
fields, repoints at a real writable directory (the same recovery an
operator fixing a full disk would perform), and confirms the very next
successful persist clears both again. Revert-verified in both packages:
reverting `persistLocked` to the round-1-only logging behavior fails the
test at "expected Status() to report a persist error" in both.

## Live end-to-end verification (2026-08-29)

With every round-2 fix merged, ran the actual RootGuard stack on the
dedicated test LXC (`192.168.178.7`) - fresh images built from `main`
(not CI artifacts), pushed to the host's own local registry, and driven
through a real guided-setup deploy, not simulated. Confirmed live, not
just via unit test:

- **Finding 6 (attestation gate on first deploy)**: the guided-setup
  deploy against unsigned local dev images failed exactly as designed -
  `attestation_failed`, `"release attestation for unbound
  (localhost:5000/rootguard-unbound:dev) is not_applicable, refusing to
  activate"` - before it ever touched the DNS containers.
- **Finding 3 (DNSSEC-bypass parsing)**: all three bypass variants
  (extra whitespace, trailing comment, no space after the colon)
  rejected with the exact "DNSSEC validation must not be weakened"
  message; the legitimate `harden-dnssec-stripped: yes` accepted.
- **Finding 4 (install.sh single-quoting)**: reproduced on this host's
  own BusyBox `awk` (a different implementation than the one used
  during development) - `docker compose config` against a
  `$HOME${MISSING}`-containing value returned it completely literal.
- **Finding 5 (SSRF pre-connect check)**: a FritzBox-discovery request
  against a public IP was refused before any connection attempt; the
  same request against an unreachable private IP got a real "no route
  to host" from the OS network stack, confirming private addresses are
  still allowed to actually attempt a connection (not blanket-blocked).
- **Finding 7 (blockpage credentials)**: the shared token both Core and
  the blockpage container hold is 64 hex characters that fail to
  base64-decode into anything resembling credentials (not
  `base64(user:pass)`); `/api/reason` round-trips correctly through
  Core's new endpoint for both a blocked and an unblocked domain; the
  blockpage container's own attempt to call AdGuard's admin API
  directly with that same token got a real `401 Unauthorized` from
  AdGuard.
- **X-Forwarded-For audit-log fix**: a login with a spoofed
  `X-Forwarded-For: 10.13.37.99` still recorded the real Docker-bridge
  peer address in the audit log, not the spoofed value.
- **theme.js**: the file actually shipped inside the running blockpage
  container has no `.innerHTML =` assignment (`innerHTML` appears only
  in the explanatory comment).
- **Frontend bundle splitting**: the webapp image actually shipped 21
  separate JS chunks; the main chunk was exactly 425,993 bytes (~426kB),
  matching the local development measurement exactly.
- **Persistence across a real restart**: killed and restarted the
  `rootguard-core` container mid-session - installation state and the
  WebApp session both survived, exercising the `atomicfile.WriteFile`
  fix in an actual production code path, not just its unit test.

**Found and fixed live, not from the audit**: the guided-setup deploy
against local dev images failed at the `pull` step before even reaching
attestation - `docker compose pull` refuses a purely local image tag
with no registry to pull from, so testing this required pushing the dev
images to a real registry first (the host's own `local-registry:5000`).
Once past that, `compose.yaml` and `compose.integration.yaml` (the local
development compose files) turned out to never forward
`ROOTGUARD_SKIP_ATTESTATION` to Core at all - only `compose.release.yaml`
did. Every unsigned local build would therefore always fail the
attestation gate with no way to opt out for local development, and
neither of CI's own integration jobs ever caught this: `ci.yml`'s
"validate" job bootstraps AdGuard directly against pre-provisioned
compose services, and `ci-unbound.yml`'s "Guided-settings scenario
tests" job is a Go-level test (`go test -tags integration
./internal/unbound/... -run TestScenario`) - neither ever drives a real
`POST /api/installation/deploy` through `installer.Manager`, so the gate
this fix touches was never actually exercised by CI at all. Fixed by
adding the same `ROOTGUARD_SKIP_ATTESTATION: "${ROOTGUARD_SKIP_ATTESTATION:-false}"`
passthrough compose.release.yaml already had, to both files.

## Follow-up review, round 3 (2026-08-29)

A third independent review, focused specifically on the release pipeline
and DNSSEC enforcement ahead of the RC. Same discipline as rounds 1 and
2: verify directly in code, scope the fix, test with real teeth,
document, PR, merge - one item at a time.

**Critical, fixed: `harden-dnssec-stripped` bypass via a quoted `"no"` or
`'no'`.** Round 2 fixed the literal-suffix-match gap for
`harden-dnssec-stripped: no` (extra whitespace, trailing comments, no
space after the colon) by comparing a properly parsed, comment-stripped,
trimmed value instead of matching the raw line. It still compared that
value's raw text, though - Unbound's own config lexer strips one layer of
matching double or single quotes from a directive value before its
parser ever sees it, so `harden-dnssec-stripped: "no"` and
`harden-dnssec-stripped: 'no'` are both ordinary, spec-legal ways to
write exactly the same disabling value that never equal-folded to the
bare `no` the round-2 check looked for.

Fixed in `rootguard-core/internal/unbound/custom.go`: `directiveValue`
now strips one matching layer of quotes, mirroring Unbound's lexer, before
any comparison happens. The `harden-dnssec-stripped` check itself was
also flipped from a blacklist (`value != "no"`) to a whitelist
(`value == "yes"`) - for this one directive specifically, refusing
anything that isn't unambiguously the safe value is judged safer than
continuing to enumerate every spelling of the unsafe one a future
Unbound-accepted quoting or aliasing might produce. `custom_test.go`
gained regression cases for both quoted-`no` spellings, an ambiguous
non-yes/no value, and quoted-`yes` acceptance (must still be allowed).

**Critical, fixed: the guided setup's first-ever deploy would refuse
activation of a real, correctly signed release image.** Round 2 wired
`stack.RequireAttestation` into `installer.Manager`, gating both
`deploy()` and `restoreDeploy()` right before `compose up`. It called
that gate with `Options.UnboundImage`/`Options.BlockpageImage`
unchanged, though - plain `repo:tag` references, exactly what a release
hands the installer (see `release-alpha.yml`). `RequireAttestation`
requires an explicit `repo@sha256:...` reference and short-circuits to
`not_applicable` - itself a hard refusal, not a skip - for anything else,
without ever invoking cosign. So every real deploy, correctly signed
release included, failed here the same way a forged one would have, just
for an unrelated reason: this would have surfaced as soon as a genuine
end user ran the actual guided setup against a real release, not just in
this review.

Root cause was purely a missing digest-resolution step: `rootguard-updater`
and `internal/updater` both already resolve a freshly pulled image to its
digest before their own attestation check (`digestFromPullOutput`/
`digestQualify`), but `installer.Manager` never had the equivalent.
Fixed with a `resolveDigest`/`resolveAndPinDigests` pair in
`internal/installer/manager.go` - a third by-hand copy of the same
~15-line `docker image inspect` lookup (separate Go modules, and here
also a separate manager with its own `CommandRunner` wiring, can't share
an `internal/` package for it; see `digestQualify`'s own comment for why
a shared module wasn't judged worth it). Called right after `pull`
succeeds and before `create`/`start`: resolves Unbound's (and, when
enabled, Blockpage's) pulled image to its digest, then rewrites the
stack definition to reference that digest instead of the original tag -
so `create`/`up` actually starts the exact image that was attested, not
whatever the tag points at if it moves in between attestation and
activation.

New regression test `TestDeployResolvesDigestBeforeAttestation`
deliberately leaves `AttestationVerifier` unset (defaults to the real
`stack.RequireAttestation`, unlike every existing attestation test here,
which uses a fake verifier that can't catch a bug in what's actually
passed to it) and asserts the resulting failure is a real, failed
attestation attempt - not the `not_applicable` short-circuit that made
every deploy fail closed regardless of whether the image was ever really
signed - plus that the written `compose.yaml` references the resolved
digest, not the original mutable tag. Also updated the existing
`TestWriteComposeSelectsBetaImage` and both call sites in `deploy()`/
`restoreDeploy()` for `writeCompose`'s new explicit
`(unboundImage, blockpageImage string)` parameters (previously read
`m.unboundImage`/`m.blockpageImage` directly, which the digest-resolved
rewrite now needs to override).

**Critical, fixed: a re-run of `release-alpha.yml` could move an
already-published release tag onto untested commits, and always
force-pushed it regardless.** Two compounding bugs in the "Point the
release tag at the pin-update commit" step:

1. `git rev-parse "refs/tags/${tag}"` resolves an *annotated* tag (which
   `git tag -a` always creates here) to the tag object's own hash, not
   the commit it points at - so the step's own "already correct, nothing
   to do" comparison against a target commit hash could never match,
   on any run, ever. Every single run force-moved the tag, whether it
   actually needed to or not. Fixed with `refs/tags/${tag}^{}`, which
   dereferences through the tag object the same way `git rev-parse`
   already does for every other object type.
2. Whenever this run's "Commit updated pins" step found nothing to
   commit (the pins in the checked-out `source_ref` already matched),
   the tag target fell back to `origin/main`'s *current* tip - reasoning
   that held only if main hadn't moved since `source_ref` was fixed. A
   legitimately retried run (e.g. after a later job failed and the whole
   workflow re-ran) re-checks that reasoning against whatever now
   happens to be on main, which can by then include unrelated commits
   this specific release run never built, tested, or security-scanned.
   Confirmed live: a real RC's tag and its own Core image's OCI revision
   label pointed at two different commits because of exactly this.
   Fixed by using `needs.version.outputs.source_ref` (the one commit
   every job in the run actually agrees on) instead, with an explicit
   `git merge-base --is-ancestor` check that aborts the release loudly
   if `source_ref` is no longer reachable from `origin/main`, rather than
   silently tagging whatever main currently points at.

Verified the YAML still parses and every embedded `run:` block in the
whole workflow file still passes `bash -n` (no dedicated shellcheck job
exists in this repo to run instead).

**Critical, fixed: candidate-image promotion trusted the candidate
blindly and could silently overwrite an already-published version
tag.** `update-alpha-pins`'s "Promote candidate images to the final
release tag" step called `docker buildx imagetools create` for every
image unconditionally, using a digest that - on a retried run where
`publish`'s build/attest steps were skipped because the candidate
already existed - was never re-verified against anything: not that it
was actually built from the commit this release run tested (the
candidate tag only encodes a 12-char commit prefix), not that it
actually carries a valid release attestation, and not whether the final
tag it was about to (re)create already pointed at something else
entirely.

Fixed with three checks added before promotion, run unconditionally for
every image on every run (so "even when the build was skipped" is
automatically covered, not a separate code path to keep in sync):

1. The candidate's `org.opencontainers.image.revision` label (read via
   `docker buildx imagetools inspect --format '{{json .Image}}'`, which
   this session verified live against a real published RootGuard image
   on ghcr.io to confirm the exact field path) must equal the *full*
   `source_ref` commit SHA, not just its 12-char prefix.
2. `cosign verify-attestation` must succeed against the candidate,
   using the identical policy `stack.RequireAttestation` enforces at
   deploy time (`internal/stack/attestation.go`) - added a
   `sigstore/cosign-installer` step (pinned to the same `v3.0.6` cosign
   release `rootguard-core`'s own Dockerfile uses) to make the binary
   available on the runner.
3. The final tag's existing digest (if any) is inspected first: missing
   → create; already the candidate's own digest → no-op (an idempotent
   retry); anything else → hard abort. A published version tag must
   never start silently resolving to different content - if `${VERSION}`
   was already published pointing elsewhere, that needs a new version
   number, not a forced overwrite of this one.

**Medium, fixed: the release smoke test never actually deployed or
verified the blockpage image it just published.** `smoke-test` never set
`ROOTGUARD_BLOCKPAGE_IMAGE` (every other RootGuard-built image had its
own env var pointing the deploy at this run's candidate), and its deploy
config never set `blockpage_enabled: true` (default off) - so the
blockpage image, built and published by the exact same `publish` job as
everything else in the run, was never actually started, never
attestation-checked, and never smoke-tested by any release run. A broken
blockpage image (bad nginx config, a broken entrypoint script, anything
short of the build itself failing) could have shipped fully undetected.

Fixed by adding `ROOTGUARD_BLOCKPAGE_IMAGE` to the job's env (same
`candidate_tag`-scoped reference every other image already uses),
turning `blockpage_enabled: true` on in the deploy config, and adding a
real reachability/content check after the existing DNS/DNSSEC checks:
`curl`s the blockpage's own root page (it binds on
`config.DNSBindAddress:80` once enabled, per `installer.Manager`'s
`blockpagePort`) and asserts the response actually contains the real
page's content, not just that the container started.

**Medium, fixed: `updater.Manager`'s multi-file persistence
(`status.json`/`images.json`/`updates.yaml`) was only atomic per file,
not as a group.** All three files derive from the same in-memory state
(`m.status`/`m.selected`) and are read back together on startup
(`load()`), but `persistLocked` called `atomicfile.WriteJSON`/`WriteFile`
once per file in sequence - a failure partway through (the review's own
example: `images.json` failing to write after `status.json` had already
been committed) left them silently inconsistent with each other, visible
on the very next `load()`.

Fixed with a new `atomicfile.WriteFiles`/`atomicfile.JSONFile` pair: every
file in a batch is staged (written to its own temp file, fsynced) before
any of them is renamed into place, and none are renamed unless every
single one staged successfully - a staging failure (the dominant real
cause: disk full, permissions, an I/O error) now leaves every file in the
batch completely untouched, not just the one that actually failed.
Renaming several files still can't be one atomic operation on POSIX, so
a residual window remains if a rename itself fails after every file
already staged - narrowed from "an arbitrarily slow write of a later
file" down to "the moment between two already-guaranteed-to-succeed
renames", documented explicitly in `WriteFiles`' own doc comment as the
best available guarantee without a write-ahead log or combining the
files into one. `persistLocked` now builds all three as one batch and
calls `WriteFiles` once.

New tests at both layers: `atomicfile_test.go` gained
`TestWriteFilesLeavesEveryFileUntouchedWhenAnyStagingFails` (a staging
failure changes nothing) and
`TestWriteFilesCleansUpRemainingTempFilesOnRenameFailure` (the documented
residual window, with no leaked temp files); `updater`'s own
`manager_test.go` gained
`TestPersistLockedKeepsMultiFileStateConsistentOnFailure`, which
reproduces the review's exact scenario end-to-end through the real
`Manager` (not just the `atomicfile` primitive) and proves the strongest
available claim - sabotaging the first file in commit order so its own
rename fails leaves all three files, including the two after it, at
their previous generation with zero partial commits.
`rootguard-webapp/backend`'s own three atomic-write call sites
(credentials/sessions/audit log) and `rootguard-updater`'s
`control-plane-images.yaml` were checked too: neither has this
tightly-coupled, read-back-together shape (each is either independent or
never re-read into memory at startup), so neither needed the same fix.

**Medium, fixed: `README.md`, `SECURITY.md`/`SECURITY.de.md`, and
`docs/release-history.md` still described RootGuard as an old-series
alpha/beta - `README.md`'s Quick Start pointed new users at
`v0.1.0-beta.1`'s `compose.alpha.yaml`/`.env.alpha.example` under the
`compose.alpha.yaml` name that release ever used, while every other
release artifact had long since moved to `v1.0.0-rc.1` and
`compose.release.yaml`. `site/index.html`/`site/docs.html` already
showed the correct `1.0.0-rc.1` version number in most places but still
called RootGuard "the [public] beta" in three prose spots -
`scripts/check-site-facts.sh`'s version check doesn't judge prose,
only version-string currency, so a stale word next to a correct version
number was invisible to it.

Nothing checked `README.md` at all: `scripts/check-site-facts.sh`/
`bump-site-versions.sh` only ever scoped themselves to `site/*.html`, so
README could (and did) drift indefinitely with no CI signal. Verified
directly: `source scripts/version-pattern.sh && rootguard_extract_versions
< README.md` cleanly extracted exactly the one stale version string with
no false positives (confirmed the AGPL-3.0-or-later license mention
doesn't accidentally match - it's only two dot-separated groups, the
extractor requires three).

Fixed: manually corrected `README.md` (both `curl` URLs' tag and
filenames, the `docker compose -f` command, and the "public beta"
callout in both language sections), `SECURITY.md`/`SECURITY.de.md`
("public alpha" -> "public release-candidate testing ahead of 1.0"),
`docs/release-history.md`'s "current public release" line, and the
three stale `site/*.html` prose spots. Then, rather than leave this to
drift stale again next release, extended both scripts' `for file in
site/*.html` loops to also cover `README.md` - the same version-string
substitution/check logic already handles it cleanly (verified: reverting
just `README.md`'s fix and re-running `check-site-facts.sh` correctly
reports 4 stale matches; the real fixed version passes clean). Left the
scripts' local link/asset check site/*.html-only - README's relative
links resolve against a different base and it has none of the broken-
local-asset kind that check exists for.

**Medium, fixed: the blockpage's `/api/reason` comment claimed `$host`
"is never a client-supplied parameter", which isn't accurate.** `$host`
resolves from the request's `Host` header (no real `server_name` to fall
back to - the block is `server_name _`), and the `Host` header is
exactly as client-controlled as any other request header: nothing stops
a client on the DNS bind network from sending an arbitrary `Host` to
this endpoint directly, without ever going through a real
AdGuard-triggered sinkhole for that domain. That makes it a real, if
narrow, "is domain X currently blocked" oracle against the household's
own AdGuard instance - not a made-up concern, but also not something a
DNS-sinkhole architecture using a custom blocking IP (RootGuard's or any
other's, e.g. Pi-hole's own equivalent) has a real channel to prevent
entirely: there's no way to prove "this request came from a genuine DNS
block" beyond the `Host` header itself.

Fixed the comment to state this accurately - what's actually true
($host reflects genuine client intent in the *legitimate* flow, simply
because that's how HTTP virtual hosting works, but this endpoint can't
distinguish that from a hand-crafted probe), what's scoped (whoever can
already reach the container - the same audience every legitimately
sinkholed client also reaches it from), and what's revealed (only a
blocked/not-blocked verdict, no browsing history). Also tightened the
real, cheap mitigation available: `limit_req_zone` from `5r/s` to `1r/s`
and the endpoint's own `burst` from `10` to `3` - the single legitimate
client (`web/meta.js`, one `fetch` per page load) never needs more than
an occasional quick reload's worth of requests, so this comfortably
covers real usage while cutting bulk domain-enumeration throughput
roughly 15x.

**Medium, fixed: `PersistError`/`PersistErrorAt` reached the frontend's
API types but were never actually rendered anywhere.** Round 2 made
`installer.Manager`/`updater.Manager` clear and re-set these fields
truthfully around every persist attempt, and the corresponding TS types
in `api/client.ts` gained the matching optional fields - but nothing in
the React UI ever read them. A user had no way to see that the state
shown was live-accurate but not durably saved, and could silently
regress on the next restart of RootGuard Core.

Fixed with a warning banner (amber `--warning`/`--warning-soft` tokens,
distinct from the existing red error banners - the live state itself is
still correct, only its durability is in question) in the two places
that already render the status objects carrying these fields: the
dashboard (`Overview.tsx`, for `InstallationStatus.persist_error`) and
the Stack page (`Stack.tsx`, for `UpdateStatus.persist_error`). Both
show the error message and a localized timestamp, in both `en`/`de`
locales. Frontend lint, all 26 tests, and the production build
(`tsc -b && vite build`) all pass.

**Low, fixed: an invalid stored `theme` value permanently broke the
blockpage's theme toggle button.** `web/theme.js`'s `applyTheme` did
`btn.appendChild(icons[mode])` - if `localStorage.getItem(...)` returned
anything other than the three real modes (a leftover from an older
schema, hand-edited/corrupted storage, nothing actually enforces the
stored value's shape), `icons[mode]` was `undefined` and
`appendChild(undefined)` throws, crashing the whole IIFE during its very
first `applyTheme` call at page load - before the click listener was
even registered, so the toggle button stayed permanently dead until the
stale value was manually cleared from the browser.

Fixed with a `storedMode()` helper that falls back to `"system"` for
anything not one of the three real modes, used at both the initial load
and the click handler. Verified live with Playwright against the real
`web/index.html` (served locally, not simulated): with
`localStorage.rootguard.blockpage.theme` deliberately set to a garbage
value, the unfixed version reliably threw the exact `TypeError` and left
the button completely unresponsive to clicks (`data-theme` never
changed); the fixed version threw nothing, correctly fell back to system
theme, and the button worked normally on the very first click. Applied
identically to `rootguard-webapp/frontend/public/blockpage-preview/theme.js`,
kept byte-for-byte in sync with the real one per existing convention.

**Low, fixed: the FritzBox router-discovery dial guard rejected
zone-qualified IPv6 link-local addresses outright.**
`rejectNonPrivateControl` used `net.ParseIP` on the dialer's resolved
`ip:port`, but link-local IPv6 addresses are ambiguous without a zone
identifier (the same address can exist on multiple interfaces) - a real
dial to one resolves to exactly the `fe80::1%en0` shape, and
`net.ParseIP` has never supported the `%zone` suffix at all, returning
`nil` unconditionally for it. Every zone-qualified link-local address
was refused regardless of actually being private, even though
`isRouterReachable` explicitly accepts link-local addresses.

Fixed by switching to `net/netip.ParseAddr`, which parses the zone
correctly and exposes the identical `IsPrivate`/`IsLinkLocalUnicast`/
`IsLoopback` methods `net.IP` has - `isRouterReachable` now takes a
`netip.Addr`. Verified the old failure mode directly (`net.ParseIP`
returns `nil` for `"fe80::1%en0"`) and added the regression case to both
`TestRejectNonPrivateControlRejectsPublicAddresses` (via the dialer's
bracketed `[fe80::1%en0]:80` shape) and `TestIsRouterReachable`.

**Low, fixed: Core's and Updater's Dockerfiles `apk add`-installed
packages unpinned, redundantly.** Both installed `docker-cli-compose`
(Updater also `ca-certificates`) via a plain `apk add --no-cache` with
no version pin - a real reproducible-builds gap, the exact package
version fetched could drift between two builds of the same commit even
though the base image digest itself is pinned. Checked whether pinning
was even the right fix first: fetched the upstream
`docker-library/docker` source for the exact `docker:29-cli` digest both
Dockerfiles already pin, and confirmed it already bundles `docker
compose` (currently v5.5.0) as a CLI plugin at
`/usr/local/libexec/docker/cli-plugins/docker-compose` - one of Docker
CLI's standard plugin search paths, and exactly the subcommand form this
codebase actually invokes (`docker compose ...`, not the legacy
hyphenated standalone binary) - and `ca-certificates` from that same
base image's own upstream Dockerfile. Both installs were purely
redundant, not filling a real gap.

Removed both `apk add` lines entirely rather than pinning packages that
were never needed - closes the reproducibility gap outright (nothing
left to fetch at build time at all) instead of just narrowing it.

## Follow-up review, round 4 (2026-08-29)

A fourth independent review of round 3's own fixes - two of them turned
out to be incomplete rather than wrong outright. Same discipline as every
round before: verify directly in code (and, for the release pipeline,
against the actual published `1.0.0-rc.1` artifacts on ghcr.io), scope
the fix, test with real teeth, document, PR, merge.

**Critical, fixed: a release re-run could still fold untested commits
into the tag, and still force-moved it on any mismatch.** Round 3 fixed
the annotated-tag dereference bug and added a `merge-base --is-ancestor`
check to the tag-pointing step's fallback path, but two gaps remained,
both confirmed live against the actual `1.0.0-rc.1` release: its Core
image's `org.opencontainers.image.revision` label (`76c178a1...`) and its
git tag's target commit (`a51602c8...`) differ by three commits, not the
one documented pin commit.

Root cause: `release-version-bump.yml` still pre-created the release tag
right after the changelog commit, before `release-alpha.yml` (dispatched
next) had run a single test against it. `release-alpha.yml`'s
`update-alpha-pins` job then had to *move* that pre-existing tag onto its
own, later pin commit once everything passed - and to build that pin
commit, it `git fetch`ed and `git rebase`d onto `origin/main`'s current
tip before pushing, so any commit that had landed on `main` in the
window between the changelog push and the pin push became an ancestor of
the pin commit, and therefore part of the tag, without this release ever
having built, tested, or security-scanned it. Separately, whenever an
existing tag didn't match the freshly computed target, the tag-pointing
step still force-moved it unconditionally (`git tag -f -a` +
`git push --force`) rather than treating any mismatch as suspicious.

Fixed with a real redesign, not another patch on the same shape:

- `release-version-bump.yml` no longer creates the tag at all - just the
  changelog commit, pushed straight to `main`.
- `release-alpha.yml`'s pin-commit step no longer rebases: it checks
  `origin/main == SOURCE_REF` first and hard-aborts the whole release if
  main has moved, with an explicit message to start a fresh release
  instead - never silently folding in commits this run never tested.
- The tag is now created in exactly one place, once, after every gate
  (test, security, smoke-test, upgrade-test) has actually passed,
  directly on the pin commit - and is *never* force-moved again
  afterward: an existing tag that already matches is a no-op, one that
  doesn't is a hard error demanding a by-hand fix, not a silent
  overwrite.
- New guard in the `version` job: `SOURCE_REF` must have the currently
  published latest release's own commit as an ancestor (or be that exact
  commit again, the legitimate same-version-retry case) before anything
  else runs - a stale re-dispatch or a version override against the
  wrong commit is rejected immediately, before wasting a full
  test/publish/smoke-test/upgrade-test cycle on a release that can't
  land cleanly anyway. This is commit-ancestry, not a version-number
  comparison, so it also catches a deliberately-higher version string
  typed against an old commit, not just a numerically-lower one.

The existing, already-published `1.0.0-rc.1` was deliberately left
untouched - the next real release (whichever version that turns out to
be) will be the first cut under the corrected pipeline.

**Critical, fixed: the installer could still pin a stale image digest,
and mis-split any registry:port image reference.** Round 2's
`resolveDigest` (`internal/installer/manager.go`) inspected the local
image object's full `.RepoDigests` list and took the *first* entry
matching the repo - already documented, on `digestFromPullOutput` in
`internal/updater/github_release.go`, as unreliable: a local image
object isn't scoped to "what was just pulled", so if it's ever
associated with more than one digest for the same repo (a real,
previously-hit failure mode in this exact codebase), the first match can
silently be a stale one. `resolveDigest` reused exactly that unreliable
shape instead of the more authoritative pattern already established
right next to it. Separately, its `strings.Cut(image, ":")` repo/tag
split mis-parses any reference naming a registry host:port (e.g.
`registry.example:5000/rootguard-unbound:tag` split into
`registry.example` and `5000/rootguard-unbound:tag`), silently breaking
the digest lookup for that entire class of reference.

Fixed by switching `resolveDigest`'s primary path to `docker pull`'s own
reported digest (`digestFromPullOutput`'s pattern - authoritative for
"what was just pulled" in a way a post-hoc inspect isn't), with the old
`.RepoDigests` inspect kept only as a last-resort fallback for an
unexpected pull-output shape; and a new `imageRepo` helper that only
treats a colon after the last `/` as the tag separator, the same rule
Docker's own reference parser uses.

While fixing this, found the identical `strings.Cut(image, ":")` bug
already live - not just as a pattern to copy, but in actual production
code paths - in both `digestQualify`/`digestFromPullOutput` copies this
installer function was modeled on:
`rootguard-core/internal/updater/github_release.go` and
`rootguard-updater/image.go`. Fixed all three with the same `imageRepo`
helper (kept in sync by hand across all three files, matching this
codebase's existing convention for this exact ~15-line lookup, and
noted as such in each copy's own comment) rather than leaving two of
them silently broken for the same reference shape.

New regression tests: `TestResolveDigestPrefersPullOutputOverStaleRepoDigests`
(a stale-first-match `.RepoDigests` fixture next to the correct
pull-reported digest - must prefer the latter),
`TestResolveDigestFallsBackToRepoDigestsWhenPullOutputIsUnparsable` (the
deliberate fallback path still works), and `TestImageRepoHandlesRegistryPort`
in all three affected files (installer, `internal/updater`,
`rootguard-updater`), covering a registry:port reference, a nested
namespace under one, a plain Docker Hub-style reference, and both
tagless-input shapes.

**Medium, fixed: candidate-image promotion only checked the first
platform's revision label, not all of them.** `release-alpha.yml`'s
promotion step read every platform's `org.opencontainers.image.revision`
label but then took only the *first* non-null one
(`... | map(select(. != null)) | first`) - a multi-arch manifest with a
correct `amd64` label and a wrong or entirely missing `arm64` one still
passed. Fixed to require exactly the two platforms the publish job's own
build matrix always produces (`linux/amd64`, `linux/arm64`) and every
one of them carrying the correct label - verified live with `jq` against
the real published `1.0.0-rc.1` Core image, both for the passing case
and a deliberately wrong expected revision.

A fourth independent review of round 3's own fixes - some turned out to
be incomplete rather than wrong outright. Same discipline as every round
before: verify directly in code, scope the fix, test with real teeth,
document, PR, merge.

**Medium, fixed: `updater.Manager`'s multi-file persistence still had a
residual split-brain window after round 3's own fix.** Round 3 made
`persistLocked` stage `status.json`/`images.json`/`updates.yaml` through
a single `atomicfile.WriteFiles` call rather than three separate
`WriteFile`/`WriteJSON` calls, closing the dominant failure mode (a
staging failure now leaves every file untouched) - but correctly flagged
by this review as still incomplete: renaming multiple files can never be
one atomic operation on POSIX, so a rename failing (or the process dying)
between two renames still left `status.json` and `images.json` in two
different generations, exactly the scenario the fix was supposed to
close. `atomicfile_test.go`'s own
`TestWriteFilesCleansUpRemainingTempFilesOnRenameFailure` demonstrates
that residual window deliberately, as its own doc comment already said.

Closed for real this time, not narrowed further: `m.status` and
`m.selected` were only ever two separate files because nobody had asked
whether they needed to be - both are plain internal JSON with no
external reader forcing them apart, unlike `updates.yaml` (a different
format, read by `docker compose -f`). Consolidated them into one
canonical file, `state.json`, written with a single
`atomicfile.WriteJSON`/`WriteFiles` call - a single file's
write-temp-then-rename is unconditionally atomic, so there is no longer
a multi-file window for the canonical state at all, residual or
otherwise. `updates.yaml` stays a separate file (still batched together
with `state.json` via `WriteFiles` for the common case), but is now
understood explicitly as a pure function of `m.selected` - a derived
artifact for `docker compose` to read, not a second source of truth this
process itself depends on, so a failure isolated to its own write no
longer blocks the canonical state from advancing, and self-heals on the
very next successful persist.

Added a migration path in `load()` for the many real installations
(every one up to and including `1.0.0-rc.1`) whose data directories are
still in the old split-file shape on disk: read the legacy
`status.json`/`images.json` once if `state.json` doesn't exist yet, then
immediately re-persist into the new combined format - a silent,
one-time, automatic migration on first boot with the fix, not a manual
step or a loss of update history. The old files are left in place as
harmless, never-read-again leftovers rather than deleted.

New/replaced tests: `TestPersistLockedStateJSONIsSingleFileAtomic`
(sabotages `updates.yaml` specifically and confirms `state.json` still
advances correctly - a derived-artifact failure must never block the
canonical state), `TestUpdatesYAMLSelfHealsAfterAFailedPersist` (proves
the self-healing claim: `updates.yaml` catches back up to the canonical
state on the next successful persist once whatever blocked it clears),
and `TestLoadMigratesLegacyStatusAndImagesJSON` (writes old-format
fixtures, constructs a real `Manager`, and confirms both the in-memory
state and the newly-migrated `state.json` on disk are correct, then
confirms a second `Manager` against the same now-migrated directory
reads `state.json` directly). Also corrected `atomicfile.go`'s own
`WriteFiles` doc comment, which had claimed combining files "existing
on-disk formats and external readers... make impractical here" - true
for `updates.yaml`, not true for the `status.json`/`images.json` pair
this fix just combined; the comment now says so and recommends
combining over `WriteFiles` whenever every file in question really is
this process's own internal format with no external reader forcing them
apart.

**Small items, fixed:**

- `internal/routerimport/fritzbox_test.go` wasn't `gofmt`-formatted
  (drifted after a hand-aligned map literal picked up a new entry in an
  earlier round) - `gofmt -w`'d, and added a `gofmt -l` gate to
  `ci-core.yml`/`ci-updater.yml`/`ci-webapp.yml` (none of `go test`/
  `go vet` check formatting at all) so this specific class of drift
  fails CI instead of waiting for a human to notice.
- `scripts/lib/semver-validate.sh` had no shebang, so `shellcheck`
  couldn't determine its dialect (`SC2148`) and skipped real analysis of
  a file using bash-specific syntax (`[[ ... =~ ... ]]`, `local`).
  Added `#!/usr/bin/env bash`, matching the sibling `version-pattern.sh`
  file's own existing convention for a sourced-only script. Swept the
  rest of `scripts/*.sh` with `shellcheck` too - every remaining
  finding is a genuine false positive (`SC2154` for a variable set by
  whatever sources the file, `SC2329` for a test-harness function
  invoked indirectly), confirmed one by one, not just assumed.
- `rootguard-core/Dockerfile`'s comment claimed the pinned `docker:29-cli`
  digest bundles Docker Compose v5.5.0 - correct for
  `docker-library/docker`'s current upstream source, but that source
  had moved on since this exact digest was built. Correlated the
  digest's own build timestamp (`docker buildx imagetools inspect`,
  2026-08-10) against upstream's commit history for that date instead of
  trusting its current `HEAD` - the digest actually bundles v5.4.0.
  Comment corrected; the underlying fix (removing the redundant `apk
  add`) was already correct regardless of the exact version claimed.
- The release smoke test's blockpage check only ever proved the static
  HTML page loads - `/api/reason` (the live-data endpoint, proxying
  through Core with the shared service token) was deployed but never
  exercised, so a broken token hand-off or a broken Core-side route
  could have shipped undetected. Added a real check: queries `/api/reason`
  for a plain, unblocked domain and asserts a real, non-empty `"reason"`
  came back - nginx's own upstream-failure fallback returns
  `{"available":false}` with no `"reason"` key, so this fails if any
  link in the chain (nginx auth, the proxy, Core's own route, AdGuard)
  is broken.
- `rootguard-blockpage/web/theme.js` and its webapp in-app preview copy
  (`rootguard-webapp/frontend/public/blockpage-preview/theme.js`) are
  deliberately duplicated byte-for-byte rather than built from one
  shared source - flagged as a low-priority drift risk, not a bug (both
  copies were, and still are, correct). Rather than restructure either
  component's build right before the RC, added a plain `diff` check to
  both `ci-blockpage.yml` and `ci-webapp.yml` - a future edit to one
  side without the other now fails CI immediately instead of silently
  drifting unnoticed.

**Medium, fixed: component-level documentation still described an old
alpha/beta lifecycle phase, and five per-component `SECURITY.md` files
were dead monorepo-migration leftovers.** `docs/platform-support.md`
and `site/roadmap.html` still said "public beta" in their framing
copy (their actual content - the `0.9`/release-candidate milestone
status, the support-policy version pattern - was already accurate,
just the surrounding lifecycle wording wasn't). `ROADMAP.md`'s own
"Status and scope" section still said "pre-release beta development"
with a "Last reviewed" date three weeks stale relative to its own
later, accurate `0.9 RC` section.

The five `rootguard-*/SECURITY.md` files turned out to be worse than
stale wording: leftovers from before the monorepo migration, each
pointing at its own now-archived, read-only per-component repo's
vulnerability-reporting page (e.g.
`github.com/foxly-it/rootguard-core/security/advisories/new`) - a dead
reporting channel, not just an outdated one. GitHub's own repository
Security tab only ever surfaces the *root* `SECURITY.md`
(already corrected to "release-candidate testing" in round 3); these
subdirectory copies have no special standing in a monorepo and nothing
in the codebase links to them (`grep`-verified). Deleted all five rather
than updating their wording - the root `SECURITY.md`/`SECURITY.de.md`
is the single canonical policy for the whole monorepo.

Fixed the genuinely current-state lifecycle claims in
`docs/platform-support.md`, `site/roadmap.html` (the page's own `<meta
description>`/`data-description-*` attributes; its milestone-history
labels like "0.1 ALPHA" are correctly left alone - those name what a
past milestone *was called*, not a claim about today), and
`ROADMAP.md`'s top summary - left every genuinely historical reference
(a specific past release's own state, a milestone's own name,
`CHANGELOG.md`'s generated entries, this very audit log's own record of
what used to say what) untouched throughout, the same
historical-vs-current distinction `check-site-facts.sh`'s own exclusion
pattern already draws.

## Follow-up review, round 5 (2026-08-29)

A fifth independent review of round 4's own fixes - three of them turned
out to be genuinely incomplete rather than wrong outright, confirmed live
against the actual repository and, where relevant, the real published
`1.0.0-rc.1` artifacts. Same discipline as every round before.

**Critical, fixed: the tag-push trigger was now structurally
incompatible with the never-move-a-tag design round 4 built.**
`release-alpha.yml` still also triggered on a pushed `v*.*.*` tag - but
that design now requires the tag to not exist yet when the workflow
starts, created exactly once at the very end, on the pin commit, and
never moved again under any circumstance. A manually pushed tag violates
that from the first step (it already exists, on the pre-pin source
commit, before a single test has run) and round 4's own "never move a
published tag" guard would then correctly, but unhelpfully, refuse to
ever finish that release. Removed the trigger entirely -
`release-version-bump.yml`'s own `workflow_dispatch` is the one
supported way to cut a release (see `docs/release-process.md`, also
corrected in this round - it still described the old atomic
commit-and-tag push and a tag that gets "moved" rather than created
once). The now-dead `GITHUB_REF_NAME`-derived version fallback that
trigger needed went with it, not left behind as unreachable code.

**Critical, fixed: final image tags were promoted before the
main-hasn't-moved check that could still reject the whole release.**
Round 4 added a check that `origin/main` still equals the tested
`SOURCE_REF`, but only inside the "Commit updated pins" step - by which
point `update-alpha-pins` had already promoted every component to its
*final* `VERSION` image tag, irreversibly, several steps earlier. A PR
merging during the long test/publish/smoke-test/upgrade-test window
ahead of this job could reach that later check, get correctly rejected,
and still leave the version number permanently unusable: its final
image tags already public, with no way to ever create a matching git
tag, GitHub Release, or pinned compose file for them. Added the
identical check right after checkout, before anything in the job writes
anything irreversible - the original check right before the actual pin
commit stays too, as defense-in-depth against the now much narrower
remaining window (a handful of setup steps, not the entire E2E phase).

**Critical, fixed: nothing prevented publishing a semantically older,
merely-unused version number.** Round 4's ancestry guard
(`git merge-base --is-ancestor`) only verified that the source commit
descends from the latest published release - never that the *version
number itself* has higher SemVer precedence. Confirmed live: a manual
override requesting `0.9.9` against a `HEAD` genuinely descending from
the real published `1.0.0-rc.1` passed that check cleanly, and could
have gone on to reset `README.md`/site/compose pins back to it. New
`scripts/lib/semver-compare.sh` implements real SemVer 2.0 precedence
(deliberately not `sort -V` - confirmed live it gets this project's own
imminent rc→stable transition backwards: `1.0.0` sorts *below*
`1.0.0-rc.1`, when a version with no prerelease suffix must always
outrank a prerelease of the same core version) and is exercised in both
`release-version-bump.yml` (guards a hand-typed version override) and
`release-alpha.yml`'s own `version` job (defense-in-depth, and the only
gate for a fully manual dispatch). Hand-verified against the full
canonical SemVer.org precedence chain
(`1.0.0-alpha < 1.0.0-alpha.1 < 1.0.0-alpha.beta < 1.0.0-beta <
1.0.0-beta.2 < 1.0.0-beta.11 < 1.0.0-rc.1 < 1.0.0`) plus every case from
`rootguard-updater`'s own `TestIsOlderReleaseVersion` table, and live
against the real repository reproducing the exact `0.9.9` scenario.

**Medium, fixed: the multi-platform revision check verified platform
*count*, not platform *names*.** Round 4 fixed "only the first
platform's label was checked" by requiring exactly two platforms, all
matching - still not enough, confirmed live with the same `jq`
expression: a manifest naming `linux/amd64` and `linux/s390x` (two
platforms, correct label on both) passed cleanly, since nothing checked
*which* two. Now compares the sorted platform key set against the exact
pair the publish job's build matrix always produces
(`["linux/amd64","linux/arm64"]`), not just its length.

**Medium, fixed: a failed `updates.yaml` write still left the new image
selected in `state.json`.** `selectImage` set `m.selected[service]` to
the new image *before* persisting - a persist failure (updates.yaml's
own write failing, while state.json's own write inside the same attempt
succeeded, exactly what round 4's own fix guarantees) still left that
new image selected, both in memory and on disk, even though every real
caller treats a `selectImage` failure as the whole operation failing and
rolls back everything else it already did (volume ownership migration,
the container swap that never happens). `manager_test.go`'s own round-4
test explicitly demonstrated this as the expected behavior. `selectImage`
now reverts the selection (and re-persists that reversion - state.json's
own rename still isn't blocked by updates.yaml failing again, per round
4's design) before returning the error, so a failed operation means
nothing changed, full stop. The two round-4 tests that asserted the old
behavior now exercise `persistLocked` directly instead (still correctly
proving that lower-level, still-true property); a new
`TestSelectImageRevertsSelectionOnPersistFailure` proves the higher-level
revert.

**Medium, fixed: eight files linked to the per-component `SECURITY.md`
files round 4 deleted.** Each component's own `README.md`/
`CONTRIBUTING.md` linked to its own `SECURITY.md` via a plain,
unqualified `[SECURITY.md](SECURITY.md)` - correct before round 4's
deletion (dead monorepo-migration leftovers, confirmed via a narrow grep
for those specific paths only), broken after it, since nothing checked
markdown links generally. Repointed all eight at the canonical root
`SECURITY.md` (`../SECURITY.md`). Added `scripts/check-markdown-links.sh`
- checks every local link across every tracked markdown file in the repo
resolves to a real file, wired into `ci.yml`'s existing always-runs
`validate` job - and verified it live both ways: reports exactly these
eight breakages against the pre-fix files, reports clean once fixed.

**Small items, fixed:** `ci-unbound.yml` was still on `actions/setup-go`
v5 (GitHub already warns about its Node.js 20 runtime) and missing
`cache-dependency-path` (a cache-miss warning every run) - every other
workflow in this repo already used the same pinned v7 + cache path
combination; this one was simply never updated to match.

## Follow-up review, round 6 (2026-08-30)

A sixth independent review, covering round 5's own fixes plus live repo
state (workflow run history, GitHub API settings) rather than only the
diff. Same discipline as every round before.

**Medium, fixed: `semver-compare.sh`'s numeric-identifier comparison
overflowed bash's signed 64-bit integer range.** Both
major/minor/patch and numeric prerelease identifiers were compared via
`$(( ))` bash arithmetic - SemVer 2.0 doesn't cap numeric identifiers at
64 bits, bash does. Confirmed live:
`semver_compare 9223372036854775807.0.0 9223372036854775808.0.0`
returned `1` (the second value ranked *lower*), because
`10#9223372036854775808` overflows and wraps negative past
`9223372036854775807`. Replaced with `_semver_compare_numeric_string`, a
pure string comparison (SemVer's numeric-identifier grammar already
forbids leading zeros, so a longer decimal digit string is always
numerically larger, and two equal-length ones compare correctly
byte-for-byte) - no arithmetic, no width limit. Also fixed: the
comparator's own header comment referenced a `semver-compare.test-cases`
file that was never actually created, so the guard shipped with zero
automated regression coverage. Added `scripts/lib/semver-compare.test.sh`
- the canonical SemVer.org precedence chain, build-metadata stripping,
the live-reproduced `0.9.9`-vs-`1.0.0-rc.1` case, and the overflow case
above - wired into `ci.yml`. Verified both ways: reverting the fix
reproduces exactly the three overflow-case failures above; the fixed
version passes all of them.

**Critical, fixed: promotion to the final release tag still happened
*before* the last main-race check, not after.** Round 5 added an early
"has main moved" check at the top of `update-alpha-pins`, but everything
between it and the actual pin commit - buildx/cosign setup, per-image
attestation checks, promoting all five images to their final `VERSION`
tags, digest-pin file edits, compose validation, site refresh - still
ran *before* the one check that actually gated anything irreversible
(inside "Commit updated pins", unchanged since round 5). A PR merging
into a narrower but still-real window between the early check and that
one could still let this run promote final image tags publicly, then
correctly abort the pin commit/tag/release - leaving the version number
unusable (a later attempt at the same version number, from the new
`main` tip, would hit the "published tag must never move" guard on
promotion) even though nothing else about the release actually shipped.

Reordered so promotion is the *last* thing that can happen, not one of
the first: candidate/attestation verification (split into its own
"Verify candidate images and attestations" step, no promotion) stays
early since it's read-only and safe to abort before; digest-pin file
edits, compose validation, and the site refresh stay local (no
dependency on `main`'s state); the pin commit's own main-race check
(unchanged) is now the single gate; actual promotion
("Promote candidate images to the final release tag", moved after the
pin commit) happens only once that commit already exists on `main` - at
which point nothing left in the job still depends on `main`'s tip at
all, so a promotion failure here is a plain retry, not a race. Git tag
and GitHub Release creation were already last (round 5).

Made the pin commit itself retry-safe for this new ordering: a run that
fails between the pin commit and promotion needs a subsequent retry to
recognize that its own earlier attempt already landed the commit
(`origin/main`'s tip no longer equals `SOURCE_REF` at that point, which
the existing race check would otherwise reject as a fresh race). "Commit
updated pins" now checks first whether `origin/main`'s current tip
already carries identical content for the three paths it writes - if so,
resumes from that existing commit instead of re-committing or
misreporting a race.

**Small, fixed: a reverted `selectImage` left behind an explicit
empty-string map entry instead of no entry at all.** `selectImage`'s
persist-failure revert unconditionally wrote
`m.selected[service] = previous` - for a service that had never been
selected before, `previous` is Go's zero value `""` for a missing map
key, so the revert left `service: ""` in `m.selected` rather than
restoring "no entry at all".
`overrideContentLocked`'s own `TargetImage` fallback treats a missing key
and an explicit empty string identically, so this never actually
surfaced in practice - but a reverted operation should leave the exact
state it found, not a lookalike. Now tracks whether the key existed
before selecting and either restores the previous value or `delete`s the
key. New `TestSelectImageRevertsToNoSelectionOnFirstPersistFailure`
covers the previously-untested first-selection case; verified it fails
against the old unconditional-write code and passes against the fix.

**Small, fixed: the repo-wide markdown-link check
(`scripts/check-markdown-links.sh`, added round 5) only ever matched one
inline-link shape.** A hand-rolled regex over `](target)` missed
reference-style links (`[text][id]`), link reference definitions
(`[id]: target`), `<...>` autolink-style targets, optional link titles,
and non-standard code-fence markers (`~~~`, indented or longer backtick
fences) - all things a real CommonMark parser handles correctly. No
broken link of any of those shapes exists in the repo today, but the
check itself was incomplete. Retired the regex script; `ci.yml`'s
`validate` job now installs `lychee` (version-pinned and
checksum-verified, same install pattern `release-alpha.yml` already uses
for `git-cliff`) and runs it `--offline` against every tracked `*.md`
file - local file links only, no network requests.

**High, fixed: the Debian package-pin refresh automation had been
silently failing for four runs in a row.** `debian-pin-freshness.yml`
detects drifted `apt` package pins in `rootguard-unbound/Dockerfile`,
commits a fix to `chore/debian-pin-refresh`, and opens a PR - the commit
and push succeeded every time (confirmed live: the remote branch already
carried the correct `libssl-dev`/`libssl3t64` bump to
`3.5.7-1~deb13u2`), but `gh pr create` failed every time with "GitHub
Actions is not permitted to create or approve pull requests", a
repository-level setting independent of the workflow's own already-
correct `pull-requests: write` permission. Confirmed via
`repos/.../actions/permissions/workflow`:
`can_approve_pull_request_reviews` was `false`. Opened
[#430](https://github.com/foxly-it/rootguard/pull/430) manually from the
already-correct branch to unblock the immediate pin drift; the user then
enabled the repository setting directly (Settings → Actions → General →
"Allow GitHub Actions to create and approve pull requests"), verified
live afterward (`can_approve_pull_request_reviews` now `true`) - the
automation will open its own PRs again the next time a real pin drifts.

**Medium, open - needs a repository-admin decision: `main` has no branch
protection or ruleset at all.** Confirmed live: both
`repos/.../branches/main/protection` (404, "Branch not protected") and
`repos/.../rulesets` (`[]`) are empty. Nothing technically stops a
force-push or branch deletion, and green PR checks aren't enforced. Not
fixed in this round - repository-ruleset mutation is a sandboxed action
this session isn't permitted to perform, and the right configuration is
a real product decision the repo owner needs to make, not something to
guess at via API: `release-alpha.yml`'s "Commit updated pins" step
pushes directly to `main` using the built-in `GITHUB_TOKEN` (as
`github-actions[bot]`), so a bare "require pull request before merging"
rule would also block that push unless it's specifically exempted.
Recommended path: a repository **ruleset** (not classic branch
protection, which has no per-actor bypass) targeting `main`, with
"Block force pushes" and "Restrict deletions" enabled unconditionally
(these never affect a normal, non-force push, so they need no bypass and
are safe to enable regardless of the rest), and separately "Require a
pull request before merging" plus "Require status checks to pass" with
**GitHub Actions** added to the ruleset's bypass list so the release
workflow's own direct pin-commit push keeps working. Left open for the
repo owner to configure via the GitHub UI (Settings → Rules → Rulesets →
New branch ruleset).

## Follow-up review, round 7 (2026-08-30)

A seventh independent review of round 6's own retry-detection fix -
found it shipped two real bugs of its own, one of which made the retry
path it was meant to add practically unreachable. Same discipline as
every round before.

**Releasekritisch, fixed: the round-6 retry fix was unreachable, and
its own fallback logic could resolve to the wrong commit.** Two
compounding bugs in `update-alpha-pins`:

1. The early "Verify main hasn't moved" guard (added round 5, kept
   round 6) still required `origin/main` to equal `SOURCE_REF`
   *exactly*. The moment "Commit updated pins" ever successfully
   pushed a pin commit, `origin/main` became that commit - a child of
   `SOURCE_REF`, never `SOURCE_REF` itself again. Every subsequent
   retry hit this early guard and aborted *before* "Commit updated
   pins" ever got a chance to run its own, round-6 retry-recognition
   logic - the documented retry path was dead code from the moment it
   shipped. Confirmed live: reproducible by pushing a same-message pin
   commit to a scratch `main`, then re-running the same check.
2. Had the early guard been removed on its own, that round-6
   retry-recognition logic itself had a separate flaw: it compared the
   *file content* of `compose.release.yaml`/`.env.release.example`/
   `site/` at `origin/main`'s current tip against what the run had just
   regenerated - true for this release's own earlier pin commit, but
   equally true for any later, unrelated commit that simply never
   touched those three paths. An unrelated commit landing after a
   genuine pin commit would have been silently accepted as
   `pin_commit`, and the release tag would then point at an untested,
   unrelated commit - exactly the tag/image provenance mismatch bug
   fixed (repeatedly) in earlier rounds, reintroduced through a new
   door.

Replaced both checks with one shared, precise resolver:
`scripts/lib/resolve-release-pin-commit.sh`. Identifies the release's
own pin commit by its unique commit message (nothing else in the
system ever produces that exact string) rather than by content, then
independently verifies its shape - a direct, single-parent child of
`SOURCE_REF` (rejects merge commits and commits built on other,
untested content), touching only the three paths the real pin-commit
step ever writes (rejects a commit that happens to carry the right
message but touches something else too). A revert's auto-generated
message, which quotes the original as a substring, is also rejected -
the check is an exact-subject match, not a substring one.

The early guard now delegates to this resolver (empty result: ordinary
first attempt, unchanged strict behavior; non-empty: a verified retry,
proceed) and "Commit updated pins" consumes its result directly instead
of re-deciding the question a second, looser way. Exercised against
synthetic git histories covering every scenario the review named -
first attempt, retry right after the pin commit, retry after main moved
further still, a foreign commit touching the same paths under a
different message, a revert's substring-matching message, a merge
commit with the right message, a right-message commit built on the
wrong parent, and a right-message-and-parentage commit that also
touches an out-of-scope path - in
`scripts/lib/resolve-release-pin-commit.test.sh`, wired into `ci.yml`.

**Open, unchanged: `main` still has no branch protection or ruleset.**
Confirmed still true; see round 6's writeup above for the recommended
configuration. Still the repo owner's decision to make.

**Small, fixed: `semver-compare.test.sh`'s `source=` shellcheck
directive resolved to "does not exist" when linted from the repo root.**
The directive itself was correct; `shellcheck -x` needs its own
`-P SCRIPTDIR` flag to resolve a `source=` path relative to the linted
file's own directory rather than the invoking shell's cwd. Documented in
the file's own header comment - there is no shellcheck job in CI to fix
a wiring for.

## Follow-up review, round 8 (2026-08-30)

An eighth independent review of round 7's own fix - found a real
release-critical edge case round 7 didn't cover: identity is not the
same as currency. Same discipline as every round before.

**Releasekritisch, fixed: a reverted pin commit still resolved as a
safe retry.** Round 7's `resolve_release_pin_commit` proved a candidate
commit *was* genuinely this release's own pin commit - unique message,
direct single-parent child of `SOURCE_REF`, only the three expected
paths touched - but never checked whether `main`'s current tip still
*reflected* it. Git history is immutable: reverting a commit doesn't
rewrite it, it adds a new one on top, so every one of round 7's checks
still passed against the original, now-superseded commit. Concrete
scenario, confirmed live: pin commit P lands, promotion fails, a
maintainer runs `git revert P` to take the public pins back, a retry of
the same release run still resolves to P, and the workflow skips
straight past every content and main-race check on the strength of
that - publishing images, a git tag, a GitHub Release, and a Pages
deploy of the reverted site, all under pins `main` had explicitly taken
back.

Two related gaps closed alongside it, both confirmed live against the
pre-fix resolver the same way:
- A same-message `git commit --allow-empty` passed every round-7 check
  (there's nothing to be "out of scope" in an empty commit) while never
  actually pinning anything.
- A valid pin commit whose paths were changed again by any later,
  normal commit (not necessarily a revert) had the exact same
  resolve-to-the-superseded-commit problem.

Fixed inside `resolve_release_pin_commit` itself, so every caller gets
it automatically: after the existing identity checks, the candidate's
own tree for the three pin paths must still match `main_ref`'s *current*
tip - a pure git-ref comparison (no working-tree regeneration needed,
so it's safe to run from the early guard too, before pin files even
exist yet). Also now rejects a candidate whose diff against `SOURCE_REF`
touches neither `compose.release.yaml` nor `.env.release.example` at
all (the empty-commit case) - a genuine pin commit always changes both,
since `VERSION` (part of every image reference they pin) always differs
from whatever the previously published release used.

`release-alpha.yml` no longer passes "Resolve main state"'s answer
downstream - the window between it and "Commit updated pins" (buildx/
cosign setup, attestation checks, digest-pin edits, compose validation,
site refresh) is exactly the kind of gap a revert could land in, so
"Commit updated pins" now resolves fresh instead of trusting a stale
answer. "Resolve main state" itself is now purely a fail-fast: it still
aborts a doomed run early, but nothing downstream reads its result
anymore. "Commit updated pins" also gained one more, narrower check
`resolve_release_pin_commit` structurally can't make on its own: it has
no way to know what the *correct* pin content actually is (that depends
on this run's own digests, invisible to git-only logic) - so once a
candidate resolves, this step compares it against what "Update digest
pins" just regenerated, defense in depth against a hand-crafted commit
that happens to pass every structural check.

Exercised against synthetic git histories for every scenario named in
review - a real pin commit later `git revert`-ed, an empty
`--allow-empty` commit with the right message, a valid pin commit whose
paths were changed again afterward, and (confirming this was already
safe, not just adding coverage) a pin commit that exists in the repo
but was force-pushed out of `main`'s history entirely - in
`scripts/lib/resolve-release-pin-commit.test.sh`. Verified the first
three fail against the pre-fix resolver and pass against the fix.

**Small, fixed: the exact pin-commit message was defined three
times by hand** (the resolver, `release-alpha.yml`'s own commit, and
this file's own test), with a comment saying to keep them in sync by
hand. Extracted `release_pin_commit_message()` into
`resolve-release-pin-commit.sh` - one function, called from all three,
so a future text change can't silently break retry detection instead of
failing loudly.

**Small, fixed: the test file used fixed paths under `/tmp`**
(`/tmp/resolve-stderr`, `/tmp/resolve-stdout`) that a parallel test run
could collide on. Moved under the test's own `mktemp -d` work
directory, unique per run.

**Open, unchanged: `main` still has no branch protection or ruleset.**
Confirmed still true; the Actions PR-creation permission fixed in round
6 is now correctly enabled. See round 6's writeup for the recommended
ruleset configuration - still the repo owner's decision to make.

## Live CI investigation during round 8 (2026-08-30)

Not from a review pass - `ci.yml`'s "Verify Unbound configuration
lifecycle" step started failing three times in a row while verifying
round 8's PR, on a branch that touches nothing near the webapp/core/
Unbound runtime. Investigated per an explicit "find and fix the root
cause, don't just retry" instruction rather than dismissed as flake.

**Medium, fixed: applying Unbound settings could return success before
Unbound was actually ready to be used.** `applyStateLocked` (the shared
implementation behind `Apply`, `ApplyBundle`, `ApplyCustom`, and
`Restore`) wrote the new config and ran `docker restart` on the Unbound
container, then returned as soon as that command exited - `docker
restart` returning success only means the container *process* started
again, not that Unbound itself has finished starting (reading the trust
anchor, opening its `remote-control` socket). Any caller that
immediately did anything else against the running daemon could lose
that race. Confirmed exactly this live: `StartDiagnosticLogging`'s own
`unbound-control verbosity 2` call, issued by the CI test right after a
settings-apply restart, intermittently failed with a bare, unguarded
`curl --fail` exit (no error message reached the log at all) because
the control socket wasn't listening yet - reproducible three times
running under CI's own load, not a one-off. Fixed by polling
`unbound-control status` (the same interface every other live check in
this package already uses) for up to 10 seconds after the restart
before returning success; a restart that never becomes ready gets the
same automatic config rollback a `docker restart` failure or a bad
`unbound-checkconf` already got. Regression tests simulate the control
socket failing for a few polls then succeeding (must retry through it)
and never succeeding at all (must roll back, same as the existing
restart-failure test).

**Separately observed, not fixed: real external DNS resolution was
unreliable in the CI environment at the time of this investigation.**
After the fix above, the same `ci.yml` step still failed intermittently
- but past the point this fix covers, on an unguarded
`test "$(jq -r .healthy <<<"$diagnostics")" = "true"` whose input comes
from live `dig` queries to real internet domains (`example.com`,
`dnssec-failed.org`). Confirmed as external, not a code defect:
`main`'s own independent, unrelated scheduled `Unbound CI` run failed
the identical way (`dig ... dnssec-failed.org ...: communications error
... timed out`) at the same time, on three separate occasions,
including after a retry. Merged both this fix and round 8's PR with
this one check still red rather than block correct, independently
unit-and-race-tested code on a transient external condition also
affecting `main` itself.

## Follow-up review, round 9 (2026-08-30)

A ninth independent review - no critical security issue and no RC
blocker in the product code found this round; round 8's pin-commit fix
holds up completely. Three real, narrower issues remained, plus two
more found live during this round's own CI verification. Same
discipline as every round before.

**Medium, fixed: CI and release gates depended on real internet DNS,
independent of round 8's own investigation of the same class of
problem.** `ci.yml`, `ci-unbound.yml`, `ci-adguard-compat.yml`, and
`release-alpha.yml`'s smoke-test/upgrade-test jobs all queried real
internet domains (`example.com`, `dnssec-failed.org`, and
`ci-unbound.yml`'s own Docker `HEALTHCHECK` - `cloudflare.com`) directly
- a transient DNS/network hiccup on the runner's own connection then
fails a PR or a release for reasons that have nothing to do with the
code under test, exactly what round 8 already hit independently.
`/api/unbound/diagnostics` (the product's own diagnostics feature) is
correct to query the real internet by design - a real deployment
genuinely wants to know "can my resolver reach and validate the real
internet" - so the fix has to live in the test infrastructure, not in
that feature's real-world behavior.

Built a small, throwaway, signed DNS zone
(`scripts/ci/dnssec-test-zone/setup.sh`) instead: generates a fresh
DNSSEC keypair and signs `good.rgtest-ci.internal.`/
`bad.rgtest-ci.internal.` on every CI run (never committed - a checked-in
signed zone would eventually expire and break CI on its own, and
committing key material is bad practice regardless of how throwaway),
deliberately corrupts `bad.`'s own RRSIG afterward, and serves both from
a local `nsd` authority on the runner itself.
`scripts/ci/dnssec-test-zone/inject.sh` wires a given Unbound container
up to it - a forward-zone plus an inline trust-anchor for the zone,
reaching the runner's own authority via the container's own network
gateway IP (`docker inspect ... .Networks`, sorted for determinism -
Go template map iteration is otherwise random order, and every caller
here has more than one network). Verified end to end against the real
`rootguard-unbound` image on a scratch host before touching any
workflow: `good.` validates (the `ad` flag), `bad.` SERVFAILs.

For the one caller that goes through `/api/unbound/diagnostics` itself
(`ci.yml`, via Core, not a raw `dig`) rather than around it: added
`Manager.SetDiagnosticDomains` and the
`ROOTGUARD_UNBOUND_DIAGNOSTIC_RESOLUTION_DOMAIN`/`_DNSSEC_DOMAIN` test-
only escape hatches in `main.go` (same shape as the existing
`ROOTGUARD_SKIP_ATTESTATION`) - empty by default (real domains, a real
deployment's correct behavior unchanged), set only by `ci.yml`'s own job
env. `release-alpha.yml`'s smoke-test/upgrade-test never call that
endpoint at all (bare `dig` against the published DNS port throughout),
so they needed the local zone but not this override.

Real internet resolution is still exercised, just not as a blocking PR/
release gate - `ci-real-dns-upstream.yml` (new) runs the same checks
against the real internet on its own weekly schedule.

**Medium, fixed live during this round's own CI verification:
`waitReady` (round 8's own fix) checked the control socket, not the
actual DNS listener - the two don't necessarily finish binding at the
same moment.** While verifying the local-zone infrastructure above in
real CI, `ci.yml`'s own diagnostics check failed with the local zone
correctly wired up - `docker exec ... dig ...` against Unbound's own
port 5335 got a full "communications error ... timed out", not a slow
response, immediately after `unbound-control status` had already
reported the daemon up. `unbound-control status` succeeding is real, but
it's a weaker guarantee than "the DNS service itself is ready" -
`waitReady` now also polls a root NS lookup against the DNS port itself
(answerable straight from Unbound's own built-in root hints, no real
network round-trip needed, so this check isn't itself hostage to network
conditions) and only returns once both succeed. New test mirrors the
existing control-socket-retry test for this second dimension; verified
it fails against the round-8-only version and passes against the fix.
Real, but not sufficient on its own - see below.

**Medium, fixed live during this round's own CI verification (second
finding in the same failure): the original `host.docker.internal`
wiring path in `inject.sh` reached NSD via an address that couldn't
carry a correct reply back, and Unbound's own anti-spoofing hardening
silently dropped what NSD sent instead - the `waitReady` fix above was
real but not what was still failing.** After the fix above, the exact
same `ci.yml` step failed again, same symptom, same step, `waitReady`
having already passed moments earlier in the same restart. Reproduced
the whole sequence locally (fresh restart, PUT settings, diagnostic-
logging start/stop, diagnostics call) against the real image with
`unbound-control verbosity 4` and a host-side `tcpdump` running
throughout, rather than continuing to guess from CI log excerpts alone.
The capture showed Unbound sending its query to
`host.docker.internal`'s address (the *default* Docker bridge's
gateway) and getting a well-formed, correctly-sized reply back
*immediately* - but from a *different* source address (the custom
compose network's own gateway, since NSD is bound to `0.0.0.0:8053`
and Linux picks a UDP reply's source address by route-to-client, not by
which local address the query arrived on). Unbound `connect()`s its
outbound UDP sockets to the exact peer it queried specifically to
reject spoofed replies at the kernel level - so it silently discarded
every one of NSD's replies as if they'd never arrived, retried with
growing backoff, and gave up after ~10-15s with nothing sent back to
the client at all: not a slow answer, no answer. `dig`, lacking that
hardening, accepted the same replies without issue, which is why manual
verification with `dig` during earlier rounds never caught this.
Fixed by dropping `host.docker.internal` entirely and using the
container's own network gateway unconditionally (see above) - the one
address guaranteed to round-trip, since it's the address the host
actually uses to reach that specific container. Verified the same way:
reproduced the hang against the old wiring, then confirmed a cold-cache
query resolves in well under a second against the fix, on the exact
same host, same image, same sequence. Once `ci.yml` got past both of
the above, a *third*, unrelated real-domain dependency this round's
original sweep had missed surfaced in the same job for the first time:
"Verify DNS through AdGuard and Unbound" still dug `example.com`/
`dnssec-failed.org` directly through the full published-port pipeline
(AdGuard -> Unbound). Same fix as everywhere else - the local test
zone domains.

**Small, fixed: a valid SemVer version with a hyphen inside its
prerelease identifier could bypass the upgrade test.** Three sites in
`release-alpha.yml` each filter `git for-each-ref`'s tag list down to
"real SemVer tags", and two of them correctly hand-wrote SemVer's own
grammar (`[0-9A-Za-z-]+`, including the hyphen it allows *inside* a
prerelease identifier like `rc-hotfix`) - `upgrade-test`'s own
"Determine the previous published release" step hand-wrote the same
regex without that hyphen (`[0-9A-Za-z]+`), so a real, valid tag like
`v1.2.3-rc-hotfix.1` was invisible to it. That step then either picked
an older release than the one actually directly before this one, or (if
no other tag happened to match) skipped the upgrade test outright -
either way, silently, with no error. Fixed all three sites to share
`$SEMVER_PATTERN` (from `scripts/lib/semver-validate.sh`, already
sourced for `require_semver`) instead of a third hand-written copy, so
a future grammar change can't silently re-diverge the same way again.
New `scripts/lib/semver-validate.test.sh` covers `require_semver` and
the exact `grep -E "^v${SEMVER_PATTERN#^}"` construction all three
sites use, including the live regression case; wired into `ci.yml`.
Verified the old hand-written pattern rejects `v1.2.3-rc-hotfix.1` and
the shared pattern accepts it.

**Small, fixed: `resolve_release_pin_commit` compared `SOURCE_REF` as a
raw string, rejecting an abbreviated SHA that points at the exact same
commit.** `release-alpha.yml` always passes the full 40-character SHA,
but the one documented manual-dispatch escape hatch
(`workflow_dispatch`'s `source_sha` input) lets a human type a short one
instead - still the same commit, but a string-equality comparison
against `origin/main`'s (always full) tip or a candidate's (always
full) parent would never match it. Normalized once, up front, via
`git rev-parse "${source_ref}^{commit}"`, so the rest of the function
never has to think about it again. Two new synthetic-history scenarios
(first attempt and retry, both with an abbreviated `SOURCE_REF`) added
to `resolve-release-pin-commit.test.sh`; verified both fail against the
pre-fix resolver and pass against the fix.

**Medium, fixed: the Unbound settings rollback could fail silently
under an already-canceled request context.** `rollbackFailedApply`
restarted the container (and, since round 8's readiness fix, waited for
it) using the *same* `ctx` `Apply` itself was called with. If that ctx
is what's canceled - the realistic case, e.g. the HTTP request that
triggered `Apply` was aborted by the client mid-flight - the rollback's
own `docker restart` could be killed or refused to even start entirely,
leaving the already-restored *files* out of sync with whatever Unbound
is actually still running. A rollback must not be at the mercy of
whatever canceled the operation it's cleaning up after. Fixed by
detaching the rollback's own restart-and-wait from the original
context's cancellation (`context.WithoutCancel`, plus its own bounded
30-second timeout so it still can't hang forever if something is
genuinely stuck) rather than inheriting it. New regression test
confirms: given an already-canceled input context, the rollback restart
still runs to completion and the previous settings end up active.
Also added a sibling test proving `rollbackFailedApply` reports honestly
- not "previous configuration restored" - when the rollback restart's
own readiness never arrives either, a case the existing test suite
hadn't separately covered.

## Follow-up review, round 10 (2026-08-31)

A tenth independent review - no critical security issue found this
round; round 9's local DNSSEC test zone and its live-found fixes hold up
completely. Same discipline as every round before: each finding verified
directly against the current code before being counted.

**Medium, fixed: two more real-internet-DNS blocking-CI dependencies
round 9 missed.** Round 9's own sweep converted `ci.yml`/`ci-unbound.yml`
to the local test zone but missed two spots: `scenario_integration_test.go`
(`TestScenarioHomeNetwork`, `TestScenarioSplitDNS`,
`TestScenarioBrokenUpstream`, `TestScenarioDNSSECFailures` - `example.com`,
`cloudflare.com`, `1.1.1.1` directly) and `verification-common.sh`'s
`verify_dns` (`example.com`/`dnssec-failed.org`, shared by
`verify-clean-install.sh` and `verify-backup-restore.sh`, run by
`clean-install.yml`/`backup-restore.yml`). Same failure mode as round 9's
own finding: a transient DNS hiccup on the runner fails the build for
reasons unrelated to the code under test.

Fixed the scenario tests by pointing every "external/unrelated domain"
check at `good.rgtest-ci.internal` (already wired up for every scenario
test via `wireUpLocalDNSSECTestZone`) instead. `TestScenarioSplitDNS`
needed its own reachable forward target distinct from
`rgtest-ci.internal` itself - forwarding that zone wouldn't have proven
anything, since `inject.sh`'s own base config already forwards all of it
before any scenario's settings are ever applied, so it would resolve
identically whether or not `Settings.Render`'s `ForwardZone` handling
actually worked. `setup.sh` now also starts a second, deliberately
*unsigned* throwaway authority (one record,
`split.rgtest-split.internal.` → `203.0.113.50`), never forwarded by
`inject.sh`'s own base wiring - so only the scenario's own guided
`ForwardZone` setting makes it resolve.

Two live failures while building that, both caught by this PR's own CI
before merging, not after. First: pointed the scenario's `ForwardZone`
at the DNSSEC authority's existing `nsd` instance via an `"ip@8053"`
server string - `Settings.Render()` calls `Settings.Validate()` first,
which requires `forward_zones[].servers[]` to be a bare canonical IP
with no port suffix (`@port` is Unbound raw config's own `forward-addr`
extension, which `inject.sh`'s base wiring uses directly, not something
the guided-settings API accepts) - failed immediately with "must be a
canonical IPv4 or IPv6 address". Fixed by giving that same `nsd` a
second bind, `0.0.0.0@53` alongside the existing `0.0.0.0@8053`, so the
split zone would be reachable at the standard port a bare guided-
settings IP always implies. Second: that second bind then failed nsd's
own startup outright - `can't bind udp socket 0.0.0.0@53: Address
already in use` - GitHub's own runners already have something bound to
the host's port 53. Fixed for real by moving the split zone to its own
throwaway Docker container instead (`alpine:3.20` + `nsd`, plain
`docker run`, no host port published) - its port 53 lives entirely
inside that container's own network namespace, so the host's port 53
being taken is irrelevant, and it's directly reachable from another
unnetworked container (as the Go scenario tests' own container is) via
Docker's default bridge. `setup.sh` resolves and writes that container's
IP to `$OUT_DIR/split-authority-ip`; the Go test reads it directly as
the bare guided-settings target.

Fixed `verify_dns` by making both domains configurable
(`ROOTGUARD_VERIFY_DNS_DOMAIN`/`ROOTGUARD_VERIFY_DNS_DNSSEC_FAIL_DOMAIN`,
defaulting to the real domains so any other caller's behavior is
unchanged) and adding a shared `wire_local_dnssec_test_zone` helper that
runs `inject.sh` against the running `rootguard-unbound` container and
waits for it to report healthy again. `clean-install.yml` and
`backup-restore.yml` now start the local test authority
(`scripts/ci/dnssec-test-zone/setup.sh`) before their verify step and
point both env vars at `good`/`bad.rgtest-ci.internal`;
`verify-backup-restore.sh` calls `wire_local_dnssec_test_zone` twice -
once for the primary instance, once more for the freshly-restored one,
since restore deploys an entirely new `rootguard-unbound` container that
needs its own wiring.

**Small, fixed alongside the above (same file, same review pass):
`setup.sh`'s own authority-readiness loop only checked `dig`'s exit
code, and it unconditionally `pkill nsd` on every run.** An exit code
alone can't distinguish a real answer from an empty NOERROR, REFUSED, or
NXDOMAIN response - any of which would have let the loop declare the
authority "ready" before it could actually serve what a caller expects.
Now checks the actual resolved address against what `good.rgtest-ci.internal`
and `split.rgtest-split.internal` are supposed to return. Separately,
`pkill nsd` would kill *any* `nsd` process, not just this script's own -
harmless on a GitHub-hosted, single-purpose, ephemeral runner, but a real
hazard on a shared self-hosted runner or a developer's own machine
running this locally. Now only stops a leftover `nsd` identified by its
own pidfile under this exact test directory, and only after confirming
via `ps` that the PID still actually belongs to a process running against
this script's own `nsd.conf` - a recycled PID now owned by an unrelated
process is left alone.

**Small, fixed: `release-alpha.yml`'s own `source_ref` was the raw,
possibly-abbreviated dispatch input, not the resolved full SHA.** Round
9's fix normalized `resolve_release_pin_commit`'s own string comparison
against an abbreviated `SOURCE_REF`, but the workflow's `source_ref`
*output* itself - what every job's checkout, the candidate tag, the OCI
revision label, and the `origin/main` equality check all consume - was
still `inputs.source_sha || github.sha` verbatim. `release-version-bump.yml`
always passes a full 40-character SHA, so the automated path was never
actually affected, but the input's own description explicitly allows a
manual by-hand dispatch to type an abbreviated one instead - and every
later full-SHA comparison in this file would then silently never match,
surfacing late (at the pin-commit step, after the candidate images are
already built, scanned, and smoke-tested) instead of failing fast.
Resolved once, via `git rev-parse HEAD` right after the initial checkout,
and recorded as `steps.release.outputs.source_ref`; every other job
already consumed `needs.version.outputs.source_ref` rather than the raw
input, so fixing it in this one place is sufficient.

**Small, cleaned up: two leftover hazards in the frontend, unrelated to
each other but both found in the same pass.** `vite.config.ts`'s dev
proxy hardcoded one developer's own LAN IP (`10.100.0.2`) as its target
- `npm run dev` only ever worked out of the box on that one machine,
silently proxying nowhere (`ECONNREFUSED`) for anyone else. Now defaults
to loopback (where WebApp listens locally by default), overridable per
machine via `VITE_API_PROXY_TARGET`. Separately, `public/vite.svg` and
`src/assets/react.svg` - the framework's own default scaffold assets -
were never referenced anywhere in the app (confirmed via a repo-wide
grep across `.html`/`.ts`/`.tsx`/`.css`/`.json`) and were removed.

**Small, fixed: three pairs of identically-named jobs across unrelated
workflows, each producing an unnamed check run with the same generic
name.** GitHub identifies a status check by its name (plus reporting
app) - two unrelated workflows each contributing a check called "Linux
amd64" (`backup-restore.yml`/`clean-install.yml`) or "test"
(`ci-core.yml`/`ci-webapp.yml`/`ci-updater.yml`) or "build"
(`ci-blockpage.yml`/`ci-updater.yml`/`ci-webapp.yml`) is at best
confusing in the Checks UI or `gh pr checks` output (confirmed live,
this round: briefly mistook a `backup-restore.yml` "Linux amd64" failure
for `clean-install.yml`'s own job while debugging an unrelated finding)
and at worst ambiguous as a required-status-check entry, which
identifies checks by name alone. Named every one of the six jobs
explicitly and distinctly: `Core unit tests`, `Updater unit tests`,
`WebApp backend and frontend tests`, `Build and push blockpage/updater/
webapp image`, `Backup and restore, Linux ${{ matrix.arch }}`, `Clean
install, Linux ${{ matrix.arch }}`. Branch protection's own required-
status-check list (configured directly via the GitHub API, not
committed to this repo) needs its `test` entry updated to the three new
names in the same change that merges this - tracked as a manual
follow-up immediately after merge, not automatable from inside a PR.

Confirmed live, this same round: merging PR #446 (`release-alpha.yml`
only, no `rootguard-core/**`/`rootguard-webapp/**`/`rootguard-updater/**`
touched) hit exactly the failure mode round 10's finding 9 warns about -
branch protection's required `test`/`validate`/etc. contexts include
checks whose owning workflow is path-filtered and never triggers for
every PR, so GitHub reported `mergeable_state: blocked` indefinitely
rather than merging. Required an explicit admin-bypass merge. A single
always-on aggregating "merge gate" job (finding 9's own recommendation)
would close this for good; not implemented this round given the scope of
restructuring it needs - recorded here as the concrete case for doing it
next.

**Small, fixed: `inject.sh`'s gateway auto-detection has no escape hatch
for Docker Desktop.** The container-own-network-gateway approach round 9
built and verified is specific to how Docker's Linux bridge networking
routes container-to-host traffic - correct on the native-Linux GitHub
runners this actually runs on in CI, but the same detection doesn't
resolve to anything reachable from inside the container on Docker
Desktop (macOS/Windows), where the Docker daemon runs inside its own VM
behind a different network layer. That made the local DNSSEC scenario
unreproducible on a developer's own Docker Desktop machine - not a
production bug (nothing here ever runs against Docker Desktop outside
local reproduction), but worth fixing for local debuggability. Added a
`DNSSEC_TEST_AUTHORITY_IP` override: when set, `inject.sh` uses it
directly instead of auto-detecting; CI itself never sets it, so the
already-verified Linux path is unaffected.

**Small, fixed: `waitReady`'s retry loop slept once more than it ever
needed to.** It unconditionally waited `unboundReadyInterval` between
every attempt, including after the very last one - a delay nothing
downstream ever consumes, since the loop is about to give up and return
an error either way. Not a bug (every existing readiness/rollback test
still passed), just a fixed, pointless delay tacked onto every readiness
timeout. Now breaks out before that final sleep. New
`TestWaitReadySkipsTheFinalSleep` counts `sleep` calls directly against
a daemon that never becomes ready: exactly `unboundReadyAttempts-1`, one
fewer than the number of attempts made.

## Follow-up review, round 11 (2026-08-31)

An eleventh independent review - no critical production or security
issue found this round. Same discipline as every round before.

**Medium, fixed: required branch-protection status checks and their own
`pull_request.paths` filters directly contradicted each other.** Round
10's own finding 9 (recorded above) predicted this and it recurred twice
more within the same round: merging PR #446 and then PR #449 each hit
`mergeable_state: blocked` - `mergeable: true`, no actual conflict, just
GitHub waiting forever for a required check whose owning workflow's path
filter meant it would never trigger for that particular PR. Round 10's
fix (naming every job distinctly) made the problem *more* visible, not
less: before that round, `ci-core.yml`/`ci-webapp.yml`/`ci-updater.yml`
happened to share one required "test" context, so any one of them
reporting satisfied it; after distinct names, each of "Core unit tests",
"Updater unit tests", "WebApp backend and frontend tests" needs its own
workflow to actually run. Fixed the fast, robust way for now (this
round's own suggestion): dropped the `pull_request.paths` filter on all
three - they run on every PR unconditionally now (each well under a
minute, cheap insurance), `push.paths` on `main` is untouched. The
proper fix - one always-on job that aggregates exactly the required
checks for whatever paths a PR actually touched - is still open; this is
the stopgap that stops every unrelated PR from needing an admin-bypass
merge in the meantime. (PR #448 needed the same admin-bypass merge a
third time before this fix landed - same root cause, same missing
"Updater unit tests"/"WebApp backend and frontend tests" checks.)

**Small, fixed: `setup.sh`'s split-DNS authority readiness check ran
from the host against the container's own bridge IP - unreachable from a
Docker Desktop host.** Round 10's `DNSSEC_TEST_AUTHORITY_IP` override
only fixed `inject.sh`'s container-to-host gateway lookup; the split
authority's own readiness loop (added in the same round, `setup.sh`) has
a different problem entirely - it `dig`s the container's bridge IP
*from the host*, which works on Linux (this script's own supported
platform - now stated explicitly in its own header comment, since it
also runs plain `apt-get`/GNU `date -d` with no portability layer at
all) but not from a macOS/Windows Docker Desktop host, where the Docker
daemon runs inside its own VM behind a network layer the host itself
can't reach into. Fixed by checking readiness from *inside* the
container instead (`docker exec ... dig @127.0.0.1`, `bind-tools` added
to its `apk` install) - needs no host-to-container routability at all,
so it works the same regardless of host OS. The container's bridge IP
is still resolved separately afterward for the Go scenario tests' own
container to use - that one only ever needs container-to-container
reachability, which stays inside the Docker daemon's own network
regardless of which OS that daemon runs on, so it was never actually
broken.

**Small, fixed alongside the above (same file, same review pass): the
split authority container had none of the reproducibility/ownership
safeguards this round's own `nsd`-pidfile fix already established for
the host authority.** Pinned `alpine:3.20` to its current manifest-list
digest (the tag alone is mutable - Alpine rebuilds patch releases in
place) and labeled the container (`io.rootguard.ci=dnssec-split-authority`),
checking that label before ever removing a same-named leftover - the
identical "only ever touch something verifiably this script's own"
reasoning already applied to the `nsd` pidfile check, now applied
consistently to this container too.

**Small, fixed: `scenario_integration_test.go` ignored `DNSSEC_TEST_ZONE_DIR`
even though `setup.sh`/`inject.sh` both honor it.** A developer running
`setup.sh` by hand with that variable set would have it write the local
authority to a custom directory while this test kept looking in the
hardcoded default - a real, if narrow, local-repro gap (every CI job
that runs this package's tests leaves the variable unset, so nothing in
CI was ever affected). Changed the constant to a package var resolved
from the environment once, at init, falling back to the same default
either script uses unset.

**Small, cleaned up: `TestMain` didn't do what its own comment claimed.**
Its comment described managing container lifecycle "here rather than in
a package-level init"; the function body was `os.Exit(m.Run())` - Go's
own default test-binary entry point does exactly that already when no
`TestMain` is defined, and the real per-test container lifecycle lives
in `startScenarioContainer`, not here. Removed the now-inaccurate,
functionally redundant override.

**Small, fixed: the frontend had no declared minimum Node version, and
already needed one.** `react-router@8.3.0`'s own `package.json` requires
`node >= 22.22.0`; nothing in this repo said so, so an older Node 22
(confirmed live: 22.17.0) installs and builds with only a silent
`EBADENGINE` warning easy to miss - not a failure, just a real version
floor with nothing enforcing or even documenting it. Added a matching
`engines.node` to `rootguard-webapp/frontend/package.json` (the exact
value read directly from `react-router`'s own installed `package.json`,
not guessed) and a `.nvmrc` (`22`, matching `ci-webapp.yml`'s own
`setup-node` `node-version: 22`) so a version manager picks up the right
major version automatically.

**Medium, closed: round 10 finding 9 (Backup/Restore and Unbound never
part of the required checks).** Requested as "one always-on aggregating
merge-gate job" - implemented as the equivalent of that instead, for a
concrete reason: this session has no way to trigger and observe a real
GitHub Actions run before merging, and a multi-workflow `workflow_call`
orchestrator (the textbook way to build that single job) has enough
moving parts - job-name remapping through the caller, `needs`/`if`
semantics across a reusable-workflow boundary - that getting it wrong
would only surface once already merged. The already-proven mechanism
from this same round (drop the `pull_request.paths` filter, so the
check always triggers) reaches the identical branch-protection outcome
with far less new surface: `backup-restore.yml` and `clean-install.yml`
now trigger unconditionally on `pull_request` (their jobs are cheap -
~3-4 and ~1-1.5 minutes respectively, both arches in parallel), added to
required status checks. `ci-unbound.yml`'s `test`/`scenario-tests` jobs
do the same (~2-3 minutes) - its `build-push` job deliberately does
*not* go unconditional alongside them: it was never required and still
shouldn't be, so a new `detect-unbound-changes` job (a plain `git diff`
against the PR's base SHA, no third-party paths-filter action) gates it
back to only actually building when a PR touches something
unbound-related, preserving today's behavior instead of adding a ~20-
minute build to every unrelated PR. `updater-rollback-integration`
(`ci-core.yml`) and `integration` (`ci-updater.yml`) needed no workflow
change at all - round 11's own path-filter removal on those two
workflows already made them unconditional; only branch protection's
required-checks list itself was missing them. Required status checks
now: `validate`, `check`, `gitleaks`, `trivy`, the three `go-security`
legs, `Core unit tests`, `Updater unit tests`,
`WebApp backend and frontend tests`, `updater-rollback-integration`,
`integration`, `Backup and restore, Linux amd64`/`arm64`,
`Clean install, Linux amd64`/`arm64`, `Test amd64`/`arm64`, and
`Guided-settings scenario tests`. The real single aggregating job is
still the cleaner end state and stays open as a future improvement, not
a correctness gap - every one of these checks now triggers
unconditionally, so the actual bug finding 9 and this entry both
describe (a required check that silently never fires) cannot recur for
any of them.

## Follow-up review, round 12 (2026-08-31)

**Low, fixed: the DNSSEC test harness's Alpine pin had passed its own
end of support.** `dnssec-test-zone/setup.sh`'s split-authority
container was pinned (by digest, round 11) to `alpine:3.20` - correct
practice for reproducibility, but 3.20 itself reached the end of its
regular support window on 2026-04-01 (confirmed against Alpine's own
release-branches table live) and now only gets patches "on request".
Repinned to `alpine:3.24@sha256:28bd5fe8b...` - the current release
with the longest support horizon (main through 2028-06-01). While
verifying this, the review's second observation checked out too: the
two integration-test fixture images
(`rootguard-core/internal/updater/testdata/fixture/Dockerfile`,
`rootguard-updater/integration/fixture/Dockerfile`) referenced
`alpine:3.23`/`golang:1.26-alpine` by tag only, with none of the
reproducibility rationale that motivated digest-pinning everything
else in this repo (see round 8's audit-log entries on that). Both are
now pinned by digest too, `alpine:3.23` left in place since it's still
within its own support window (main through 2027-11-01) - only the
missing digest was the gap.

**Low, fixed: the frontend's Node version floor wasn't fully wired
up.** Round 11 added `"engines": {"node": ">=22.22.0"}` to
`rootguard-webapp/frontend/package.json` (react-router@8.3.0's own
stated minimum), but two things were still out of step with it: the
committed `package-lock.json` predated that change and didn't carry it
in its own root-package metadata (`npm install` against the current
`package.json` regenerates exactly the three lines - `engines` -
`package.json` declares, confirmed live, no dependency or version
changes otherwise), and `.nvmrc` only pinned the major version (`22`),
which lets an already-installed Node as old as 22.0.0 through -
tightened to `22.22.0` to actually match the floor it's meant to
communicate. `npm run lint`/`npm run test` both still pass unchanged.

**Low, fixed: `ci-real-dns-upstream.yml`'s own failures were
undiagnosable.** Found live, matching the review: this workflow's dig
exit code 9 failure on its own schedule was expected (it deliberately
checks the real internet, see this workflow's header comment on why)
but genuinely uninformative - both checks assigned dig's output via a
bare `var="$(dig ...)"`, and under this shell's default `set -e`, dig's
own non-zero exit (9 here) killed the step at that exact line, before
the following `grep` that would say which check failed ever ran. The
log showed nothing beyond a bare non-zero exit: not which domain
(example.com vs dnssec-failed.org), not dig's own error text (only
`2>&1` captures that; the old code only kept stdout), not the resolver
status. Rewrote the step to run both checks unconditionally (`|| true`
on each dig call, `2>&1` to keep dig's real error text, `::group::`
blocks so each check's own output is visible even on success), collect
failures into one `fail` variable, and `exit "$fail"` at the end -
verified locally against a stand-in for a failing dig call that the
rewritten logic reports the failing domain, dig's real error text, and
still exits non-zero. This workflow still never runs on `pull_request`
(schedule/`workflow_dispatch` only) and was already outside branch
protection's required checks - this is a diagnostics fix, not a
blocking-behavior change.

## Follow-up review, round 13 (2026-08-31)

**Medium, fixed: no warning existed for a host Docker Engine vulnerable
to two `docker cp` CVEs RootGuard's own code path relies on.** Confirmed
live against Docker's own release notes and the upstream advisories:
CVE-2026-41567 (arbitrary host-binary execution via `PATH` resolution
during `docker cp` archive decompression) and CVE-2026-42306 (a TOCTOU
race letting `docker cp` redirect a bind-mount target to an arbitrary
host path) were both fixed upstream in Docker Engine 29.5.1 - and
RootGuard itself calls `docker cp` in three places (backupexport,
backuprestore, updater rollback), so a host running an older, unpatched
Engine is a real exposure, not a theoretical one. `Preflight` only ever
checked Docker's *reachability*, never its patch level. Added a new
advisory check (`docker_engine_cp_cve`) that parses the Server version
already fetched for the reachability check and warns when it reads
unambiguously below 29.5.1 - deliberately never failing `Ready`, per the
review's own explicit caution: a distro package (Debian/Ubuntu's
`docker.io`, e.g.) can backport this fix while still reporting an
older-looking upstream version string, so blocking on the version number
alone would produce real false positives. A version string that doesn't
parse as a plain `MAJOR.MINOR.PATCH` (any distro-suffixed one) is treated
identically to "already patched" for the same reason. Added a new
`Check.Level` field (`omitempty`, every existing check leaves it unset)
so the frontend can render this distinctly from a real pass/fail -
amber, not green, and its action text now shows even though `ok` stays
true, which the existing `!check.ok &&` render guard would otherwise have
hidden. Documented the same requirement in `platform-support.md`
directly, per the review's ask to require a patched Engine in the docs
too. Covered by two new `installer` package tests (the warning firing for
an old clean version, and *not* firing for a patched version, a newer
major, and an unparseable/suffixed one).

**Low, fixed: Core and Updater's shared `docker:29-cli` runtime pin
carried a fixed OpenSSL CVE and a genuinely unused CLI plugin.** Verified
live: this exact pinned digest's baked-in `libssl3`/`libcrypto3` sit at
3.5.7-r0, which has CVE-2026-14456 - already fixed at 3.5.8-r0 and
already published on Alpine 3.24's own `main` repo (confirmed via
pkgs.alpinelinux.org), just not yet what this particular digest's own
image layer contains. Added an explicit
`apk add --no-cache --upgrade libssl3=3.5.8-r0 libcrypto3=3.5.8-r0` to
both Dockerfiles as a stopgap, meant to be replaced by a plain digest
bump once upstream publishes a `docker:29-cli` build with the fix baked
in. Separately, confirmed (via the upstream docker-library/docker
Dockerfile this digest is built from, and a repo-wide grep of every
`docker ...` invocation this codebase's own code and CI make) that the
image's bundled buildx CLI plugin
(`/usr/local/libexec/docker/cli-plugins/docker-buildx`) is genuinely dead
weight, not a stopgap - nothing at runtime ever calls `docker buildx`;
only this repo's own `release-alpha.yml` does, on the GitHub runner's own
Docker CLI, entirely unrelated to this image. Removed it from both
images with `rm -f`, a real content reduction rather than something a
future digest bump would need to add back.

**Medium, fixed: no CI job ever scanned a *built* container image.**
`ci-security.yml`'s trivy job only ever ran `trivy fs .` - repo files,
dependency manifests, and Dockerfile misconfigurations - never a built
image's actual base-layer content. Found in review, confirmed by the
review's own live scan: the pinned `docker:29-cli` runtime base
(Core and Updater's shared runtime, see their Dockerfiles) carried 35
HIGH-severity package findings across 14 distinct CVEs at scan time,
none of which any CI job would ever have caught. Added
`scripts/ci/trivy-image-scan.sh`, called from `ci-core.yml`,
`ci-updater.yml`, and `ci-webapp.yml` right after each workflow's own
`docker build`, scanning the exact image content a real PR/release would
ship. Updater and WebApp's `test` jobs previously had no single-platform
build step to scan at all - their only image build lived in the `build`
job's multi-arch `docker/build-push-action` step, which (a) never
executes for real on a PR (`push: false` discards a multi-platform
result entirely - there is nothing left to scan) and (b) couldn't be
loaded locally to scan even if it did (a multi-arch manifest can't be
`--load`ed into the local daemon). Added a plain single-platform
`docker build` step to both `test` jobs specifically to give trivy
something real to scan, mirroring `ci-core.yml`'s own `test` job, which
already built (but never scanned) its image this way.

**High, fixed: the pinned cosign binary itself carried 39 HIGH findings,
including a real signature-verification bypass - found live by the new
trivy-image-scan.sh, not by the review.** Running the new scan (round 13
finding 1, PR #460) against a real `rootguard-updater:test` image before
it had PR #459's fixes surfaced five separate targets inside the image,
not just the `docker:29-cli` base the review's own scan covered:
`usr/local/bin/cosign` alone reported 39 HIGH findings - mostly the same
Go-stdlib CVEs `usr/local/bin/docker` has (cosign v3.0.6 embeds the same
stale toolchain), plus several sigstore/fulcio-specific ones. Checked
upstream: three cosign releases exist past v3.0.6 (v3.1.1, v3.1.2,
v3.1.3), and v3.1.3 fixes GHSA-fx35-mq7g-6g98, a real signature-
verification bypass via an unexpected public key in a legacy bundle -
directly relevant here, since this exact binary is what
`stack.RequireAttestation` (`rootguard-core/internal/stack`) calls at
deploy time to verify a published image's attestation before trusting
it. Bumped the pin to v3.1.3 (by digest) in both Dockerfiles and in
`release-alpha.yml`'s own `cosign-installer` step, which verifies a
release candidate's attestation before promotion using the identical
version - the two were already required to move together (see the
existing comment there) and previously both said v3.0.6.

**Closed out: the new image scan's remaining findings, verified against
a real post-fix build.** With every actionable round-13 fix landed
(PRs #458, #459, #461), re-ran `trivy-image-scan.sh` against a fresh
`rootguard-updater:test` build: Alpine's own findings are now 0, the
buildx plugin's 13 are gone with the file itself, and cosign's 39 dropped
to 13. What's left - 33 HIGH findings across 14 distinct CVE IDs, spread
across the docker CLI binary, its compose plugin, and cosign itself - is
genuinely not fixable by RootGuard today: 12 of the 14 are Go-stdlib CVEs
baked into the upstream Go toolchain each of those three binaries was
built with (not something a version bump can fix - even cosign's newest
release, v3.1.3 from 2026-08-06, predates the Go release carrying the
fix, 1.26.6 from 2026-08-13), and the remaining two
(CVE-2026-41567/CVE-2026-42306, the same `docker cp` CVEs the new
Preflight advisory covers at the host-Engine level) are present only in
the compose plugin's *vendored* docker client library, which compose's
own binary never actually calls into (compose doesn't implement or
expose `docker cp`). Added all 14 to `.trivyignore.yaml`, each dated
2026-11-30, so this doesn't stay silently suppressed once a newer
upstream build of any of the three exists. One more surfaced only after
that re-run: bumping cosign to v3.1.3 (this round's own fix, above) pulled
in a newer `google.golang.org/grpc` that itself has one known HIGH
finding (GHSA-hrxh-6v49-42gf, fixed at grpc-go 1.82.1) - not present in
v3.0.6's own dependency tree, not yet fixed in any cosign release
(confirmed live: v3.1.3 is still the newest). Added with the same dated,
justified pattern.

## Follow-up review, round 14 (2026-08-31)

**High, fixed: Blockpage's base image carried 9 CVEs across 13 findings,
none caught by any CI job.** Found in review: `trivy-image-scan.sh`
(round 13) only runs for Core/Updater/WebApp - Unbound and Blockpage,
the other two images the release pipeline actually publishes, were
never scanned at all. Verified live by scanning the currently-published
`ghcr.io/foxly-it/rootguard-blockpage:latest` directly (no local Docker
daemon available this session either): exactly 13 HIGH findings across
9 CVE IDs and 8 packages (c-ares, curl, libcurl, libcrypto3, libssl3,
libexpat, libxml2, nghttp2-libs), matching the review's own numbers
exactly. Confirmed live that `nginx:1.29-alpine` (the pinned base) is
already the current tag - a plain digest bump doesn't help here, since
this pin already *is* the newest available build of it. Pinned all 8
packages to their actual current version on Alpine 3.23's own `main`
repo (confirmed live via pkgs.alpinelinux.org) - not always trivy's own
reported "fixed version" field: expat and nghttp2 have both moved past
their respective CVE fixes since trivy's vulnerability DB snapshot
(2.8.3-r0/1.69.0-r0 vs. trivy's 2.8.2-r0/1.68.1), so pinning to trivy's
literal field would have under-shot what's actually available. Same
stopgap pattern as docker:29-cli's own OpenSSL pin (round 13): replace
with a plain digest bump once upstream ships a build with these baked
in. Also wired `trivy-image-scan.sh` into `ci-blockpage.yml`'s `build`
job, so this same PR's own CI proves the fix (Unbound's identical
wiring is its own separate PR, alongside its own package fixes).

**High, fixed: Unbound's base image carried 58 HIGH/CRITICAL findings
across 24 CVEs, none caught by any CI job.** Same root cause as
Blockpage's own entry this round: `trivy-image-scan.sh` never covered
Unbound either. Verified live by scanning the currently-published
`ghcr.io/foxly-it/rootguard-unbound:latest`: 58 findings across 24 CVE
IDs, matching the review's own numbers exactly - including the review's
own headline finding that 36 of those 58 come from just 4 CVEs
(CVE-2026-53612..53615) spread across the 9 binary packages the
util-linux source package builds. Confirmed live via the Debian
Security Tracker: Debian fixed all 4 in trixie at 2.41.5-0+deb13u1
(already the current trixie version, not just trixie-security).
Explicit `apt-get install` pin for all 9 packages closes that.

The remaining 20 CVEs were each checked individually, live, against the
Debian Security Tracker - not assumed, per the review's own explicit
caution against a blanket ignore. Every one is genuinely unfixed in
trixie today: Debian's own tracker marks each `<no-dsa>` ("minor
issue"), "postponed" ("wait for regressions upstream sorted out"), or
notes the fix ships "first in unstable, then a point release" - none
have a trixie-stable fix to pin to the way util-linux did. Checked what
actually pulls in the more surprising packages rather than assuming: `dig
+deps` (`packages.debian.org` for `bind9-libs`, confirmed live) is what
brings in `liblmdb0` and this image's second copy of `libxml2` - Unbound's
own resolver process never touches either, only the `dig`/`host` tools
this Dockerfile installs for health checks and diagnostics. `perl-base`
is Debian's own base-install component; nothing in this image ever
invokes perl. Added all 20 to `.trivyignore.yaml`, each scoped to its
exact Debian package via `purls` (not left global - see this round's
separate finding on why that matters) and dated 2026-11-30. Two bugs
surfaced only once this PR's own CI actually ran the new scan on this
image: `trivy-image-scan.sh` hardcoded the amd64 trivy binary/checksum,
which fails with "Exec format error" on `ci-unbound.yml`'s own arm64
matrix leg - the only caller with an arm64 runner; `uname -m` now picks
the right release asset. And CVE-2025-69720's ignore entry only scoped
`libtinfo6`, missing that the same CVE also hits `ncurses-base`/
`ncurses-bin` in this image - added both.

**Medium, fixed: round 13's own trivy ignores had no `paths`/`purls`,
so each applied globally - and three of their statements were factually
wrong.** Confirmed live against trivy's own documentation: an entry with
neither field "is applied to all files"/"all packages" - so, e.g., the
same CVE ID would have been silently hidden even if it later showed up
in a genuinely reachable RootGuard binary or an unrelated image, not
just the specific package these entries were written for. Added `purls`
to every one of the 16 round-13 entries, scoping each to the exact
package trivy reported it against (`pkg:golang/stdlib` for the genuine
Go-stdlib ones, `pkg:golang/github.com/docker/docker` for the two
docker-cp CVEs, etc.). Also confirmed live, individually, against each
CVE's own advisory: CVE-2026-56852 is a `golang.org/x/text` CVE, and
CVE-2026-56864/CVE-2026-56865 are `golang.org/x/mod` CVEs - none of the
three are Go standard-library code the way the original statements
claimed, so a Go-toolchain bump alone doesn't fix them; cosign/compose
would need to bump their own vendored `golang.org/x/mod`/`x/text`
dependency instead. Corrected all three statements. (The
`trivy-image-scan.sh` arm64 bug this same review pass found is covered
above, in Unbound's own entry - discovered and fixed there first.)

**Low, fixed: the docker-cp preflight advisory (round 13) only named
two of the three CVEs Docker Engine 29.5.1 actually fixed.** Confirmed
live against Docker's own 29.5.1 release notes: CVE-2026-41568 (a
second, separate TOCTOU race letting a container create empty
files/directories at an arbitrary host path) is fixed in the exact same
release as the two already covered, listed alongside them in Docker's
own notes - a real gap, not a different-severity omission. The version
threshold (29.5.1) was already correct and needed no change. Updated the
advisory's message text, `manager.go`'s and `manager_test.go`'s own doc
comments, `platform-support.md`, and both `en.ts`/`de.ts` i18n strings to
name all three.

**Low, fixed: four smaller gaps from this round's own new tooling.**

- `ci-core.yml`/`ci-updater.yml`/`ci-webapp.yml`/`ci-blockpage.yml`/
  `ci-unbound.yml`'s `push.paths` filters didn't include
  `scripts/ci/trivy-image-scan.sh` or `.trivyignore.yaml` - a direct
  push to main touching only those (a dated-ignore expiry fix, e.g.)
  wouldn't have triggered any of them. `pull_request` was already
  unconditional on four of the five (round 10/11); blockpage's own PR
  trigger keeps a paths filter (its checks aren't required), so it
  needed the same addition on both triggers. Added to all five.
- `Preflight`'s docker-cp advisory treated "confirmed patched" and
  "genuinely can't tell" identically - both produced total silence.
  Added a distinct `docker_engine_cp_cve_unknown` advisory (still
  `Level: "warning"`, still never fails `Ready`) for a version that
  looks version-shaped but isn't the clean, confident form the real
  warning needs (a distro-suffixed version, e.g.) - scoped narrowly
  enough (a loose `^\d+\.\d+` check) that this package's own test
  suite's generic `"ok"` `CommandRunner` stand-in, used across most of
  its other tests, still resolves to true silence rather than gaining
  an advisory none of those tests are about.
- `Setup.tsx`'s check-row icon ternary
  (`status === "failed" ? "!" : status === "warning" ? "!" : "✓"`)
  simplified to `status === "ok" ? "✓" : "!"` - both branches already
  rendered the same glyph.
- The round-13 warning-level check had backend test coverage but no
  frontend test of its own. Added one: it renders with a distinct
  `warning` class (neither `ok` nor `failed`), its action text stays
  visible despite `check.ok` being `true` (the exact bug the round-13
  render-guard fix addressed), and the install button stays enabled.

## Follow-up review, round 15 (2026-09-01)

**High, fixed: CI-blocking CVEs trivy's own vulnerability DB surfaced
overnight, unrelated to anything either review found.** Not a review
finding - discovered live because every round-15 PR runs the same
required scans round 13/14 already wired up, and the DB had moved on
since. Two real, actionable fixes and one genuinely-unfixed set:

- Core/Updater's shared `docker:29-cli` pin: openssh 10.3_p1-r0 has
  CVE-2026-60002 (CRITICAL), CVE-2026-59999/CVE-2026-60000 (HIGH), fixed
  at 10.3_p1-r1 - already published on Alpine 3.24's own `main` repo
  (confirmed live). Same stopgap-apk-pin pattern as the existing
  openssl/libcrypto3 entries.
- cosign v3.1.3 (still the newest release, confirmed live) and the
  compose plugin both carry CVE-2026-56854
  (`golang.org/x/crypto/ssh`, CRITICAL, fixed at 0.55.0) in their own
  dependency trees - not fixed in any release of either. Added with the
  same dated, justified pattern as the existing grpc entry, versioned
  purls for both resolved versions.
- Unbound's `libevent-2.1-7t64` (2.1.12-stable-10+b1) carries four new
  HIGH findings (CVE-2026-63383/63384/63387/63388), all unfixed in
  trixie (Debian's own tracker doesn't even have a `<no-dsa>`
  classification yet - simply too new). Unlike every other Unbound
  ignore entry so far, libevent is a real, direct build dependency
  (`--with-libevent`) - checked accordingly, not assumed: two are in
  libevent's own RPC/tagging framework (`event_tagging.c`), one is in
  its bundled `evdns` DNS-server-response helper, one requires an
  AF_UNIX listener - Unbound implements DNS wire-format parsing entirely
  itself (never libevent's own `evdns` API or RPC/tagging framework)
  and its own remote-control interface is TCP-only (confirmed live in
  `unbound.conf`), never AF_UNIX. None of the four are reachable through
  anything this image actually does.

**High, fixed: one more CI-blocking CVE, surfaced by the very next PR
rebase.** `libexpat` 2.8.2-r0 in Core/Updater's shared `docker:29-cli`
base - a different package from Blockpage's own nginx-base libexpat pin
(round 14) - carries CVE-2026-66046/CVE-2026-76641 (both "Expat through
2.8.3" DoS), fixed at 2.8.4-r0, already published on Alpine 3.24's own
`main` repo (confirmed live). Same stopgap-apk-pin pattern as the rest
of this round's own openssh/openssl entries.

# RootGuard release history

Version-by-version release notes and incidents for every published
RootGuard release. Split out of `docs/project-state.md` on 2026-08-29 to
keep that file focused on current architecture/feature state - see
`docs/project-state.md` for that, and `docs/security-audit-log.md` for
the security-review finding/fix journal.

## Release status

`v0.1.0-beta.7` is the current public release, published with digest-pinned
`amd64`/`arm64` images for all five RootGuard components and a live-verified
`upgrade-test` job in the release pipeline. Milestones 0.1 through 0.6 are
complete and verified; the remaining gates before 1.0 are 0.9 (release
candidate) and the 1.0.0 stable-appliance checklist itself (see
`ROADMAP.md`).

Cutting beta.4 (2026-08-22) surfaced three real release-pipeline bugs, none
caught before because this was the first release since #298 added the
site-refresh/pin-consistency automation and since the alpha->beta series
transition: `update-alpha-pins`'s checkout lacked tag history (`fetch-depth`
default of 1), so `bump-site-versions.sh` died silently under
`set -e`+`pipefail` before its own error message could print
([rootguard#311](https://github.com/foxly-it/rootguard/issues/311));
`upgrade-test`'s "previous release" detection only ever listed
`v0.1.0-alpha.*` tags, silently testing an upgrade from `alpha.16` instead of
the real N-1 release for every beta release so far, until the
`compose.alpha.yaml`->`compose.release.yaml` rename turned the
already-wrong-scope lookup into a hard crash (same issue, fixed by re-deriving
via `git for-each-ref --sort=-creatordate` across both series); and a push
made with the workflow's own `GITHUB_TOKEN` never auto-triggers `pages.yml`
(the same GitHub anti-recursion rule `release-version-bump.yml` already
works around for tag pushes), so the pin-refresh commit's site changes
never deployed on their own
([rootguard#314](https://github.com/foxly-it/rootguard/issues/314)) - now an
explicit `gh workflow run pages.yml` dispatch, mirroring that same existing
pattern. A fourth, narrower bug: the rename intentionally left the public
quick start's `curl` commands pointing at the old filename under the
still-current `v0.1.0-beta.3` tag so they wouldn't 404 immediately, but
`bump-site-versions.sh` only ever substitutes the version-number substring,
not filenames - so the very next release advanced the version in that same
text without the filename, producing "new version, old filename," a
combination that never existed and did 404 live on the public site for a
few minutes before being caught and fixed.

`v0.1.0-beta.5` shipped the same day, adding live GitHub-Releases update
discovery ([rootguard#320](https://github.com/foxly-it/rootguard/pull/320))
and updater self-update
([rootguard#322](https://github.com/foxly-it/rootguard/pull/322)). Live
verification of the latter immediately surfaced three more real bugs -
a self-referential compose-mount resolution failure, a digest-less
attestation gap, and genuinely broken YAML in beta.5's own committed
pin-refresh commit - all fixed same-day
([rootguard#324](https://github.com/foxly-it/rootguard/pull/324)). That
fix's own follow-up correction for the already-published beta.5 pin
commit then hit two release-pipeline mechanics for the first time: a
squash-merged two-commit historical correction collapses into a
net-zero diff and silently undoes itself (fixed by re-merging with a
real merge commit,
[rootguard#326](https://github.com/foxly-it/rootguard/pull/326)), and a
tag-triggered workflow run stays pinned to its original checkout even
when rerun (worked around with a fresh `workflow_dispatch` against
`main` for the same version). `v0.1.0-beta.6`, cut immediately after
with all of the above already on `main`, passed its full pipeline
including `upgrade-test` cleanly on the first real (non-rerun) attempt.

**2026-08-24:** dashboard/AdGuard polish and bugfix round, released as
`v0.1.0-beta.7` ([rootguard#331](https://github.com/foxly-it/rootguard/pull/331)).
Code review of the metrics-caching work from the previous session found
two remaining perf/concurrency gaps: `dashboardHandler` still ran `docker
inspect` five times per request (only `docker stats` had been cached),
and concurrent stale-cache callers (the background ticker plus several
requests/tabs at once) could each start their own redundant refresh.
Fixed by mirroring the existing metrics cache for container status
(`CollectStatus` in `stack/status.go`) and adding a small singleflight
(`stack/refresh.go`) shared by both caches; `fetchAdGuardStatus()` (up to
four real AdGuard API calls) also moved off the 500ms poll onto its own
5s interval, since it was still hitting AdGuard directly on every tick.
Metrics now carry their real `collected_at` so the frontend can skip
re-recording an unchanged cached sample under a fresher-looking age. Live
verification on `.7` caught a real bug in this batch before it shipped:
the webapp backend round-trips Core's `/api/dashboard` response through
its own mirrored Go struct rather than forwarding raw bytes, and that
struct hadn't been given the new field - `collected_at` was silently
dropped even though Core sent it, until a direct `curl` comparison
surfaced the gap.

Also this session: removed the dashboard's "DATENFLUSS" card (both the
live app and the marketing site's hand-built mockup, which had gone
stale the moment the real card was removed and was still showing a
fourth-of-five runtime list to boot); and added an AdGuard
protection-pause toggle (Off/10 minutes/1 hour) mirroring AdGuard Home's
own native "disable protection" control via its real `/control/protection`
endpoint - AdGuard's own server-side timer re-enables it, no RootGuard
scheduling needed. A second code-review pass on that toggle found three
more real bugs before it shipped: `protectedState` (and AdGuard.tsx's own
`ready`) never actually factored in `protection_enabled`/`filtering_enabled`,
so pausing protection still showed a green "PROTECTED" dashboard; the
select derived its displayed value straight from `protection_enabled`, so
a 10-minute pause displayed identically to "off indefinitely" the instant
it was chosen, and the AdGuard page never polled again afterward, so it
stayed on "paused" after AdGuard's own timer re-enabled protection until
a manual reload; and the protection handler took a plain `bool` for
`enabled`, so an empty `{}` request body silently decoded to
`{"enabled":false,"duration_seconds":0}` and disabled protection
indefinitely with no expressed intent. Fixed: `reachable` (configured/
healthy/upstream-ready) now gates the pause control itself so pausing
protection can never hide the control needed to un-pause it, while
`ready`/`protectedState` additionally require protection and filtering
both enabled; the select is now a pure action trigger with the real
state plus a live countdown (sourced from AdGuard's own
`protection_disabled_duration` via `/control/status`, previously only
`version` was read from that response) rendered separately, and the page
polls every 5s while paused; `enabled` is now `*bool` (a missing field is
a 400, not a silent false), and `duration_seconds` is restricted to the
three values the UI actually offers - enforced identically in Core and
the webapp proxy.

`v0.1.0-beta.7`'s full release-pipeline run (test/publish/pin-refresh/
smoke-test/upgrade-test) passed cleanly on the first attempt. Used it to
verify RootGuard's own update mechanism end-to-end rather than just
trusting the pipeline: reset the `.7` test LXC to a genuine, freshly
pulled `v0.1.0-beta.6` (core/webapp/updater) plus its existing
`v0.1.0-beta.5` Unbound - i.e. what a real user still on the previous
release actually has - then drove the real in-app flows to bring every
component to beta.7: `/api/control-plane-updates/check` +
`/install` for the atomic Core+WebApp update (WebApp restarting itself
mid-update and reconnecting cleanly afterward), `/api/updates/check` +
`/api/updates/unbound` for Unbound, and `/api/updater-updates/check` +
`/install` for the Updater's own self-update. All four succeeded; the
post-update dashboard's `collected_at` field (absent on the pre-update
beta.6 response, since that fix hadn't shipped yet) confirmed the live
containers were genuinely running the new code, not just freshly
recreated with the same tag. Also re-verified the protection-pause
round trip against the real published beta.7 build (not just the local
`:dev` build used during development): paused for 600s, confirmed
`protection_disabled_duration_ms` counting down live, then restored.

**2026-08-24, same day:** added a one-command installer,
`install.sh` at the repo root, for non-technical ("DAU") users - user ask
was a one-liner alternative to the manual quick start that detects/
installs Docker, downloads the release, generates the secret tokens, and
only asks for a WebGUI username/password, without taking anything away
from the existing manual path for people who want to see every step.
Detects Docker and, if missing, installs it via the separate
[foxly-it/dockerinstall](https://github.com/foxly-it/dockerinstall)
project (Debian/Ubuntu; that script already rejects unsupported OSes
with a clear message on its own, so `install.sh` doesn't duplicate that
logic). Resolves the current release the same way `rootguard-core`'s own
updater does - GitHub Releases API, newest tag matching the release
pattern, since `/releases/latest` excludes prereleases and every
RootGuard release is one - so this script carries no version string of
its own to remember to bump at release time, unlike `site/*.html`.
Generates `ROOTGUARD_API_TOKEN`/`ROOTGUARD_RECOVERY_TOKEN`, prompts for a
username/password (or auto-generates and prints one for
`--non-interactive` use) explicitly via `/dev/tty` - a plain `read` in a
`curl | bash` script reads from the download pipe instead of the
terminal - and substitutes secrets into `.env` through `awk`+`ENVIRON`
rather than sed/awk program text, so a password containing `&`, `#`, or
`\` can't corrupt the substitution (verified live with exactly such a
password). `site/index.html`'s quick start now leads with the one-liner;
`site/docs.html`'s full manual walkthrough is untouched, with a callout
linking to the new fast path instead. Verified live end-to-end twice, not
just via shellcheck: once with a typed password, once via
`--non-interactive`, both pulling the real published beta.7 images via
the real GitHub API and ending in a successful login with the credentials
the script set.

**2026-08-24, later the same day:** four follow-ups found by the user
actually using the shipped beta.7.

- **Authenticated account settings had never been merged.** User: "Und in
  Beta 7 hast du anscheinend die Benutzerverwaltung gelöscht" - it hadn't
  been deleted, PR #330 (`feat/account-settings`, the change username/
  password work from the prior session) had simply sat open, unmerged,
  the whole time beta.7 was cut and this session's other work landed.
  Rebased it onto current main (clean merge, no conflicts across 30+
  commits of divergence), re-verified the full test/build/lint suite,
  and merged it. Live-verified the actual endpoint on `.7` afterward
  (not just CI): renamed the logged-in session's own username twice in a
  row (round-trip back to the original) with the session staying alive
  throughout, exactly as designed. Prompted by this, swept every other
  branch in the repo for the same "PR open but never merged" gap
  (checked all ~40 branches sitting ahead of `main` against the GitHub
  API's own merge state, not just git ancestry, since a squash-merged
  PR's source branch is *expected* to look "ahead" by one commit)  -
  `feat/account-settings` was the only real gap found.
- **The new AdGuard protection select didn't match the app's other
  dropdowns.** Was using ad-hoc padding/radius/background instead of the
  `--field-bg`/7px-radius/34px-padding tokens every other themed
  `<select>` in the app already uses (`.resource-profile-field` in
  unbound-polish.css, `.logs-toolbar select`). Fixed to match.
- **The marketing site's hero mockup didn't structurally match the real
  dashboard.** User: fake metrics are fine, but it should be 1:1. The
  mockup's 5 metric cards were missing the sparkline chart + Min/Max
  caption every real `SparkMetric` card has had since the metrics-history
  work, and the runtime list was a plain bullet list instead of the real
  `.service-card` treatment (icon box, restart-button corner, status
  text). Rebuilt both to match, live-verified via headless Chromium.
- **`install.sh` now checks for busy ports before installing** (user
  ask, tying back to Setup's own existing two-stage port-53 preflight
  described in docs.html). Port 8080 - what the script's own `docker
  compose up` actually binds - is a hard check with an interactive
  alternate-port prompt; 53/80 are informational only, since install.sh
  doesn't bind those itself (Core creates the DNS/blockpage containers
  later, during guided Setup). **Real incident while testing this
  live on `.7`**: `compose.release.yaml`'s containers use fixed
  `container_name` values, so running the script's actual `docker
  compose up` step against `.7` recreated the *real* deployment's
  containers regardless of `--dir` (isolating the download directory
  does not isolate the Docker container namespace on the same host) -
  and a `ROOTGUARD_WEB_PORT` override exported for that test leaked into
  the recreated real webapp's own port binding via environment
  inheritance, moving it off 8080. Both caught and fixed within the same
  session (recreated from the real `/root/rootguard-test` checkout with
  its correct `.env`, confirmed login and `collected_at` afterward) -
  no data loss (named volumes are independent of container identity),
  but a concrete reminder that `.7`/`.61` are shared persistent
  environments, not disposable ones: any `install.sh`-style test that
  actually runs `docker compose up` needs an isolated host (verified the
  rest of this feature locally on macOS Docker Desktop instead, once
  this was understood).

**2026-08-25:** a formal code-review pass on the accumulated work above
found four concrete issues, one security-critical, worked through in
priority order.

- **Login/recovery/account rate limiting had a TOCTOU race** (PR #342).
  `blocked()` (check) and `recordFailure()` (record) were two separate,
  non-atomic operations around an expensive PBKDF2 verification -
  concurrent requests could all pass the check before any of them got
  counted, so the limiter only ever bounded *sequential* guessing, not
  concurrent. Fixed with an atomic reservation pattern: `beginAttempt`
  combines the check with reserving an in-flight slot (counted as if it
  were already a failure for admission purposes, without permanently
  recording it), `endAttempt` releases that slot and optionally converts
  it into a real, window-tracked failure. Verified the fix is load-
  bearing, not just plausible: added a test that fires 30 truly
  simultaneous login attempts via a `sync.WaitGroup` release gate,
  confirmed it fails against the pre-fix code (temporarily reverted via
  `sed`, saw a 30/0 pass/block split instead of the correct 5/25) before
  restoring the fix and re-verifying green.
- **AdGuard filtering endpoint still had the `{}`-disables-filtering
  bug** the protection endpoint was already fixed for earlier (PR #343).
  Same root cause, same fix shape: `Enabled bool` → `*bool` with an
  explicit nil check, plus a `decoder.More()` check on both the
  filtering and protection handlers (Core and WebApp) to reject trailing
  JSON after the first decoded value - a bare `Decode()` call silently
  ignores anything after the first object. Live-verified on `.7`: both
  `{}` and trailing-JSON bodies now return 400, a real on/off filtering
  toggle still works.
- **`install.sh` ignored `ROOTGUARD_ADMIN_USER`.** Unlike the password
  handling, the username prompt was called unconditionally instead of
  checking the environment variable first - fixed to match the password
  pattern (check env, only prompt if empty).
- **`install.sh` accepted any string as `ROOTGUARD_WEB_PORT`**, both the
  env var and the interactive alternate-port prompt, only failing later
  and less clearly inside Docker Compose. Added a `valid_port()` check
  (numeric, 1-65535) at both points.
- **`install.sh` left an empty target directory behind on a failed
  download**, which then tripped the "already exists" abort on retry.
  Downloads now land in a `mktemp -d` scratch directory first and are
  only moved into `$TARGET_DIR` once both succeed.

Two follow-up review passes on this same work (PR #345, #346) closed the
remaining low-priority items:

- **AdGuard page polish** (screenshot feedback): the "DNS filtering"
  checkbox is now a green/red on-off switch; the "AdGuard protection"
  select was too short and its color (`--field-bg`, a near-black
  recessed-field tone designed for softer backgrounds) looked mismatched
  directly on `.adguard-panel`'s solid `--surface` - restyled to
  `--surface-soft` + a full-strength border at the app's primary field
  size. Live-verified in both themes on `.7` via Playwright screenshots,
  including toggling filtering off/on to confirm the underlying request
  still works.
- **Shared `decodeStrictJSON` helper**: login/recovery/account in
  `auth.go` each ran a bare `Decode()` with no trailing-data check,
  unlike the already-fixed AdGuard handlers. Extracted one helper
  (decode + `decoder.More()`) and switched all three over.
- **Recovery rate-limit slot released too early**: `handleRecovery`
  freed its limiter slot right after the token check, before the
  expensive PBKDF2 derivation and session-wipe/credential persistence
  that follow a valid token - anyone holding the (already
  high-privilege) recovery token could fire unbounded concurrent resets.
  Now holds the slot through the whole handler, mirroring
  `handleAccount`'s existing pattern. Regression-tested the same way as
  the earlier login race fix: reverted, confirmed the test fails (18
  accepted instead of 5), restored.
- **`install.sh`'s target-directory check ran too late** - after
  `check_ports` and a potential real Docker install. Moved to the very
  first thing `main()` does.
- **First React component tests in this repo**: added
  `@testing-library/react` + `jsdom` (this project had none before -
  only plain-function utility tests) and covered the new switch/select
  - click/keyboard toggling, busy/disabled state, `filtering_enabled`
  rendering, the select's reset-after-action behavior.

A third, full-repository review pass (PR #347, #348, #349, #350) closed
one more security-relevant finding, two low-priority bugs, unified strict
JSON decoding across every remaining handler, and shipped one user-driven
website feature:

- **Weak startup secrets accepted** (medium priority, security). WebApp,
  Core, and the Updater only checked that
  `ROOTGUARD_API_TOKEN`/`ROOTGUARD_ADMIN_PASSWORD`/`ROOTGUARD_RECOVERY_TOKEN`/`ROOTGUARD_UPDATER_TOKEN`
  were non-empty, accepting e.g. `ROOTGUARD_ADMIN_PASSWORD=a` -
  inconsistent with the 12-character minimum a password *change* already
  enforces. Added a `requireSecretStrength` check to each binary's
  `main()` (12 chars for the admin password, 32 for tokens) that also
  rejects an unedited `.env.release.example` placeholder outright, since
  those placeholder strings happen to be long enough to pass a length
  check alone. `install.sh`'s own manual/env-supplied password path got
  the matching check, moved to before any directory/download side
  effects. **Caught live**: the first CI run against this fix failed -
  two `scripts/verify-*.sh` fixtures and one hardcoded literal in
  `ci.yml`'s own recovery-flow test still used short placeholder tokens
  that the grep sweep for the env var *name* had missed (they set the
  var via `export VAR="literal"`, not a workflow `env:` block); fixed in
  a follow-up commit once CI surfaced it.
- **FritzBox IPv6 URLs malformed.** `FritzBoxClient` built its base URL
  with a bare `Sprintf`, producing an invalid `http://fd00::1:49000` for
  an IPv6 router address instead of `http://[fd00::1]:49000`. Switched to
  `net.JoinHostPort`.
- **Backup restore off-by-one.** The entry-count guard used
  `count > MaxFiles` instead of `>=`, accepting exactly one more archive
  entry than the documented limit.
- **JSON decode unification finished**: PR #346 only covered
  login/recovery/account in the WebApp's `httpapi` package. Core's
  `routes.go` still had 13 near-identical decoder blocks (none with a
  trailing-data check), and the WebApp's own proxy handler package had 12
  more. Added one generic `decodeJSON[T any]` helper per module and
  converted every call site - no behavior change for endpoints that
  already worked, every endpoint that previously accepted trailing JSON
  data now rejects it with 400.
- **Duplicate frontend API client removed.** `services/api.ts` existed
  solely for `GET /api/version`, with its own raw `fetch()` that never
  checked for a 401 - unlike the central client's `request<T>` helper,
  it never fired the `rootguard:unauthorized` event on an expired
  session. Moved into `api/client.ts`, deleted the duplicate.
- **Website: copy-to-clipboard button + animated install demo**
  (user idea, refined in conversation). The quick-start command block
  was missing a copy button entirely. Added one, plus a new "Live
  preview" section showing a faithful example `install.sh` session
  (real `log()`/`prompt()` message wording) with lines staggering into
  view via `IntersectionObserver`, respecting `prefers-reduced-motion`.
  The version line stays live via the existing `project-data.json`
  fetch; its static fallback is kept in sync for free by the existing
  `bump-site-versions.sh` release-pin step.

Two remaining code-compression suggestions from the same review (splitting
`routes.go`/`auth.go`/other large files by responsibility, and unifying
each module's own temp-file-then-rename atomic-write pattern into a
shared helper) were explicitly deferred - the user picked only the API-
client removal above; the review found no bugs in either, just organizational
improvement.

**Real bug found while cutting the resulting beta (0.1.0-beta.10), not by
review**: the release pipeline's `upgrade-test` job failed twice in a row -
"Core und WebApp verwenden bereits die aktuellen Images", skipping the
upgrade entirely, even though beta.10's images are genuinely different
from beta.9's (independently verified against the registry). Root cause
in `rootguard-updater`'s `digestQualify`: it resolved a freshly-pulled
tag's digest via `docker image inspect --format {{.RepoDigests}}`,
returning the *first* repo-prefixed match - `RepoDigests` belongs to the
local image object as a whole and can list more than one digest for a
repository whose tag recently moved between releases, so the loop had no
way to distinguish "the digest just pulled" from "some older digest this
local image happens to also carry," and reproducibly returned the stale
one on the GitHub Actions runner's Docker setup (did not reproduce on a
different host with a containerd-backed image store). **This is a real
correctness bug in the live `/api/control-plane-updates/check` path**, not
just a CI artifact - any real installation could plausibly hit the same
false "already up to date" under the right local Docker state, on any
release before this fix. Fixed by reading the digest `docker pull` itself
reports in its own output instead of round-tripping through the ambiguous
local `RepoDigests` lookup (kept as a fallback for an already-qualified
static pin or unexpected output). `beta.10`'s images themselves are fine
(fresh install verified via its own passing smoke-test) - only the
*upgrade path* was affected; `beta.11` carries the fix and re-verifies the
upgrade-test passes for real.

**That fix wasn't the whole story - two more real bugs surfaced chasing
this, across `beta.11` through `beta.13`:**

- **Core has its own separate copy of the exact `digestQualify` bug**
  (`rootguard-core/internal/updater/manager.go`, used for AdGuard/Unbound's
  own self-update checking) - a different Go module than
  `rootguard-updater`, so PR #352/#353's fix there didn't cover it. Same
  `digestFromPullOutput` fix applied here too (PR #355).
- **The GitHub Releases API doesn't reliably return releases newest-first**,
  despite `pickLatestReleaseImage`'s own doc comment promising it. Directly
  querying the live API showed `v0.1.0-beta.9` listed *ahead of*
  `v0.1.0-beta.12`, on a repository that had several releases cut in quick
  succession. Every live self-update resolution (core, webapp, updater,
  unbound) silently trusted that ordering. Fixed by ranking every matching
  release locally by `(series, build number)` parsed from the tag itself,
  ignoring API response order entirely (PR #355).

**Why the upgrade-test kept failing even after both of those landed** (root
cause found by the user, not guessed): the upgrade-test's `check`/`install`
calls run against the *previous* release's own Core - `beta.12`'s Core for
the `beta.13` test - which still has the API-ordering bug, since that fix
only ships *in* `beta.13`. Core resolves "latest core/webapp release"
itself and sends that as a `target_images` override, which *wins* over the
updater's own correctly-set `ROOTGUARD_*_UPDATE_IMAGE` env vars
(`targetImageFor` in `rootguard-updater/main.go` prefers the override).
`beta.12`'s Core kept resolving to whatever the live API listed first
(`v0.1.0-beta.9`) and pushing that as the override - so the "upgrade"
silently installed `beta.9` instead of `beta.13` every single time,
explaining why the exact same wrong digest recurred across every version
pair tested (`beta.10→11`, `11→12`, `12→13`). A fix only helps upgrades
*after* the release that first ships it; it can't fix the one jump onto it,
since the code doing the resolving at that point is still the old, buggy
version. Worked around for exactly that one CI transition (PR #356):
`release-alpha.yml` special-cases `previous == 0.1.0-beta.12` and calls the
updater's own `/api/control-plane/{check,update}` directly with an explicit
override via `docker exec rootguard-core wget ...`, bypassing only the old
Core's resolution - still exercises the real
updater/pull/digest/compose/verify/rollback path either way. Reverts to the
normal path automatically once `beta.13` (which carries the fix) becomes
"previous". **Confirmed working**: `beta.13`'s full release pipeline,
including `upgrade-test`, passed end to end.

**Real-installation impact, deliberately not mitigated further**: any
`beta.12` installation clicking "check for updates" in the WebGUI could hit
this same bug and silently resolve to an older release than `beta.13`.
Given `beta.12` was published for roughly an hour before `beta.13` and this
is pre-1.0 software with no known external adopters caught in that exact
window, building dedicated tooling or public documentation for it was
judged disproportionate. If anyone ever does report being stuck on
`beta.12`, the same manual `docker exec rootguard-core wget ... http://
updater:8082/api/control-plane/update` bootstrap documented above (with
`target_images` pointed at the desired version) is the escape hatch.

**Durable safety net added on top, for the future** (PR #357, user asked
"and fixed going forward?" after the postmortem above): the fixes so far
close the three *specific* bugs found, but nothing stopped `update()` from
applying a *future* bad resolution the same way, whatever causes it -
"different image ID" was treated as "newer," full stop. `update()` now
compares both images' `org.opencontainers.image.version` label (every
Dockerfile already sets it from its own build) using the same
`(series, build number)` ranking `pickLatestReleaseImage` uses, and refuses
with a clear error if the candidate is genuinely older - silently skipping
the check for anything that doesn't parse as a RootGuard release version
(a local `:dev` build) rather than blocking it. This can't rescue a release
that shipped before it exists (the code doing the resolving during an
upgrade is whichever version is *currently running*), but it protects
every upgrade *from `beta.14` onward* against this entire bug class.
Confirmed live: `beta.14`'s `upgrade-test` passed via the normal
WebApp-proxied path (the `beta.12`-specific bypass in `release-alpha.yml`
correctly stayed dormant, since `beta.13` - carrying the ordering fix - is
now "previous").

**SemVer/tagging generalization, part 1** (PR #359, branch
`feat/general-semver-release-tagging`): the Go-side update mechanism
(`isOlderReleaseVersion`, `pickLatestReleaseImage`) was already moved onto
`golang.org/x/mod/semver` for full generality (see above). The user's own
review of the branch before merge found three real bugs in the
*surrounding shell scripts*, none caught by CI because none of the
existing checks exercise a stable (non-prerelease) or malformed version:

- `release-alpha.yml`'s version-gate used a shell glob
  (`[0-9]*.[0-9]*.[0-9]*`) that isn't a SemVer check at all - accepted
  `1foo.2bar.3baz`, a leading-zero core like `01.2.3`, and build metadata
  (`1.2.3+build.4`, which isn't even a legal Docker tag). Replaced with a
  full SemVer 2.0 grammar via bash `[[ =~ ]]` (build metadata excluded on
  purpose, same reason it's invalid in a tag).
- `install.sh`'s `resolve_latest_tag` used `sort -V | tail -1`, which
  ranks a prerelease *above* its own stable release (`1.0.0-rc.1` above
  `1.0.0`) - verified empirically, and previously documented as an
  "accepted gap" that turned out to be a real, fixable bug. Fixed by
  rewriting each tag's version-core/prerelease separator from `-` to `~`
  before sorting (GNU `sort -V` treats `~` as sorting before everything,
  same as dpkg) and back after - correct precedence, still no jq/semver
  dependency.
- `check-site-facts.sh`/`bump-site-versions.sh` required a letter-leading
  prerelease suffix on every version match, so a bare stable tag
  (`v1.0.0`) was invisible to `latest_tag` resolution entirely - the
  scripts would keep tracking an old `rc.N` as "current" forever. Split
  into two patterns: a wider, suffix-optional `tag_version_pattern` for
  resolving the latest real git tag (safe - no prose to false-positive
  against), and the original letter-leading `version_pattern` kept
  unchanged for prose scanning, since widening *that* one was verified
  live to reintroduce the exact SVG-path-coordinate false positives the
  letter requirement was added to fix in the first place (`site/*.html`
  is full of SVG icon path data shaped like fake dotted-number
  "versions"). Originally left as a documented accepted gap (a correct
  bare mention would be invisible to the checker); superseded by a real
  fix in the same PR's second review round - see below.

All three verified directly against the user's own reported reproduction
cases (glob-fooling strings, `v1.0.0`/`v1.0.0-rc.1` ordering, tag
resolution finding a bare stable tag), plus a live run of
`check-site-facts.sh`/`bump-site-versions.sh` against the real repo
(idempotent, no site content changed) and `shellcheck` (clean).

Also restructured `ROADMAP.md`'s 30-day soak test
([rootguard#271](https://github.com/foxly-it/rootguard/issues/271)) from
a 0.9 (pre-`rc.1`) gate to a 1.0.0 (stable-promotion) gate, at the user's
suggestion: the soak test exercises the update/backup/restore machinery
generically, not one specific build, and an RC period is exactly what
longer-running validation like this is for - there's no reason to hold
`1.0.0-rc.1` itself hostage to the calendar. 0.9 is now 5/7 with only two
non-time-bound items left (final blocker sweep, migration docs).

**Same PR, second review round** (commit `17fafd1` reviewed independently
by the user, two more real gaps found):

- Different components recognized different subsets of valid SemVer:
  `install.sh`'s tag regex and the site scripts' `tag_version_pattern`
  both rejected a hyphen *inside* a prerelease identifier (`rc-one.1`),
  even though release-alpha.yml's gate and Core's `semver.IsValid` both
  accept it - a release using that shape could publish and be picked up
  by Core's updater while `install.sh` and the site silently kept
  ignoring it as "not a real release." Fixed by widening each identifier
  character class to `[0-9A-Za-z-]+` everywhere a *tag* gets recognized
  (not everywhere a *version gets minted* - release-alpha.yml's own gate
  stays strict on purpose, leading zeros and all, since that's the one
  place actually deciding what's allowed to exist).
- The "accepted gap" from the first round turned out to be a real,
  ongoing problem, not just a missing positive check: once site prose
  says a bare `1.0.0` with no letter-suffix, `check-site-facts.sh`'s old
  narrow pattern could never recognize *any* future bare mention again -
  so a later `1.0.0` → `1.0.1` patch bump would go completely undetected
  by CI forever after the first stable release, not just through the
  rc→stable transition. Fixed for real rather than re-documented: added
  `scripts/version-pattern.sh` (sourced by both site scripts;
  `install.sh` can't source it - no repo checkout exists yet on a fresh
  machine - so it keeps its own hand-synced copy of the tag grammar) with
  a `rootguard_extract_versions` helper that strips SVG `<path d="...">`
  coordinate data first (the original false-positive source), then keeps
  a dotted-number candidate only if it has exactly three groups with a
  ≤3-digit last group - cheap enough to reject a 4-group IP address and a
  `dd.mm.yyyy` date (both proven live to otherwise look exactly like a
  bare version) without needing lookahead, which check-site-facts.sh's
  own header already rules out for BSD/GNU grep portability. Verified
  live end to end: injected a fake stale `1.0.1` mention into `index.html`,
  confirmed `check-site-facts.sh` flags it, confirmed
  `bump-site-versions.sh` correctly fixes it and leaves an
  already-correct bare mention untouched, confirmed the real repo content
  still passes/no-ops afterward, all test edits reverted before
  committing.

**Same PR, third review round** (commit `9555e67` reviewed independently
by the user as an explicit RC-readiness check ahead of triggering
`1.0.0-rc.1`, three real findings plus one acknowledged-low-priority one):

- `release-version-bump.yml` (the "Cut next release" workflow a human
  runs by hand, optionally with a manual version override) accepted the
  override completely unvalidated, then immediately ran `git-cliff`,
  committed `CHANGELOG.md` to `main`, pushed a real tag, and created a
  real GitHub Release - only the separately-dispatched
  `release-alpha.yml` validated the version, by which point a typo'd
  override would already have left a half-published release needing
  manual cleanup. Exactly the risk that mattered right before triggering
  the actual RC by hand with an override. Fixed by validating with the
  same SemVer 2.0 grammar as release-alpha.yml's own gate, immediately
  after `next_version` is computed and before anything external happens -
  duplicated rather than shared (no simple way to share a bash snippet
  across two separate workflow files without a composite action).
- Same workflow's GitHub Release creation always passed `--prerelease`
  unconditionally - correct for every version so far (all had a
  suffix), silently wrong for a future bare stable release. Now
  conditional on whether the version string contains a `-`.
- `install.sh`'s `sort -V` + `-`→`~` precedence workaround (from the
  first review round) turned out not to be genuine SemVer precedence in
  every case: a prerelease identifier containing its own literal hyphen
  ("rc-one.1") sorted *below* the plain "rc.1" it should rank above,
  because the trick only ever repositions the version-core/prerelease
  separator, not every hyphen SemVer identifiers are allowed to contain.
  Doesn't affect the actually-planned `beta.N` → `rc.N` → stable series.
  Replaced with a real, from-scratch SemVer 2.0 precedence comparator in
  pure bash (`version_gt` + helpers) - numeric version-core comparison,
  then prerelease-identifier-by-identifier comparison per spec (numeric
  identifiers compare numerically and always rank below alphanumeric
  ones; more identifiers with all preceding ones equal ranks higher) -
  rather than trying to make `sort -V`'s undocumented ordering behavior
  cover a case it was never designed for. Verified against SemVer.org's
  own canonical precedence example chain, the specific reported bug case,
  and a live call against the real GitHub API (still correctly resolves
  `0.1.0-beta.14` as latest).
- Acknowledged, left as is: `scripts/version-pattern.sh`'s website
  extractor rejects a patch/build number over 3 digits (to keep excluding
  a `dd.mm.yyyy` date's 4-digit year, the whole reason that check exists)
  - flagged by the user themselves as low-priority and "practically far
  away," not something to chase now.

**Same PR, fourth review round** (commit `deb46eb`, explicit RC-readiness
check ahead of triggering `1.0.0-rc.1` - user's conclusion: no blocker
left for that specific version, branch mergeable, both findings below
non-blocking):

- `release-version-bump.yml`'s own "find the previous tag" step (used to
  pick `git-cliff`'s changelog range) still used the pre-fix, hyphen-less
  regex - the one spot that hadn't been updated when every *other*
  tag-recognition site (release-alpha.yml's gate, Core, the updater,
  `install.sh`) already accepted a hyphenated prerelease identifier like
  `rc-one.1`. Widened to match, for consistency - doesn't affect the
  actually-planned `beta.N`/`rc.N` series either way.
- `install.sh`'s new `version_gt` comparator used bash's native
  `((10#$a < 10#$b))` arithmetic for numeric-identifier and version-core
  comparisons, which silently overflows for a value beyond bash's integer
  range - SemVer itself never caps a numeric identifier's size. Replaced
  with `numeric_compare`: strips leading zeros, then decides by digit
  *count* first (no leading zeros left means length alone is decisive),
  falling back to a lexical compare only once lengths already match
  (numerically correct at that point) - arbitrary-precision, no
  conversion to a native integer anywhere. Verified with a 36-digit
  identifier and a 20-digit major-version component, plus the full
  canonical SemVer.org chain and every previously-reported case again, to
  confirm nothing regressed.

**PR #359 merged** (2026-08-26, commit `ab6b1bf`) after four independent
review rounds - final `gh pr checks 359` confirmed fully green on the
merged commit before merging.

**`v1.0.0-rc.1` cut and published same day.** Final `gh issue list` sweep
right before cutting: 4 open issues, none release-blocking (see
`ROADMAP.md`'s 0.9 section for the per-issue reasoning). Triggered via
`release-version-bump.yml`'s `version` override; tag, GitHub prerelease,
and all five component images published cleanly - but `upgrade-test`
failed on the first run, for exactly the reason `release-alpha.yml`'s own
comments already warned about: `beta.14 -> 1.0.0-rc.1` is the *first*
jump onto the release that ships PR #359's SemVer generalization, so
`beta.14`'s still-running, pre-fix Core resolved "latest" back to itself
instead of `1.0.0-rc.1` (`update_available: false`) - the identical
bootstrap class as the earlier `beta.12 -> beta.13` failure, just
triggered by a different fix this time. The existing workaround only
special-cased `previous == 0.1.0-beta.12`; generalized properly this time
(PR #360, merged same day) - `upgrade-test` now always calls the updater
with an explicit `target_images` override instead of trusting whichever
previous release's Core to resolve "latest" correctly, since this job
already knows exactly which version it's testing and doesn't need live
discovery at all. Re-ran `release-alpha.yml` for `1.0.0-rc.1` (images
already published and immutable, so only `upgrade-test`,
`update-alpha-pins`, and `smoke-test` did real work this time) - fully
green. [`v1.0.0-rc.1`](https://github.com/foxly-it/rootguard/releases/tag/v1.0.0-rc.1)
is out, correctly marked as a GitHub prerelease, site pinned to it.

RootGuard is now in the `0.9` exit state: "only bug fixes and
documentation" until the two remaining 1.0.0-gating items close (the
30-day soak test, `rootguard#271`, and the migration/rollback docs that
wait on 1.0.0's scope being final).

**The `upgrade-test` failure above isn't only a CI artifact.** Verified
directly against the `v0.1.0-beta.14` source: its Core's
`releaseTagPattern` is `^v0\.1\.0-(alpha|beta)\.([0-9]+)$` - hard-limited
to the pre-#359 scheme, so `v1.0.0-rc.1` isn't merely ranked wrong, it's
completely invisible to `pickLatestReleaseImage`. Every installation still
running `beta.14` or earlier (i.e. every installation, since that pattern
was unchanged since `alpha.2`) has the identical blind spot: its WebGUI
"check for updates" will never surface `1.0.0-rc.1` at all. Structurally
unfixable after the fact (the fix ships *in* the release that needs it),
same as the `beta.12` case earlier - except this is the actual 1.0-line
transition, not an interim beta bump, so it warranted more than "note it
and move on." Added a warning callout plus the exact one-time
`docker exec rootguard-core wget ... /api/control-plane/update` bootstrap
command to `docs.html`'s Updates & Rollback section (the same escape
hatch already used for `beta.12`, now public and documented rather than
only living in this file). Also widened `check-site-facts.sh`/
`bump-site-versions.sh`'s `historical_reference_pattern` with a "bis
einschließlich"/"up to and including" marker (mirroring the existing
"Ab"/"Starting with" one, just for the opposite direction) so the new
callout's `0.1.0-beta.14` mention doesn't get flagged as a stale
current-version claim - verified the callout's `1.0.0-rc.1` mentions
still get checked and will auto-update via `bump-site-versions.sh` on
every future release, same as everywhere else on the site.


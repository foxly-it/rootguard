# 0.9 final accessibility and security review

Covers `ROADMAP.md`'s 0.9 checklist item "Final accessibility and security
review." Builds on the 0.5 milestone's original audit rather than repeating
it from scratch - this is a re-verification against the current, post-0.6
codebase plus the auth hardening from this cycle
([rootguard#274](https://github.com/foxly-it/rootguard/pull/274)).

## Accessibility

`@axe-core/playwright` against a real `v0.1.0-beta.1` instance (not a mock),
authenticated, across every route and both themes:

| Routes scanned | Themes | Total scans | Violations |
| --- | --- | --- | --- |
| `/login`, `/dashboard`, `/unbound`, `/unbound/resolver`, `/unbound/zones`, `/unbound/advanced`, `/adguard`, `/setup`, `/stack`, `/backups`, `/logs` (11) | light, dark | 22 | **0** |

The Unbound sub-routes are scanned individually since the 0.5 audit's real
bugs were specifically in that page's collapsed-sidebar sub-navigation
(missing accessible names on 3/4 tabs, a sub-nav that never rendered on one
tab) - both classes of bug are covered by axe's `button-name`/`link-name`/
`aria-command-name` rules, so 0 violations here is a real re-confirmation,
not just a rerun of the same routes checked before.

Not re-run: manual keyboard-only navigation and screen-reader passes (the
0.5 audit's own methodology beyond axe). No code touching focus order,
`tabIndex`, or ARIA roles has landed since 0.5, so a full manual repeat
wasn't judged necessary - flagged here for transparency rather than silently
assumed.

## Security

Builds on the auth hardening already shipped this cycle
([rootguard#274](https://github.com/foxly-it/rootguard/pull/274): rate
limiting no longer trusts `X-Forwarded-For`, password recovery writes are
now failure-safe, logout surfaces persistence errors, the static-file guard
no longer misinterprets `[ ] * ?` as glob patterns). Additional checks this
pass:

- **Route audit/rate-limit coverage**: cross-referenced every mutating
  (`POST`/`PUT`/`DELETE`) route in `router.go` against `guardDestructive`
  wrapping. Every route that actually *mutates* state (deploy, service
  actions, backup export/restore, cleanup, control-plane/service updates,
  Unbound settings/custom-config/import apply, diagnostic logging,
  AdGuard bootstrap/filtering) is wrapped. Routes correctly left unwrapped
  are read-only by design: `preflight`, `updates/check`,
  `control-plane-updates/check`, `restore/preview`, `unbound/preview`,
  `unbound/advice`, `unbound/forward-check`, `unbound/custom/preview`,
  `unbound/import/preview`, `unbound/import-conf` (classifies, doesn't
  apply), and the FRITZ!Box/reverse-DNS discovery endpoints (network scans,
  not state changes).
- **Common anti-pattern sweep** across both Go modules and the frontend:
  no `InsecureSkipVerify`/disabled TLS verification, no shell-interpolated
  `exec.Command`, no `dangerouslySetInnerHTML`, no `eval`/`new Function`,
  no hardcoded credentials outside test/example fixtures. All clean.
- **`docs/threat-model.md` currency check**: no references to the
  since-fixed `X-Forwarded-For` behavior or anything else this cycle's
  fixes would have made inaccurate - the document's level of abstraction
  (rate limiting as a feature, not the specific key it's computed from)
  didn't need updating.

No new findings. Combined with the user-provided independent review this
cycle (full test suites, `go vet`, race detector, `gofmt`, ESLint, TS/Vite
build, `npm audit --offline`, Compose validation, `shellcheck`, `gitleaks`
across the full history, `trivy`) - see PRs
[#274](https://github.com/foxly-it/rootguard/pull/274),
[#275](https://github.com/foxly-it/rootguard/pull/275),
[#278](https://github.com/foxly-it/rootguard/pull/278) for what that review
found and how it was fixed - this closes the 0.9 checklist item.

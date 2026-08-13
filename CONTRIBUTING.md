**English** · [Deutsch](CONTRIBUTING.de.md)

# Contributing to RootGuard

Thanks for your interest in RootGuard. Contributions can be code, tests,
documentation, translations, or reproducible bug reports.

## Getting started

1. Check existing [issues](https://github.com/foxly-it/rootguard/issues) and
   the [roadmap](ROADMAP.md).
2. [`good first issue`](https://github.com/foxly-it/rootguard/labels/good%20first%20issue)
   and [`help wanted`](https://github.com/foxly-it/rootguard/labels/help%20wanted)
   are good starting points for a first contribution.
3. For larger changes, briefly describe your planned approach in the issue
   before implementing it.

Security vulnerabilities don't belong in public issues. Use the
confidential reporting path described in [SECURITY.md](SECURITY.md)
instead.

## Development environment

```sh
git clone https://github.com/foxly-it/rootguard.git
cd rootguard
cp .env.example .env
```

Set a strong administrator password plus separate random API and recovery
tokens in `.env`. You can then build the development stack:

```sh
docker compose up --build -d
```

## Choosing the right directory

RootGuard is a monorepo. Application code belongs in its component
directory, each with its own Dockerfile and path-filtered CI workflow:

- `rootguard-core` – orchestration and internal API
- `rootguard-webapp` – user interface and authentication
- `rootguard-updater` – controlled Core/WebApp updates
- `rootguard-unbound` – Unbound image and resolver base

A pull request can change one or more of these directories together with
the matching doc updates (`ROADMAP.md`, `docs/project-state.md`) in one
step.

## Making changes

- Keep a pull request scoped to one clearly defined problem.
- Add or update tests for changed behavior.
- Update the manual, Wiki, and project status when visible behavior or the
  documented feature set changes.
- Never publish credentials, `.env` files, tokens, or private network
  details.
- Provide both the German and English text for new UI copy when the
  affected surface is bilingual.

## Commit messages

Every commit's title (or, for a squashed pull request, the merge commit's
title) follows [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <short description>
```

Common types: `feat` (new behavior), `fix` (bug fix), `docs`
(documentation), `refactor`, `perf`, `test`, `ci`, `chore`. A `!` after the
type/scope (e.g. `feat!:`) or a `BREAKING CHANGE:` line in the commit body
marks a backwards-incompatible change.

`cliff.toml` generates the [CHANGELOG.md](CHANGELOG.md) section for each
release from this history automatically - a commit without a matching type
just lands under "Other" instead of the right category, without breaking
anything.

## Before the pull request

For changes in the main repository:

```sh
git diff --check
docker compose config
```

Also run the tests for the affected component directory. The integration
CI additionally starts a complete stack and checks login, setup, DNS
resolution, and DNSSEC.

## Pull request

A pull request should include:

- a short explanation of the problem and the solution;
- the checks you ran;
- screenshots for visible changes;
- notes on migration, configuration, or known limitations;
- a link to the related issue, if one exists.

By contributing, you agree to release your contribution under the
project's [AGPL-3.0-or-later](LICENSE) license.

## Release process (maintainers only)

A new alpha release is triggered through the "Cut next alpha release"
workflow (`.github/workflows/release-version-bump.yml`, manually invoked
via `workflow_dispatch`). It determines the next version number itself,
generates the matching section in [CHANGELOG.md](CHANGELOG.md) from the
commit history since the last tag, commits, and tags - the existing
`release-alpha.yml` workflow then takes over build, signing, and
publishing exactly as before.

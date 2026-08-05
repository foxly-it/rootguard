# Contributing to RootGuard WebApp

Thanks for helping make RootGuard easier and safer to use.

## Before you start

Check the [issues](https://github.com/foxly-it/rootguard-webapp/issues) and the
main [RootGuard roadmap](https://github.com/foxly-it/rootguard/blob/main/ROADMAP.md).
Discuss larger changes to authentication, API contracts, navigation, or setup
workflows in an issue first.

Security vulnerabilities must be reported privately through
[SECURITY.md](SECURITY.md).

## Development

Backend:

```sh
cd backend
go test ./...
go vet ./...
```

Frontend:

```sh
cd frontend
npm ci
npm run lint
npm run build
```

Preserve the WebApp/Core trust boundary and never add Docker-socket access or
arbitrary host execution. User-facing changes must support German and English,
include accessible labels, and provide screenshots without secrets or private
network data.

## Pull requests

Explain the problem, solution, security implications, and checks performed.
Link the related issue and include screenshots for visible changes.
Contributions are accepted under [AGPL-3.0-or-later](LICENSE).

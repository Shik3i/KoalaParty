# Contributing to KoalaParty

By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Requirements

- Go 1.26.5+
- Node.js 24+ and npm 11+
- Docker with Compose for full-stack and image checks

## Setup

```sh
git clone https://github.com/Shik3i/KoalaParty.git
cd KoalaParty
cp .env.example .env
cd frontend && npm ci && cd ..
make verify
docker compose up --build
```

The production-like service is available at `http://127.0.0.1:8080`.

## Before a pull request

Run the same checks as CI:

```sh
cd backend && gofmt -l . && go vet ./... && go test ./...
cd frontend && npm run check && npm run lint && npm test -- --run && npm run build
cd frontend && npm run test:e2e
node --test scripts/*.test.mjs
docker build -t koalaparty:verify .
```

For synchronization or layout changes, also use two tabs, two identities, a narrow mobile viewport, and the manual YouTube smoke test in [docs/testing.md](docs/testing.md).

## Conventions

- Use Conventional Commit prefixes such as `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, and `chore:`.
- Keep HTTP and WebSocket state authoritative on the server. Every state mutation must be transactional and revision-checked.
- Validate all untrusted JSON, URL, identity, storage, and provider data before use.
- Preserve the privacy boundary: no analytics, ads, tracking, fingerprinting, remote fonts, or unrelated third-party requests.
- Keep runtime dependencies minimal and explain new dependencies in the pull request.
- Update relevant documentation and the `[Unreleased]` changelog section with user-visible changes.

## Releases

Only maintainers publish releases. See [docs/releasing.md](docs/releasing.md). A release tag must match `vX.Y.Z`, point at a green `main` commit, and have a matching non-empty `CHANGELOG.md` section before the tag is pushed.

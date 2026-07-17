# KoalaParty

[![CI](https://github.com/Shik3i/KoalaParty/actions/workflows/ci.yml/badge.svg)](https://github.com/Shik3i/KoalaParty/actions/workflows/ci.yml)
[![Release](https://github.com/Shik3i/KoalaParty/actions/workflows/release.yml/badge.svg)](https://github.com/Shik3i/KoalaParty/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/Shik3i/KoalaParty)](https://github.com/Shik3i/KoalaParty/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Privacy-first shared YouTube rooms. Synchronized playback and a collaborative queue without advertising, analytics, fingerprinting, or a required account for unlisted rooms.

## Features

- Server-authoritative play, pause, seek, queue, reorder, skip, and reconnect behavior.
- Persistent anonymous browser identities; optional accounts and friend-only rooms.
- Owner/admin/member roles, per-member capabilities, kick, ban, visibility, and activity history.
- Optional public discovery with controlled provider metadata; disabled by default for early-beta deployments.
- Responsive SvelteKit UI with accessible loading, empty, error, connection, light, dark, and system-theme states.
- Go, WebSockets, SQLite WAL, embedded migrations, Docker, health/readiness probes, and Caddy-ready TLS deployment.

## Quick start

Requirements: Go 1.26.5, Node.js 24 LTS, npm 12, and Docker.

```sh
git clone https://github.com/Shik3i/KoalaParty.git
cd KoalaParty
cd frontend && npm ci && cd ..
make verify
docker compose up --build
```

Open `http://127.0.0.1:8080`. The anonymous identity belongs to the current browser storage; create an account before clearing it if room ownership must be recoverable.

## Deploy a release image

Release images support `linux/amd64` and `linux/arm64`:

```sh
cp .env.example .env
docker pull ghcr.io/shik3i/koalaparty:0.1.0
docker compose -f deploy/docker-compose.ghcr.yml up -d
```

The official deployment is preconfigured for `https://party.koalastuff.net` and the shared external `caddy_net` used by the other Koala services. Pin an exact image version and verify `KOALAPARTY_TRUSTED_PROXIES` against that Docker network. Public room discovery remains disabled until `KOALAPARTY_PUBLIC_ROOMS=true` is explicitly selected. See [deployment](docs/deployment.md).

## Verification

```sh
make verify
cd frontend && npm run test:e2e
cd ../backend && go test -race -count=1 ./...
cd .. && node --test scripts/*.test.mjs
docker compose up -d --build
```

CI additionally runs dependency scanning, a frontend audit, Docker build/health smoke tests, and the browser suite. See [testing](docs/testing.md).

## Documentation

- [Architecture](docs/architecture.md) and [project structure](docs/project-structure.md)
- [Authentication](docs/authentication.md), [permissions](docs/permissions.md), and [protocol](docs/protocol.md)
- [Database](docs/database.md), [privacy](docs/privacy.md), and [known limitations](docs/limitations.md)
- [Deployment](docs/deployment.md), [testing](docs/testing.md), and [releasing](docs/releasing.md)
- [Changelog](CHANGELOG.md), [security policy](SECURITY.md), and [contributing guide](CONTRIBUTING.md)

## Releases

Stable tags matching `vX.Y.Z` publish a multi-architecture GHCR image with SBOM, provenance, and attestation, plus checksummed deployment bundles and a GitHub Release generated from the matching changelog section.

## License

KoalaParty is free and open-source software licensed under the [MIT License](LICENSE).

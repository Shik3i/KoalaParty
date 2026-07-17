# KoalaParty

Privacy-first, open-source shared YouTube rooms. No tracking, advertising, or account required for unlisted rooms.

> No software license has been selected yet. Until the project owner selects one, normal copyright restrictions apply despite the open-source product intention.

## Development

Requirements: Go 1.26, Node.js 24, npm 11.

```sh
cp .env.example .env
cd frontend && npm ci
make verify
make dev
```

Frontend development runs at `http://localhost:5173`; the production-like Docker service runs at `http://localhost:8080`.

## Architecture

- Static SvelteKit/TypeScript SPA with semantic forest-green design tokens and system/light/dark themes.
- Go REST + WebSocket backend with server-authoritative playback, queue, roles, visibility, and activity.
- SQLite WAL database with embedded versioned migrations, foreign keys, transactions, retention, and soft cleanup.
- Anonymous Argon2id-authenticated identities plus optional username/password accounts and HTTP-only sessions.
- Multi-stage Docker image, persistent Compose volume, health/readiness endpoints, and Caddy-ready TLS proxy.

Documentation: [architecture](docs/architecture.md), [database](docs/database.md), [protocol](docs/protocol.md), [authentication](docs/authentication.md), [permissions](docs/permissions.md), [privacy](docs/privacy.md), [deployment](docs/deployment.md), [testing](docs/testing.md), and [known limitations](docs/limitations.md).

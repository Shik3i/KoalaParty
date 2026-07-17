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

See [docs/architecture.md](docs/architecture.md), [docs/protocol.md](docs/protocol.md), [docs/privacy.md](docs/privacy.md), and [docs/testing.md](docs/testing.md).


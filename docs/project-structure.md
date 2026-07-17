# Project structure

```text
KoalaParty/
├── .github/workflows/       CI and tag-triggered release automation
├── backend/
│   ├── cmd/server/          Server entrypoint and CLI commands
│   └── internal/
│       ├── app/             HTTP, WebSocket, auth, room, account, and maintenance logic
│       └── database/        SQLite bootstrap and embedded ordered migrations
├── deploy/                  GHCR Compose and Caddy production examples
├── docs/                    Architecture, operations, protocol, privacy, and release guides
├── frontend/
│   ├── e2e/                 Playwright multi-context browser flows
│   └── src/                 SvelteKit routes, UI components, state helpers, styles, and unit tests
├── scripts/                 Release metadata validation and its unit tests
├── Dockerfile               Reproducible frontend/backend production image
├── docker-compose.yml       Local build-and-run stack
├── CHANGELOG.md             Release notes and unreleased changes
└── Makefile                 Common development and verification commands
```

## Boundaries

- The backend owns identity, authorization, room membership, revisions, playback, queues, moderation, and persistence.
- The frontend renders snapshots and sends intent; it must not infer authoritative state transitions.
- Database changes are append-only numbered SQL migrations embedded into the server binary.
- Provider media metadata is controlled and bounded. User-authored room titles, descriptions, chat, analytics, and tracking are outside the current release.
- The Docker image is the primary deployment artifact. The release bundle contains Compose, environment, proxy, and documentation files for that image.

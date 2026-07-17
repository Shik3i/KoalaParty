# Testing strategy

`make verify` runs backend tests and static analysis plus frontend formatting, lint, type checks, unit tests, and production build. CI also builds the Docker image. Browser synchronization tests use a mock media provider; they do not claim to test YouTube.

Exact commands:

```sh
cd backend && go vet ./... && go test ./...
cd frontend && npm run lint && npm run check && npm test -- --run && npm run build
cd frontend && npx playwright install chromium && npm run test:e2e
docker compose build
```

The Playwright suite uses isolated browser contexts for owner, member, and banned identities. It checks room creation/join, presence, default playback, queue synchronization, server-side permission denial, admin restoration, owner protection, ban reconnect denial, and owner restoration after reload. SQLite unit tests verify clean migration, WAL/foreign keys, persistence, stale revision rejection, Argon2id round trips, activity retention, and abandoned-room cleanup.

## Manual YouTube smoke test

1. Open one room in two browser profiles and select **Start watching** in both.
2. Add `https://www.youtube.com/watch?v=dQw4w9WgXcQ` and use **Play now**.
3. Confirm privacy-enhanced iframe loading, play/pause/seek synchronization, drift correction, queue auto-advance, and a clear unavailable-video state.

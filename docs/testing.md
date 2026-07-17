# Testing strategy

`make verify` runs backend tests and static analysis plus frontend formatting, lint, type checks, unit tests, and production build. CI also builds the Docker image. Automated browser tests cover application synchronization and consent-gated YouTube API loading; real YouTube playback remains a manual smoke test.

Exact commands:

```sh
cd backend && go vet ./... && go test ./...
cd frontend && npm run lint && npm run check && npm test -- --run && npm run build
cd frontend && npx playwright install chromium && npm run test:e2e
node --test scripts/*.test.mjs
node scripts/verify-release.mjs v0.1.0
docker compose build
```

The Playwright suite uses isolated browser contexts for owner, member, and banned identities plus two tabs sharing one owner session. It checks room creation/join, presence, multi-tab session reuse, consent-gated YouTube loading, advancing pause positions, queue synchronization, server-side permission denial, admin restoration, owner protection, ban reconnect denial, and owner restoration after reload. SQLite unit tests verify clean migration, WAL/foreign keys, persistence, stale revision rejection, Argon2id round trips, activity retention, and abandoned-room cleanup.

`scripts/verify-release.test.mjs` covers strict stable SemVer tag parsing and exact changelog-section extraction. CI also runs `govulncheck`, `npm audit --audit-level=high`, a Docker build, `/api/ready`, and `/api/version` against a clean container. Release jobs repeat the test gates before publishing.

## Manual YouTube smoke test

1. Open one room in two browser tabs or profiles and select **Start watching** in both.
2. Start `https://www.youtube.com/watch?v=M7lc1UVf-VE` with **Play now**.
3. Queue `https://www.youtube.com/watch?v=aqz-KE-bpKQ`, then use **Skip next**.
4. Confirm privacy-enhanced iframe loading, play/pause/seek synchronization, elapsed-position preservation, queue advance, reload recovery, and reconnect after a brief server restart.
5. Optionally try `https://www.youtube.com/watch?v=dQw4w9WgXcQ` to confirm the embedded player's unavailable-video state.

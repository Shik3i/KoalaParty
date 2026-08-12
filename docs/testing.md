# Testing strategy

`make verify` runs backend race tests and static analysis plus frontend formatting, lint, type checks, unit tests, and production build. CI also builds the Docker image. Automated browser tests cover application synchronization and automatic privacy-enhanced YouTube API loading on room entry; real YouTube playback remains a manual smoke test. The player also records a bounded local diagnostic ring (never uploaded automatically), handles YouTube autoplay blocking, retries one transient start failure, watches for a stuck start, and re-requests playback after online/visibility recovery. Use **Copy diagnostics** in a room when a browser-specific playback failure needs investigation.

Exact commands:

```sh
cd backend && go vet ./... && go test -race -count=1 ./...
cd frontend && npm run lint && npm run check && npm test -- --run && npm run build
cd frontend && npx playwright install chromium && npm run test:e2e
node --test scripts/*.test.mjs
node scripts/verify-release.mjs v0.2.0
docker compose build
```

The Playwright suite uses isolated browser contexts for owner, member, and banned identities plus two tabs sharing one owner session. It checks room creation/join, presence, multi-tab session reuse, automatic YouTube loading, advancing pause positions, queue synchronization, server-side permission denial, admin restoration, owner protection, ban reconnect denial, owner restoration after reload, private invitations, the cross-device room library, ownership transfer, leaving and deleting rooms, profile updates, password changes, and account deletion. SQLite integration tests verify clean migration, WAL/foreign keys, persistence, stale revision rejection, Argon2id round trips, activity retention, abandoned-room cleanup, online backup/restore integrity, account/session self-service, invitation access, room lifecycle management, privacy deletion, and report handling. Security regression tests cover cross-origin session bootstrap, JSON content types, linked-device reauthentication, active-room capacity, aggregate WebSocket limits, and bounded SponsorBlock data. Configuration tests cover production fail-fast validation and trusted-proxy address parsing, including spoofed forwarding headers.

`scripts/verify-release.test.mjs` covers strict stable SemVer tag parsing and exact changelog-section extraction. CI also runs `govulncheck`, `npm audit --audit-level=high`, a Docker build, `/api/ready`, and `/api/version` against a clean container. Release jobs repeat the test gates before publishing.

## Manual YouTube smoke test

1. Open one room in two browser tabs or profiles and confirm the privacy-enhanced YouTube player loads in both.
2. Start `https://www.youtube.com/watch?v=M7lc1UVf-VE` with **Play now**.
3. Queue `https://www.youtube.com/watch?v=aqz-KE-bpKQ`, then use **Skip next**.
4. Confirm privacy-enhanced iframe loading, play/pause/seek synchronization, elapsed-position preservation, queue advance, reload recovery, and reconnect after a brief server restart.
5. Optionally try `https://www.youtube.com/watch?v=dQw4w9WgXcQ` to confirm the embedded player's unavailable-video state.

## Playback failure matrix

- Open two rooms/tabs with autoplay blocked or sound permissions denied: the room must continue muted and show the one-tap unmute action, without broadcasting a phantom pause.
- Replace a video while the previous iframe is buffering: a late error or `ENDED` callback must not skip or cover the replacement.
- Use an unavailable, private, or embed-disabled video: the error stays attached to that media item; it is never auto-skipped. `Try again` is bounded, while `Skip this video` explicitly discards it.
- Hide the tab, go offline, restore connectivity, and return to the tab while a room is playing: the client records local lifecycle events and re-requests playback after recovery.
- Repeat the same REST command with the same `requestId`: the room revision and queue change once. Reuse the same key for another command: expect HTTP 409 with `request_id_conflict`.

Playwright starts a compiled backend binary through `scripts/build-e2e-server.mjs`; the test-only shutdown hook closes it before Playwright's Windows process cleanup, so no `go run` wrapper or orphan server remains.

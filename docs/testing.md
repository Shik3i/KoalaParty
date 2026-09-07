# Testing strategy

`make verify` runs backend race tests and static analysis plus frontend formatting, lint, type checks, unit tests, and production build. CI also builds the Docker image. Automated browser tests cover application synchronization and automatic privacy-enhanced YouTube API loading on room entry; the isolated-browser real YouTube smoke test below is mandatory before every release tag. The player also records a bounded local diagnostic ring (never uploaded automatically), handles YouTube autoplay blocking, retries one transient start failure, watches for a stuck start, and re-requests playback after online/visibility recovery. Use **Copy diagnostics** in a room when a browser-specific playback failure needs investigation.

Exact commands:

```sh
cd backend && go vet ./... && go test -race -count=1 ./...
cd frontend && npm run lint && npm run check && npm test -- --run && npm run build
cd frontend && npx playwright install chromium firefox webkit && npm run test:e2e
node --test scripts/*.test.mjs
node scripts/verify-release.mjs v0.2.0
docker compose build
```

The Playwright suite runs in Chromium, Firefox, and WebKit. It uses isolated browser contexts for owner, member, and banned identities plus two tabs sharing one owner session. It checks room creation/join, presence, multi-tab session reuse, automatic YouTube loading, personalized queue votes, advancing pause positions, rejected-command recovery, queue synchronization, validated end-of-video advancement, standard and WebKit fullscreen teardown, autoplay prompt lifecycle, compact mobile error states, keyboard controls, picture-in-picture iframe permission, diagnostics download, page-suspend recovery, offline reconnect, server-side permission denial, admin restoration, owner protection, ban reconnect denial, owner restoration after reload, private invitations, the cross-device room library, ownership transfer, leaving and deleting rooms, profile updates, password changes, and account deletion. SQLite integration tests verify clean migration, WAL/foreign keys, persistence, stale revision rejection, Argon2id round trips, activity retention, abandoned-room cleanup, online backup/restore integrity, account/session self-service, invitation access, room lifecycle management, privacy deletion, and report handling. Security regression tests cover cross-origin session bootstrap, JSON content types, linked-device reauthentication, personalized 100-viewer room fan-out and capacity, aggregate WebSocket limits, bounded SponsorBlock data, and terminal playback-report validation. Configuration tests cover production fail-fast validation and trusted-proxy address parsing, including spoofed forwarding headers.

`scripts/verify-release.test.mjs` covers strict stable SemVer tag parsing and exact changelog-section extraction. CI also runs `govulncheck`, `npm audit --audit-level=high`, a Docker build, `/api/ready`, and `/api/version` against a clean container. Release jobs repeat the test gates before publishing.

## Manual YouTube smoke test

Run this against the exact production build in an isolated browser before creating a release tag. Record the viewport and measured player/iframe bounds; visual inspection alone is insufficient.

1. Open one room in two browser tabs or profiles and confirm the privacy-enhanced YouTube player loads in both.
2. Start an embeddable real video with **Play now**. Confirm both players advance together, then pause from the second tab and verify both remain at the same stable position.
3. At desktop and `390 × 844`, confirm the player mount and iframe exactly fill the 16:9 player and the document has no horizontal overflow.
4. Confirm theater mode keeps the iframe equal to the enlarged player. Float the mini-player on desktop and mobile; it must remain fully visible and must not overlap the mobile bottom navigation.
5. Enter and exit the real YouTube fullscreen control. Confirm the iframe fills the viewport in fullscreen, returns to the player bounds afterward, and playback resynchronizes.
6. Start the 19-second `https://www.youtube.com/watch?v=jNQXAC9IVRw`, queue another embeddable video, and let the first video end naturally. Confirm the queued video becomes current and starts automatically with sound in both tabs. Repeat with an empty queue in fullscreen; confirm fullscreen closes, the iframe is removed, the player shows the empty state, and the room records `finished the video`.
7. Reload while the shared clock is at the completed video's duration. Confirm the old video does not loop between its end and the first second; it must submit one terminal report and start the next queued video with sound.
8. Queue `https://www.youtube.com/watch?v=aqz-KE-bpKQ`, then use **Skip next**. Confirm queue advance, elapsed-position preservation, reload recovery, and reconnect after a brief server restart.
9. Try an unavailable or embed-disabled video at desktop and `390 × 844`. Confirm the opaque error state remains inside the player, both actions remain usable, and error notices stay above mobile navigation. Then block the API request once and verify **Reload player** creates a clean retry.

## Playback failure matrix

- Open two rooms/tabs with autoplay blocked or sound permissions denied: KoalaParty must never mute either player. The blocked tab must show the one-tap **Autoplay blocked — play with sound** action without broadcasting a phantom pause.
- Replace a video while the previous iframe is buffering: a late error or `ENDED` callback must not skip or cover the replacement.
- Use an unavailable, private, or embed-disabled video: the error stays attached to that media item; it is never auto-skipped. `Try again` is bounded, while `Skip this video` explicitly discards it.
- Hide the tab, go offline, restore connectivity, and return to the tab while a room is playing: the client records local lifecycle events and re-requests playback after recovery.
- Repeat the same REST command with the same `requestId`: the room revision and queue change once. Reuse the same key for another command: expect HTTP 409 with `request_id_conflict`.

Playwright starts a compiled backend binary through `scripts/build-e2e-server.mjs`; the test-only shutdown hook closes it before Playwright's Windows process cleanup, so no `go run` wrapper or orphan server remains.

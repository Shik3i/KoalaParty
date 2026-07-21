# Changelog

All notable changes are documented here. KoalaParty follows semantic versioning.

## [Unreleased]

### Added

- Added up to five recently visited rooms to the start page for quick return without creating another room. The browser-local list works without an account and supports removing individual entries.

### Fixed

- Removed a data race between live session-TTL updates and concurrent session creation.
- Prevented client-supplied fallback titles from overwriting shared YouTube metadata across rooms, and broadcast trusted metadata updates to every affected active room.
- Counted display-name limits by Unicode characters consistently in the browser and server.
- Released the WebSocket hub lock before the admin statistics endpoint queries SQLite.

## [0.8.0] - 2026-07-20

### Added

- Color designs: pick a palette (Eucalyptus, Ocean, Amber, Grape or Rose) from the new dropdown in the header. It is independent of the light/dark/system theme — each design has its own light and dark variant — and your choice persists across sessions.
- When another participant presses play, the video now starts for everyone immediately. Browsers block autoplay with sound until you interact with the page, so passive viewers now start playback muted automatically (in sync with everyone) and get a one-tap "Muted — tap for sound" button, instead of sitting on a paused video until they click.

### Changed

- Moved the theater-mode toggle out of the controls panel to a row directly under the player, next to the seek bar, so it is easier to find and reach.

## [0.7.0] - 2026-07-20

### Changed

- Switched the interface typeface to Inter throughout (self-hosted) for a cleaner, more neutral look; removed the previous display font.
- Removed the click-to-consent "Start watching" gate. Since a room now always has a video cued, the embedded YouTube player loads as soon as you open a room, with a small persistent notice next to it and in the privacy policy. Self-hosters should note this changes the privacy posture: YouTube's privacy-enhanced player is loaded on room entry rather than after an explicit click.

## [0.6.0] - 2026-07-19

### Added

- A freshly created room now starts with a random one of the quick-add videos already cued, so the player is never a blank "add a video" screen — just press play. The real title is filled in automatically in the background.

### Changed

- Participant avatars now use the emoji from a member's generated name as a little profile picture, and the name is shown without the emoji prefix (e.g. a "🦭" avatar next to "Sleepy Seal", instead of a broken initial). Custom names still fall back to an initial.

## [0.5.3] - 2026-07-19

### Fixed

- Entering a room no longer dead-ends on a transient failure. A brief `502 Bad Gateway` or network blip — e.g. while the server is restarting or during a deploy — now shows a "Reconnecting to the room…" state and retries automatically, recovering on its own once the server is back, instead of the old "Couldn't enter this room" screen that required a manual reload. Genuine access errors (private room, ban, unknown room) still surface immediately.

## [0.5.2] - 2026-07-19

### Changed

- YouTube consent is remembered: you confirm once and afterwards any room loads the player automatically, instead of clicking "Start watching" on every visit. The privacy gate still applies on the first ever visit and returns if you clear browser storage.

### Fixed

- The "Start watching" and "Play from queue" buttons no longer flee from the cursor on hover — the global hover-lift was overriding their centering transform and knocking them out of place.
- Removed the unfinished-looking "Chat · Later" placeholder from the activity panel; the header is now a clean "Activity".
- Navigation labels no longer wrap onto two lines on narrow screens.

## [0.5.1] - 2026-07-19

### Fixed

- Critical: the server could crash — returning `502 Bad Gateway` for everyone trying to join a room — if the background title-enrichment goroutine introduced in 0.5.0 panicked. An unrecovered panic in a goroutine takes down the whole Go process; the goroutine now recovers (and logs) instead of crashing the server.
- Generated names pair each animal with its own matching emoji (e.g. "🐳 Gentle Whale", "🦉 Sunny Owl", "🦈 Snug Shark"). Previously the emoji and animal were chosen independently, so you could end up as a "Kangaroo" with a butterfly emoji.

### Added

- The running app version is shown in the footer next to the GitHub link, linking to the matching release.
- Online presence: the participants panel shows how many people are currently online, with a live status dot on each avatar.

## [0.5.0] - 2026-07-18

### Fixed

- Adding a video is now instant. The video is queued immediately with a placeholder title while the real YouTube title is fetched in the background and filled in a moment later. Previously the add request blocked on the title lookup (up to several seconds), which could make adding feel like the app had frozen — especially when the server's outbound connection to YouTube was slow or unavailable.
- Anonymous identities now work over plain-HTTP LAN addresses. `crypto.randomUUID()` only exists in secure contexts, so opening a self-hosted instance over `http://<lan-ip>` previously threw when creating an identity or sending a command; a `getRandomValues`-based UUID fallback now handles those origins.
- Added a global error boundary: an unexpected client-side error now shows a recoverable message instead of a blank page.

### Added

- Theater mode: a toggle enlarges the player to the full content width and moves the queue/people/activity panel below it, like YouTube's theater view.

### Changed

- Redesigned the theme switcher into an on-brand segmented control with sun / moon / monitor icons instead of a plain dropdown.
- Renamed the landing "Start a living room" card to "Jump into a room" and refreshed its copy.

## [0.4.1] - 2026-07-18

### Changed

- Much greater variety in generated names: anonymous display names and room labels now draw from a shared pool of emoji, adjectives, and ~30 animals (over 25,000 combinations) instead of everyone being "Koala" — e.g. "🦋 Cheerful Kangaroo", "🦊 Calm Koala". Display names stay within the server's length limit.

## [0.4.0] - 2026-07-18

### Added

- A synchronized playback progress bar under the player with elapsed/total time, visible to everyone in the room (even before pressing "Start watching") and reflecting the shared position live.
- Consistent [Phosphor](https://phosphoricons.com/) icons across navigation, room controls, queue actions, status toasts, player states, and the landing page (the playful quick-add emoji are kept).
- Space Grotesk display typeface for headings, self-hosted (no third-party font requests) and bundled with its SIL Open Font License.
- Open Graph and Twitter Card metadata plus light/dark `theme-color`, so shared links render with a proper title and description.

### Changed

- Motion pass, all respecting `prefers-reduced-motion`: status toasts slide in, the confirm dialog fades and scales, queue reordering animates items into place, buttons lift subtly on hover, and the landing hero has a slow ambient glow.

### Fixed

- Seeking now propagates immediately. The player watches for a discontinuity in its own timeline, so a scrub is detected even while the player briefly buffers — previously the seek was swallowed during buffering and only took effect after the next play/pause.
- Playback stays far tighter in sync. The live position is now anchored to the last confirmed playback change and extrapolated from there, instead of being re-based to "now" on every unrelated snapshot (which collapsed the expected position backwards and caused several seconds of drift).
- Followers are continuously realigned. The player periodically corrects drift toward the server's expected position (silently, without re-broadcasting), keeping participants within a small tolerance instead of drifting 3–4 seconds apart.
- The member management menu (make admin, kick, ban) is no longer clipped by the surrounding scroll panels; it renders as a fixed popover above other content.

### Added

- The invite link is copied to your clipboard automatically when you create a room, with a confirmation shown in the new room.
- Queue items can be reordered with up/down buttons, so reordering now works on touch devices and by keyboard in addition to drag-and-drop.
- A "Skip this video" button appears when a video cannot be played or embedded, so an unplayable video no longer blocks the room.

### Changed

- Kick, ban, leave, delete, and ownership-transfer confirmations now use an in-app themed dialog (Escape or the backdrop cancels) instead of the browser's native prompt.
- Status messages are colour-coded for success and error states.
- Added a bundled `favicon.ico` (silencing the previous 404), a loading spinner for the "Joining room…" state, a queue-aware idle-player message, and debounced player seek commands.

## [0.3.0] - 2026-07-18

### Fixed

- Playback now stays in sync when the viewer uses the video's own controls: play, pause, and seek performed directly in the YouTube player are broadcast to every participant and recorded in the activity feed. Previously only the in-app buttons synchronized, so scrubbing or pausing inside the player was invisible to others.
- Live synchronization during local `npm run dev`: the Vite dev proxy now forwards WebSocket connections, so rooms no longer sit on a permanent "Reconnecting" state in development.

### Added

- Real YouTube video titles, resolved server-side through YouTube's public oEmbed endpoint when a video is queued and shown in the queue, now-playing, and activity feed. Configurable with `KOALAPARTY_YOUTUBE_METADATA` (default `true`); set to `false` for zero outbound calls, which falls back to the video ID.
- One-click "Play from queue" control on the idle player to start the queue without hunting for the skip button.
- `HEALTHCHECK` in the Docker image using the built-in `healthcheck` subcommand, so plain `docker run` and GHCR deployments report health without Compose.

### Changed

- Unified the landing page into a single entry point: the room field creates a fresh room when empty and joins when a code or link is present, replacing the separate create/join buttons.
- Replaced the manual "seek in seconds" field with direct scrubbing on the player's own timeline, now that seeks synchronize.
- Improved light-theme contrast for muted text and warnings to meet WCAG AA; the home and room pages score 100 for Lighthouse accessibility, best practices, and SEO.
- `/api/me` now returns `204 No Content` instead of `401` for anonymous first-time visitors, removing a spurious browser-console error.
- Documented the server-side oEmbed title lookup in the privacy policy and notes, and corrected the release description in the README.

## [0.2.7] - 2026-07-18

### Added

- Web-based Admin Console (`/admin`) to view stats (online users, registered users, active rooms) and manage reports.
- Database-backed dynamic settings editor for configuration variables.
- One-click "Quick-Add Presets" and "Paste from Clipboard" helpers in the room queue UI.

## [0.2.1] - 2026-07-18

### Changed

- Upgraded Docker build base image to Node.js 26-alpine.
- Upgraded GitHub Actions softprops/action-gh-release to 3d0d9888cb7fd7b750713d6e236d1fcb99157228.

## [0.2.0] - 2026-07-17

### Added

- Full operator, hosting, retention, third-party, GDPR-rights, copyright, and self-hosting notices on dedicated privacy and imprint pages.
- Responsive KoalaSync cross-promotion with locally bundled Netflix, YouTube, Twitch, Prime Video, Disney+, Jellyfin, and Emby marks.
- Unit and browser coverage for legal disclosures, local platform artwork, external links, and responsive rendering.
- MIT License for open-source use, modification, distribution, and self-hosting.
- Host-only operator commands for verified SQLite backups/restores, report review and delisting, room deletion, and privacy-preserving account deletion.
- Hardened pinned-image Compose example with a read-only root filesystem, dropped capabilities, resource limits, and log rotation.
- Cross-device My Rooms library for owned and joined rooms with live status, open, leave, and delete actions.
- Private-room invitation management with account lookup, listing, and revocation.
- Account self-service for display names, password changes, active-session review/revocation, logout, and verified account deletion.
- Room settings for visibility, invitations, ownership transfer, leaving, and permanent room closure.
- Backend integration and multi-account browser coverage for the complete room and account management lifecycle.

### Changed

- YouTube playback now presents an explicit third-party consent notice before loading the privacy-enhanced player.
- The footer links directly to Privacy, Imprint, GitHub, and the KoalaSync landing page.
- YouTube thumbnails no longer contact third-party hosts before explicit playback consent.
- Public room discovery is opt-in and disabled by default for early-beta deployments.
- Production mode now rejects insecure cookies, non-HTTPS origins, malformed durations, booleans, proxy networks, and retention values instead of silently accepting unsafe fallbacks.
- Official deployment examples now use `party.koalastuff.net` and the shared external `caddy_net` convention used by the other Koala services.

### Fixed

- Crawlers now receive a valid local `robots.txt` instead of the application fallback document.
- Rate limits now identify clients through forwarding headers only when the immediate peer is an explicitly trusted proxy, preventing global throttling behind Caddy and header spoofing from untrusted peers.
- WebSocket broadcasts now personalize the snapshot identity per connected client, preventing another participant's join or command from temporarily changing the local UI role.

## [0.1.0] - 2026-07-17

### Added

- Privacy-first shared YouTube rooms with synchronized play, pause, seek, queue, and skip controls.
- Persistent anonymous browser identities plus optional username/password accounts and friend relationships.
- Owner, admin, member, per-capability permission, kick, ban, visibility, activity, and public-discovery flows.
- Static SvelteKit interface with system, light, and dark themes; responsive desktop and mobile layouts; accessible empty, loading, error, and connection states.
- Go REST and WebSocket server backed by SQLite WAL, embedded versioned migrations, retention, room cleanup, health, readiness, and build-information endpoints.
- Multi-stage non-root Docker image, local Compose stack, GHCR deployment Compose file, and Caddy example.
- Unit, integration, race, browser, build, formatting, lint, dependency-audit, Docker, and release-metadata checks.
- Tag-triggered multi-architecture GHCR publishing with SBOM, provenance, attestation, deployment bundles, checksums, and GitHub Releases.

### Fixed

- Room-wide optimistic revision checks now prevent competing tabs or clients from applying stale queue, moderation, visibility, and playback commands.
- Repeated snapshots no longer create duplicate join events, and multiple tabs no longer produce premature disconnect activity.
- Registration, friendship, room-join, migration, command-payload, queue, WebSocket, storage, and YouTube-player edge cases now fail safely.
- Mobile navigation, horizontal overflow, duplicate submissions, clipboard failures, stale drag state, YouTube loading timeouts, unavailable videos, and paused-video autoplay were hardened.

### Security and privacy

- Anonymous secrets and account passwords use Argon2id-backed server authentication; session tokens are stored as hashes in HttpOnly SameSite cookies.
- State-changing HTTP requests require CSRF tokens; WebSockets validate trusted origins before joining rooms.
- Server-side authorization is deny-by-default, with owner protection and transactional moderation changes.
- Restrictive CSP, framing, referrer, MIME, and browser-permission headers are enabled.
- No analytics, advertising, fingerprinting, third-party fonts, or tracking scripts are included.

### Known limitations

- YouTube playback depends on the external privacy-enhanced embed API after explicit user consent.
- Password recovery, MFA, passkeys, and anonymous identity recovery are not available yet.
- SQLite and the in-memory WebSocket hub target a single application instance; operators must perform WAL-aware backups.
- Public-room moderation reports require operator-side review tooling outside the current UI.
- The v0.1.0 release tag predates the MIT license added on the main branch.

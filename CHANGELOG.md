# Changelog

All notable changes are documented here. KoalaParty follows semantic versioning.

## [Unreleased]

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

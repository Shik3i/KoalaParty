# Changelog

All notable changes are documented here. KoalaParty follows semantic versioning.

## [Unreleased]

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
- Password recovery, MFA, passkeys, account deletion, anonymous identity recovery, and session-management UI are not available yet.
- SQLite and the in-memory WebSocket hub target a single application instance; operators must perform WAL-aware backups.
- Public-room moderation reports require operator-side review tooling outside the current UI.
- No software license has been selected; normal copyright restrictions currently apply.

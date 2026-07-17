# Security Policy

## Supported versions

KoalaParty is an early release. Security fixes target the latest release and `main`.

| Version | Supported |
| --- | --- |
| 0.1.x | Yes |
| < 0.1 | No |

## Reporting a vulnerability

Do not open a public issue for exploitable vulnerabilities. Use a [private GitHub security advisory](https://github.com/Shik3i/KoalaParty/security/advisories/new) and include the affected version, reproduction steps, impact, and any proposed mitigation.

## Current controls

- Argon2id secret and password hashing with random salts.
- Cryptographically random sessions; only token hashes are stored server-side.
- HttpOnly, SameSite cookies with configurable `Secure` enforcement.
- CSRF protection on authenticated state-changing HTTP requests.
- Trusted-origin checks before WebSocket room joins.
- Deny-by-default server authorization, owner invariants, per-member capabilities, and transactional moderation.
- Room-wide optimistic revisions for competing playback, queue, role, permission, moderation, and visibility commands.
- Strict request sizes, single-object JSON decoding, provider-ID validation, bounded playback positions, and rate limiting.
- SQLite foreign keys, WAL, busy timeout, embedded migrations, transactions, and non-root container execution.
- Restrictive Content Security Policy, frame denial, no-referrer, MIME-sniffing prevention, and browser-permission restrictions.
- No analytics, advertising, fingerprinting, remote fonts, raw session-token storage, or request-payload logging.
- CI tests, static checks, dependency audit, browser tests, Docker builds, release SBOM/provenance, and image attestations.

## Operator responsibilities

- Terminate TLS, set `KOALAPARTY_COOKIE_SECURE=true`, and configure exact HTTPS values in `KOALAPARTY_TRUSTED_ORIGINS`.
- Protect `.env`, the SQLite database, WAL files, backups, container registry credentials, and reverse-proxy configuration.
- Use WAL-aware backups and test restores.
- Keep the image and reverse proxy updated and review public-room reports.

## Known security limitations

- Password recovery, MFA, passkeys, session management, and account deletion are not implemented.
- Rate limits and active WebSocket presence are process-local and reset on restart.
- The SQLite/in-memory-hub architecture supports one application instance, not active-active replicas.
- YouTube embeds are an explicitly consented external dependency and remain subject to YouTube availability and policy.

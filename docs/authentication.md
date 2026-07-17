# Authentication and identity

On first use, the browser creates a UUIDv4 and 256-bit random secret. Both persist in local storage with the local display name and avatar seed. The UUID is an identifier, never a credential. The server stores an Argon2id hash of the secret and issues a short-lived, HTTP-only, SameSite=Lax session cookie after a valid exchange. State-changing REST requests also require the per-session CSRF token. WebSocket upgrades require the session and a configured trusted origin.

Tabs in the same browser reuse a matching active session instead of rotating the shared cookie. If another tab or login flow does replace the session, the frontend refreshes the current CSRF token once and retries the rejected state-changing request.

Accounts use a unique username and Argon2id password hash. No email is collected. Registration attaches the current identity instead of replacing it, preserving ownership, roles, permissions, bans, and activity attribution. Login uses the account's linked identity. Account settings support display-name and password changes, active-session listing/revocation, logout from other devices, and password-confirmed account deletion. Account deletion revokes credentials, anonymizes retained identity references, and closes rooms that were not transferred first. There is no password recovery because no email address is collected.

Authentication endpoints are rate-limited in memory. Rate-limit addresses expire with their short fixed window and are not written to SQLite or application logs. Production cookies must set `KOALAPARTY_COOKIE_SECURE=true` behind HTTPS.

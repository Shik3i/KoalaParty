# Deployment

1. Copy `.env.example` to `.env` and replace `KOALAPARTY_ADMIN_TOKEN` with a 32-byte random value.
2. Set `KOALAPARTY_TRUSTED_ORIGINS` to the exact HTTPS origin and `KOALAPARTY_COOKIE_SECURE=true`.
3. Set DNS for the chosen hostname and adapt `deploy/Caddyfile.example`.
4. Run `docker compose up -d --build`.
5. Verify `/api/health`, `/api/ready`, the persistent `koalaparty-data` volume, WebSocket upgrades, and a real YouTube smoke test.

The multi-stage image builds the static SvelteKit application and a static Go binary, then runs as an unprivileged user. Caddy terminates TLS. Do not expose the backend on a public non-TLS port. Back up the SQLite database using a WAL-aware snapshot procedure.

Room activity is pruned to 200 events and 30 days by default. Rooms inactive for 12 months are soft-deleted unless currently connected. All limits use environment variables. Container/runtime logs should use short external retention; application logs omit secrets and request payloads.


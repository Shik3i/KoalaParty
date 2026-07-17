# Deployment

1. Choose local source builds with `docker-compose.yml` or a published release with `deploy/docker-compose.ghcr.yml`.
2. Copy `.env.example` to `.env` and select `KOALAPARTY_VERSION`.
3. Set `KOALAPARTY_TRUSTED_ORIGINS` to the exact HTTPS origin and `KOALAPARTY_COOKIE_SECURE=true`.
4. Set DNS for the chosen hostname and adapt `deploy/Caddyfile.example`.
5. Run `docker compose up -d --build` for source builds or `docker compose -f deploy/docker-compose.ghcr.yml up -d` for GHCR releases.
6. Verify `/api/health`, `/api/ready`, `/api/version`, the persistent `koalaparty-data` volume, WebSocket upgrades, and a real YouTube smoke test.

The multi-stage image builds the static SvelteKit application and a static Go binary, then runs as an unprivileged user. Caddy terminates TLS. Do not expose the backend on a public non-TLS port. Back up the SQLite database using a WAL-aware snapshot procedure.

Published tags produce immutable version tags plus moving major, minor, and `latest` tags. Pin `KOALAPARTY_VERSION` to the full `X.Y.Z` version for reproducible deployments. Verify deployment-archive checksums from the GitHub Release and inspect the published image attestation when supply-chain provenance matters.

Room activity is pruned to 200 events and 30 days by default. Rooms inactive for 12 months are soft-deleted unless currently connected. All limits use environment variables. Container/runtime logs should use short external retention; application logs omit secrets and request payloads.

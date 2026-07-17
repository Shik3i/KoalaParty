# Deployment

## Production setup

1. Copy `docker-compose.example.yml` and `.env.example` to the deployment host.
2. Pin `KOALAPARTY_VERSION` to the full released version, never `latest`.
3. Use the existing external `caddy_net` shared by the other Koala services. The production Compose example deliberately publishes no host port.
4. Add `deploy/Caddyfile.example` to the central Caddy configuration. It serves `party.koalastuff.net` and proxies to `KoalaParty:8080` on `caddy_net`.
5. Keep `KOALAPARTY_PRODUCTION=true`, `KOALAPARTY_COOKIE_SECURE=true`, and `KOALAPARTY_PUBLIC_ROOMS=false` for the early beta.
6. Confirm the backend's immediate proxy address and restrict `KOALAPARTY_TRUSTED_PROXIES` to that IP or Docker subnet. Forwarding headers from every other peer are ignored.
7. Point the `party.koalastuff.net` DNS record at the existing Koala host and let Caddy obtain TLS automatically.
8. Start with `docker compose pull && docker compose up -d`.
9. Verify `/api/health`, `/api/ready`, `/api/version`, WebSocket upgrades, the persistent volume, and the manual two-client YouTube test through `https://party.koalastuff.net`.

Production mode fails before opening the database when secure cookies, exact HTTPS origins, durations, booleans, retention values, or trusted proxy networks are invalid. The example container binds only to loopback, runs without Linux capabilities, uses a read-only root filesystem, limits processes/memory/CPU, and rotates runtime logs.

`KOALAPARTY_TRUSTED_PROXIES=172.16.0.0/12` matches the common private Docker network range used by the shared Caddy setup. Narrow it to the actual `caddy_net` subnet after observing the immediate peer. Never use `0.0.0.0/0` or `::/0`.

## Backups and restore drills

Create a unique, timestamped backup inside the persistent volume:

```sh
docker compose exec koalaparty /koalaparty operator backup /data/backups/koalaparty-2026-07-17T120000Z.db
docker compose cp koalaparty:/data/backups/koalaparty-2026-07-17T120000Z.db ./backups/koalaparty-2026-07-17T120000Z.db
```

The command uses SQLite `VACUUM INTO`, refuses to overwrite a destination, and verifies the result with `PRAGMA integrity_check`. Schedule it daily with a host timer, copy backups off-host, encrypt the destination, retain at least seven daily and four weekly snapshots, and alert when the newest successful backup exceeds 25 hours.

Perform a restore drill without touching the live database:

```sh
docker compose exec koalaparty /koalaparty operator restore /data/backups/koalaparty-2026-07-17T120000Z.db /data/restore-drill.db
docker compose exec -e KOALAPARTY_DB=/data/restore-drill.db koalaparty /koalaparty operator reports list
```

For a real restore: stop the service, retain the live database and its `-wal`/`-shm` files as a rollback set, place the verified restored database at the configured `KOALAPARTY_DB` path, then start and verify readiness, version, login, room access, and WebSockets. Never replace a live WAL database file while the server is running.

## Moderation and privacy operations

These commands are available only to Timo Schmidt as operator of the official KoalaParty service, or to the respective host of an independent deployment. There is no additional in-app "operator" account to create:

```sh
docker compose exec koalaparty /koalaparty operator reports list
docker compose exec koalaparty /koalaparty operator reports resolve REPORT_ID
docker compose exec koalaparty /koalaparty operator reports delist REPORT_ID
docker compose exec koalaparty /koalaparty operator delete-room ROOM_ID
docker compose exec koalaparty /koalaparty operator delete-account USERNAME
```

`reports delist` resolves the report and immediately changes the room to unlisted. Account deletion revokes sessions and anonymous recovery, removes account and friendship data, unlinks/anonymizes retained identities, and preserves referential integrity for durable room history. Record the request, identity verification, command result, and completion timestamp outside application data.

Enable `KOALAPARTY_PUBLIC_ROOMS=true` only when Timo actively reviews reports and can respond to abuse and deletion requests. No separate KoalaParty role or account is required.

## Monitoring and rollback

- Probe `/api/ready` every minute and alert after three failures.
- Alert on container restart loops, volume usage above 80%, and backups older than 25 hours.
- Keep the previous exact image version available. Roll back by changing only `KOALAPARTY_VERSION`, then running `docker compose pull && docker compose up -d`.
- Take and verify a backup before every upgrade. Test schema compatibility in staging before rollback across a migration.
- Retain short container logs. Application logs intentionally omit secrets and request payloads.

Published tags produce immutable full-version tags plus moving major, minor, and `latest` tags. Deployments must use the full version. Verify deployment-archive checksums from the GitHub Release and inspect the published image attestation when supply-chain provenance matters.

Room activity is pruned to 200 events and 30 days by default. Rooms inactive for 12 months are soft-deleted unless currently connected. All limits use validated environment variables.

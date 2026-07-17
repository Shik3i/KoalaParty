# Architecture

KoalaParty is a static SvelteKit single-page application served by one Go binary. REST handles durable resources and authentication; WebSockets carry authoritative room snapshots and commands. SQLite runs in WAL mode with foreign keys and versioned migrations. Repositories isolate SQL so a future PostgreSQL adapter can implement the same service interfaces. The in-process room hub is deliberately replaceable by a future event bus.

The browser creates a random anonymous UUID and 256-bit secret. The server stores only an Argon2id hash and exchanges valid credentials for a short-lived HTTP-only session. Accounts attach to the existing identity rather than replacing it. Room authorization is always server-side.

Production: Caddy terminates TLS and proxies to the container. The Go binary serves the static frontend and `/api`. SQLite lives in the `koalaparty-data` volume.


# Database model and migrations

SQLite is opened with WAL mode, foreign keys, a five-second busy timeout, and one writer connection. `internal/database/migrations` contains embedded, ordered migrations; migration `001` creates the normalized initial model and records schema version 1 transactionally.

Core ownership and access are relational: `identities`, `accounts`, `friendships`, `rooms`, `room_members`, `room_permissions`, `room_invites`, and `room_bans`. `media_items`, `room_queue_items`, and `playback_states` persist shared media state. `room_events` contains structured event-specific JSON only; core state never depends on event JSON. `room_reports` stores controlled reasons and the public metadata snapshot reviewed by an operator.

Queue reordering, room-wide optimistic revisions, playback revisions, permissions, roles, bans, and activity insertion use transactions. Public identifiers are random; internal numeric IDs are not exposed. Repository/service boundaries are intentionally thin for the single-instance release, but SQL is isolated in the backend so a future PostgreSQL repository can replace SQLite without changing the wire protocol.

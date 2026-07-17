PRAGMA foreign_keys = ON;

CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE accounts (
  id TEXT PRIMARY KEY, username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  password_hash TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE identities (
  id TEXT PRIMARY KEY, account_id TEXT REFERENCES accounts(id), secret_hash TEXT NOT NULL,
  display_name TEXT NOT NULL CHECK(length(display_name) BETWEEN 1 AND 32), avatar_seed TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX identities_account_idx ON identities(account_id);
CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY, identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
  csrf_token TEXT NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX sessions_identity_idx ON sessions(identity_id);
CREATE TABLE friendships (
  requester_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  addressee_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK(status IN ('pending','accepted','declined','blocked')),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(requester_account_id, addressee_account_id), CHECK(requester_account_id <> addressee_account_id)
);
CREATE INDEX friendships_addressee_idx ON friendships(addressee_account_id, status);
CREATE TABLE rooms (
  id TEXT PRIMARY KEY, owner_identity_id TEXT NOT NULL REFERENCES identities(id),
  visibility TEXT NOT NULL DEFAULT 'unlisted' CHECK(visibility IN ('unlisted','public','private','friends_only')),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_active_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TEXT
);
CREATE INDEX rooms_public_idx ON rooms(visibility, last_active_at DESC) WHERE deleted_at IS NULL;
CREATE TABLE room_members (
  room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE, identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
  role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner','admin','member')),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(room_id, identity_id)
);
CREATE TABLE room_permissions (
  room_id TEXT NOT NULL, identity_id TEXT NOT NULL, permission TEXT NOT NULL, allowed INTEGER NOT NULL CHECK(allowed IN (0,1)),
  updated_by_identity_id TEXT NOT NULL REFERENCES identities(id), updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(room_id, identity_id, permission), FOREIGN KEY(room_id, identity_id) REFERENCES room_members(room_id, identity_id) ON DELETE CASCADE
);
CREATE TABLE room_invites (
  id TEXT PRIMARY KEY, room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE, created_by_identity_id TEXT NOT NULL REFERENCES identities(id),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(room_id, account_id)
);
CREATE TABLE room_bans (
  id TEXT PRIMARY KEY, room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  identity_id TEXT REFERENCES identities(id), account_id TEXT REFERENCES accounts(id), banned_by_identity_id TEXT NOT NULL REFERENCES identities(id),
  reason_code TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, revoked_at TEXT, revoked_by_identity_id TEXT REFERENCES identities(id),
  CHECK(identity_id IS NOT NULL OR account_id IS NOT NULL)
);
CREATE INDEX room_bans_active_idx ON room_bans(room_id, identity_id, account_id) WHERE revoked_at IS NULL;
CREATE TABLE media_items (
  id TEXT PRIMARY KEY, provider TEXT NOT NULL, provider_media_id TEXT NOT NULL, title TEXT, thumbnail_url TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(provider, provider_media_id)
);
CREATE TABLE room_queue_items (
  id TEXT PRIMARY KEY, room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE, media_id TEXT NOT NULL REFERENCES media_items(id),
  position INTEGER NOT NULL, added_by_identity_id TEXT NOT NULL REFERENCES identities(id), created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX room_queue_position_idx ON room_queue_items(room_id, position);
CREATE TABLE playback_states (
  room_id TEXT PRIMARY KEY REFERENCES rooms(id) ON DELETE CASCADE, current_media_id TEXT REFERENCES media_items(id),
  status TEXT NOT NULL DEFAULT 'paused' CHECK(status IN ('playing','paused','ended')),
  position_seconds REAL NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_by_identity_id TEXT REFERENCES identities(id)
);
CREATE TABLE room_events (
  id TEXT PRIMARY KEY, room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  actor_identity_id TEXT REFERENCES identities(id), event_type TEXT NOT NULL, payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX room_events_room_idx ON room_events(room_id, created_at DESC);
CREATE TABLE room_reports (
  id TEXT PRIMARY KEY, room_id TEXT NOT NULL REFERENCES rooms(id), reporter_identity_id TEXT REFERENCES identities(id),
  reason TEXT NOT NULL CHECK(reason IN ('illegal_content','sexual_content','violent_content','harassment','spam','other')),
  metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at TEXT, delisted_at TEXT
);


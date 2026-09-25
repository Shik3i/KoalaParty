ALTER TABLE rooms ADD COLUMN name TEXT CHECK(name IS NULL OR length(name) BETWEEN 1 AND 60);

ALTER TABLE room_queue_items ADD COLUMN start_seconds REAL NOT NULL DEFAULT 0 CHECK(start_seconds >= 0);

CREATE TABLE skip_votes (
  room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  media_id TEXT NOT NULL REFERENCES media_items(id),
  identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(room_id, identity_id),
  FOREIGN KEY(room_id, identity_id) REFERENCES room_members(room_id, identity_id) ON DELETE CASCADE
);

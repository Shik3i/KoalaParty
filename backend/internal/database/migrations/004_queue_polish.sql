ALTER TABLE rooms ADD COLUMN queue_loop INTEGER NOT NULL DEFAULT 0 CHECK(queue_loop IN (0,1));

CREATE TABLE queue_votes (
  room_id TEXT NOT NULL,
  queue_item_id TEXT NOT NULL REFERENCES room_queue_items(id) ON DELETE CASCADE,
  identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(room_id, queue_item_id, identity_id),
  FOREIGN KEY(room_id, identity_id) REFERENCES room_members(room_id, identity_id) ON DELETE CASCADE
);

CREATE TABLE room_history (
  id TEXT PRIMARY KEY,
  room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  media_id TEXT NOT NULL REFERENCES media_items(id),
  played_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX room_history_room_idx ON room_history(room_id, played_at DESC);

CREATE TABLE command_receipts (
  room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
  request_id TEXT NOT NULL,
  command_type TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(room_id, identity_id, request_id)
);

CREATE INDEX command_receipts_created_at_idx ON command_receipts(created_at);

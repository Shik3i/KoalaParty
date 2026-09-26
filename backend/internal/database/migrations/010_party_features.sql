ALTER TABLE rooms ADD COLUMN slug TEXT COLLATE NOCASE CHECK(slug IS NULL OR length(slug) BETWEEN 3 AND 32);
CREATE UNIQUE INDEX rooms_slug_idx ON rooms(slug) WHERE slug IS NOT NULL;

ALTER TABLE rooms ADD COLUMN mode TEXT NOT NULL DEFAULT 'party' CHECK(mode IN ('party','cinema','host'));
ALTER TABLE rooms ADD COLUMN wait_for_all INTEGER NOT NULL DEFAULT 1 CHECK(wait_for_all IN (0,1));
ALTER TABLE rooms ADD COLUMN countdown_seconds INTEGER NOT NULL DEFAULT 3 CHECK(countdown_seconds BETWEEN 0 AND 5);
ALTER TABLE rooms ADD COLUMN scheduled_at TEXT;

ALTER TABLE playback_states ADD COLUMN auto_paused INTEGER NOT NULL DEFAULT 0 CHECK(auto_paused IN (0,1));

CREATE TABLE saved_queues (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 60),
  items_json TEXT NOT NULL,
  item_count INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX saved_queues_account_idx ON saved_queues(account_id, created_at DESC);

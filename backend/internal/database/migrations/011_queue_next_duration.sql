ALTER TABLE media_items ADD COLUMN duration_seconds REAL CHECK(duration_seconds IS NULL OR duration_seconds > 0);
ALTER TABLE room_queue_items ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0 CHECK(pinned IN (0,1));

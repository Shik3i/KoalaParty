ALTER TABLE playback_states ADD COLUMN playback_rate REAL NOT NULL DEFAULT 1 CHECK(playback_rate > 0 AND playback_rate <= 4);

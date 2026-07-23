ALTER TABLE rooms ADD COLUMN sponsorblock_enabled INTEGER NOT NULL DEFAULT 1 CHECK(sponsorblock_enabled IN (0,1));

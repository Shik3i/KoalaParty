ALTER TABLE accounts ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0;

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

INSERT INTO settings(key, value) VALUES ('session_ttl', '168h');
INSERT INTO settings(key, value) VALUES ('activity_max_age', '720h');
INSERT INTO settings(key, value) VALUES ('activity_max_events', '200');
INSERT INTO settings(key, value) VALUES ('room_max_idle', '8760h');
INSERT INTO settings(key, value) VALUES ('public_rooms', '0');

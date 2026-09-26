package database

import (
	"path/filepath"
	"testing"
)

func TestMigrationFromEmptyDatabase(t *testing.T) {
	db, e := Open(filepath.Join(t.TempDir(), "empty.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var version int
	if e = db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); e != nil || version != 11 {
		t.Fatalf("migration version=%d err=%v", version, e)
	}
	var rateColumn int
	if e = db.QueryRow("SELECT count(*) FROM pragma_table_info('playback_states') WHERE name='playback_rate'").Scan(&rateColumn); e != nil || rateColumn != 1 {
		t.Fatalf("playback_rate column unavailable: count=%d err=%v", rateColumn, e)
	}
	var fk int
	_ = db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Fatal("foreign keys are disabled")
	}
	var mode string
	_ = db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Fatalf("journal mode=%s", mode)
	}
	var revisionColumn int
	if e = db.QueryRow("SELECT count(*) FROM pragma_table_info('rooms') WHERE name='revision'").Scan(&revisionColumn); e != nil || revisionColumn != 1 {
		t.Fatalf("room revision column unavailable: count=%d err=%v", revisionColumn, e)
	}
	var queueLoopColumn, queueTables int
	if e = db.QueryRow("SELECT count(*) FROM pragma_table_info('rooms') WHERE name='queue_loop'").Scan(&queueLoopColumn); e != nil || queueLoopColumn != 1 {
		t.Fatalf("queue_loop migration missing: count=%d err=%v", queueLoopColumn, e)
	}
	if e = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('queue_votes','room_history')").Scan(&queueTables); e != nil || queueTables != 2 {
		t.Fatalf("queue tables missing: count=%d err=%v", queueTables, e)
	}
	var receipts int
	if e = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='command_receipts'").Scan(&receipts); e != nil || receipts != 1 {
		t.Fatalf("command receipt table missing: count=%d err=%v", receipts, e)
	}
	var launchColumns, skipVotes int
	if e = db.QueryRow("SELECT (SELECT count(*) FROM pragma_table_info('rooms') WHERE name='name')+(SELECT count(*) FROM pragma_table_info('room_queue_items') WHERE name='start_seconds')").Scan(&launchColumns); e != nil || launchColumns != 2 {
		t.Fatalf("launch polish columns missing: count=%d err=%v", launchColumns, e)
	}
	if e = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='skip_votes'").Scan(&skipVotes); e != nil || skipVotes != 1 {
		t.Fatalf("skip vote table missing: count=%d err=%v", skipVotes, e)
	}
	var partyColumns int
	if e = db.QueryRow("SELECT (SELECT count(*) FROM pragma_table_info('rooms') WHERE name IN ('slug','mode','wait_for_all','countdown_seconds','scheduled_at'))+(SELECT count(*) FROM pragma_table_info('playback_states') WHERE name='auto_paused')+(SELECT count(*) FROM sqlite_master WHERE type='table' AND name='saved_queues')").Scan(&partyColumns); e != nil || partyColumns != 7 {
		t.Fatalf("party feature schema missing: count=%d err=%v", partyColumns, e)
	}
}

func TestReportLimitMigrationResolvesLegacyDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
		DROP INDEX room_reports_pending_reporter_idx;
		DROP TABLE command_receipts;
		DROP TABLE skip_votes;
		ALTER TABLE rooms DROP COLUMN name;
		ALTER TABLE room_queue_items DROP COLUMN start_seconds;
		DROP INDEX rooms_slug_idx;
		DROP TABLE saved_queues;
		ALTER TABLE rooms DROP COLUMN slug;
		ALTER TABLE rooms DROP COLUMN mode;
		ALTER TABLE rooms DROP COLUMN wait_for_all;
		ALTER TABLE rooms DROP COLUMN countdown_seconds;
		ALTER TABLE rooms DROP COLUMN scheduled_at;
		ALTER TABLE playback_states DROP COLUMN auto_paused;
		ALTER TABLE media_items DROP COLUMN duration_seconds;
		ALTER TABLE room_queue_items DROP COLUMN pinned;
		DELETE FROM schema_migrations WHERE version IN (7, 8, 9, 10, 11);
		INSERT INTO identities(id,secret_hash,display_name,avatar_seed) VALUES('owner','hash','Owner','owner');
		INSERT INTO rooms(id,owner_identity_id) VALUES('AAAAAAAAAAAAAAAA','owner');
		INSERT INTO room_reports(id,room_id,reporter_identity_id,reason) VALUES
			('old-report','AAAAAAAAAAAAAAAA','owner','spam'),
			('new-report','AAAAAAAAAAAAAAAA','owner','spam');`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatalf("upgrade with legacy duplicate reports failed: %v", err)
	}
	defer db.Close()
	var pending, resolved int
	if err = db.QueryRow("SELECT count(*) FROM room_reports WHERE resolved_at IS NULL").Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT count(*) FROM room_reports WHERE resolved_at IS NOT NULL").Scan(&resolved); err != nil {
		t.Fatal(err)
	}
	if pending != 1 || resolved != 1 {
		t.Fatalf("legacy duplicate cleanup pending=%d resolved=%d", pending, resolved)
	}
	if _, err = db.Exec("INSERT INTO room_reports(id,room_id,reporter_identity_id,reason) VALUES('third-report','AAAAAAAAAAAAAAAA','owner','spam')"); err == nil {
		t.Fatal("pending-report uniqueness index was not created")
	}
}

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
	if e = db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); e != nil || version != 5 {
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
}

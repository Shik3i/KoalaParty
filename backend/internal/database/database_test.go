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
	if e = db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); e != nil || version != 1 {
		t.Fatalf("migration version=%d err=%v", version, e)
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
}

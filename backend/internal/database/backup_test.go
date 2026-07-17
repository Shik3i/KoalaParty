package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO accounts(id,username,password_hash) VALUES('A','backup-user','hash')"); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backups", "snapshot.db")
	if err = Backup(db, backup); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(dir, "restored.db")
	if err = Restore(backup, restored); err != nil {
		t.Fatal(err)
	}
	restoredDB, err := Open(restored)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	var username string
	if err = restoredDB.QueryRow("SELECT username FROM accounts WHERE id='A'").Scan(&username); err != nil || username != "backup-user" {
		t.Fatalf("restored username = %q, err = %v", username, err)
	}
	if err = Backup(restoredDB, backup); err == nil {
		t.Fatal("existing backup was overwritten")
	}
	if _, err = os.Stat(backup); err != nil {
		t.Fatal(err)
	}
}

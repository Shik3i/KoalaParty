package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

func Backup(db *sql.DB, destination string) error {
	if destination == "" {
		return fmt.Errorf("backup destination is required")
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if _, err = os.Stat(abs); err == nil {
		return fmt.Errorf("backup destination already exists: %s", abs)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return err
	}
	if _, err = db.Exec("VACUUM INTO ?", abs); err != nil {
		return fmt.Errorf("create SQLite backup: %w", err)
	}
	return verify(abs)
}

func Restore(source, destination string) error {
	if source == "" || destination == "" {
		return fmt.Errorf("backup source and restore destination are required")
	}
	if err := verify(source); err != nil {
		return fmt.Errorf("verify backup: %w", err)
	}
	sourceDB, err := Open(source)
	if err != nil {
		return err
	}
	defer sourceDB.Close()
	return Backup(sourceDB, destination)
}

func verify(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite integrity check returned %q", result)
	}
	return nil
}

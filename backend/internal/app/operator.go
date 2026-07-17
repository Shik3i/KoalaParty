package app

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Shik3i/KoalaParty/backend/internal/database"
)

func Operator(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return operatorUsage()
	}
	dbPath := env("KOALAPARTY_DB", "koalaparty.db")
	switch args[0] {
	case "backup":
		if len(args) != 2 {
			return fmt.Errorf("usage: koalaparty operator backup <destination.db>")
		}
		db, err := database.Open(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		if err = database.Backup(db, args[1]); err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "backup created: %s\n", args[1])
		return err
	case "restore":
		if len(args) != 3 {
			return fmt.Errorf("usage: koalaparty operator restore <backup.db> <destination.db>")
		}
		if err := database.Restore(args[1], args[2]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(stdout, "restore created: %s\n", args[2])
		return err
	}
	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	switch args[0] {
	case "reports":
		return operateReports(db, args[1:], stdout)
	case "delete-room":
		if len(args) != 2 || !roomIDPattern.MatchString(args[1]) {
			return fmt.Errorf("usage: koalaparty operator delete-room <room-id>")
		}
		result, execErr := db.Exec("UPDATE rooms SET visibility='unlisted',deleted_at=coalesce(deleted_at,CURRENT_TIMESTAMP),updated_at=CURRENT_TIMESTAMP WHERE id=?", args[1])
		return requireChanged(result, execErr, "room")
	case "delete-account":
		if len(args) != 2 {
			return fmt.Errorf("usage: koalaparty operator delete-account <username>")
		}
		return deleteAccount(db, args[1])
	default:
		return operatorUsage()
	}
}

func operatorUsage() error {
	return fmt.Errorf("usage: koalaparty operator <backup|restore|reports|delete-room|delete-account> ...")
}

func operateReports(db *sql.DB, args []string, stdout io.Writer) error {
	if len(args) == 1 && args[0] == "list" {
		rows, err := db.Query("SELECT id,room_id,reason,metadata_json,created_at FROM room_reports WHERE resolved_at IS NULL ORDER BY created_at")
		if err != nil {
			return err
		}
		defer rows.Close()
		encoder := json.NewEncoder(stdout)
		for rows.Next() {
			var id, room, reason, metadata, created string
			if err = rows.Scan(&id, &room, &reason, &metadata, &created); err != nil {
				return err
			}
			var snapshot any
			if err = json.Unmarshal([]byte(metadata), &snapshot); err != nil {
				return err
			}
			if err = encoder.Encode(map[string]any{"id": id, "roomId": room, "reason": reason, "metadata": snapshot, "createdAt": created}); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if len(args) == 2 && (args[0] == "resolve" || args[0] == "delist") {
		query := "UPDATE room_reports SET resolved_at=CURRENT_TIMESTAMP WHERE id=? AND resolved_at IS NULL"
		if args[0] == "delist" {
			query = "UPDATE room_reports SET resolved_at=CURRENT_TIMESTAMP,delisted_at=CURRENT_TIMESTAMP WHERE id=? AND resolved_at IS NULL"
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var room string
		if err = tx.QueryRow("SELECT room_id FROM room_reports WHERE id=?", args[1]).Scan(&room); err != nil {
			return fmt.Errorf("report not found: %s", args[1])
		}
		result, err := tx.Exec(query, args[1])
		if err = requireChanged(result, err, "unresolved report"); err != nil {
			return err
		}
		if args[0] == "delist" {
			if _, err = tx.Exec("UPDATE rooms SET visibility='unlisted',updated_at=CURRENT_TIMESTAMP WHERE id=?", room); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	return fmt.Errorf("usage: koalaparty operator reports <list|resolve <id>|delist <id>>")
}

func deleteAccount(db *sql.DB, username string) error {
	var accountID string
	if err := db.QueryRow("SELECT id FROM accounts WHERE username=?", strings.TrimSpace(username)).Scan(&accountID); err != nil {
		return fmt.Errorf("account not found: %s", username)
	}
	return deleteAccountByID(db, accountID)
}

func deleteAccountByID(db *sql.DB, accountID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM sessions WHERE identity_id IN (SELECT id FROM identities WHERE account_id=?)", accountID); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM friendships WHERE requester_account_id=? OR addressee_account_id=?", accountID, accountID); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE rooms SET visibility='unlisted',deleted_at=coalesce(deleted_at,CURRENT_TIMESTAMP),updated_at=CURRENT_TIMESTAMP WHERE owner_identity_id IN (SELECT id FROM identities WHERE account_id=?)", accountID); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM room_bans WHERE account_id=? AND identity_id IS NULL", accountID); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE room_bans SET account_id=NULL WHERE account_id=?", accountID); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE identities SET account_id=NULL,secret_hash=?,display_name='Deleted user',avatar_seed='deleted' WHERE account_id=?", "deleted:"+hex.EncodeToString(random), accountID); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM accounts WHERE id=?", accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func requireChanged(result sql.Result, err error, label string) error {
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%s not found or already handled", label)
	}
	return nil
}

package app

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shik3i/KoalaParty/backend/internal/database"
)

func TestOperatorReportLifecycleAndRoomDeletion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "operator.db")
	t.Setenv("KOALAPARTY_DB", dbPath)
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO identities(id,secret_hash,display_name,avatar_seed) VALUES('owner','hash','Owner','seed'),('reporter','hash','Reporter','seed');
		INSERT INTO rooms(id,owner_identity_id,visibility) VALUES('AAAAAAAAAAAAAAAA','owner','public');
		INSERT INTO room_reports(id,room_id,reporter_identity_id,reason) VALUES('REPORT1','AAAAAAAAAAAAAAAA','reporter','spam');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = Operator([]string{"reports", "list"}, &out); err != nil || !strings.Contains(out.String(), `"id":"REPORT1"`) {
		t.Fatalf("list reports: %v, %q", err, out.String())
	}
	if err = Operator([]string{"reports", "delist", "REPORT1"}, &out); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var visibility string
	var resolved sql.NullString
	if err = db.QueryRow("SELECT r.visibility,p.resolved_at FROM rooms r JOIN room_reports p ON p.room_id=r.id WHERE p.id='REPORT1'").Scan(&visibility, &resolved); err != nil || visibility != "unlisted" || !resolved.Valid {
		t.Fatalf("delist state: visibility=%q resolved=%v err=%v", visibility, resolved.Valid, err)
	}
}

func TestOperatorPrivacyDeletionRevokesAccount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "privacy.db")
	t.Setenv("KOALAPARTY_DB", dbPath)
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO accounts(id,username,password_hash) VALUES('account','forest_friend','hash');
		INSERT INTO identities(id,account_id,secret_hash,display_name,avatar_seed) VALUES('identity','account','secret','Forest Friend','seed');
		INSERT INTO sessions(token_hash,identity_id,csrf_token,expires_at) VALUES('token','identity','csrf',datetime('now','+1 day'));`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if err = Operator([]string{"delete-account", "forest_friend"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var accounts, sessions int
	var account sql.NullString
	var secret, name string
	_ = db.QueryRow("SELECT count(*) FROM accounts").Scan(&accounts)
	_ = db.QueryRow("SELECT count(*) FROM sessions").Scan(&sessions)
	if err = db.QueryRow("SELECT account_id,secret_hash,display_name FROM identities WHERE id='identity'").Scan(&account, &secret, &name); err != nil {
		t.Fatal(err)
	}
	if accounts != 0 || sessions != 0 || account.Valid || !strings.HasPrefix(secret, "deleted:") || name != "Deleted user" {
		t.Fatalf("privacy deletion incomplete: accounts=%d sessions=%d account=%v secret=%q name=%q", accounts, sessions, account.Valid, secret, name)
	}
}

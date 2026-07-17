package app

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,24}$`)

func hashPassword(password string) (string, error) {
	if len(password) < 10 || len(password) > 128 {
		return "", errors.New("password must be 10 to 128 characters")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	h := argon2.IDKey([]byte(password), salt, 2, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=2,p=4$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}
func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, e := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); e != nil {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[4])
	if e != nil {
		return false
	}
	expected, e := base64.RawStdEncoding.DecodeString(parts[5])
	if e != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *application) register(w http.ResponseWriter, r *http.Request, p principal) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if !usernamePattern.MatchString(in.Username) {
		problem(w, 400, "invalid_username", "Username must be 3-24 letters, numbers, or underscores.")
		return
	}
	hash, e := hashPassword(in.Password)
	if e != nil {
		problem(w, 400, "invalid_password", e.Error())
		return
	}
	accountID := newID(10)
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 500, "database_error", "Account could not be created.")
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("INSERT INTO accounts(id,username,password_hash) VALUES(?,?,?)", accountID, in.Username, hash)
	if e == nil {
		var result sql.Result
		result, e = tx.Exec("UPDATE identities SET account_id=? WHERE id=? AND account_id IS NULL", accountID, p.IdentityID)
		if e == nil {
			if changed, resultErr := result.RowsAffected(); resultErr != nil || changed != 1 {
				e = errors.New("identity is already linked")
			}
		}
	}
	if e != nil {
		problem(w, 409, "username_unavailable", "Username is unavailable or identity is already linked.")
		return
	}
	if e = tx.Commit(); e != nil {
		problem(w, 500, "database_error", "Account could not be created.")
		return
	}
	out, _ := a.principalByIdentity(p.IdentityID)
	out.CSRF = p.CSRF
	writeJSON(w, 201, out)
}
func (a *application) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	var accountID, hash, identityID string
	e := a.db.QueryRow(`SELECT a.id,a.password_hash,i.id FROM accounts a JOIN identities i ON i.account_id=a.id WHERE a.username=? ORDER BY i.created_at LIMIT 1`, strings.TrimSpace(in.Username)).Scan(&accountID, &hash, &identityID)
	if e != nil || !verifyPassword(hash, in.Password) {
		problem(w, 401, "invalid_credentials", "Username or password is invalid.")
		return
	}
	a.issueSession(w, r, identityID)
}
func (a *application) logout(w http.ResponseWriter, r *http.Request, p principal) {
	c, _ := r.Cookie("kp_session")
	if c != nil {
		_, _ = a.db.Exec("DELETE FROM sessions WHERE token_hash=?", tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "kp_session", Value: "", Path: "/", HttpOnly: true, Secure: a.cookieSecure, MaxAge: -1, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(204)
}

type friendRequest struct {
	Username string `json:"username"`
}

func (a *application) friends(w http.ResponseWriter, r *http.Request, p principal) {
	if p.AccountID == "" {
		problem(w, 403, "account_required", "Friends require an account.")
		return
	}
	if r.Method == "GET" {
		rows, e := a.db.Query(`SELECT a.username,f.status,CASE WHEN f.requester_account_id=? THEN 'outgoing' ELSE 'incoming' END FROM friendships f JOIN accounts a ON a.id=CASE WHEN f.requester_account_id=? THEN f.addressee_account_id ELSE f.requester_account_id END WHERE f.requester_account_id=? OR f.addressee_account_id=? ORDER BY f.updated_at DESC`, p.AccountID, p.AccountID, p.AccountID, p.AccountID)
		if e != nil {
			problem(w, 500, "database_error", "Could not list friends.")
			return
		}
		defer rows.Close()
		out := []map[string]string{}
		for rows.Next() {
			var u, s, d string
			if e = rows.Scan(&u, &s, &d); e != nil {
				problem(w, 500, "database_error", "Could not list friends.")
				return
			}
			out = append(out, map[string]string{"username": u, "status": s, "direction": d})
		}
		if e = rows.Err(); e != nil {
			problem(w, 500, "database_error", "Could not list friends.")
			return
		}
		writeJSON(w, 200, out)
		return
	}
	var in friendRequest
	if !decode(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	var target string
	e := a.db.QueryRow("SELECT id FROM accounts WHERE username=?", in.Username).Scan(&target)
	if e != nil || target == p.AccountID {
		problem(w, 404, "account_not_found", "Account was not found.")
		return
	}
	var existingRequester, existingStatus string
	e = a.db.QueryRow(`SELECT requester_account_id,status FROM friendships WHERE (requester_account_id=? AND addressee_account_id=?) OR (requester_account_id=? AND addressee_account_id=?) LIMIT 1`, p.AccountID, target, target, p.AccountID).Scan(&existingRequester, &existingStatus)
	if e == nil {
		if existingRequester == target && existingStatus == "pending" {
			_, e = a.db.Exec("UPDATE friendships SET status='accepted',updated_at=CURRENT_TIMESTAMP WHERE requester_account_id=? AND addressee_account_id=?", target, p.AccountID)
			if e == nil {
				w.WriteHeader(204)
				return
			}
		} else {
			problem(w, 409, "relationship_exists", "A friendship relationship already exists.")
			return
		}
	} else if !errors.Is(e, sql.ErrNoRows) {
		problem(w, 500, "database_error", "Friend request could not be sent.")
		return
	}
	_, e = a.db.Exec("INSERT INTO friendships(requester_account_id,addressee_account_id,status) VALUES(?,?,'pending')", p.AccountID, target)
	if e != nil {
		problem(w, 409, "friend_request_failed", "Friend request could not be sent.")
		return
	}
	w.WriteHeader(204)
}
func (a *application) friendAction(w http.ResponseWriter, r *http.Request, p principal) {
	if p.AccountID == "" {
		problem(w, 403, "account_required", "Friends require an account.")
		return
	}
	username := r.PathValue("username")
	action := r.PathValue("action")
	var target string
	if a.db.QueryRow("SELECT id FROM accounts WHERE username=?", username).Scan(&target) != nil {
		problem(w, 404, "account_not_found", "Account was not found.")
		return
	}
	var e error
	var result sql.Result
	switch action {
	case "accept", "decline":
		status := map[string]string{"accept": "accepted", "decline": "declined"}[action]
		result, e = a.db.Exec("UPDATE friendships SET status=?,updated_at=CURRENT_TIMESTAMP WHERE requester_account_id=? AND addressee_account_id=? AND status='pending'", status, target, p.AccountID)
	case "remove":
		result, e = a.db.Exec("DELETE FROM friendships WHERE (requester_account_id=? AND addressee_account_id=?) OR (requester_account_id=? AND addressee_account_id=?)", p.AccountID, target, target, p.AccountID)
	case "block":
		tx, txErr := a.db.BeginTx(r.Context(), nil)
		if txErr == nil {
			_, txErr = tx.Exec("DELETE FROM friendships WHERE (requester_account_id=? AND addressee_account_id=?) OR (requester_account_id=? AND addressee_account_id=?)", p.AccountID, target, target, p.AccountID)
		}
		if txErr == nil {
			_, txErr = tx.Exec("INSERT INTO friendships(requester_account_id,addressee_account_id,status) VALUES(?,?,'blocked')", p.AccountID, target)
		}
		if txErr == nil {
			txErr = tx.Commit()
		} else if tx != nil {
			_ = tx.Rollback()
		}
		e = txErr
	default:
		problem(w, 404, "unknown_action", "Unknown friendship action.")
		return
	}
	if e != nil {
		problem(w, 500, "database_error", "Friendship update failed.")
		return
	}
	if action != "block" {
		if changed, resultErr := result.RowsAffected(); resultErr != nil || changed == 0 {
			problem(w, 409, "friendship_not_found", "No matching friendship action was available.")
			return
		}
	}
	w.WriteHeader(204)
}
func (a *application) discover(w http.ResponseWriter, r *http.Request) {
	if !a.publicRooms {
		problem(w, 404, "public_rooms_disabled", "Public room discovery is disabled during the early beta.")
		return
	}
	rows, e := a.db.Query(`SELECT r.id,coalesce(m.title,''),coalesce(m.thumbnail_url,''),p.status FROM rooms r JOIN playback_states p ON p.room_id=r.id LEFT JOIN media_items m ON m.id=p.current_media_id WHERE r.visibility='public' AND r.deleted_at IS NULL ORDER BY r.last_active_at DESC LIMIT 50`)
	if e != nil {
		problem(w, 500, "database_error", "Discovery failed.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, thumb, status string
		if e = rows.Scan(&id, &title, &thumb, &status); e != nil {
			problem(w, 500, "database_error", "Discovery failed.")
			return
		}
		out = append(out, map[string]any{"id": id, "label": roomLabel(id), "title": title, "thumbnail": thumb, "status": status, "participants": a.hub.activeCount(id)})
	}
	if e = rows.Err(); e != nil {
		problem(w, 500, "database_error", "Discovery failed.")
		return
	}
	writeJSON(w, 200, out)
}
func (a *application) report(w http.ResponseWriter, r *http.Request, p principal) {
	if !a.publicRooms {
		problem(w, 404, "public_rooms_disabled", "Public rooms are disabled.")
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !contains([]string{"illegal_content", "sexual_content", "violent_content", "harassment", "spam", "other"}, in.Reason) {
		problem(w, 400, "invalid_reason", "Invalid report reason.")
		return
	}
	id := r.PathValue("roomId")
	var metadata string
	e := a.db.QueryRow(`SELECT json_object('title',coalesce(m.title,''),'thumbnail',coalesce(m.thumbnail_url,'')) FROM rooms r JOIN playback_states p ON p.room_id=r.id LEFT JOIN media_items m ON m.id=p.current_media_id WHERE r.id=? AND r.visibility='public'`, id).Scan(&metadata)
	if errors.Is(e, sql.ErrNoRows) {
		problem(w, 404, "room_not_found", "Public room was not found.")
		return
	}
	if e != nil {
		problem(w, 500, "database_error", "Report failed.")
		return
	}
	_, e = a.db.Exec("INSERT INTO room_reports(id,room_id,reporter_identity_id,reason,metadata_json) VALUES(?,?,?,?,?)", newID(10), id, p.IdentityID, in.Reason, metadata)
	if e != nil {
		problem(w, 500, "database_error", "Report failed.")
		return
	}
	w.WriteHeader(204)
}

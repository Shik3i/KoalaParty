package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

func requireAccount(w http.ResponseWriter, p principal) bool {
	if p.AccountID == "" {
		problem(w, 403, "account_required", "This action requires an account.")
		return false
	}
	return true
}

func (a *application) myRooms(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT r.id,r.visibility,
			CASE WHEN owner.account_id=? THEN 'owner' ELSE coalesce((
				SELECT rm.role FROM room_members rm JOIN identities member_identity ON member_identity.id=rm.identity_id
				WHERE rm.room_id=r.id AND member_identity.account_id=?
				ORDER BY CASE rm.role WHEN 'admin' THEN 0 ELSE 1 END LIMIT 1
			),'member') END,
		r.last_active_at,coalesce(media.title,''),playback.status
		FROM rooms r
		JOIN identities owner ON owner.id=r.owner_identity_id
		JOIN playback_states playback ON playback.room_id=r.id
		LEFT JOIN media_items media ON media.id=playback.current_media_id
		WHERE r.deleted_at IS NULL AND (owner.account_id=? OR EXISTS(
			SELECT 1 FROM room_members rm JOIN identities member_identity ON member_identity.id=rm.identity_id
			WHERE rm.room_id=r.id AND member_identity.account_id=?
		))
		ORDER BY r.last_active_at DESC`, p.AccountID, p.AccountID, p.AccountID, p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not list rooms.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, visibility, role, lastActive, title, status string
		if err = rows.Scan(&id, &visibility, &role, &lastActive, &title, &status); err != nil {
			problem(w, 500, "database_error", "Could not list rooms.")
			return
		}
		out = append(out, map[string]any{"id": id, "label": roomLabel(id), "visibility": visibility, "role": role, "lastActiveAt": lastActive, "title": title, "status": status, "participants": a.hub.activeCount(id)})
	}
	if err = rows.Err(); err != nil {
		problem(w, 500, "database_error", "Could not list rooms.")
		return
	}
	writeJSON(w, 200, out)
}

func (a *application) deleteRoom(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("roomId")
	result, err := a.db.ExecContext(r.Context(), `UPDATE rooms SET visibility='unlisted',deleted_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND deleted_at IS NULL AND owner_identity_id IN (SELECT id FROM identities WHERE id=? OR (account_id IS NOT NULL AND account_id=?))`, id, p.IdentityID, p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not delete room.")
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		problem(w, 403, "owner_required", "Only the room owner can delete this room.")
		return
	}
	a.hub.disconnectRoom(id)
	w.WriteHeader(204)
}

func (a *application) leaveRoom(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("roomId")
	var role string
	err := a.db.QueryRowContext(r.Context(), "SELECT role FROM room_members WHERE room_id=? AND identity_id=?", id, p.IdentityID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "membership_not_found", "You are not a member of this room.")
		return
	}
	if err != nil {
		problem(w, 500, "database_error", "Could not leave room.")
		return
	}
	if role == "owner" {
		problem(w, 409, "ownership_transfer_required", "Transfer ownership or delete the room before leaving.")
		return
	}
	if _, err = a.db.ExecContext(r.Context(), "DELETE FROM room_members WHERE room_id=? AND identity_id=?", id, p.IdentityID); err != nil {
		problem(w, 500, "database_error", "Could not leave room.")
		return
	}
	a.hub.disconnect(id, p.IdentityID)
	w.WriteHeader(204)
}

func (a *application) roomInvites(w http.ResponseWriter, r *http.Request, p principal) {
	room := r.PathValue("roomId")
	role, _ := a.roleAndAllowed(room, p.IdentityID, "room.manage_invites")
	if role != "owner" && role != "admin" {
		problem(w, 403, "permission_denied", "Only room managers can manage invitations.")
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.QueryContext(r.Context(), `SELECT a.username,i.created_at FROM room_invites i JOIN accounts a ON a.id=i.account_id WHERE i.room_id=? ORDER BY i.created_at`, room)
		if err != nil {
			problem(w, 500, "database_error", "Could not list invitations.")
			return
		}
		defer rows.Close()
		out := []map[string]string{}
		for rows.Next() {
			var username, created string
			if err = rows.Scan(&username, &created); err != nil {
				problem(w, 500, "database_error", "Could not list invitations.")
				return
			}
			out = append(out, map[string]string{"username": username, "createdAt": created})
		}
		if err = rows.Err(); err != nil {
			problem(w, 500, "database_error", "Could not list invitations.")
			return
		}
		writeJSON(w, 200, out)
		return
	}
	var in struct {
		Username string `json:"username"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	var accountID string
	if a.db.QueryRowContext(r.Context(), "SELECT id FROM accounts WHERE username=?", in.Username).Scan(&accountID) != nil {
		problem(w, 404, "account_not_found", "Account was not found.")
		return
	}
	_, err := a.db.ExecContext(r.Context(), `INSERT INTO room_invites(id,room_id,account_id,created_by_identity_id) VALUES(?,?,?,?) ON CONFLICT(room_id,account_id) DO NOTHING`, newID(10), room, accountID, p.IdentityID)
	if err != nil {
		problem(w, 500, "database_error", "Could not create invitation.")
		return
	}
	w.WriteHeader(204)
}

func (a *application) revokeInvite(w http.ResponseWriter, r *http.Request, p principal) {
	room := r.PathValue("roomId")
	role, _ := a.roleAndAllowed(room, p.IdentityID, "room.manage_invites")
	if role != "owner" && role != "admin" {
		problem(w, 403, "permission_denied", "Only room managers can manage invitations.")
		return
	}
	result, err := a.db.ExecContext(r.Context(), "DELETE FROM room_invites WHERE room_id=? AND account_id=(SELECT id FROM accounts WHERE username=?)", room, r.PathValue("username"))
	if err != nil {
		problem(w, 500, "database_error", "Could not revoke invitation.")
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		problem(w, 404, "invite_not_found", "Invitation was not found.")
		return
	}
	w.WriteHeader(204)
}

func (a *application) accountProfile(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	var in struct {
		DisplayName string `json:"displayName"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	nameLength := utf8.RuneCountInString(in.DisplayName)
	if nameLength < 1 || nameLength > 32 {
		problem(w, 400, "invalid_display_name", "Display name must be 1 to 32 characters.")
		return
	}
	if _, err := a.db.ExecContext(r.Context(), "UPDATE identities SET display_name=? WHERE id=?", in.DisplayName, p.IdentityID); err != nil {
		problem(w, 500, "database_error", "Could not update profile.")
		return
	}
	p.DisplayName = in.DisplayName
	writeJSON(w, 200, p)
}

func (a *application) accountPassword(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decode(w, r, &in) {
		return
	}
	var currentHash string
	if err := a.db.QueryRowContext(r.Context(), "SELECT password_hash FROM accounts WHERE id=?", p.AccountID).Scan(&currentHash); err != nil || !verifyPassword(currentHash, in.CurrentPassword) {
		problem(w, 401, "invalid_password", "Current password is incorrect.")
		return
	}
	newHash, err := hashPassword(in.NewPassword)
	if err != nil {
		problem(w, 400, "invalid_password", err.Error())
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE accounts SET password_hash=? WHERE id=?", newHash, p.AccountID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash<>? AND identity_id IN (SELECT id FROM identities WHERE account_id=?)`, currentSessionHash(r), p.AccountID)
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	if err != nil {
		problem(w, 500, "database_error", "Could not change password.")
		return
	}
	w.WriteHeader(204)
}

func (a *application) accountSessions(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	currentHash := currentSessionHash(r)
	if r.Method == http.MethodGet {
		rows, err := a.db.QueryContext(r.Context(), `SELECT s.token_hash,s.created_at,s.expires_at FROM sessions s JOIN identities i ON i.id=s.identity_id WHERE i.account_id=? AND s.expires_at>CURRENT_TIMESTAMP ORDER BY s.created_at DESC`, p.AccountID)
		if err != nil {
			problem(w, 500, "database_error", "Could not list sessions.")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, created, expires string
			if err = rows.Scan(&id, &created, &expires); err != nil {
				problem(w, 500, "database_error", "Could not list sessions.")
				return
			}
			out = append(out, map[string]any{"id": id, "createdAt": created, "expiresAt": expires, "current": id == currentHash})
		}
		if err = rows.Err(); err != nil {
			problem(w, 500, "database_error", "Could not list sessions.")
			return
		}
		writeJSON(w, 200, out)
		return
	}
	_, err := a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash<>? AND identity_id IN (SELECT id FROM identities WHERE account_id=?)`, currentHash, p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not revoke sessions.")
		return
	}
	w.WriteHeader(204)
}

func (a *application) revokeSession(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	id := r.PathValue("sessionId")
	if id == currentSessionHash(r) {
		problem(w, 409, "current_session", "Use log out to end the current session.")
		return
	}
	result, err := a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash=? AND identity_id IN (SELECT id FROM identities WHERE account_id=?)`, id, p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not revoke session.")
		return
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		problem(w, 404, "session_not_found", "Session was not found.")
		return
	}
	w.WriteHeader(204)
}

func currentSessionHash(r *http.Request) string {
	cookie, err := r.Cookie("kp_session")
	if err != nil {
		return ""
	}
	return tokenHash(cookie.Value)
}

func (a *application) deleteOwnAccount(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	var hash string
	if err := a.db.QueryRowContext(r.Context(), "SELECT password_hash FROM accounts WHERE id=?", p.AccountID).Scan(&hash); err != nil || !verifyPassword(hash, in.Password) {
		problem(w, 401, "invalid_password", "Password is incorrect.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id FROM identities WHERE account_id=?", p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not delete account.")
		return
	}
	var identities []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			problem(w, 500, "database_error", "Could not delete account.")
			return
		}
		identities = append(identities, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		problem(w, 500, "database_error", "Could not delete account.")
		return
	}
	rows.Close()
	roomRows, err := a.db.QueryContext(r.Context(), "SELECT r.id FROM rooms r JOIN identities i ON i.id=r.owner_identity_id WHERE i.account_id=? AND r.deleted_at IS NULL", p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not delete account.")
		return
	}
	var rooms []string
	for roomRows.Next() {
		var id string
		if err = roomRows.Scan(&id); err != nil {
			roomRows.Close()
			problem(w, 500, "database_error", "Could not delete account.")
			return
		}
		rooms = append(rooms, id)
	}
	if err = roomRows.Err(); err != nil {
		roomRows.Close()
		problem(w, 500, "database_error", "Could not delete account.")
		return
	}
	roomRows.Close()
	if err = deleteAccountByID(a.db, p.AccountID); err != nil {
		problem(w, 500, "database_error", "Could not delete account.")
		return
	}
	for _, identity := range identities {
		a.hub.disconnectIdentity(identity)
	}
	for _, room := range rooms {
		a.hub.disconnectRoom(room)
	}
	http.SetCookie(w, &http.Cookie{Name: "kp_session", Value: "", Path: "/", HttpOnly: true, Secure: a.cookieSecure, MaxAge: -1, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(204)
}

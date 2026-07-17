package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type command struct {
	Type             string          `json:"type"`
	RequestID        string          `json:"requestId"`
	ExpectedRevision int64           `json:"expectedRevision"`
	Payload          json.RawMessage `json:"payload"`
}

func (a *application) roomCommand(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("roomId")
	var c command
	if !decode(w, r, &c) {
		return
	}
	s, e := a.applyCommand(r.Context(), id, p, c)
	if e != nil {
		if errors.Is(e, errDenied) {
			problem(w, 403, "permission_denied", "The server denied this room action.")
		} else if errors.Is(e, errStale) {
			problem(w, 409, "stale_revision", "Room state changed; use the latest snapshot.")
		} else {
			problem(w, 400, "command_failed", e.Error())
		}
		return
	}
	a.hub.broadcast(id, s)
	writeJSON(w, 200, s)
}

var errDenied = errors.New("permission denied")
var errStale = errors.New("stale revision")

func (a *application) roleAndAllowed(room, identity, cap string) (string, bool) {
	var role string
	if a.db.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, identity).Scan(&role) != nil {
		return "", false
	}
	if role == "owner" || role == "admin" {
		return role, true
	}
	allowed := true
	var override bool
	e := a.db.QueryRow("SELECT allowed FROM room_permissions WHERE room_id=? AND identity_id=? AND permission=?", room, identity, cap).Scan(&override)
	if e == nil {
		allowed = override
	}
	return role, allowed
}
func capFor(t string) string {
	switch t {
	case "player.play", "player.pause":
		return "playback.play_pause"
	case "player.seek":
		return "playback.seek"
	case "queue.add":
		return "queue.add"
	case "queue.play_now":
		return "media.play_now"
	case "queue.remove":
		return "queue.remove"
	case "queue.reorder":
		return "queue.reorder"
	case "queue.skip":
		return "queue.skip"
	case "member.kick":
		return "members.kick"
	case "member.ban", "member.unban":
		return "members.ban"
	case "member.role":
		return "members.manage_admins"
	case "member.permission":
		return "members.manage_permissions"
	case "room.visibility":
		return "room.manage_visibility"
	}
	return ""
}
func (a *application) applyCommand(ctx context.Context, room string, p principal, c command) (snapshot, error) {
	cap := capFor(c.Type)
	role, allowed := a.roleAndAllowed(room, p.IdentityID, cap)
	if cap == "" || !allowed {
		return snapshot{}, errDenied
	}
	management := strings.HasPrefix(cap, "members.") || strings.HasPrefix(cap, "room.")
	if management && role == "member" {
		return snapshot{}, errDenied
	}
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return snapshot{}, e
	}
	defer tx.Rollback()
	var current int64
	_ = tx.QueryRow("SELECT revision FROM playback_states WHERE room_id=?", room).Scan(&current)
	if strings.HasPrefix(c.Type, "player.") && c.ExpectedRevision != current {
		return snapshot{}, errStale
	}
	eventType := c.Type
	payload := map[string]any{}
	switch c.Type {
	case "player.play", "player.pause", "player.seek":
		var in struct {
			Position float64 `json:"position"`
		}
		_ = json.Unmarshal(c.Payload, &in)
		if in.Position < 0 || in.Position > 604800 {
			return snapshot{}, errors.New("invalid playback position")
		}
		status := "paused"
		if c.Type == "player.play" {
			status = "playing"
		}
		if c.Type == "player.seek" {
			_, e = tx.Exec("UPDATE playback_states SET position_seconds=?,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", in.Position, p.IdentityID, room)
		} else {
			_, e = tx.Exec("UPDATE playback_states SET status=?,position_seconds=?,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", status, in.Position, p.IdentityID, room)
		}
		payload["position"] = in.Position
	case "queue.add", "queue.play_now":
		var in struct {
			VideoID string `json:"videoId"`
			Title   string `json:"title"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || !youtubeID.MatchString(in.VideoID) {
			return snapshot{}, errors.New("invalid YouTube video ID")
		}
		if len(in.Title) > 200 {
			in.Title = in.Title[:200]
		}
		mediaID := "YT" + in.VideoID
		_, e = tx.Exec("INSERT INTO media_items(id,provider,provider_media_id,title,thumbnail_url) VALUES(?,'youtube',?,?,?) ON CONFLICT(provider,provider_media_id) DO UPDATE SET title=excluded.title", mediaID, in.VideoID, in.Title, "https://i.ytimg.com/vi/"+in.VideoID+"/mqdefault.jpg")
		if e == nil && c.Type == "queue.add" {
			var pos int
			_ = tx.QueryRow("SELECT coalesce(max(position),-1)+1 FROM room_queue_items WHERE room_id=?", room).Scan(&pos)
			_, e = tx.Exec("INSERT INTO room_queue_items(id,room_id,media_id,position,added_by_identity_id) VALUES(?,?,?,?,?)", newID(10), room, mediaID, pos, p.IdentityID)
		} else if e == nil {
			_, e = tx.Exec("UPDATE playback_states SET current_media_id=?,status='playing',position_seconds=0,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", mediaID, p.IdentityID, room)
			eventType = "media.activated"
		}
		payload["videoId"] = in.VideoID
		payload["title"] = in.Title
	case "queue.remove":
		var in struct {
			ItemID string `json:"itemId"`
		}
		_ = json.Unmarshal(c.Payload, &in)
		_, e = tx.Exec("DELETE FROM room_queue_items WHERE room_id=? AND id=?", room, in.ItemID)
		if e == nil {
			e = resequence(tx, room)
		}
	case "queue.reorder":
		var in struct {
			ItemIDs []string `json:"itemIds"`
		}
		_ = json.Unmarshal(c.Payload, &in)
		var count int
		_ = tx.QueryRow("SELECT count(*) FROM room_queue_items WHERE room_id=?", room).Scan(&count)
		if len(in.ItemIDs) != count {
			return snapshot{}, errors.New("reorder must contain every queue item")
		}
		_, e = tx.Exec("UPDATE room_queue_items SET position=position+1000000 WHERE room_id=?", room)
		for pos, id := range in.ItemIDs {
			if e == nil {
				var res sql.Result
				res, e = tx.Exec("UPDATE room_queue_items SET position=? WHERE room_id=? AND id=?", pos, room, id)
				if n, _ := res.RowsAffected(); n != 1 {
					return snapshot{}, errors.New("unknown queue item")
				}
			}
		}
	case "queue.skip":
		var mediaID string
		e = tx.QueryRow("SELECT media_id FROM room_queue_items WHERE room_id=? ORDER BY position LIMIT 1", room).Scan(&mediaID)
		if errors.Is(e, sql.ErrNoRows) {
			mediaID = ""
			e = nil
		}
		if mediaID != "" {
			_, e = tx.Exec("DELETE FROM room_queue_items WHERE room_id=? AND position=(SELECT min(position) FROM room_queue_items WHERE room_id=?)", room, room)
			if e == nil {
				e = resequence(tx, room)
			}
		}
		if e == nil {
			_, e = tx.Exec("UPDATE playback_states SET current_media_id=?,status=?,position_seconds=0,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", nullable(mediaID), map[bool]string{true: "playing", false: "paused"}[mediaID != ""], p.IdentityID, room)
		}
	case "member.role":
		var in struct{ IdentityID, Role string }
		_ = json.Unmarshal(c.Payload, &in)
		var targetRole string
		_ = tx.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, in.IdentityID).Scan(&targetRole)
		if targetRole == "owner" || !(in.Role == "admin" || in.Role == "member") {
			return snapshot{}, errDenied
		}
		_, e = tx.Exec("UPDATE room_members SET role=? WHERE room_id=? AND identity_id=?", in.Role, room, in.IdentityID)
		eventType = map[bool]string{true: "role.admin_granted", false: "role.admin_removed"}[in.Role == "admin"]
	case "member.permission":
		var in struct {
			IdentityID, Permission string
			Allowed                bool
		}
		_ = json.Unmarshal(c.Payload, &in)
		var targetRole string
		_ = tx.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, in.IdentityID).Scan(&targetRole)
		if targetRole == "owner" || !contains(memberCapabilities, in.Permission) {
			return snapshot{}, errDenied
		}
		_, e = tx.Exec(`INSERT INTO room_permissions(room_id,identity_id,permission,allowed,updated_by_identity_id) VALUES(?,?,?,?,?) ON CONFLICT(room_id,identity_id,permission) DO UPDATE SET allowed=excluded.allowed,updated_by_identity_id=excluded.updated_by_identity_id,updated_at=CURRENT_TIMESTAMP`, room, in.IdentityID, in.Permission, in.Allowed, p.IdentityID)
		payload["permission"] = in.Permission
		payload["allowed"] = in.Allowed
	case "member.kick", "member.ban":
		var in struct {
			IdentityID string `json:"identityId"`
		}
		_ = json.Unmarshal(c.Payload, &in)
		var targetRole string
		var account sql.NullString
		_ = tx.QueryRow("SELECT m.role,i.account_id FROM room_members m JOIN identities i ON i.id=m.identity_id WHERE m.room_id=? AND i.id=?", room, in.IdentityID).Scan(&targetRole, &account)
		if targetRole == "owner" {
			return snapshot{}, errDenied
		}
		if c.Type == "member.ban" {
			_, e = tx.Exec("INSERT INTO room_bans(id,room_id,identity_id,account_id,banned_by_identity_id) VALUES(?,?,?,?,?)", newID(10), room, in.IdentityID, nullable(account.String), p.IdentityID)
		}
		if e == nil {
			_, e = tx.Exec("DELETE FROM room_members WHERE room_id=? AND identity_id=?", room, in.IdentityID)
		}
		eventType = map[bool]string{true: "member.banned", false: "member.kicked"}[c.Type == "member.ban"]
		payload["identityId"] = in.IdentityID
	case "member.unban":
		var in struct {
			IdentityID string `json:"identityId"`
		}
		_ = json.Unmarshal(c.Payload, &in)
		res, updateErr := tx.Exec("UPDATE room_bans SET revoked_at=CURRENT_TIMESTAMP,revoked_by_identity_id=? WHERE room_id=? AND identity_id=? AND revoked_at IS NULL", p.IdentityID, room, in.IdentityID)
		if updateErr != nil {
			return snapshot{}, updateErr
		}
		if changed, _ := res.RowsAffected(); changed == 0 {
			return snapshot{}, errors.New("active ban not found")
		}
		eventType = "member.unbanned"
		payload["identityId"] = in.IdentityID
	case "room.visibility":
		var in struct {
			Visibility string `json:"visibility"`
		}
		_ = json.Unmarshal(c.Payload, &in)
		if !contains([]string{"unlisted", "public", "private", "friends_only"}, in.Visibility) {
			return snapshot{}, errors.New("invalid visibility")
		}
		if in.Visibility == "public" {
			var ownerAccount sql.NullString
			_ = tx.QueryRow("SELECT i.account_id FROM rooms r JOIN identities i ON i.id=r.owner_identity_id WHERE r.id=?", room).Scan(&ownerAccount)
			if !ownerAccount.Valid {
				return snapshot{}, errors.New("public rooms require an account owner")
			}
		}
		_, e = tx.Exec("UPDATE rooms SET visibility=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", in.Visibility, room)
		payload["visibility"] = in.Visibility
	default:
		return snapshot{}, fmt.Errorf("unsupported command %s", c.Type)
	}
	if e != nil {
		return snapshot{}, e
	}
	if e = a.insertEventTx(tx, room, p.IdentityID, eventType, payload); e != nil {
		return snapshot{}, e
	}
	_, _ = tx.Exec("UPDATE rooms SET last_active_at=CURRENT_TIMESTAMP WHERE id=?", room)
	if e = tx.Commit(); e != nil {
		return snapshot{}, e
	}
	if c.Type == "member.kick" || c.Type == "member.ban" {
		if id, ok := payload["identityId"].(string); ok {
			a.hub.disconnect(room, id)
		}
	}
	return a.snapshot(ctx, room, p.IdentityID)
}
func resequence(tx *sql.Tx, room string) error {
	rows, e := tx.Query("SELECT id FROM room_queue_items WHERE room_id=? ORDER BY position", room)
	if e != nil {
		return e
	}
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	_, e = tx.Exec("UPDATE room_queue_items SET position=position+1000000 WHERE room_id=?", room)
	for i, id := range ids {
		if e == nil {
			_, e = tx.Exec("UPDATE room_queue_items SET position=? WHERE id=?", i, id)
		}
	}
	return e
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (a *application) insertEvent(room, actor, t string, p map[string]any) error {
	tx, e := a.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = a.insertEventTx(tx, room, actor, t, p); e != nil {
		return e
	}
	return tx.Commit()
}
func (a *application) insertEventTx(tx *sql.Tx, room, actor, t string, p map[string]any) error {
	b, _ := json.Marshal(p)
	_, e := tx.Exec("INSERT INTO room_events(id,room_id,actor_identity_id,event_type,payload_json) VALUES(?,?,?,?,?)", newID(10), room, nullable(actor), t, string(b))
	return e
}

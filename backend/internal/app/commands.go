package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	if a.db.QueryRow("SELECT m.role FROM room_members m JOIN rooms r ON r.id=m.room_id WHERE m.room_id=? AND m.identity_id=? AND r.deleted_at IS NULL", room, identity).Scan(&role) != nil {
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
	case "player.play", "player.pause", "player.rate":
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
	case "queue.shuffle", "queue.loop":
		return "queue.reorder"
	case "queue.vote":
		return "queue.vote"
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
	case "room.sponsorblock":
		return "room.manage_visibility"
	case "room.transfer":
		return "room.manage_ownership"
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
	// A queued video is stored immediately with a placeholder title so the add is
	// instant; the real oEmbed title is fetched afterwards by enrichTitle. This
	// keeps adding a video fast even when the server's outbound network to YouTube
	// is slow or unavailable.
	var mediaTitle, enrichVideoID string
	if c.Type == "queue.add" || c.Type == "queue.play_now" {
		var in struct {
			VideoID string `json:"videoId"`
			Title   string `json:"title"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || !youtubeID.MatchString(in.VideoID) {
			return snapshot{}, errors.New("invalid YouTube video ID")
		}
		mediaTitle = fallbackTitle("", in.VideoID)
		if a.fetchTitle != nil {
			enrichVideoID = in.VideoID
		}
	}
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return snapshot{}, e
	}
	defer tx.Rollback()
	var current int64
	if e = tx.QueryRow("SELECT revision FROM rooms WHERE id=? AND deleted_at IS NULL", room).Scan(&current); e != nil {
		return snapshot{}, errDenied
	}
	if c.ExpectedRevision != current {
		return snapshot{}, errStale
	}
	eventType := c.Type
	payload := map[string]any{}
	// The video ID that becomes the current media as a result of this command, if any.
	// Used after commit to kick off a background SponsorBlock segment fetch.
	var activatedVideoID string
	switch c.Type {
	case "player.play", "player.pause", "player.seek":
		var in struct {
			Position float64 `json:"position"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || math.IsNaN(in.Position) || math.IsInf(in.Position, 0) || in.Position < 0 || in.Position > 604800 {
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
	case "player.rate":
		// The rate carries the current position so the server can re-baseline it at the
		// moment of the change — exactly like a seek — otherwise the stored position
		// (anchored at the previous update) would be extrapolated at the new rate and
		// jump. Position is validated identically to the playback cases above.
		var in struct {
			Rate     float64 `json:"rate"`
			Position float64 `json:"position"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || math.IsNaN(in.Rate) || math.IsInf(in.Rate, 0) || in.Rate <= 0 || in.Rate > 4 {
			return snapshot{}, errors.New("invalid playback rate")
		}
		if math.IsNaN(in.Position) || math.IsInf(in.Position, 0) || in.Position < 0 || in.Position > 604800 {
			return snapshot{}, errors.New("invalid playback position")
		}
		_, e = tx.Exec("UPDATE playback_states SET playback_rate=?,position_seconds=?,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", in.Rate, in.Position, p.IdentityID, room)
		payload["rate"] = in.Rate
		payload["position"] = in.Position
	case "queue.add", "queue.play_now":
		var in struct {
			VideoID string `json:"videoId"`
			Title   string `json:"title"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || !youtubeID.MatchString(in.VideoID) {
			return snapshot{}, errors.New("invalid YouTube video ID")
		}
		mediaID := "YT" + in.VideoID
		var duplicate int
		if e = tx.QueryRow(`SELECT count(*) FROM (
			SELECT media_id FROM room_queue_items WHERE room_id=? AND media_id=?
			UNION ALL SELECT current_media_id FROM playback_states WHERE room_id=? AND current_media_id=?
		)`, room, mediaID, room, mediaID).Scan(&duplicate); e != nil {
			return snapshot{}, e
		}
		if duplicate > 0 {
			return snapshot{}, errors.New("video is already in the queue")
		}
		_, e = tx.Exec("INSERT INTO media_items(id,provider,provider_media_id,title,thumbnail_url) VALUES(?,'youtube',?,?,?) ON CONFLICT(provider,provider_media_id) DO NOTHING", mediaID, in.VideoID, mediaTitle, "https://i.ytimg.com/vi/"+in.VideoID+"/mqdefault.jpg")
		if e == nil && c.Type == "queue.add" {
			var pos int
			_ = tx.QueryRow("SELECT coalesce(max(position),-1)+1 FROM room_queue_items WHERE room_id=?", room).Scan(&pos)
			_, e = tx.Exec("INSERT INTO room_queue_items(id,room_id,media_id,position,added_by_identity_id) VALUES(?,?,?,?,?)", newID(10), room, mediaID, pos, p.IdentityID)
		} else if e == nil {
			e = addCurrentToHistory(tx, room)
		}
		if e == nil && c.Type == "queue.play_now" {
			_, e = tx.Exec("UPDATE playback_states SET current_media_id=?,status='playing',position_seconds=0,playback_rate=1,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", mediaID, p.IdentityID, room)
			eventType = "media.activated"
			activatedVideoID = in.VideoID
		}
		payload["videoId"] = in.VideoID
		payload["title"] = mediaTitle
	case "queue.remove":
		var in struct {
			ItemID string `json:"itemId"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || in.ItemID == "" {
			return snapshot{}, errors.New("invalid queue item")
		}
		var res sql.Result
		res, e = tx.Exec("DELETE FROM room_queue_items WHERE room_id=? AND id=?", room, in.ItemID)
		if e == nil {
			if changed, resultErr := res.RowsAffected(); resultErr != nil || changed != 1 {
				return snapshot{}, errors.New("unknown queue item")
			}
		}
		if e == nil {
			e = resequence(tx, room)
		}
	case "queue.reorder":
		var in struct {
			ItemIDs []string `json:"itemIds"`
		}
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid queue order")
		}
		var count int
		if e = tx.QueryRow("SELECT count(*) FROM room_queue_items WHERE room_id=?", room).Scan(&count); e != nil {
			return snapshot{}, e
		}
		if len(in.ItemIDs) != count {
			return snapshot{}, errors.New("reorder must contain every queue item")
		}
		seen := make(map[string]struct{}, len(in.ItemIDs))
		for _, id := range in.ItemIDs {
			if id == "" {
				return snapshot{}, errors.New("invalid queue item")
			}
			if _, duplicate := seen[id]; duplicate {
				return snapshot{}, errors.New("queue order contains duplicates")
			}
			seen[id] = struct{}{}
		}
		_, e = tx.Exec("UPDATE room_queue_items SET position=position+1000000 WHERE room_id=?", room)
		for pos, id := range in.ItemIDs {
			if e == nil {
				var res sql.Result
				res, e = tx.Exec("UPDATE room_queue_items SET position=? WHERE room_id=? AND id=?", pos, room, id)
				if e == nil {
					if n, resultErr := res.RowsAffected(); resultErr != nil || n != 1 {
						return snapshot{}, errors.New("unknown queue item")
					}
				}
			}
		}
	case "queue.shuffle":
		rows, queryErr := tx.Query("SELECT id FROM room_queue_items WHERE room_id=? ORDER BY random()", room)
		if queryErr != nil {
			return snapshot{}, queryErr
		}
		var shuffled []string
		for rows.Next() {
			var id string
			if queryErr = rows.Scan(&id); queryErr != nil {
				rows.Close()
				return snapshot{}, queryErr
			}
			shuffled = append(shuffled, id)
		}
		rows.Close()
		_, e = tx.Exec("UPDATE room_queue_items SET position=position+1000000 WHERE room_id=?", room)
		for position, id := range shuffled {
			if e == nil {
				_, e = tx.Exec("UPDATE room_queue_items SET position=? WHERE room_id=? AND id=?", position, room, id)
			}
		}
	case "queue.loop":
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid loop setting")
		}
		_, e = tx.Exec("UPDATE rooms SET queue_loop=? WHERE id=?", in.Enabled, room)
		payload["enabled"] = in.Enabled
	case "queue.vote":
		var in struct {
			ItemID string `json:"itemId"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || in.ItemID == "" {
			return snapshot{}, errors.New("invalid queue item")
		}
		var exists int
		if e = tx.QueryRow("SELECT count(*) FROM room_queue_items WHERE room_id=? AND id=?", room, in.ItemID).Scan(&exists); e != nil || exists == 0 {
			return snapshot{}, errors.New("unknown queue item")
		}
		var voted int
		_ = tx.QueryRow("SELECT count(*) FROM queue_votes WHERE room_id=? AND queue_item_id=? AND identity_id=?", room, in.ItemID, p.IdentityID).Scan(&voted)
		if voted > 0 {
			_, e = tx.Exec("DELETE FROM queue_votes WHERE room_id=? AND queue_item_id=? AND identity_id=?", room, in.ItemID, p.IdentityID)
		} else {
			_, e = tx.Exec("INSERT INTO queue_votes(room_id,queue_item_id,identity_id) VALUES(?,?,?)", room, in.ItemID, p.IdentityID)
		}
	case "queue.skip":
		var loop bool
		_ = tx.QueryRow("SELECT queue_loop FROM rooms WHERE id=?", room).Scan(&loop)
		if e = addCurrentToHistory(tx, room); e != nil {
			return snapshot{}, e
		}
		if loop {
			var currentMedia sql.NullString
			_ = tx.QueryRow("SELECT current_media_id FROM playback_states WHERE room_id=?", room).Scan(&currentMedia)
			if currentMedia.Valid {
				var pos int
				_ = tx.QueryRow("SELECT coalesce(max(position),-1)+1 FROM room_queue_items WHERE room_id=?", room).Scan(&pos)
				_, e = tx.Exec("INSERT INTO room_queue_items(id,room_id,media_id,position,added_by_identity_id) VALUES(?,?,?,?,?)", newID(10), room, currentMedia.String, pos, p.IdentityID)
			}
		}
		var mediaID, queueItemID string
		e = tx.QueryRow("SELECT q.id,q.media_id FROM room_queue_items q LEFT JOIN queue_votes v ON v.queue_item_id=q.id WHERE q.room_id=? GROUP BY q.id ORDER BY count(v.identity_id) DESC,q.position LIMIT 1", room).Scan(&queueItemID, &mediaID)
		if errors.Is(e, sql.ErrNoRows) {
			mediaID = ""
			e = nil
		}
		if mediaID != "" {
			_, e = tx.Exec("DELETE FROM room_queue_items WHERE room_id=? AND id=?", room, queueItemID)
			if e == nil {
				e = resequence(tx, room)
			}
		}
		if e == nil {
			_, e = tx.Exec("UPDATE playback_states SET current_media_id=?,status=?,position_seconds=0,playback_rate=1,revision=revision+1,updated_at=CURRENT_TIMESTAMP,updated_by_identity_id=? WHERE room_id=?", nullable(mediaID), map[bool]string{true: "playing", false: "paused"}[mediaID != ""], p.IdentityID, room)
		}
		if mediaID != "" {
			activatedVideoID = strings.TrimPrefix(mediaID, "YT")
		}
	case "member.role":
		var in struct{ IdentityID, Role string }
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid member role")
		}
		var targetRole string
		if e = tx.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, in.IdentityID).Scan(&targetRole); e != nil {
			return snapshot{}, errors.New("member not found")
		}
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
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid member permission")
		}
		var targetRole string
		if e = tx.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, in.IdentityID).Scan(&targetRole); e != nil {
			return snapshot{}, errors.New("member not found")
		}
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
		if json.Unmarshal(c.Payload, &in) != nil || in.IdentityID == "" {
			return snapshot{}, errors.New("invalid member")
		}
		var targetRole string
		var account sql.NullString
		if e = tx.QueryRow("SELECT m.role,i.account_id FROM room_members m JOIN identities i ON i.id=m.identity_id WHERE m.room_id=? AND i.id=?", room, in.IdentityID).Scan(&targetRole, &account); e != nil {
			return snapshot{}, errors.New("member not found")
		}
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
		if json.Unmarshal(c.Payload, &in) != nil || in.IdentityID == "" {
			return snapshot{}, errors.New("invalid member")
		}
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
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid visibility")
		}
		if !contains([]string{"unlisted", "public", "private", "friends_only"}, in.Visibility) {
			return snapshot{}, errors.New("invalid visibility")
		}
		if in.Visibility == "public" {
			if !a.getPublicRooms() {
				return snapshot{}, errors.New("public rooms are disabled")
			}
			var ownerAccount sql.NullString
			_ = tx.QueryRow("SELECT i.account_id FROM rooms r JOIN identities i ON i.id=r.owner_identity_id WHERE r.id=?", room).Scan(&ownerAccount)
			if !ownerAccount.Valid {
				return snapshot{}, errors.New("public rooms require an account owner")
			}
		}
		_, e = tx.Exec("UPDATE rooms SET visibility=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", in.Visibility, room)
		payload["visibility"] = in.Visibility
	case "room.sponsorblock":
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid sponsorblock setting")
		}
		_, e = tx.Exec("UPDATE rooms SET sponsorblock_enabled=? WHERE id=?", in.Enabled, room)
		payload["enabled"] = in.Enabled
	case "room.transfer":
		if role != "owner" {
			return snapshot{}, errDenied
		}
		var in struct {
			IdentityID string `json:"identityId"`
		}
		if json.Unmarshal(c.Payload, &in) != nil || in.IdentityID == "" || in.IdentityID == p.IdentityID {
			return snapshot{}, errors.New("invalid ownership target")
		}
		var targetAccount sql.NullString
		if e = tx.QueryRow(`SELECT i.account_id FROM room_members m JOIN identities i ON i.id=m.identity_id WHERE m.room_id=? AND m.identity_id=?`, room, in.IdentityID).Scan(&targetAccount); e != nil || !targetAccount.Valid {
			return snapshot{}, errors.New("new owner must be an account-linked room member")
		}
		if _, e = tx.Exec("UPDATE room_members SET role='admin' WHERE room_id=? AND identity_id=?", room, p.IdentityID); e == nil {
			_, e = tx.Exec("UPDATE room_members SET role='owner' WHERE room_id=? AND identity_id=?", room, in.IdentityID)
		}
		if e == nil {
			_, e = tx.Exec("UPDATE rooms SET owner_identity_id=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", in.IdentityID, room)
		}
		payload["identityId"] = in.IdentityID
	default:
		return snapshot{}, fmt.Errorf("unsupported command %s", c.Type)
	}
	if e != nil {
		return snapshot{}, e
	}
	if e = a.insertEventTx(tx, room, p.IdentityID, eventType, payload); e != nil {
		return snapshot{}, e
	}
	if _, e = tx.Exec("UPDATE rooms SET revision=revision+1,last_active_at=CURRENT_TIMESTAMP WHERE id=?", room); e != nil {
		return snapshot{}, e
	}
	if e = tx.Commit(); e != nil {
		return snapshot{}, e
	}
	if c.Type == "member.kick" || c.Type == "member.ban" {
		if id, ok := payload["identityId"].(string); ok {
			a.hub.disconnect(room, id)
		}
	}
	if enrichVideoID != "" {
		go a.enrichTitle(room, "YT"+enrichVideoID, enrichVideoID)
	}
	if a.segments != nil && activatedVideoID != "" {
		go a.enrichSegments(room, activatedVideoID)
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
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return e
	}
	rows.Close()
	_, e = tx.Exec("UPDATE room_queue_items SET position=position+1000000 WHERE room_id=?", room)
	for i, id := range ids {
		if e == nil {
			_, e = tx.Exec("UPDATE room_queue_items SET position=? WHERE room_id=? AND id=?", i, room, id)
		}
	}
	return e
}

func addCurrentToHistory(tx *sql.Tx, room string) error {
	var mediaID sql.NullString
	if e := tx.QueryRow("SELECT current_media_id FROM playback_states WHERE room_id=?", room).Scan(&mediaID); e != nil {
		return e
	}
	if !mediaID.Valid {
		return nil
	}
	if _, e := tx.Exec("INSERT INTO room_history(id,room_id,media_id) VALUES(?,?,?)", newID(10), room, mediaID.String); e != nil {
		return e
	}
	_, e := tx.Exec(`DELETE FROM room_history WHERE id IN (SELECT id FROM room_history WHERE room_id=? ORDER BY played_at DESC LIMIT -1 OFFSET 20)`, room)
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

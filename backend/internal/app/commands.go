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
	"unicode"
	"unicode/utf8"
)

type command struct {
	Type                     string          `json:"type"`
	RequestID                string          `json:"requestId"`
	ExpectedRevision         int64           `json:"expectedRevision"`
	ExpectedPlaybackRevision *int64          `json:"expectedPlaybackRevision,omitempty"`
	Payload                  json.RawMessage `json:"payload"`
}

func (a *application) roomCommand(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("roomId")
	var c command
	if !decode(w, r, &c) {
		return
	}
	s, e := a.applyCommand(r.Context(), id, p, c)
	if e != nil {
		status, code, message := commandProblem(e)
		if a.metrics != nil {
			a.metrics.commandsRejected.Add(1)
		}
		a.logCommand(r.Context(), id, p, c, "rejected", code)
		problem(w, status, code, message)
		return
	}
	if a.metrics != nil {
		a.metrics.commandsAccepted.Add(1)
	}
	a.logCommand(r.Context(), id, p, c, "accepted", "")
	a.hub.broadcast(id, s)
	writeJSON(w, 200, s)
}

// userError is a rejection whose message is safe and useful to show verbatim.
type userError struct{ code, message string }

func (e userError) Error() string { return e.message }

func reject(code, message string) error { return userError{code: code, message: message} }

// commandProblem maps a command rejection to the HTTP status, stable code and
// human-readable message shared by the REST and WebSocket command paths.
func commandProblem(err error) (int, string, string) {
	var user userError
	if errors.As(err, &user) {
		return http.StatusBadRequest, user.code, user.message
	}
	code := commandErrorCode(err)
	switch code {
	case "permission_denied":
		return http.StatusForbidden, code, "You don't have permission to do that in this room."
	case "stale_revision":
		return http.StatusConflict, code, "Someone changed the room at the same moment. Please try again."
	case "request_id_conflict":
		return http.StatusConflict, code, "Request ID was already used for another command."
	case "invalid_command":
		return http.StatusBadRequest, code, "The room command was invalid."
	case "not_found":
		return http.StatusNotFound, code, "That item no longer exists in this room."
	case "unsupported_command":
		return http.StatusBadRequest, code, "This room command is not supported."
	}
	return http.StatusBadRequest, code, "Room command failed."
}

var errDenied = errors.New("permission denied")
var errStale = errors.New("stale revision")
var errNoActiveMedia = errors.New("invalid player command without active media")

func (a *application) roleAndAllowed(room, identity, cap string) (string, bool) {
	return roleAndAllowed(context.Background(), a.db, room, identity, cap)
}
func capFor(t string) string {
	switch t {
	case "player.play", "player.pause", "player.rate":
		return "playback.play_pause"
	case "player.seek":
		return "playback.seek"
	case "playback.ended":
		return "queue.skip"
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
	case "queue.vote", "queue.vote_skip":
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
	case "room.sponsorblock", "room.rename":
		return "room.manage_visibility"
	case "room.transfer":
		return "room.manage_ownership"
	}
	return ""
}
func (a *application) applyCommand(ctx context.Context, room string, p principal, c command) (snapshot, error) {
	if c.RequestID != "" && !validRequestID(c.RequestID) {
		return snapshot{}, errors.New("invalid request ID")
	}
	cap := capFor(c.Type)
	if cap == "" {
		return snapshot{}, errDenied
	}
	// A queued video is stored immediately with a placeholder title so the add is
	// instant; the real oEmbed title is fetched afterwards by enrichTitle. This
	// keeps adding a video fast even when the server's outbound network to YouTube
	// is slow or unavailable.
	var additions []queueAddition
	var enrichVideoIDs []string
	var insertAt *int
	var conditionalSkipMediaID string
	var discardSkippedMedia bool
	var endedPosition, endedDuration float64
	if c.Type == "queue.add" || c.Type == "queue.play_now" {
		var in struct {
			queueAddition
			Items    []queueAddition `json:"items"`
			Position *int            `json:"position"`
		}
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid YouTube video ID")
		}
		additions = in.Items
		if len(additions) == 0 {
			additions = []queueAddition{in.queueAddition}
		}
		if len(additions) > maxQueueItems || (c.Type == "queue.play_now" && len(additions) != 1) {
			return snapshot{}, errors.New("invalid number of videos")
		}
		for _, item := range additions {
			if !youtubeID.MatchString(item.VideoID) {
				return snapshot{}, errors.New("invalid YouTube video ID")
			}
			if math.IsNaN(item.Start) || math.IsInf(item.Start, 0) || item.Start < 0 || item.Start > 604800 {
				return snapshot{}, errors.New("invalid start position")
			}
		}
		if in.Position != nil {
			if *in.Position < 0 {
				return snapshot{}, errors.New("invalid queue position")
			}
			insertAt = in.Position
		}
	} else if c.Type == "queue.skip" || c.Type == "playback.ended" {
		var in struct {
			MediaID        string  `json:"mediaId"`
			DiscardCurrent bool    `json:"discardCurrent"`
			Position       float64 `json:"position"`
			Duration       float64 `json:"duration"`
		}
		if len(c.Payload) > 0 && json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid queue skip")
		}
		if c.Type == "playback.ended" {
			if c.ExpectedPlaybackRevision == nil || in.MediaID == "" || !validEndedPosition(in.Position, in.Duration) {
				return snapshot{}, errors.New("invalid playback end report")
			}
			endedPosition, endedDuration = in.Position, in.Duration
		}
		conditionalSkipMediaID = in.MediaID
		discardSkippedMedia = in.DiscardCurrent
	}
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return snapshot{}, e
	}
	defer tx.Rollback()
	role, allowed := roleAndAllowed(ctx, tx, room, p.IdentityID, cap)
	management := strings.HasPrefix(cap, "members.") || strings.HasPrefix(cap, "room.")
	if !allowed || (management && role == "member") {
		return snapshot{}, errDenied
	}
	var current int64
	if e = tx.QueryRow("SELECT revision FROM rooms WHERE id=? AND deleted_at IS NULL", room).Scan(&current); e != nil {
		return snapshot{}, errDenied
	}
	if c.RequestID != "" {
		result, receiptErr := tx.Exec("INSERT OR IGNORE INTO command_receipts(room_id,identity_id,request_id,command_type) VALUES(?,?,?,?)", room, p.IdentityID, c.RequestID, c.Type)
		if receiptErr != nil {
			return snapshot{}, receiptErr
		}
		if changed, rowsErr := result.RowsAffected(); rowsErr != nil {
			return snapshot{}, rowsErr
		} else if changed == 0 {
			var previousType string
			if scanErr := tx.QueryRow("SELECT command_type FROM command_receipts WHERE room_id=? AND identity_id=? AND request_id=?", room, p.IdentityID, c.RequestID).Scan(&previousType); scanErr != nil {
				return snapshot{}, scanErr
			}
			_ = tx.Rollback()
			if previousType != c.Type {
				return snapshot{}, errors.New("request ID already used for another command")
			}
			return a.snapshot(ctx, room, p.IdentityID)
		}
	}
	playbackCommand := strings.HasPrefix(c.Type, "player.")
	if playbackCommand || conditionalSkipMediaID != "" {
		var playbackRevision int64
		var currentMediaID sql.NullString
		var playbackStatus string
		if e = tx.QueryRow("SELECT revision,current_media_id,status FROM playback_states WHERE room_id=?", room).Scan(&playbackRevision, &currentMediaID, &playbackStatus); e != nil {
			return snapshot{}, e
		}
		if c.ExpectedPlaybackRevision != nil && *c.ExpectedPlaybackRevision != playbackRevision {
			return snapshot{}, errStale
		}
		if playbackCommand {
			// Older clients do not send the dedicated playback revision. Preserve
			// their room-revision guard instead of silently accepting stale commands.
			if c.ExpectedPlaybackRevision == nil && c.ExpectedRevision != current {
				return snapshot{}, errStale
			}
			if !currentMediaID.Valid {
				return snapshot{}, errNoActiveMedia
			}
		} else if !currentMediaID.Valid || currentMediaID.String != conditionalSkipMediaID {
			return snapshot{}, errStale
		} else if c.Type == "playback.ended" && playbackStatus != "playing" {
			return snapshot{}, errStale
		}
	} else if !revisionFreeCommands[c.Type] && c.ExpectedRevision != current {
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
			_, e = tx.Exec("UPDATE playback_states SET position_seconds=?,revision=revision+1,updated_at=strftime('%Y-%m-%d %H:%M:%f','now'),updated_by_identity_id=? WHERE room_id=?", in.Position, p.IdentityID, room)
		} else {
			_, e = tx.Exec("UPDATE playback_states SET status=?,position_seconds=?,revision=revision+1,updated_at=strftime('%Y-%m-%d %H:%M:%f','now'),updated_by_identity_id=? WHERE room_id=?", status, in.Position, p.IdentityID, room)
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
		_, e = tx.Exec("UPDATE playback_states SET playback_rate=?,position_seconds=?,revision=revision+1,updated_at=strftime('%Y-%m-%d %H:%M:%f','now'),updated_by_identity_id=? WHERE room_id=?", in.Rate, in.Position, p.IdentityID, room)
		payload["rate"] = in.Rate
		payload["position"] = in.Position
	case "queue.add", "queue.play_now":
		var queueCount int
		if e = tx.QueryRow("SELECT count(*) FROM room_queue_items WHERE room_id=?", room).Scan(&queueCount); e != nil {
			return snapshot{}, e
		}
		added := []queueAddition{}
		seen := map[string]bool{}
		full := false
		for _, item := range additions {
			mediaID := "YT" + item.VideoID
			if seen[mediaID] {
				continue
			}
			seen[mediaID] = true
			var duplicate int
			if e = tx.QueryRow(`SELECT count(*) FROM (
				SELECT media_id FROM room_queue_items WHERE room_id=? AND media_id=?
				UNION ALL SELECT current_media_id FROM playback_states WHERE room_id=? AND current_media_id=?
			)`, room, mediaID, room, mediaID).Scan(&duplicate); e != nil {
				return snapshot{}, e
			}
			if duplicate > 0 {
				continue
			}
			if c.Type == "queue.add" && queueCount+len(added) >= maxQueueItems {
				full = true
				break
			}
			if _, e = tx.Exec("INSERT INTO media_items(id,provider,provider_media_id,title,thumbnail_url) VALUES(?,'youtube',?,?,?) ON CONFLICT(provider,provider_media_id) DO NOTHING", mediaID, item.VideoID, fallbackTitle("", item.VideoID), "https://i.ytimg.com/vi/"+item.VideoID+"/mqdefault.jpg"); e != nil {
				return snapshot{}, e
			}
			added = append(added, item)
		}
		if len(added) == 0 {
			if full {
				return snapshot{}, reject("queue_full", "The queue is full (100 videos). Remove something first.")
			}
			return snapshot{}, reject("already_queued", "That video is already in the queue or playing.")
		}
		for _, item := range added {
			enrichVideoIDs = append(enrichVideoIDs, item.VideoID)
		}
		if c.Type == "queue.play_now" {
			if e = addCurrentToHistory(tx, room); e == nil {
				e = setCurrentMedia(tx, room, p.IdentityID, "YT"+added[0].VideoID, added[0].Start)
			}
			eventType = "media.activated"
			activatedVideoID = added[0].VideoID
		} else {
			e = insertQueueItems(tx, room, p.IdentityID, added, insertAt)
			if e == nil {
				// An idle room starts the first added video right away instead of
				// leaving it waiting behind an extra "play" click.
				var currentMedia sql.NullString
				if e = tx.QueryRow("SELECT current_media_id FROM playback_states WHERE room_id=?", room).Scan(&currentMedia); e == nil && !currentMedia.Valid {
					activatedVideoID, e = advanceQueue(tx, room, p.IdentityID)
				}
			}
		}
		payload["videoId"] = added[0].VideoID
		payload["title"] = fallbackTitle("", added[0].VideoID)
		if len(added) > 1 {
			payload["count"] = len(added)
		}
		if insertAt != nil && *insertAt == 0 {
			payload["next"] = true
		}
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
	case "queue.skip", "playback.ended":
		if c.Type == "playback.ended" {
			eventType = "media.ended"
			payload["position"] = endedPosition
			payload["duration"] = endedDuration
		}
		var loop bool
		_ = tx.QueryRow("SELECT queue_loop FROM rooms WHERE id=?", room).Scan(&loop)
		if e = addCurrentToHistory(tx, room); e != nil {
			return snapshot{}, e
		}
		if loop && !discardSkippedMedia {
			var currentMedia sql.NullString
			_ = tx.QueryRow("SELECT current_media_id FROM playback_states WHERE room_id=?", room).Scan(&currentMedia)
			if currentMedia.Valid {
				var pos int
				_ = tx.QueryRow("SELECT coalesce(max(position),-1)+1 FROM room_queue_items WHERE room_id=?", room).Scan(&pos)
				_, e = tx.Exec("INSERT INTO room_queue_items(id,room_id,media_id,position,added_by_identity_id) VALUES(?,?,?,?,?)", newID(10), room, currentMedia.String, pos, p.IdentityID)
			}
		}
		activatedVideoID, e = advanceQueue(tx, room, p.IdentityID)
	case "queue.vote_skip":
		var currentMedia sql.NullString
		if e = tx.QueryRow("SELECT current_media_id FROM playback_states WHERE room_id=?", room).Scan(&currentMedia); e != nil {
			return snapshot{}, e
		}
		if !currentMedia.Valid {
			return snapshot{}, reject("nothing_playing", "Nothing is playing right now.")
		}
		var voted int
		_ = tx.QueryRow("SELECT count(*) FROM skip_votes WHERE room_id=? AND identity_id=? AND media_id=?", room, p.IdentityID, currentMedia.String).Scan(&voted)
		if voted > 0 {
			_, e = tx.Exec("DELETE FROM skip_votes WHERE room_id=? AND identity_id=?", room, p.IdentityID)
			eventType = "media.skip_vote_withdrawn"
			break
		}
		if _, e = tx.Exec("INSERT INTO skip_votes(room_id,media_id,identity_id) VALUES(?,?,?) ON CONFLICT(room_id,identity_id) DO UPDATE SET media_id=excluded.media_id,created_at=CURRENT_TIMESTAMP", room, currentMedia.String, p.IdentityID); e != nil {
			return snapshot{}, e
		}
		var votes int
		if e = tx.QueryRow("SELECT count(*) FROM skip_votes WHERE room_id=? AND media_id=?", room, currentMedia.String).Scan(&votes); e != nil {
			return snapshot{}, e
		}
		eventType = "media.skip_voted"
		if votes >= skipVotesNeeded(a.hub.activeCount(room)) {
			eventType = "media.vote_skipped"
			if e = addCurrentToHistory(tx, room); e == nil {
				activatedVideoID, e = advanceQueue(tx, room, p.IdentityID)
			}
		}
	case "room.rename":
		var in struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid room name")
		}
		name := cleanName(in.Name)
		if utf8.RuneCountInString(name) > 60 {
			return snapshot{}, reject("invalid_room_name", "Room names can be at most 60 characters.")
		}
		_, e = tx.Exec("UPDATE rooms SET name=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", nullable(name), room)
		payload["name"] = name
	case "member.role":
		var in struct{ IdentityID, Role string }
		if json.Unmarshal(c.Payload, &in) != nil {
			return snapshot{}, errors.New("invalid member role")
		}
		var targetRole string
		if e = tx.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, in.IdentityID).Scan(&targetRole); e != nil {
			return snapshot{}, reject("member_not_found", "That person is no longer in this room.")
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
			return snapshot{}, reject("member_not_found", "That person is no longer in this room.")
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
			return snapshot{}, reject("member_not_found", "That person is no longer in this room.")
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
				return snapshot{}, reject("public_rooms_disabled", "Public rooms are disabled on this server.")
			}
		}
		if in.Visibility != "unlisted" {
			var ownerAccount sql.NullString
			_ = tx.QueryRow("SELECT i.account_id FROM rooms r JOIN identities i ON i.id=r.owner_identity_id WHERE r.id=?", room).Scan(&ownerAccount)
			if !ownerAccount.Valid {
				return snapshot{}, reject("account_required", "Only rooms owned by an account can be private or friends-only.")
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
		if e == nil && in.Enabled {
			// Turning it on mid-video should skip right away, so fetch the current
			// video's segments in the background after commit.
			var mediaID sql.NullString
			_ = tx.QueryRow("SELECT current_media_id FROM playback_states WHERE room_id=?", room).Scan(&mediaID)
			if mediaID.Valid {
				activatedVideoID = strings.TrimPrefix(mediaID.String, "YT")
			}
		}
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
			return snapshot{}, reject("account_required", "The new owner needs an account and must be a room member.")
		}
		// Transfer retains the former owner as admin. Preserve their private-room
		// eligibility once ownership no longer supplies it implicitly.
		if _, e = tx.Exec(`INSERT INTO room_invites(id,room_id,account_id,created_by_identity_id)
			SELECT ?,r.id,i.account_id,? FROM rooms r JOIN identities i ON i.id=r.owner_identity_id
			WHERE r.id=? AND r.visibility='private' AND i.account_id IS NOT NULL
			ON CONFLICT(room_id,account_id) DO NOTHING`, newID(10), p.IdentityID, room); e != nil {
			return snapshot{}, e
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
	if len(enrichVideoIDs) > 0 && a.fetchTitle != nil {
		go func() {
			for _, videoID := range enrichVideoIDs {
				a.enrichTitle(room, "YT"+videoID, videoID)
			}
		}()
	}
	if a.segments != nil && activatedVideoID != "" {
		go a.enrichSegments(room, activatedVideoID)
	}
	return a.snapshot(ctx, room, p.IdentityID)
}

func validEndedPosition(position, duration float64) bool {
	if math.IsNaN(position) || math.IsInf(position, 0) || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return false
	}
	if position <= 0 || duration <= 0 || duration > 604800 || position > duration+2 {
		return false
	}
	tolerance := math.Min(5, math.Max(1.5, duration*0.02))
	return duration-position <= tolerance
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

type queueAddition struct {
	VideoID string  `json:"videoId"`
	Start   float64 `json:"start"`
}

// revisionFreeCommands are intents whose effect does not depend on the exact
// room state the sender saw. They must not fail just because someone else
// played, seeked or joined a moment earlier.
var revisionFreeCommands = map[string]bool{
	"queue.add":         true,
	"queue.play_now":    true,
	"queue.remove":      true,
	"queue.vote":        true,
	"queue.vote_skip":   true,
	"queue.loop":        true,
	"room.sponsorblock": true,
	"room.rename":       true,
}

// skipVotesNeeded is a simple majority of the people currently connected.
func skipVotesNeeded(active int) int {
	return max(1, active/2+1)
}

// cleanName trims a user-chosen name and drops control characters.
func cleanName(raw string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw))
}

// setCurrentMedia makes mediaID the room's current video, playing from start,
// or clears playback when mediaID is empty. Pending skip votes are discarded.
func setCurrentMedia(tx *sql.Tx, room, actor, mediaID string, start float64) error {
	status := "playing"
	if mediaID == "" {
		status = "paused"
	}
	if _, e := tx.Exec("UPDATE playback_states SET current_media_id=?,status=?,position_seconds=?,playback_rate=1,revision=revision+1,updated_at=strftime('%Y-%m-%d %H:%M:%f','now'),updated_by_identity_id=? WHERE room_id=?", nullable(mediaID), status, start, actor, room); e != nil {
		return e
	}
	_, e := tx.Exec("DELETE FROM skip_votes WHERE room_id=?", room)
	return e
}

// advanceQueue makes the next queued item (most votes first) current, or clears
// playback when the queue is empty. It returns the activated YouTube ID.
func advanceQueue(tx *sql.Tx, room, actor string) (string, error) {
	var mediaID, queueItemID string
	var start float64
	e := tx.QueryRow("SELECT q.id,q.media_id,q.start_seconds FROM room_queue_items q LEFT JOIN queue_votes v ON v.queue_item_id=q.id WHERE q.room_id=? GROUP BY q.id ORDER BY count(v.identity_id) DESC,q.position LIMIT 1", room).Scan(&queueItemID, &mediaID, &start)
	if errors.Is(e, sql.ErrNoRows) {
		return "", setCurrentMedia(tx, room, actor, "", 0)
	}
	if e != nil {
		return "", e
	}
	if _, e = tx.Exec("DELETE FROM room_queue_items WHERE room_id=? AND id=?", room, queueItemID); e != nil {
		return "", e
	}
	if e = resequence(tx, room); e != nil {
		return "", e
	}
	if e = setCurrentMedia(tx, room, actor, mediaID, start); e != nil {
		return "", e
	}
	return strings.TrimPrefix(mediaID, "YT"), nil
}

// insertQueueItems appends items, or inserts them before index `at` when given.
func insertQueueItems(tx *sql.Tx, room, actor string, items []queueAddition, at *int) error {
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
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	index := len(ids)
	if at != nil && *at < index {
		index = *at
	}
	newIDs := make([]string, 0, len(items))
	for i, item := range items {
		id := newID(10)
		if _, e = tx.Exec("INSERT INTO room_queue_items(id,room_id,media_id,position,added_by_identity_id,start_seconds) VALUES(?,?,?,?,?,?)", id, room, "YT"+item.VideoID, 2000000+i, actor, item.Start); e != nil {
			return e
		}
		newIDs = append(newIDs, id)
	}
	ordered := append(append(append([]string{}, ids[:index]...), newIDs...), ids[index:]...)
	if _, e = tx.Exec("UPDATE room_queue_items SET position=position+1000000 WHERE room_id=? AND position<2000000", room); e != nil {
		return e
	}
	for position, id := range ordered {
		if _, e = tx.Exec("UPDATE room_queue_items SET position=? WHERE room_id=? AND id=?", position, room, id); e != nil {
			return e
		}
	}
	return nil
}

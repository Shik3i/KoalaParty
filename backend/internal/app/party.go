package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Room modes are permission presets for ordinary members. Owners and admins
// always keep every capability, and per-member overrides still win.
var roomModes = map[string]bool{"party": true, "cinema": true, "host": true}

// In cinema mode members suggest, vote and chat while hosts drive playback; in
// host mode members can only vote on the queue and chat.
var modeMemberCapabilities = map[string]map[string]bool{
	"cinema": {"queue.add": true, "queue.vote": true, "chat.send": true},
	"host":   {"queue.vote": true, "chat.send": true},
}

func modeAllows(mode, capability string) bool {
	allowed, restricted := modeMemberCapabilities[mode]
	if !restricted || !contains(memberCapabilities, capability) {
		return true
	}
	return allowed[capability]
}

// anchorSQL is the playback anchor expression, optionally in the future so the
// room counts down before playing.
func anchorSQL(startInSeconds int) string {
	if startInSeconds <= 0 {
		return "strftime('%Y-%m-%d %H:%M:%f','now')"
	}
	return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:%%M:%%f','now','+%d seconds')", startInSeconds)
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,30}[a-z0-9])$`)

// Short links share the site's path space and must never look like an app route.
var reservedSlugs = map[string]bool{
	"api": true, "app": true, "admin": true, "account": true, "login": true, "register": true, "logout": true,
	"room": true, "rooms": true, "r": true, "share": true, "discover": true, "friends": true, "privacy": true,
	"imprint": true, "about": true, "help": true, "support": true, "koalaparty": true, "static": true,
}

// normalizeSlug lowercases a requested short link and validates it. An empty
// value removes the link.
func normalizeSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" {
		return "", nil
	}
	if !slugPattern.MatchString(slug) || strings.Contains(slug, "--") {
		return "", reject("invalid_slug", "Use 3–32 letters, numbers or dashes for the room link.")
	}
	if reservedSlugs[slug] {
		return "", reject("slug_taken", "That room link is already taken.")
	}
	return slug, nil
}

// roomBySlug resolves a short link to its room ID. It reveals nothing a visitor
// with the link could not see anyway; room access is still enforced on join.
func (a *application) roomBySlug(w http.ResponseWriter, r *http.Request) {
	slug, err := normalizeSlug(r.PathValue("slug"))
	if err != nil || slug == "" {
		problem(w, 404, "room_not_found", "Room was not found.")
		return
	}
	var id string
	if e := a.db.QueryRowContext(r.Context(), "SELECT id FROM rooms WHERE slug=? AND deleted_at IS NULL", slug).Scan(&id); e != nil {
		problem(w, 404, "room_not_found", "Room was not found.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]string{"id": id})
}

func (a *application) managesRoom(ctx context.Context, room string, p principal) bool {
	role, _ := roleAndAllowed(ctx, a.db, room, p.IdentityID, "members.ban")
	return role == "owner" || role == "admin"
}

// roomBans lists active bans so managers can lift them.
func (a *application) roomBans(w http.ResponseWriter, r *http.Request, p principal) {
	room := r.PathValue("roomId")
	if !a.managesRoom(r.Context(), room, p) {
		problem(w, 403, "permission_denied", "Only owners and admins can see bans.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT b.identity_id,coalesce(i.display_name,''),b.created_at FROM room_bans b
		LEFT JOIN identities i ON i.id=b.identity_id WHERE b.room_id=? AND b.revoked_at IS NULL ORDER BY b.created_at DESC LIMIT 200`, room)
	if err != nil {
		problem(w, 500, "database_error", "Could not load bans.")
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var identityID, name, createdAt string
		if rows.Scan(&identityID, &name, &createdAt) != nil {
			problem(w, 500, "database_error", "Could not load bans.")
			return
		}
		out = append(out, map[string]string{"identityId": identityID, "displayName": name, "createdAt": createdAt})
	}
	writeJSON(w, 200, out)
}

// Saved queues let account holders keep a list of videos and load it into any
// room later. Only video IDs and start times are stored.
const (
	maxSavedQueues     = 50
	maxSavedQueueItems = maxQueueItems
)

type savedQueue struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Items     []queueAddition `json:"items"`
	ItemCount int             `json:"itemCount"`
	CreatedAt string          `json:"createdAt"`
}

func (a *application) savedQueues(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	if r.Method == http.MethodPost {
		a.createSavedQueue(w, r, p)
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,name,items_json,item_count,created_at FROM saved_queues WHERE account_id=? ORDER BY created_at DESC", p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not load saved queues.")
		return
	}
	defer rows.Close()
	out := []savedQueue{}
	for rows.Next() {
		var q savedQueue
		var raw string
		if rows.Scan(&q.ID, &q.Name, &raw, &q.ItemCount, &q.CreatedAt) != nil || json.Unmarshal([]byte(raw), &q.Items) != nil {
			problem(w, 500, "database_error", "Could not load saved queues.")
			return
		}
		out = append(out, q)
	}
	writeJSON(w, 200, out)
}

func (a *application) createSavedQueue(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		Name  string          `json:"name"`
		Items []queueAddition `json:"items"`
	}
	if !decode(w, r, &in) {
		return
	}
	name := cleanName(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 60 {
		problem(w, 400, "invalid_name", "Give the queue a name of up to 60 characters.")
		return
	}
	if len(in.Items) == 0 || len(in.Items) > maxSavedQueueItems {
		problem(w, 400, "invalid_queue", "A saved queue holds 1 to 100 videos.")
		return
	}
	for _, item := range in.Items {
		if !youtubeID.MatchString(item.VideoID) || item.Start < 0 || item.Start > 604800 {
			problem(w, 400, "invalid_queue", "The queue contains an invalid video.")
			return
		}
	}
	raw, _ := json.Marshal(in.Items)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "database_error", "Could not save the queue.")
		return
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM saved_queues WHERE account_id=?", p.AccountID).Scan(&count); err != nil {
		problem(w, 500, "database_error", "Could not save the queue.")
		return
	}
	if count >= maxSavedQueues {
		problem(w, 409, "saved_queue_limit", "You can keep up to 50 saved queues. Delete one first.")
		return
	}
	q := savedQueue{ID: newID(10), Name: name, Items: in.Items, ItemCount: len(in.Items), CreatedAt: time.Now().UTC().Format("2006-01-02 15:04:05")}
	if _, err = tx.Exec("INSERT INTO saved_queues(id,account_id,name,items_json,item_count) VALUES(?,?,?,?,?)", q.ID, p.AccountID, q.Name, string(raw), q.ItemCount); err != nil {
		problem(w, 500, "database_error", "Could not save the queue.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "database_error", "Could not save the queue.")
		return
	}
	writeJSON(w, 201, q)
}

func (a *application) deleteSavedQueue(w http.ResponseWriter, r *http.Request, p principal) {
	if !requireAccount(w, p) {
		return
	}
	res, err := a.db.ExecContext(r.Context(), "DELETE FROM saved_queues WHERE id=? AND account_id=?", r.PathValue("queueId"), p.AccountID)
	if err != nil {
		problem(w, 500, "database_error", "Could not delete the queue.")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		problem(w, 404, "not_found", "That saved queue no longer exists.")
		return
	}
	w.WriteHeader(204)
}

// Reaction heatmap: counts of reactions per few seconds of the current video,
// kept in memory like chat and dropped with the room. Only counts are kept.
const heatmapBucketSeconds = 5

type heatmap struct {
	mediaID string
	buckets map[int]int
}

func (h *hub) recordReaction(room, mediaID string, position float64) (int, bool) {
	if mediaID == "" || position < 0 {
		return 0, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rooms[room]) == 0 {
		return 0, false
	}
	current := h.heatmaps[room]
	if current == nil || current.mediaID != mediaID {
		current = &heatmap{mediaID: mediaID, buckets: map[int]int{}}
		h.heatmaps[room] = current
	}
	bucket := int(position) / heatmapBucketSeconds
	if len(current.buckets) >= 2000 && current.buckets[bucket] == 0 {
		return bucket, false
	}
	current.buckets[bucket]++
	return bucket, true
}

func (h *hub) heatmapFor(room string) map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	current := h.heatmaps[room]
	if current == nil {
		return nil
	}
	buckets := make(map[string]int, len(current.buckets))
	for bucket, count := range current.buckets {
		buckets[fmt.Sprint(bucket)] = count
	}
	return map[string]any{"type": "heatmap", "mediaId": current.mediaID, "bucketSeconds": heatmapBucketSeconds, "buckets": buckets}
}

// currentPlayback returns the current media ID and live position for a room.
func (a *application) currentPlayback(ctx context.Context, room string) (string, float64, error) {
	var media sql.NullString
	var status, updatedAt string
	var position, rate float64
	err := a.db.QueryRowContext(ctx, "SELECT current_media_id,status,position_seconds,playback_rate,updated_at FROM playback_states WHERE room_id=?", room).Scan(&media, &status, &position, &rate, &updatedAt)
	if err != nil {
		return "", 0, err
	}
	if !media.Valid {
		return "", 0, errors.New("nothing playing")
	}
	if status == "playing" {
		if updated, parseErr := time.Parse("2006-01-02 15:04:05", updatedAt); parseErr == nil {
			position += max(0, time.Since(updated.UTC()).Seconds()) * rate
		}
	}
	return media.String, position, nil
}

// handleDuration stores the current video's length the first time a viewer
// who may skip reports it, so the server can stop its clock at the end.
func (a *application) handleDuration(ctx context.Context, c *client, room string, p principal, cmd command) {
	var in struct {
		MediaID  string  `json:"mediaId"`
		Duration float64 `json:"duration"`
	}
	if json.Unmarshal(cmd.Payload, &in) != nil || in.MediaID == "" || math.IsNaN(in.Duration) || in.Duration <= 0 || in.Duration > 604800 {
		c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "invalid_command", "message": "Invalid duration."})
		return
	}
	if _, allowed := roleAndAllowed(ctx, a.db, room, p.IdentityID, "queue.skip"); !allowed {
		return
	}
	_, _ = a.db.ExecContext(ctx, `UPDATE media_items SET duration_seconds=? WHERE id=? AND duration_seconds IS NULL
		AND EXISTS (SELECT 1 FROM playback_states WHERE room_id=? AND current_media_id=?)`, in.Duration, in.MediaID, room, in.MediaID)
}

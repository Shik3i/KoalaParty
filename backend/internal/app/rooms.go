package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var roomIDPattern = regexp.MustCompile(`^[A-Z2-7]{16}$`)
var memberCapabilities = []string{"playback.play_pause", "playback.seek", "media.play_now", "queue.add", "queue.remove", "queue.reorder", "queue.skip"}

type media struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerId"`
	Title      string `json:"title"`
	Thumbnail  string `json:"thumbnail"`
}
type queueItem struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
	Media    media  `json:"media"`
}
type member struct {
	IdentityID  string          `json:"identityId"`
	DisplayName string          `json:"displayName"`
	Role        string          `json:"role"`
	Permissions map[string]bool `json:"permissions"`
}
type playback struct {
	Media     *media  `json:"media"`
	Status    string  `json:"status"`
	Position  float64 `json:"position"`
	Revision  int64   `json:"revision"`
	UpdatedAt string  `json:"updatedAt"`
}
type event struct {
	ID        string         `json:"id"`
	ActorID   string         `json:"actorId,omitempty"`
	ActorName string         `json:"actorName,omitempty"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"createdAt"`
}
type snapshot struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`
	Visibility string      `json:"visibility"`
	Me         string      `json:"me"`
	Members    []member    `json:"members"`
	Queue      []queueItem `json:"queue"`
	Playback   playback    `json:"playback"`
	Events     []event     `json:"events"`
	Revision   int64       `json:"revision"`
}

func newID(bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b)
	return strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), "=")
}
func roomLabel(id string) string {
	adjectives := []string{"Calm", "Gentle", "Mossy", "Quiet", "Sunny", "Cozy", "Bamboo", "Forest"}
	animals := []string{"Koala", "Wombat", "Kookaburra", "Possum"}
	n := 0
	for _, c := range []byte(id) {
		n = n*31 + int(c)
	}
	if n < 0 {
		n = -n
	}
	return fmt.Sprintf("%s %s %03d", adjectives[n%len(adjectives)], animals[(n/7)%len(animals)], n%1000)
}
func (a *application) createRoom(w http.ResponseWriter, r *http.Request, p principal) {
	id := newID(10)
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 500, "database_error", "Could not create room.")
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec("INSERT INTO rooms(id,owner_identity_id) VALUES(?,?)", id, p.IdentityID)
	if e == nil {
		_, e = tx.Exec("INSERT INTO room_members(room_id,identity_id,role) VALUES(?,?,'owner')", id, p.IdentityID)
	}
	if e == nil {
		_, e = tx.Exec("INSERT INTO playback_states(room_id) VALUES(?)", id)
	}
	if e == nil {
		e = a.insertEventTx(tx, id, p.IdentityID, "room.created", map[string]any{})
	}
	if e != nil {
		problem(w, 500, "database_error", "Could not create room.")
		return
	}
	if e = tx.Commit(); e != nil {
		problem(w, 500, "database_error", "Could not create room.")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "label": roomLabel(id)})
}
func (a *application) roomSnapshot(w http.ResponseWriter, r *http.Request, p principal) {
	id := r.PathValue("roomId")
	s, e := a.joinAndSnapshot(r.Context(), id, p)
	if e != nil {
		roomProblem(w, e)
		return
	}
	writeJSON(w, 200, s)
}
func roomProblem(w http.ResponseWriter, e error) {
	switch e.Error() {
	case "not_found":
		problem(w, 404, "room_not_found", "Room was not found.")
	case "banned":
		problem(w, 403, "banned", "You are banned from this room.")
	case "account_required":
		problem(w, 403, "account_required", "This room requires an account.")
	case "not_allowed":
		problem(w, 403, "not_allowed", "You are not allowed to join this room.")
	default:
		problem(w, 500, "database_error", "Room request failed.")
	}
}
func (a *application) joinAndSnapshot(ctx context.Context, id string, p principal) (snapshot, error) {
	if !roomIDPattern.MatchString(id) {
		return snapshot{}, errors.New("not_found")
	}
	var visibility, owner string
	e := a.db.QueryRowContext(ctx, "SELECT visibility,owner_identity_id FROM rooms WHERE id=? AND deleted_at IS NULL", id).Scan(&visibility, &owner)
	if e != nil {
		return snapshot{}, errors.New("not_found")
	}
	var banned int
	_ = a.db.QueryRowContext(ctx, `SELECT count(*) FROM room_bans WHERE room_id=? AND revoked_at IS NULL AND (identity_id=? OR (account_id IS NOT NULL AND account_id=?))`, id, p.IdentityID, p.AccountID).Scan(&banned)
	if banned > 0 {
		return snapshot{}, errors.New("banned")
	}
	if visibility == "private" || visibility == "friends_only" {
		if p.AccountID == "" {
			return snapshot{}, errors.New("account_required")
		}
		allowed := p.IdentityID == owner
		if visibility == "private" && !allowed {
			_ = a.db.QueryRowContext(ctx, "SELECT count(*) FROM room_invites WHERE room_id=? AND account_id=?", id, p.AccountID).Scan(&banned)
			allowed = banned > 0
		}
		if visibility == "friends_only" && !allowed {
			var ownerAccount sql.NullString
			_ = a.db.QueryRowContext(ctx, "SELECT account_id FROM identities WHERE id=?", owner).Scan(&ownerAccount)
			if ownerAccount.Valid {
				_ = a.db.QueryRowContext(ctx, `SELECT count(*) FROM friendships WHERE status='accepted' AND ((requester_account_id=? AND addressee_account_id=?) OR (requester_account_id=? AND addressee_account_id=?))`, ownerAccount.String, p.AccountID, p.AccountID, ownerAccount.String).Scan(&banned)
				allowed = banned > 0
			}
		}
		if !allowed {
			return snapshot{}, errors.New("not_allowed")
		}
	}
	res, e := a.db.ExecContext(ctx, "INSERT INTO room_members(room_id,identity_id,role) VALUES(?,?,'member') ON CONFLICT(room_id,identity_id) DO UPDATE SET last_seen_at=CURRENT_TIMESTAMP", id, p.IdentityID)
	if e != nil {
		return snapshot{}, e
	}
	if n, _ := res.RowsAffected(); n > 0 {
		_ = a.insertEvent(id, p.IdentityID, "member.joined", map[string]any{})
	}
	return a.snapshot(ctx, id, p.IdentityID)
}
func (a *application) snapshot(ctx context.Context, id, me string) (snapshot, error) {
	s := snapshot{ID: id, Label: roomLabel(id), Me: me, Members: []member{}, Queue: []queueItem{}, Events: []event{}}
	if e := a.db.QueryRowContext(ctx, "SELECT visibility FROM rooms WHERE id=?", id).Scan(&s.Visibility); e != nil {
		return s, e
	}
	rows, e := a.db.QueryContext(ctx, "SELECT i.id,i.display_name,m.role FROM room_members m JOIN identities i ON i.id=m.identity_id WHERE m.room_id=? ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,i.display_name", id)
	if e != nil {
		return s, e
	}
	for rows.Next() {
		var m member
		m.Permissions = map[string]bool{}
		_ = rows.Scan(&m.IdentityID, &m.DisplayName, &m.Role)
		for _, c := range memberCapabilities {
			m.Permissions[c] = m.Role != "member"
		}
		s.Members = append(s.Members, m)
	}
	rows.Close()
	for i := range s.Members {
		if s.Members[i].Role == "member" {
			pr, _ := a.db.QueryContext(ctx, "SELECT permission,allowed FROM room_permissions WHERE room_id=? AND identity_id=?", id, s.Members[i].IdentityID)
			for pr.Next() {
				var c string
				var allowed bool
				_ = pr.Scan(&c, &allowed)
				s.Members[i].Permissions[c] = allowed
			}
			pr.Close()
			for _, c := range memberCapabilities {
				if _, ok := s.Members[i].Permissions[c]; !ok {
					s.Members[i].Permissions[c] = true
				}
			}
		}
	}
	q, _ := a.db.QueryContext(ctx, `SELECT q.id,q.position,m.id,m.provider_media_id,coalesce(m.title,''),coalesce(m.thumbnail_url,'') FROM room_queue_items q JOIN media_items m ON m.id=q.media_id WHERE q.room_id=? ORDER BY q.position`, id)
	for q.Next() {
		var x queueItem
		_ = q.Scan(&x.ID, &x.Position, &x.Media.ID, &x.Media.ProviderID, &x.Media.Title, &x.Media.Thumbnail)
		s.Queue = append(s.Queue, x)
	}
	q.Close()
	var mid, title, thumb, provider sql.NullString
	_ = a.db.QueryRowContext(ctx, `SELECT p.status,p.position_seconds,p.revision,p.updated_at,m.id,m.provider_media_id,m.title,m.thumbnail_url FROM playback_states p LEFT JOIN media_items m ON m.id=p.current_media_id WHERE p.room_id=?`, id).Scan(&s.Playback.Status, &s.Playback.Position, &s.Playback.Revision, &s.Playback.UpdatedAt, &mid, &provider, &title, &thumb)
	if mid.Valid {
		s.Playback.Media = &media{ID: mid.String, ProviderID: provider.String, Title: title.String, Thumbnail: thumb.String}
	}
	s.Revision = s.Playback.Revision
	er, _ := a.db.QueryContext(ctx, `SELECT e.id,coalesce(e.actor_identity_id,''),coalesce(i.display_name,''),e.event_type,e.payload_json,e.created_at FROM room_events e LEFT JOIN identities i ON i.id=e.actor_identity_id WHERE e.room_id=? ORDER BY e.created_at DESC LIMIT 200`, id)
	for er.Next() {
		var x event
		var raw string
		_ = er.Scan(&x.ID, &x.ActorID, &x.ActorName, &x.Type, &raw, &x.CreatedAt)
		_ = json.Unmarshal([]byte(raw), &x.Payload)
		s.Events = append(s.Events, x)
	}
	er.Close()
	sort.Slice(s.Events, func(i, j int) bool { return s.Events[i].CreatedAt < s.Events[j].CreatedAt })
	return s, nil
}

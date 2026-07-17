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
	"time"
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
	Active      bool            `json:"active"`
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
	var n uint64
	for _, c := range []byte(id) {
		n = n*31 + uint64(c)
	}
	return fmt.Sprintf("%s %s %03d", adjectives[n%uint64(len(adjectives))], animals[(n/7)%uint64(len(animals))], n%1000)
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
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return snapshot{}, e
	}
	defer tx.Rollback()
	var visibility, owner string
	e = tx.QueryRowContext(ctx, "SELECT visibility,owner_identity_id FROM rooms WHERE id=? AND deleted_at IS NULL", id).Scan(&visibility, &owner)
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return snapshot{}, errors.New("not_found")
		}
		return snapshot{}, e
	}
	var banned int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM room_bans WHERE room_id=? AND revoked_at IS NULL AND (identity_id=? OR (account_id IS NOT NULL AND account_id=?))`, id, p.IdentityID, p.AccountID).Scan(&banned); e != nil {
		return snapshot{}, e
	}
	if banned > 0 {
		return snapshot{}, errors.New("banned")
	}
	if visibility == "private" || visibility == "friends_only" {
		if p.AccountID == "" {
			return snapshot{}, errors.New("account_required")
		}
		allowed := p.IdentityID == owner
		if visibility == "private" && !allowed {
			if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM room_invites WHERE room_id=? AND account_id=?", id, p.AccountID).Scan(&banned); e != nil {
				return snapshot{}, e
			}
			allowed = banned > 0
		}
		if visibility == "friends_only" && !allowed {
			var ownerAccount sql.NullString
			if e = tx.QueryRowContext(ctx, "SELECT account_id FROM identities WHERE id=?", owner).Scan(&ownerAccount); e != nil {
				return snapshot{}, e
			}
			if ownerAccount.Valid {
				if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM friendships WHERE status='accepted' AND ((requester_account_id=? AND addressee_account_id=?) OR (requester_account_id=? AND addressee_account_id=?))`, ownerAccount.String, p.AccountID, p.AccountID, ownerAccount.String).Scan(&banned); e != nil {
					return snapshot{}, e
				}
				allowed = banned > 0
			}
		}
		if !allowed {
			return snapshot{}, errors.New("not_allowed")
		}
	}
	res, e := tx.ExecContext(ctx, "INSERT INTO room_members(room_id,identity_id,role) VALUES(?,?,'member') ON CONFLICT(room_id,identity_id) DO NOTHING", id, p.IdentityID)
	if e != nil {
		return snapshot{}, e
	}
	inserted, e := res.RowsAffected()
	if e != nil {
		return snapshot{}, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE room_members SET last_seen_at=CURRENT_TIMESTAMP WHERE room_id=? AND identity_id=?", id, p.IdentityID); e != nil {
		return snapshot{}, e
	}
	if inserted == 1 {
		if e = a.insertEventTx(tx, id, p.IdentityID, "member.joined", map[string]any{}); e != nil {
			return snapshot{}, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE rooms SET revision=revision+1,last_active_at=CURRENT_TIMESTAMP WHERE id=?", id); e != nil {
			return snapshot{}, e
		}
	}
	if e = tx.Commit(); e != nil {
		return snapshot{}, e
	}
	return a.snapshot(ctx, id, p.IdentityID)
}
func (a *application) snapshot(ctx context.Context, id, me string) (snapshot, error) {
	s := snapshot{ID: id, Label: roomLabel(id), Me: me, Members: []member{}, Queue: []queueItem{}, Events: []event{}}
	if e := a.db.QueryRowContext(ctx, "SELECT visibility,revision FROM rooms WHERE id=?", id).Scan(&s.Visibility, &s.Revision); e != nil {
		return s, e
	}
	rows, e := a.db.QueryContext(ctx, "SELECT i.id,i.display_name,m.role FROM room_members m JOIN identities i ON i.id=m.identity_id WHERE m.room_id=? ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,i.display_name", id)
	if e != nil {
		return s, e
	}
	for rows.Next() {
		var m member
		m.Permissions = map[string]bool{}
		if e = rows.Scan(&m.IdentityID, &m.DisplayName, &m.Role); e != nil {
			rows.Close()
			return s, e
		}
		for _, c := range memberCapabilities {
			m.Permissions[c] = true
		}
		s.Members = append(s.Members, m)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return s, e
	}
	rows.Close()
	for i := range s.Members {
		s.Members[i].Active = a.hub.isActive(id, s.Members[i].IdentityID)
		if s.Members[i].Role == "member" {
			pr, queryErr := a.db.QueryContext(ctx, "SELECT permission,allowed FROM room_permissions WHERE room_id=? AND identity_id=?", id, s.Members[i].IdentityID)
			if queryErr != nil {
				return s, queryErr
			}
			for pr.Next() {
				var c string
				var allowed bool
				if e = pr.Scan(&c, &allowed); e != nil {
					pr.Close()
					return s, e
				}
				s.Members[i].Permissions[c] = allowed
			}
			if e = pr.Err(); e != nil {
				pr.Close()
				return s, e
			}
			pr.Close()
			for _, c := range memberCapabilities {
				if _, ok := s.Members[i].Permissions[c]; !ok {
					s.Members[i].Permissions[c] = true
				}
			}
		}
	}
	q, e := a.db.QueryContext(ctx, `SELECT q.id,q.position,m.id,m.provider_media_id,coalesce(m.title,''),coalesce(m.thumbnail_url,'') FROM room_queue_items q JOIN media_items m ON m.id=q.media_id WHERE q.room_id=? ORDER BY q.position`, id)
	if e != nil {
		return s, e
	}
	for q.Next() {
		var x queueItem
		if e = q.Scan(&x.ID, &x.Position, &x.Media.ID, &x.Media.ProviderID, &x.Media.Title, &x.Media.Thumbnail); e != nil {
			q.Close()
			return s, e
		}
		s.Queue = append(s.Queue, x)
	}
	if e = q.Err(); e != nil {
		q.Close()
		return s, e
	}
	q.Close()
	var mid, title, thumb, provider sql.NullString
	if e = a.db.QueryRowContext(ctx, `SELECT p.status,p.position_seconds,p.revision,p.updated_at,m.id,m.provider_media_id,m.title,m.thumbnail_url FROM playback_states p LEFT JOIN media_items m ON m.id=p.current_media_id WHERE p.room_id=?`, id).Scan(&s.Playback.Status, &s.Playback.Position, &s.Playback.Revision, &s.Playback.UpdatedAt, &mid, &provider, &title, &thumb); e != nil {
		return s, e
	}
	if mid.Valid {
		s.Playback.Media = &media{ID: mid.String, ProviderID: provider.String, Title: title.String, Thumbnail: thumb.String}
	}
	if s.Playback.Status == "playing" {
		if updated, err := time.Parse("2006-01-02 15:04:05", s.Playback.UpdatedAt); err == nil {
			s.Playback.Position += time.Since(updated.UTC()).Seconds()
		}
	}
	er, e := a.db.QueryContext(ctx, `SELECT e.id,coalesce(e.actor_identity_id,''),coalesce(i.display_name,''),e.event_type,e.payload_json,e.created_at FROM room_events e LEFT JOIN identities i ON i.id=e.actor_identity_id WHERE e.room_id=? ORDER BY e.created_at DESC LIMIT 200`, id)
	if e != nil {
		return s, e
	}
	for er.Next() {
		var x event
		var raw string
		if e = er.Scan(&x.ID, &x.ActorID, &x.ActorName, &x.Type, &raw, &x.CreatedAt); e != nil {
			er.Close()
			return s, e
		}
		if e = json.Unmarshal([]byte(raw), &x.Payload); e != nil {
			er.Close()
			return s, e
		}
		s.Events = append(s.Events, x)
	}
	if e = er.Err(); e != nil {
		er.Close()
		return s, e
	}
	er.Close()
	sort.Slice(s.Events, func(i, j int) bool { return s.Events[i].CreatedAt < s.Events[j].CreatedAt })
	return s, nil
}

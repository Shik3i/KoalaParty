package app

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	conn         *websocket.Conn
	identity     string
	sessionHash  string
	writeMu      sync.Mutex
	lastReaction time.Time
	commandCount int
	commandReset time.Time
}

func (c *client) write(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(v)
}
func (c *client) ping() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	_ = c.conn.SetWriteDeadline(deadline)
	return c.conn.WriteControl(websocket.PingMessage, nil, deadline)
}
func (c *client) allowCommand(now time.Time) bool {
	if c.commandReset.IsZero() || now.After(c.commandReset) {
		c.commandReset = now.Add(time.Minute)
		c.commandCount = 0
	}
	c.commandCount++
	return c.commandCount <= 180
}

type hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*client]struct{}
}

func newHub() *hub { return &hub{rooms: map[string]map[*client]struct{}{}} }
func (h *hub) add(room string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = map[*client]struct{}{}
	}
	h.rooms[room][c] = struct{}{}
}
func (h *hub) remove(room string, c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[room], c)
	lastForIdentity := true
	for remaining := range h.rooms[room] {
		if remaining.identity == c.identity {
			lastForIdentity = false
			break
		}
	}
	if len(h.rooms[room]) == 0 {
		delete(h.rooms, room)
	}
	return lastForIdentity
}
func (h *hub) isActive(room, identity string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[room] {
		if c.identity == identity {
			return true
		}
	}
	return false
}
func (h *hub) activeIdentities(room string) map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	active := make(map[string]bool, len(h.rooms[room]))
	for c := range h.rooms[room] {
		active[c.identity] = true
	}
	return active
}
func (h *hub) activeRoom(room string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room]) > 0
}
func (h *hub) activeCount(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	identities := map[string]struct{}{}
	for c := range h.rooms[room] {
		identities[c.identity] = struct{}{}
	}
	return len(identities)
}
func (h *hub) disconnect(room, identity string) {
	h.mu.RLock()
	clients := []*client{}
	for c := range h.rooms[room] {
		if c.identity == identity {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		_ = c.conn.Close()
	}
}
func (h *hub) disconnectRoom(room string) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		_ = c.conn.Close()
	}
}
func (h *hub) disconnectIdentity(identity string) {
	h.mu.RLock()
	clients := []*client{}
	for _, roomClients := range h.rooms {
		for c := range roomClients {
			if c.identity == identity {
				clients = append(clients, c)
			}
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		_ = c.conn.Close()
	}
}
func (h *hub) disconnectSession(sessionHash string) {
	h.mu.RLock()
	clients := []*client{}
	for _, roomClients := range h.rooms {
		for c := range roomClients {
			if c.sessionHash == sessionHash {
				clients = append(clients, c)
			}
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		_ = c.conn.Close()
	}
}
func (h *hub) disconnectSessions(sessionHashes []string) {
	for _, sessionHash := range sessionHashes {
		h.disconnectSession(sessionHash)
	}
}
func (h *hub) broadcast(room string, s snapshot) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		personalized := s
		personalized.Me = c.identity
		_ = c.write(map[string]any{"type": "snapshot", "payload": personalized})
	}
}
func (h *hub) broadcastReaction(room, identity, emoji string) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		_ = c.write(map[string]any{"type": "reaction", "identityId": identity, "emoji": emoji})
	}
}
func (a *application) websocket(w http.ResponseWriter, r *http.Request, p principal) {
	room := r.PathValue("roomId")
	if !a.originAllowed(r.Header.Get("Origin")) {
		problem(w, 403, "origin_denied", "WebSocket origin is not trusted.")
		return
	}
	s, e := a.joinAndSnapshot(r.Context(), room, p)
	if e != nil {
		roomProblem(w, e)
		return
	}
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, e := up.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	conn.SetReadLimit(64 << 10)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})
	c := &client{conn: conn, identity: p.IdentityID, sessionHash: p.SessionHash}
	a.hub.add(room, c)
	if refreshed, snapshotErr := a.snapshot(r.Context(), room, p.IdentityID); snapshotErr == nil {
		s = refreshed
	}
	a.hub.broadcast(room, s)
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-ticker.C:
				if c.ping() != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	expiryTimer := time.AfterFunc(max(time.Until(p.SessionExpires), 0), func() {
		_ = conn.Close()
	})
	// Ensure the current video's SponsorBlock segments are fetched even when it was
	// cued at room creation (never activated via play_now/skip). enrichSegments is a
	// no-op once cached, so this is cheap on repeat joins.
	if a.segments != nil && s.Playback.Media != nil {
		go a.enrichSegments(room, s.Playback.Media.ProviderID)
	}
	defer func() {
		close(pingDone)
		expiryTimer.Stop()
		lastForIdentity := a.hub.remove(room, c)
		conn.Close()
		if lastForIdentity {
			var membership int
			_ = a.db.QueryRow("SELECT count(*) FROM room_members WHERE room_id=? AND identity_id=?", room, p.IdentityID).Scan(&membership)
			if membership > 0 {
				tx, txErr := a.db.Begin()
				if txErr == nil {
					if txErr = a.insertEventTx(tx, room, p.IdentityID, "member.left", map[string]any{}); txErr == nil {
						_, txErr = tx.Exec("UPDATE rooms SET revision=revision+1 WHERE id=?", room)
					}
					if txErr == nil {
						txErr = tx.Commit()
					} else {
						_ = tx.Rollback()
					}
				}
			}
		}
		if latest, e := a.snapshot(context.Background(), room, p.IdentityID); e == nil {
			a.hub.broadcast(room, latest)
		}
	}()
	for {
		_, raw, e := conn.ReadMessage()
		if e != nil {
			return
		}
		var cmd command
		if json.Unmarshal(raw, &cmd) != nil {
			_ = c.write(map[string]any{"type": "error", "code": "invalid_json"})
			continue
		}
		current, authErr := a.principalBySessionHash(c.sessionHash)
		if authErr != nil || current.IdentityID != c.identity {
			_ = c.write(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "session_expired", "message": "Session expired or was revoked."})
			return
		}
		p = current
		if cmd.Type == "reaction.send" {
			var payload struct {
				Emoji string `json:"emoji"`
			}
			allowed := map[string]bool{"❤️": true, "😂": true, "🔥": true, "👀": true, "😴": true, "👏": true}
			if json.Unmarshal(cmd.Payload, &payload) != nil || !allowed[payload.Emoji] {
				_ = c.write(map[string]any{"type": "error", "requestId": cmd.RequestID, "message": "invalid reaction"})
				continue
			}
			if time.Since(c.lastReaction) < 750*time.Millisecond {
				continue
			}
			c.lastReaction = time.Now()
			a.hub.broadcastReaction(room, p.IdentityID, payload.Emoji)
			continue
		}
		if !c.allowCommand(time.Now()) {
			_ = c.write(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "rate_limited", "message": "Too many room commands. Try again shortly."})
			continue
		}
		s, e = a.applyCommand(r.Context(), room, p, cmd)
		if e != nil {
			_ = c.write(map[string]any{"type": "error", "requestId": cmd.RequestID, "message": e.Error()})
			continue
		}
		a.hub.broadcast(room, s)
	}
}

func (h *hub) onlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	unique := map[string]struct{}{}
	for _, clients := range h.rooms {
		for c := range clients {
			unique[c.identity] = struct{}{}
		}
	}
	return len(unique)
}

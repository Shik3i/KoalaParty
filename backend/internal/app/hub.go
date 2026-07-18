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
	conn     *websocket.Conn
	identity string
	writeMu  sync.Mutex
}

func (c *client) write(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(v)
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
	c := &client{conn: conn, identity: p.IdentityID}
	a.hub.add(room, c)
	s, _ = a.snapshot(r.Context(), room, p.IdentityID)
	a.hub.broadcast(room, s)
	defer func() {
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

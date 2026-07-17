package app

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

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
func (h *hub) remove(room string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[room], c)
	if len(h.rooms[room]) == 0 {
		delete(h.rooms, room)
	}
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
func (h *hub) broadcast(room string, s snapshot) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		_ = c.write(map[string]any{"type": "snapshot", "payload": s})
	}
}
func (a *application) websocket(w http.ResponseWriter, r *http.Request, p principal) {
	room := r.PathValue("roomId")
	s, e := a.joinAndSnapshot(r.Context(), room, p)
	if e != nil {
		roomProblem(w, e)
		return
	}
	if !a.originAllowed(r.Header.Get("Origin")) {
		problem(w, 403, "origin_denied", "WebSocket origin is not trusted.")
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
		a.hub.remove(room, c)
		conn.Close()
		_ = a.insertEvent(room, p.IdentityID, "member.left", map[string]any{})
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

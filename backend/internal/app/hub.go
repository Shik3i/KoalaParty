package app

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*websocket.Conn]struct{}
}

func newHub() *hub { return &hub{rooms: map[string]map[*websocket.Conn]struct{}{}} }
func (h *hub) add(room string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = map[*websocket.Conn]struct{}{}
	}
	h.rooms[room][c] = struct{}{}
}
func (h *hub) remove(room string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[room], c)
	if len(h.rooms[room]) == 0 {
		delete(h.rooms, room)
	}
}
func (h *hub) broadcast(room string, s snapshot) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[room] {
		_ = c.WriteJSON(map[string]any{"type": "snapshot", "payload": s})
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
	c, e := up.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	c.SetReadLimit(64 << 10)
	a.hub.add(room, c)
	defer func() {
		a.hub.remove(room, c)
		c.Close()
		_ = a.insertEvent(room, p.IdentityID, "member.left", map[string]any{})
	}()
	_ = c.WriteJSON(map[string]any{"type": "snapshot", "payload": s})
	for {
		_, raw, e := c.ReadMessage()
		if e != nil {
			return
		}
		var cmd command
		if json.Unmarshal(raw, &cmd) != nil {
			_ = c.WriteJSON(map[string]any{"type": "error", "code": "invalid_json"})
			continue
		}
		s, e = a.applyCommand(r.Context(), room, p, cmd)
		if e != nil {
			_ = c.WriteJSON(map[string]any{"type": "error", "requestId": cmd.RequestID, "message": e.Error()})
			continue
		}
		a.hub.broadcast(room, s)
	}
}

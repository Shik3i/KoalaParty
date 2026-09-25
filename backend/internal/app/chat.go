package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Room chat is deliberately ephemeral: messages live only in memory, the last
// few are replayed to people who join, and everything is forgotten as soon as
// the last viewer leaves the room or the server restarts. Nothing is written to
// the database or logs.
const (
	chatHistoryLimit  = 60
	maxChatMessageLen = 500
)

type chatMessage struct {
	ID         string `json:"id"`
	IdentityID string `json:"identityId"`
	Name       string `json:"name"`
	Text       string `json:"text"`
	At         string `json:"at"`
}

// presenceStates are the per-viewer player states other people may see, so a
// room can tell who it is waiting for.
var presenceStates = map[string]bool{"playing": true, "paused": true, "buffering": true, "blocked": true, "idle": true}

// cleanChatText keeps line breaks but drops other control characters.
func cleanChatText(raw string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw))
}

func (h *hub) appendChat(room string, message chatMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rooms[room]) == 0 {
		return
	}
	history := append(h.chats[room], message)
	if len(history) > chatHistoryLimit {
		history = append([]chatMessage(nil), history[len(history)-chatHistoryLimit:]...)
	}
	h.chats[room] = history
}

func (h *hub) chatHistory(room string) []chatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]chatMessage{}, h.chats[room]...)
}

// setPresence records a viewer's player state and reports whether it changed.
func (h *hub) setPresence(room, identity, state string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rooms[room]) == 0 {
		return false
	}
	if h.presence[room] == nil {
		h.presence[room] = map[string]string{}
	}
	if h.presence[room][identity] == state {
		return false
	}
	h.presence[room][identity] = state
	return true
}

func (h *hub) presenceStates(room string) map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	states := make(map[string]string, len(h.presence[room]))
	for identity, state := range h.presence[room] {
		states[identity] = state
	}
	return states
}

func (h *hub) roomsForIdentity(identity string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	rooms := []string{}
	for room, clients := range h.rooms {
		for c := range clients {
			if c.identity == identity {
				rooms = append(rooms, room)
				break
			}
		}
	}
	return rooms
}

func (h *hub) send(room string, message map[string]any) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		c.enqueue(message)
	}
}

func (h *hub) allowChat(identity string, now time.Time) bool {
	return h.allowIdentity(h.chatBuckets, identity, 20, 10*time.Second, now)
}

func (h *hub) allowPresence(identity string, now time.Time) bool {
	return h.allowIdentity(h.presenceBuckets, identity, 60, time.Minute, now)
}

// handleChat validates, rate-limits and fans out one chat message.
func (a *application) handleChat(ctx context.Context, c *client, room string, p principal, cmd command) {
	var payload struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(cmd.Payload, &payload) != nil {
		c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "invalid_chat", "message": "Invalid chat message."})
		return
	}
	text := cleanChatText(payload.Text)
	if text == "" {
		return
	}
	if utf8.RuneCountInString(text) > maxChatMessageLen {
		c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "invalid_chat", "message": "Chat messages can be at most 500 characters."})
		return
	}
	if _, allowed := roleAndAllowed(ctx, a.db, room, p.IdentityID, "chat.send"); !allowed {
		c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "permission_denied", "message": "You are muted in this room."})
		return
	}
	now := time.Now()
	if now.Sub(c.lastChat) < 300*time.Millisecond || !a.hub.allowChat(p.IdentityID, now) {
		c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "rate_limited", "message": "You're sending messages too quickly."})
		return
	}
	c.lastChat = now
	var name string
	if a.db.QueryRowContext(ctx, "SELECT display_name FROM identities WHERE id=?", p.IdentityID).Scan(&name) != nil {
		name = p.DisplayName
	}
	message := chatMessage{ID: newID(8), IdentityID: p.IdentityID, Name: name, Text: text, At: now.UTC().Format(time.RFC3339)}
	a.hub.appendChat(room, message)
	a.hub.send(room, map[string]any{"type": "chat", "message": message})
}

// handlePresence relays a viewer's player state when it changes.
func (a *application) handlePresence(c *client, room string, p principal, cmd command) {
	var payload struct {
		State string `json:"state"`
	}
	if json.Unmarshal(cmd.Payload, &payload) != nil || !presenceStates[payload.State] {
		return
	}
	if !a.hub.allowPresence(p.IdentityID, time.Now()) {
		return
	}
	if a.hub.setPresence(room, p.IdentityID, payload.State) {
		a.hub.send(room, map[string]any{"type": "presence", "identityId": p.IdentityID, "state": payload.State})
	}
}

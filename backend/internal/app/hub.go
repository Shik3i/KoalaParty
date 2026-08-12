package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	conn         *websocket.Conn
	identity     string
	sessionHash  string
	remoteIP     string
	send         chan any
	done         chan struct{}
	closeOnce    sync.Once
	lastReaction time.Time
	commandCount int
	commandReset time.Time
	logger       *slog.Logger
	metrics      *runtimeMetrics
	roomHash     string
}

func (c *client) enqueue(v any) bool {
	select {
	case <-c.done:
		return false
	case c.send <- v:
		return true
	default:
		if c.metrics != nil {
			c.metrics.websocketQueueFull.Add(1)
		}
		if c.logger != nil {
			c.logger.Warn("websocket send queue full", "room_hash", c.roomHash, "identity_hash", shortHash(c.identity))
		}
		c.shutdown()
		return false
	}
}
func (c *client) shutdown() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}
func (c *client) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteJSON(message); err != nil {
				if c.logger != nil {
					c.logger.Warn("websocket write failed", "room_hash", c.roomHash, "identity_hash", shortHash(c.identity), "error", err.Error())
				}
				c.shutdown()
				return
			}
		case <-ticker.C:
			deadline := time.Now().Add(5 * time.Second)
			_ = c.conn.SetWriteDeadline(deadline)
			if err := c.conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				if c.logger != nil {
					c.logger.Warn("websocket ping failed", "room_hash", c.roomHash, "identity_hash", shortHash(c.identity), "error", err.Error())
				}
				c.shutdown()
				return
			}
		}
	}
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
	mu              sync.RWMutex
	rooms           map[string]map[*client]struct{}
	commandBuckets  map[string]rateBucket
	reactionBuckets map[string]rateBucket
	metrics         *runtimeMetrics
}

type rateBucket struct {
	count int
	reset time.Time
}

const (
	maxRoomIdentities         = maxRoomMembers
	maxConnectionsPerIdentity = 3
	maxConnectionsPerSession  = 3
	maxConnectionsPerIP       = 12
	maxTotalConnections       = 5000
)

func newHub(metrics ...*runtimeMetrics) *hub {
	var runtime *runtimeMetrics
	if len(metrics) > 0 {
		runtime = metrics[0]
	}
	return &hub{
		rooms:           map[string]map[*client]struct{}{},
		commandBuckets:  map[string]rateBucket{},
		reactionBuckets: map[string]rateBucket{},
		metrics:         runtime,
	}
}
func (h *hub) tryAdd(room string, c *client) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	identities := map[string]struct{}{}
	identityConnections, sessionConnections, ipConnections, totalConnections := 0, 0, 0, 0
	for _, clients := range h.rooms {
		for existing := range clients {
			totalConnections++
			if existing.remoteIP == c.remoteIP {
				ipConnections++
			}
		}
	}
	for existing := range h.rooms[room] {
		identities[existing.identity] = struct{}{}
		if existing.identity == c.identity {
			identityConnections++
		}
		if existing.sessionHash == c.sessionHash {
			sessionConnections++
		}
	}
	if _, active := identities[c.identity]; !active && len(identities) >= maxRoomIdentities {
		return errors.New("room_full")
	}
	if identityConnections >= maxConnectionsPerIdentity || sessionConnections >= maxConnectionsPerSession || ipConnections >= maxConnectionsPerIP || totalConnections >= maxTotalConnections {
		return errors.New("connection_limit")
	}
	if h.rooms[room] == nil {
		h.rooms[room] = map[*client]struct{}{}
	}
	h.rooms[room][c] = struct{}{}
	return nil
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
		c.shutdown()
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
		c.shutdown()
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
		c.shutdown()
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
		c.shutdown()
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
		c.enqueue(map[string]any{"type": "snapshot", "payload": personalized})
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
		c.enqueue(map[string]any{"type": "reaction", "identityId": identity, "emoji": emoji})
	}
}
func (h *hub) allowIdentity(bucketMap map[string]rateBucket, identity string, limit int, window time.Duration, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	bucket := bucketMap[identity]
	if bucket.reset.IsZero() || now.After(bucket.reset) {
		bucket = rateBucket{reset: now.Add(window)}
	}
	bucket.count++
	bucketMap[identity] = bucket
	if len(bucketMap) > 10000 {
		for key, candidate := range bucketMap {
			if now.After(candidate.reset) {
				delete(bucketMap, key)
			}
		}
		for key := range bucketMap {
			if len(bucketMap) <= 10000 {
				break
			}
			if key != identity {
				delete(bucketMap, key)
			}
		}
	}
	return bucket.count <= limit
}
func (h *hub) allowCommand(identity string, now time.Time) bool {
	return h.allowIdentity(h.commandBuckets, identity, 180, time.Minute, now)
}
func (h *hub) allowReaction(identity string, now time.Time) bool {
	return h.allowIdentity(h.reactionBuckets, identity, 80, time.Minute, now)
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
	c := &client{conn: conn, identity: p.IdentityID, sessionHash: p.SessionHash, remoteIP: clientIP(r, a.trustedProxies), send: make(chan any, 32), done: make(chan struct{}), logger: a.logger, metrics: a.metrics, roomHash: shortHash(room)}
	if e = a.hub.tryAdd(room, c); e != nil {
		if a.logger != nil {
			a.logger.Warn("websocket connection rejected", "room_hash", shortHash(room), "identity_hash", shortHash(p.IdentityID), "reason", e.Error())
		}
		code, message := websocket.CloseTryAgainLater, "Connection limit reached."
		if e.Error() == "room_full" {
			code, message = websocket.ClosePolicyViolation, "Room participant limit reached."
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, message), time.Now().Add(time.Second))
		_ = conn.Close()
		return
	}
	if a.metrics != nil {
		a.metrics.websocketOpened.Add(1)
	}
	if a.logger != nil {
		a.logger.Info("websocket connected", "room_hash", shortHash(room), "identity_hash", shortHash(p.IdentityID))
	}
	go c.writePump()
	if refreshed, snapshotErr := a.snapshot(r.Context(), room, p.IdentityID); snapshotErr == nil {
		s = refreshed
	}
	a.hub.broadcast(room, s)
	expiryTimer := time.AfterFunc(max(time.Until(p.SessionExpires), 0), func() {
		c.shutdown()
	})
	// Ensure the current video's SponsorBlock segments are fetched even when it was
	// cued at room creation (never activated via play_now/skip). enrichSegments is a
	// no-op once cached, so this is cheap on repeat joins.
	if a.segments != nil && s.Playback.Media != nil {
		go a.enrichSegments(room, s.Playback.Media.ProviderID)
	}
	defer func() {
		expiryTimer.Stop()
		if a.metrics != nil {
			a.metrics.websocketClosed.Add(1)
		}
		if a.logger != nil {
			a.logger.Info("websocket disconnected", "room_hash", shortHash(room), "identity_hash", shortHash(p.IdentityID))
		}
		lastForIdentity := a.hub.remove(room, c)
		c.shutdown()
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
			c.enqueue(map[string]any{"type": "error", "code": "invalid_json", "message": "Invalid command JSON."})
			continue
		}
		current, authErr := a.principalBySessionHash(c.sessionHash)
		if authErr != nil || current.IdentityID != c.identity {
			c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "session_expired", "message": "Session expired or was revoked."})
			return
		}
		p = current
		if cmd.Type == "reaction.send" {
			var payload struct {
				Emoji string `json:"emoji"`
			}
			allowed := map[string]bool{"❤️": true, "😂": true, "🔥": true, "👀": true, "😴": true, "👏": true}
			if json.Unmarshal(cmd.Payload, &payload) != nil || !allowed[payload.Emoji] {
				c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "invalid_reaction", "message": "Invalid reaction."})
				continue
			}
			if time.Since(c.lastReaction) < 750*time.Millisecond || !a.hub.allowReaction(p.IdentityID, time.Now()) {
				continue
			}
			c.lastReaction = time.Now()
			a.hub.broadcastReaction(room, p.IdentityID, payload.Emoji)
			continue
		}
		if !c.allowCommand(time.Now()) || !a.hub.allowCommand(p.IdentityID, time.Now()) {
			c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": "rate_limited", "message": "Too many room commands. Try again shortly."})
			if a.metrics != nil {
				a.metrics.commandsRejected.Add(1)
			}
			a.logCommand(r.Context(), room, p, cmd, "rejected", "rate_limited")
			continue
		}
		s, e = a.applyCommand(r.Context(), room, p, cmd)
		if e != nil {
			code := commandErrorCode(e)
			message := "Room command failed."
			if code == "stale_revision" {
				message = "Room state changed; use the latest snapshot."
			} else if code == "permission_denied" {
				message = "The server denied this room action."
			} else if code == "request_id_conflict" {
				message = "Request ID was already used for another command."
			}
			c.enqueue(map[string]any{"type": "error", "requestId": cmd.RequestID, "code": code, "message": message})
			if a.metrics != nil {
				a.metrics.commandsRejected.Add(1)
			}
			a.logCommand(r.Context(), room, p, cmd, "rejected", code)
			continue
		}
		if a.metrics != nil {
			a.metrics.commandsAccepted.Add(1)
		}
		a.logCommand(r.Context(), room, p, cmd, "accepted", "")
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

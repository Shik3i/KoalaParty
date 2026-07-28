package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSessionBootstrapRejectsCrossOriginAndNonJSON(t *testing.T) {
	a := testApp(t)
	handler := a.sessionBootstrap(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	tests := []struct {
		name        string
		contentType string
		origin      string
		fetchSite   string
		want        int
	}{
		{name: "non-json", contentType: "text/plain", want: http.StatusUnsupportedMediaType},
		{name: "foreign-origin", contentType: "application/json", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "cross-site-metadata", contentType: "application/json", fetchSite: "cross-site", want: http.StatusForbidden},
		{name: "trusted-origin", contentType: "application/json; charset=utf-8", origin: "http://example.test", want: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/identity/exchange", strings.NewReader(`{}`))
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), test.want)
			}
		})
	}
}

func TestLinkedIdentitySecretCannotRecreateAccountSession(t *testing.T) {
	a := testApp(t)
	id := "123e4567-e89b-42d3-a456-426614174099"
	secret := strings.Repeat("z", 43)
	cookie, p := exchange(t, a, id, secret)
	response := httptest.NewRecorder()
	a.requireAuth(a.register)(response, authed(http.MethodPost, "/api/accounts/register", credentials{
		Username: "linked_user",
		Password: "correct horse battery",
	}, cookie, p.CSRF))
	if response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.db.Exec("DELETE FROM sessions WHERE identity_id=?", id); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(identityRequest{ID: id, Secret: secret, DisplayName: "Calm Koala"})
	request := httptest.NewRequest(http.MethodPost, "/api/identity/exchange", bytes.NewReader(body))
	response = httptest.NewRecorder()
	a.exchangeIdentity(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "account_login_required") {
		t.Fatalf("linked secret recreated session: %d %s", response.Code, response.Body.String())
	}
	var sessions int
	if err := a.db.QueryRow("SELECT count(*) FROM sessions WHERE identity_id=?", id).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("linked identity has %d sessions after rejected exchange", sessions)
	}
}

func TestRoomPreviewAdvancesAtPlaybackRate(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174098", strings.Repeat("y", 43))
	room := createTestRoom(t, a, cookie, owner)
	if _, err := a.db.Exec(`UPDATE playback_states
		SET status='playing',position_seconds=5,playback_rate=2,updated_at=datetime('now','-10 seconds')
		WHERE room_id=?`, room); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	a.requireAuth(a.roomPreviews)(response, authed(http.MethodPost, "/api/rooms/previews", map[string]any{"ids": []string{room}}, cookie, owner.CSRF))
	if response.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", response.Code, response.Body.String())
	}
	var previews []struct {
		Position float64 `json:"position"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &previews); err != nil || len(previews) != 1 {
		t.Fatalf("decode previews: %v body=%s", err, response.Body.String())
	}
	if previews[0].Position < 24 || previews[0].Position > 30 {
		t.Fatalf("2x preview position=%f, want approximately 25", previews[0].Position)
	}
}

func testClient(identity, session, ip string) *client {
	return &client{
		identity:    identity,
		sessionHash: session,
		remoteIP:    ip,
		send:        make(chan any, 1),
		done:        make(chan struct{}),
	}
}

func TestHubEnforcesActiveRoomAndConnectionLimits(t *testing.T) {
	h := newHub()
	for index := 0; index < maxRoomIdentities; index++ {
		c := testClient(fmt.Sprintf("identity-%d", index), fmt.Sprintf("session-%d", index), fmt.Sprintf("192.0.2.%d", index+1))
		if err := h.tryAdd("room", c); err != nil {
			t.Fatalf("identity %d rejected early: %v", index, err)
		}
	}
	if err := h.tryAdd("room", testClient("overflow", "overflow", "198.51.100.1")); err == nil || err.Error() != "room_full" {
		t.Fatalf("room capacity result=%v, want room_full", err)
	}

	limited := newHub()
	for index := 0; index < maxConnectionsPerIdentity; index++ {
		if err := limited.tryAdd("room", testClient("same", fmt.Sprintf("session-%d", index), fmt.Sprintf("203.0.113.%d", index+1))); err != nil {
			t.Fatalf("connection %d rejected early: %v", index, err)
		}
	}
	if err := limited.tryAdd("room", testClient("same", "session-extra", "203.0.113.20")); err == nil || err.Error() != "connection_limit" {
		t.Fatalf("identity connection limit result=%v", err)
	}
}

func TestHubAggregateCommandLimitAndSlowClientDrop(t *testing.T) {
	h := newHub()
	now := time.Now()
	for index := 0; index < 180; index++ {
		if !h.allowCommand("identity", now) {
			t.Fatalf("aggregate command %d rejected early", index+1)
		}
	}
	if h.allowCommand("identity", now) {
		t.Fatal("aggregate command limit accepted command 181")
	}

	slow := testClient("slow", "slow-session", "192.0.2.1")
	if err := h.tryAdd("room", slow); err != nil {
		t.Fatal(err)
	}
	h.broadcast("room", snapshot{})
	h.broadcast("room", snapshot{})
	select {
	case <-slow.done:
	default:
		t.Fatal("slow client with a full write queue was not disconnected")
	}
}

func TestConcurrentAdminSettingsRemainConsistent(t *testing.T) {
	a := testApp(t)
	a.settingOverrides = map[string]bool{}
	type settings struct {
		SessionTTL        string `json:"sessionTTL"`
		ActivityMaxAge    string `json:"activityMaxAge"`
		ActivityMaxEvents int    `json:"activityMaxEvents"`
		RoomMaxIdle       string `json:"roomMaxIdle"`
		PublicRooms       bool   `json:"publicRooms"`
	}
	inputs := []settings{
		{SessionTTL: "24h", ActivityMaxAge: "48h", ActivityMaxEvents: 100, RoomMaxIdle: "72h", PublicRooms: false},
		{SessionTTL: "36h", ActivityMaxAge: "60h", ActivityMaxEvents: 120, RoomMaxIdle: "84h", PublicRooms: true},
	}
	var wait sync.WaitGroup
	for _, input := range inputs {
		wait.Add(1)
		go func() {
			defer wait.Done()
			body, _ := json.Marshal(input)
			request := httptest.NewRequest(http.MethodPost, "/api/admin/settings", bytes.NewReader(body))
			response := httptest.NewRecorder()
			a.adminSettings(response, request, principal{})
			if response.Code != http.StatusNoContent {
				t.Errorf("save settings: %d %s", response.Code, response.Body.String())
			}
		}()
	}
	wait.Wait()
	var persisted string
	if err := a.db.QueryRow("SELECT value FROM settings WHERE key='session_ttl'").Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	persistedDuration, err := time.ParseDuration(persisted)
	if err != nil {
		t.Fatal(err)
	}
	if persistedDuration != a.getSessionTTL() {
		t.Fatalf("settings diverged: database=%q runtime=%q", persisted, a.getSessionTTL())
	}
}

func TestStaticCachePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "_app", "immutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("home"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "_app", "immutable", "app.hash.js"), []byte("js"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := spaHandler(root)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/_app/immutable/app.hash.js", nil))
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("immutable cache policy=%q", got)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := response.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("HTML cache policy=%q", got)
	}
}

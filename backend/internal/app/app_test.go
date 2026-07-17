package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Shik3i/KoalaParty/backend/internal/database"
)

func testApp(t *testing.T) *application {
	t.Helper()
	db, e := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return &application{db: db, hub: newHub(), sessionTTL: time.Hour, trustedOrigins: map[string]bool{"http://example.test": true}}
}
func exchange(t *testing.T, a *application, id, secret string) (*http.Cookie, principal) {
	t.Helper()
	body, _ := json.Marshal(identityRequest{ID: id, Secret: secret, DisplayName: "Calm Koala"})
	r := httptest.NewRequest("POST", "/api/identity/exchange", bytes.NewReader(body))
	w := httptest.NewRecorder()
	a.exchangeIdentity(w, r)
	if w.Code != 200 {
		t.Fatalf("exchange: %d %s", w.Code, w.Body.String())
	}
	var p principal
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	return w.Result().Cookies()[0], p
}
func authed(method, path string, body any, c *http.Cookie, csrf string) *http.Request {
	var b bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	r.AddCookie(c)
	if method != "GET" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	return r
}

func TestIdentityCreationAuthenticationAndRejection(t *testing.T) {
	a := testApp(t)
	id := "123e4567-e89b-42d3-a456-426614174000"
	secret := strings.Repeat("s", 43)
	cookie, p := exchange(t, a, id, secret)
	if p.IdentityID != id || cookie.HttpOnly == false {
		t.Fatal("identity session properties missing")
	}
	var stored string
	_ = a.db.QueryRow("SELECT secret_hash FROM identities WHERE id=?", id).Scan(&stored)
	if stored == secret || !strings.HasPrefix(stored, "$argon2id$") {
		t.Fatal("identity secret was not securely hashed")
	}
	if _, restored := exchange(t, a, id, secret); restored.IdentityID != id {
		t.Fatal("valid identity secret did not restore identity")
	}
	body, _ := json.Marshal(identityRequest{ID: id, Secret: strings.Repeat("x", 43), DisplayName: "Calm Koala"})
	w := httptest.NewRecorder()
	a.exchangeIdentity(w, httptest.NewRequest("POST", "/api/identity/exchange", bytes.NewReader(body)))
	if w.Code != 401 {
		t.Fatalf("invalid secret accepted: %d", w.Code)
	}
}
func TestRoomPersistenceAndOwnerProtection(t *testing.T) {
	a := testApp(t)
	ownerCookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174001", strings.Repeat("a", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, ownerCookie, owner.CSRF))
	if w.Code != 201 {
		t.Fatalf("create room: %d %s", w.Code, w.Body.String())
	}
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	memberCookie, memberP := exchange(t, a, "123e4567-e89b-42d3-a456-426614174002", strings.Repeat("b", 43))
	s, e := a.joinAndSnapshot(t.Context(), created["id"], memberP)
	if e != nil || len(s.Members) != 2 {
		t.Fatalf("join failed: %v", e)
	}
	for _, candidate := range s.Members {
		if candidate.IdentityID == memberP.IdentityID && !candidate.Permissions["playback.play_pause"] {
			t.Fatal("default member playback permission missing from snapshot")
		}
	}
	cmd := command{Type: "member.ban", Payload: json.RawMessage(`{"identityId":"` + owner.IdentityID + `"}`)}
	if _, e = a.applyCommand(t.Context(), created["id"], memberP, cmd); e != errDenied {
		t.Fatalf("member moderation should be denied: %v", e)
	}
	_ = memberCookie
	cmd = command{Type: "player.play", ExpectedRevision: 0, Payload: json.RawMessage(`{"position":12}`)}
	s, e = a.applyCommand(t.Context(), created["id"], memberP, cmd)
	if e != nil || s.Playback.Status != "playing" || s.Playback.Revision != 1 {
		t.Fatalf("default playback permission failed: %v", e)
	}
	if _, e = a.applyCommand(t.Context(), created["id"], memberP, cmd); e != errStale {
		t.Fatalf("stale command accepted: %v", e)
	}
	override := command{Type: "member.permission", Payload: json.RawMessage(`{"identityId":"` + memberP.IdentityID + `","permission":"playback.play_pause","allowed":false}`)}
	s, e = a.applyCommand(t.Context(), created["id"], owner, override)
	if e != nil {
		t.Fatal(e)
	}
	for _, candidate := range s.Members {
		if candidate.IdentityID == memberP.IdentityID && candidate.Permissions["playback.play_pause"] {
			t.Fatal("permission override missing from snapshot")
		}
	}
}
func TestRegistrationLinksExistingIdentity(t *testing.T) {
	a := testApp(t)
	cookie, p := exchange(t, a, "123e4567-e89b-42d3-a456-426614174003", strings.Repeat("c", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.register)(w, authed("POST", "/api/accounts/register", credentials{Username: "forest_friend", Password: "very-long-test-password"}, cookie, p.CSRF))
	if w.Code != 201 {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	var account string
	if e := a.db.QueryRow("SELECT account_id FROM identities WHERE id=?", p.IdentityID).Scan(&account); e != nil || account == "" {
		t.Fatal("identity was not linked")
	}
	loginBody, _ := json.Marshal(credentials{Username: "forest_friend", Password: "very-long-test-password"})
	loginResponse := httptest.NewRecorder()
	a.login(loginResponse, httptest.NewRequest("POST", "/api/accounts/login", bytes.NewReader(loginBody)))
	if loginResponse.Code != 200 {
		t.Fatalf("login failed: %d %s", loginResponse.Code, loginResponse.Body.String())
	}
}

func TestActivityRetentionAndRoomCleanup(t *testing.T) {
	a := testApp(t)
	a.activityMaxAge = 30 * 24 * time.Hour
	a.activityMaxEvents = 10
	a.roomMaxIdle = time.Hour
	cookie, p := exchange(t, a, "123e4567-e89b-42d3-a456-426614174004", strings.Repeat("d", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, cookie, p.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	for i := 0; i < 15; i++ {
		if e := a.insertEvent(created["id"], p.IdentityID, "queue.reordered", map[string]any{"i": i}); e != nil {
			t.Fatal(e)
		}
	}
	a.runMaintenance(t.Context())
	var count int
	_ = a.db.QueryRow("SELECT count(*) FROM room_events WHERE room_id=?", created["id"]).Scan(&count)
	if count != 10 {
		t.Fatalf("retained %d events, want 10", count)
	}
	_, _ = a.db.Exec("UPDATE rooms SET last_active_at=datetime('now','-2 hours') WHERE id=?", created["id"])
	a.runMaintenance(t.Context())
	var deleted sql.NullString
	_ = a.db.QueryRow("SELECT deleted_at FROM rooms WHERE id=?", created["id"]).Scan(&deleted)
	if !deleted.Valid {
		t.Fatal("abandoned room was not soft deleted")
	}
}

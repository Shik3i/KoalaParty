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

func TestIdentityDisplayNameCountsUnicodeCharacters(t *testing.T) {
	a := testApp(t)
	body, _ := json.Marshal(identityRequest{
		ID:          "123e4567-e89b-42d3-a456-426614174010",
		Secret:      strings.Repeat("u", 43),
		DisplayName: strings.Repeat("ä", 32),
	})
	w := httptest.NewRecorder()
	a.exchangeIdentity(w, httptest.NewRequest("POST", "/api/identity/exchange", bytes.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("32-character Unicode display name rejected: %d %s", w.Code, w.Body.String())
	}
}

func TestIdentityExchangeReusesMatchingSession(t *testing.T) {
	a := testApp(t)
	id := "123e4567-e89b-42d3-a456-426614174005"
	secret := strings.Repeat("e", 43)
	cookie, first := exchange(t, a, id, secret)
	body, _ := json.Marshal(identityRequest{ID: id, Secret: secret, DisplayName: "Calm Koala"})
	r := httptest.NewRequest("POST", "/api/identity/exchange", bytes.NewReader(body))
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	a.exchangeIdentity(w, r)
	if w.Code != 200 {
		t.Fatalf("repeat exchange: %d %s", w.Code, w.Body.String())
	}
	var second principal
	_ = json.Unmarshal(w.Body.Bytes(), &second)
	if second.CSRF != first.CSRF || len(w.Result().Cookies()) != 0 {
		t.Fatal("matching session was rotated")
	}
	var sessions int
	_ = a.db.QueryRow("SELECT count(*) FROM sessions WHERE identity_id=?", id).Scan(&sessions)
	if sessions != 1 {
		t.Fatalf("found %d sessions, want 1", sessions)
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
	cmd := command{Type: "member.ban", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"identityId":"` + owner.IdentityID + `"}`)}
	if _, e = a.applyCommand(t.Context(), created["id"], memberP, cmd); e != errDenied {
		t.Fatalf("member moderation should be denied: %v", e)
	}
	_ = memberCookie
	cmd = command{Type: "player.play", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"position":12}`)}
	s, e = a.applyCommand(t.Context(), created["id"], memberP, cmd)
	if e != nil || s.Playback.Status != "playing" || s.Playback.Revision != 1 {
		t.Fatalf("default playback permission failed: %v", e)
	}
	if _, e = a.applyCommand(t.Context(), created["id"], memberP, cmd); e != errStale {
		t.Fatalf("stale command accepted: %v", e)
	}
	override := command{Type: "member.permission", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"identityId":"` + memberP.IdentityID + `","permission":"playback.play_pause","allowed":false}`)}
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

func TestQueueTitleCannotOverwriteSharedMediaMetadata(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174011", strings.Repeat("m", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, cookie, owner.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	s, err := a.snapshot(t.Context(), created["id"], owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	videoID := "abc12345678"
	add := command{Type: "queue.add", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"videoId":"` + videoID + `","title":"Untrusted title"}`)}
	s, err = a.applyCommand(t.Context(), created["id"], owner, add)
	if err != nil {
		t.Fatal(err)
	}
	var title string
	if err = a.db.QueryRow("SELECT title FROM media_items WHERE id=?", "YT"+videoID).Scan(&title); err != nil || title != "YouTube video "+videoID {
		t.Fatalf("stored client-supplied title %q: %v", title, err)
	}

	if _, err = a.db.Exec("UPDATE media_items SET title='Trusted title' WHERE id=?", "YT"+videoID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.db.Exec("DELETE FROM room_queue_items WHERE room_id=?", created["id"]); err != nil {
		t.Fatal(err)
	}
	add.ExpectedRevision = s.Revision
	add.Payload = json.RawMessage(`{"videoId":"` + videoID + `","title":"Overwrite attempt"}`)
	if _, err = a.applyCommand(t.Context(), created["id"], owner, add); err != nil {
		t.Fatal(err)
	}
	if err = a.db.QueryRow("SELECT title FROM media_items WHERE id=?", "YT"+videoID).Scan(&title); err != nil || title != "Trusted title" {
		t.Fatalf("shared trusted title overwritten with %q: %v", title, err)
	}
}

func TestQueuePolishCommands(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174014", strings.Repeat("q", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, cookie, owner.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	s, err := a.snapshot(t.Context(), created["id"], owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(kind, payload string) {
		t.Helper()
		s, err = a.applyCommand(t.Context(), created["id"], owner, command{Type: kind, ExpectedRevision: s.Revision, Payload: json.RawMessage(payload)})
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	apply("queue.add", `{"videoId":"abc12345678"}`)
	apply("queue.add", `{"videoId":"def12345678"}`)
	if _, duplicateErr := a.applyCommand(t.Context(), created["id"], owner, command{Type: "queue.add", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"videoId":"abc12345678"}`)}); duplicateErr == nil {
		t.Fatal("duplicate queue item accepted")
	}
	apply("queue.shuffle", `{}`)
	var votedID string
	for _, item := range s.Queue {
		if item.Media.ProviderID == "def12345678" {
			votedID = item.ID
		}
	}
	apply("queue.vote", `{"itemId":"`+votedID+`"}`)
	if s.Queue[0].ID != votedID || s.Queue[0].Votes != 1 || !s.Queue[0].Voted {
		t.Fatalf("vote did not prioritize item: %+v", s.Queue)
	}
	apply("queue.loop", `{"enabled":true}`)
	apply("queue.skip", `{}`)
	if !s.QueueLoop || s.Playback.Media == nil || s.Playback.Media.ProviderID != "def12345678" || len(s.History) == 0 {
		t.Fatalf("loop, voted skip, or history missing: %+v", s)
	}
}

func TestSponsorBlockRoomToggle(t *testing.T) {
	a := testApp(t)
	ownerCookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174031", strings.Repeat("s", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, ownerCookie, owner.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	s, err := a.snapshot(t.Context(), created["id"], owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SponsorBlock {
		t.Fatal("expected SponsorBlock enabled by default")
	}
	// Segments must serialize as an array, never null, or the client's filter crashes.
	if s.Playback.Segments == nil {
		t.Fatal("expected playback.segments to be a non-nil array")
	}
	if body, _ := json.Marshal(s.Playback); !strings.Contains(string(body), `"segments":[`) {
		t.Fatalf("segments should marshal as an array: %s", body)
	}

	// A plain member must not be able to change a room-level setting.
	_, memberP := exchange(t, a, "123e4567-e89b-42d3-a456-426614174032", strings.Repeat("t", 43))
	if _, e := a.joinAndSnapshot(t.Context(), created["id"], memberP); e != nil {
		t.Fatal(e)
	}
	deny := command{Type: "room.sponsorblock", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"enabled":false}`)}
	if _, e := a.applyCommand(t.Context(), created["id"], memberP, deny); e != errDenied {
		t.Fatalf("member should not toggle SponsorBlock: %v", e)
	}

	s, err = a.snapshot(t.Context(), created["id"], owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	off := command{Type: "room.sponsorblock", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"enabled":false}`)}
	if s, err = a.applyCommand(t.Context(), created["id"], owner, off); err != nil {
		t.Fatalf("owner toggle off: %v", err)
	}
	if s.SponsorBlock {
		t.Fatal("expected SponsorBlock disabled after toggle")
	}
}

func TestRoomPreviewsDoNotJoinUnassociatedRooms(t *testing.T) {
	a := testApp(t)
	ownerCookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174015", strings.Repeat("r", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, ownerCookie, owner.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	preview := httptest.NewRecorder()
	a.requireAuth(a.roomPreviews)(preview, authed("POST", "/api/rooms/previews", map[string]any{"ids": []string{created["id"]}}, ownerCookie, owner.CSRF))
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), created["id"]) {
		t.Fatalf("owner preview missing: %d %s", preview.Code, preview.Body.String())
	}

	outsiderCookie, outsider := exchange(t, a, "123e4567-e89b-42d3-a456-426614174016", strings.Repeat("s", 43))
	preview = httptest.NewRecorder()
	a.requireAuth(a.roomPreviews)(preview, authed("POST", "/api/rooms/previews", map[string]any{"ids": []string{created["id"]}}, outsiderCookie, outsider.CSRF))
	if preview.Code != http.StatusOK || strings.TrimSpace(preview.Body.String()) != "[]" {
		t.Fatalf("unassociated room leaked: %d %s", preview.Code, preview.Body.String())
	}
	var memberships int
	_ = a.db.QueryRow("SELECT count(*) FROM room_members WHERE room_id=? AND identity_id=?", created["id"], outsider.IdentityID).Scan(&memberships)
	if memberships != 0 {
		t.Fatal("preview joined outsider to room")
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

func TestRepeatedSnapshotDoesNotDuplicateJoinEvent(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174006", strings.Repeat("f", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, cookie, owner.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	_, member := exchange(t, a, "123e4567-e89b-42d3-a456-426614174007", strings.Repeat("g", 43))
	first, e := a.joinAndSnapshot(t.Context(), created["id"], member)
	if e != nil {
		t.Fatal(e)
	}
	second, e := a.joinAndSnapshot(t.Context(), created["id"], member)
	if e != nil {
		t.Fatal(e)
	}
	if second.Revision != first.Revision {
		t.Fatalf("repeat snapshot changed revision: first=%d second=%d", first.Revision, second.Revision)
	}
	var joins int
	if e = a.db.QueryRow("SELECT count(*) FROM room_events WHERE room_id=? AND actor_identity_id=? AND event_type='member.joined'", created["id"], member.IdentityID).Scan(&joins); e != nil || joins != 1 {
		t.Fatalf("found %d join events, want 1: %v", joins, e)
	}
}

func TestSecondRegistrationRollsBackOrphanAccount(t *testing.T) {
	a := testApp(t)
	cookie, p := exchange(t, a, "123e4567-e89b-42d3-a456-426614174008", strings.Repeat("h", 43))
	first := httptest.NewRecorder()
	a.register(first, authed("POST", "/api/accounts/register", credentials{Username: "first_account", Password: "very-long-test-password"}, cookie, p.CSRF), p)
	if first.Code != 201 {
		t.Fatalf("first registration: %d %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	a.register(second, authed("POST", "/api/accounts/register", credentials{Username: "orphan_account", Password: "very-long-test-password"}, cookie, p.CSRF), p)
	if second.Code != 409 {
		t.Fatalf("second registration: %d %s", second.Code, second.Body.String())
	}
	var accounts int
	_ = a.db.QueryRow("SELECT count(*) FROM accounts").Scan(&accounts)
	if accounts != 1 {
		t.Fatalf("found %d accounts, want 1", accounts)
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

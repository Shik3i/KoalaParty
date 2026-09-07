package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type auditBeforeRead struct {
	io.Reader
	before func()
}

func (r *auditBeforeRead) Read(p []byte) (int, error) {
	if r.before != nil {
		r.before()
		r.before = nil
	}
	return r.Reader.Read(p)
}

func TestInvitationAuthorizationAfterSlowRequestBody(t *testing.T) {
	a := testApp(t)
	cookie, owner, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174089", "slow_owner")
	_, manager, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174090", "slow_manager")
	_, target, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174091", "slow_target")
	room := createTestRoom(t, a, cookie, owner)
	if _, err := a.joinAndSnapshot(t.Context(), room, manager); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec("UPDATE room_members SET role='admin' WHERE room_id=? AND identity_id=?", room, manager.IdentityID); err != nil {
		t.Fatal(err)
	}
	body := &auditBeforeRead{Reader: strings.NewReader(`{"username":"slow_target"}`), before: func() {
		if _, err := a.db.Exec("UPDATE room_members SET role='member' WHERE room_id=? AND identity_id=?", room, manager.IdentityID); err != nil {
			t.Fatal(err)
		}
	}}
	r := httptest.NewRequest("POST", "/api/rooms/"+room+"/invites", body)
	r.SetPathValue("roomId", room)
	w := httptest.NewRecorder()
	a.roomInvites(w, r, manager)
	if w.Code != http.StatusForbidden {
		t.Errorf("demoted manager invitation: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := a.db.QueryRow("SELECT count(*) FROM room_invites WHERE room_id=? AND account_id=?", room, target.AccountID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("demoted manager inserted invitation: count=%d err=%v", count, err)
	}
}

func TestRestrictedRoomRevokesExistingMemberAccess(t *testing.T) {
	a := testApp(t)
	cookie, owner, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174081", "audit_owner")
	_, viewer, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174082", "audit_viewer")
	room := createTestRoom(t, a, cookie, owner)
	s, err := a.joinAndSnapshot(t.Context(), room, viewer)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{identity: viewer.IdentityID, send: make(chan any, 32), done: make(chan struct{})}
	if err = a.hub.tryAdd(room, c); err != nil {
		t.Fatal(err)
	}
	s, err = a.applyCommand(t.Context(), room, owner, command{Type: "room.visibility", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"visibility":"private"}`)})
	if err != nil {
		t.Fatal(err)
	}
	a.hub.broadcast(room, s)
	select {
	case <-c.done:
	default:
		t.Error("existing uninvited socket remained connected after private visibility")
	}
	_, err = a.applyCommand(t.Context(), room, viewer, command{Type: "player.play", ExpectedRevision: s.Revision, ExpectedPlaybackRevision: &s.Playback.Revision, Payload: json.RawMessage(`{"position":0}`)})
	if !errors.Is(err, errDenied) {
		t.Errorf("uninvited existing member command: %v", err)
	}
}

func TestRevokedInvitationDisconnectsExistingViewer(t *testing.T) {
	a := testApp(t)
	cookie, owner, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174083", "invite_owner")
	_, viewer, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174084", "invite_viewer")
	room := createTestRoom(t, a, cookie, owner)
	if _, err := a.db.Exec("UPDATE rooms SET visibility='private' WHERE id=?", room); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec("INSERT INTO room_invites(id,room_id,account_id,created_by_identity_id) VALUES(?,?,?,?)", newID(10), room, viewer.AccountID, owner.IdentityID); err != nil {
		t.Fatal(err)
	}
	s, err := a.joinAndSnapshot(t.Context(), room, viewer)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{identity: viewer.IdentityID, send: make(chan any, 32), done: make(chan struct{})}
	if err = a.hub.tryAdd(room, c); err != nil {
		t.Fatal(err)
	}
	r := authed("DELETE", "/api/rooms/"+room+"/invites/invite_viewer", nil, cookie, owner.CSRF)
	r.SetPathValue("roomId", room)
	r.SetPathValue("username", "invite_viewer")
	w := httptest.NewRecorder()
	a.requireAuth(a.revokeInvite)(w, r)
	if w.Code != 204 {
		t.Fatalf("revoke: %d %s", w.Code, w.Body.String())
	}
	select {
	case <-c.done:
	default:
		t.Error("revoked invite kept existing socket connected")
	}
	_, err = a.applyCommand(t.Context(), room, viewer, command{Type: "player.play", ExpectedRevision: s.Revision, ExpectedPlaybackRevision: &s.Playback.Revision, Payload: json.RawMessage(`{"position":0}`)})
	if !errors.Is(err, errDenied) {
		t.Errorf("revoked invite command: %v", err)
	}
}

func TestProductionRejectsE2EMode(t *testing.T) {
	t.Setenv("KOALAPARTY_PRODUCTION", "true")
	t.Setenv("KOALAPARTY_TRUSTED_ORIGINS", "https://party.example.com")
	t.Setenv("KOALAPARTY_E2E", "true")
	if _, err := loadConfig(); err == nil {
		t.Fatal("production accepted unauthenticated E2E shutdown and relaxed limits")
	}
}

func TestAnonymousOwnerCannotLockThemselvesOut(t *testing.T) {
	for _, visibility := range []string{"private", "friends_only"} {
		t.Run(visibility, func(t *testing.T) {
			a := testApp(t)
			cookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174088", strings.Repeat("v", 43))
			room := createTestRoom(t, a, cookie, owner)
			s, err := a.snapshot(t.Context(), room, owner.IdentityID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = a.applyCommand(t.Context(), room, owner, command{Type: "room.visibility", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"visibility":"` + visibility + `"}`)})
			if err == nil {
				t.Error("anonymous owner changed room to an account-only visibility")
			}
			if _, err := a.joinAndSnapshot(t.Context(), room, owner); err != nil {
				t.Errorf("owner lost access after rejected visibility change: %v", err)
			}
		})
	}
}

func TestRequestLogsExcludeDynamicIdentifiers(t *testing.T) {
	var logs bytes.Buffer
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/account/sessions/{sessionId}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("POST /api/friends/{username}/{action}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	handler := requestLogging(slog.New(slog.NewJSONHandler(&logs, nil)), nil, mux)
	for _, item := range []struct{ method, path string }{
		{"DELETE", "/api/account/sessions/sensitive-session-hash"},
		{"POST", "/api/friends/private-username/block"},
		{"GET", "/unknown/private-identifier"},
	} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(item.method, item.path, nil))
	}
	for _, secret := range []string{"sensitive-session-hash", "private-username", "private-identifier"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("request log disclosed %s", secret)
		}
	}
}

func TestMetadataPanicLogsExcludeRawIdentifiers(t *testing.T) {
	for _, worker := range []string{"title", "segments"} {
		t.Run(worker, func(t *testing.T) {
			var logs bytes.Buffer
			a := &application{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			const room = "private-room-identifier"
			a.fetchTitle = func(context.Context, string) string { panic("private-panic-payload") }
			a.segments = newSegmentCache(func(context.Context, string) []sponsorSegment { panic("private-panic-payload") })
			if worker == "title" {
				a.enrichTitle(room, "media", "video")
			} else {
				a.enrichSegments(room, "video")
			}
			var entry map[string]any
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry["room_hash"] != shortHash(room) || strings.Contains(logs.String(), room) || strings.Contains(logs.String(), "private-panic-payload") {
				t.Fatalf("unsafe panic log: %s", logs.String())
			}
		})
	}
}

func TestFriendRemovalRevokesAdminRoomAccess(t *testing.T) {
	for _, action := range []string{"remove", "block"} {
		t.Run(action, func(t *testing.T) {
			a := testApp(t)
			cookie, owner, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174085", "friend_owner")
			_, viewer, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174086", "friend_viewer")
			room := createTestRoom(t, a, cookie, owner)
			if _, err := a.joinAndSnapshot(t.Context(), room, viewer); err != nil {
				t.Fatal(err)
			}
			if _, err := a.db.Exec("UPDATE room_members SET role='admin' WHERE room_id=? AND identity_id=?", room, viewer.IdentityID); err != nil {
				t.Fatal(err)
			}
			if _, err := a.db.Exec("INSERT INTO friendships(requester_account_id,addressee_account_id,status) VALUES(?,?,'accepted')", owner.AccountID, viewer.AccountID); err != nil {
				t.Fatal(err)
			}
			if _, err := a.db.Exec("UPDATE rooms SET visibility='friends_only' WHERE id=?", room); err != nil {
				t.Fatal(err)
			}
			c := &client{identity: viewer.IdentityID, send: make(chan any, 32), done: make(chan struct{})}
			if err := a.hub.tryAdd(room, c); err != nil {
				t.Fatal(err)
			}
			r := authed("POST", "/api/friends/friend_viewer/"+action, nil, cookie, owner.CSRF)
			r.SetPathValue("username", "friend_viewer")
			r.SetPathValue("action", action)
			w := httptest.NewRecorder()
			a.requireAuth(a.friendAction)(w, r)
			if w.Code != 204 {
				t.Fatalf("friend action: %d %s", w.Code, w.Body.String())
			}
			select {
			case <-c.done:
			default:
				t.Error("former friend retained room socket")
			}
			if _, allowed := a.roleAndAllowed(room, viewer.IdentityID, "room.manage_invites"); allowed {
				t.Error("admin bypassed removed friendship")
			}
			w = httptest.NewRecorder()
			a.myRooms(w, httptest.NewRequest("GET", "/api/rooms", nil), viewer)
			if strings.Contains(w.Body.String(), room) {
				t.Error("room library leaked inaccessible room metadata")
			}
			w = httptest.NewRecorder()
			a.roomPreviews(w, httptest.NewRequest("POST", "/api/rooms/previews", strings.NewReader(`{"ids":["`+room+`"]}`)), viewer)
			if w.Body.String() != "[]\n" {
				t.Errorf("room preview leaked inaccessible room: %s", w.Body.String())
			}
		})
	}
}

func TestSnapshotKeepsConcurrentRevisionAndPlaybackConsistent(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "123e4567-e89b-42d3-a456-426614174087", strings.Repeat("s", 43))
	room := createTestRoom(t, a, cookie, owner)
	done := make(chan error, 1)
	go func() {
		for i := 1; i <= 100; i++ {
			tx, err := a.db.BeginTx(t.Context(), nil)
			if err != nil {
				done <- err
				return
			}
			if _, err = tx.Exec("UPDATE rooms SET revision=? WHERE id=?", i, room); err == nil {
				_, err = tx.Exec("UPDATE playback_states SET revision=? WHERE room_id=?", i, room)
			}
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 100; i++ {
		s, err := a.snapshot(t.Context(), room, owner.IdentityID)
		if err != nil {
			t.Error(err)
			break
		}
		if s.Revision != s.Playback.Revision {
			t.Errorf("mixed snapshot revisions: room=%d playback=%d", s.Revision, s.Playback.Revision)
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
